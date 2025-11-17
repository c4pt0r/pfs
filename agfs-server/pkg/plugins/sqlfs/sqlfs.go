package sqlfs

import (
	"database/sql"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/config"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
)

const (
	PluginName    = "sqlfs"
	MaxFileSize   = 5 * 1024 * 1024 // 5MB maximum file size
	MaxFileSizeMB = 5
)

// SQLFSPlugin provides a database-backed file system
type SQLFSPlugin struct {
	fs      *SQLFS
	backend DBBackend
	config  map[string]interface{}
}

// NewSQLFSPlugin creates a new SQLFS plugin
func NewSQLFSPlugin() *SQLFSPlugin {
	return &SQLFSPlugin{}
}

func (p *SQLFSPlugin) Name() string {
	return PluginName
}

func (p *SQLFSPlugin) Validate(cfg map[string]interface{}) error {
	// Check for unknown parameters
	allowedKeys := []string{"backend", "db_path", "dsn", "user", "password", "host", "port", "database",
		"cache_enabled", "cache_max_size", "cache_ttl_seconds", "mount_path"}
	if err := config.ValidateOnlyKnownKeys(cfg, allowedKeys); err != nil {
		return err
	}

	// Validate backend type
	backendType := config.GetStringConfig(cfg, "backend", "sqlite")
	validBackends := map[string]bool{
		"sqlite":  true,
		"sqlite3": true,
		"tidb":    true,
		"mysql":   true,
	}
	if !validBackends[backendType] {
		return fmt.Errorf("unsupported database backend: %s (valid options: sqlite, sqlite3, tidb, mysql)", backendType)
	}

	// Validate optional string parameters
	for _, key := range []string{"db_path", "dsn", "user", "password", "host", "database"} {
		if err := config.ValidateStringType(cfg, key); err != nil {
			return err
		}
	}

	// Validate optional integer parameters
	for _, key := range []string{"port", "cache_max_size", "cache_ttl_seconds"} {
		if err := config.ValidateIntType(cfg, key); err != nil {
			return err
		}
	}

	// Validate cache_enabled (optional boolean)
	if err := config.ValidateBoolType(cfg, "cache_enabled"); err != nil {
		return err
	}

	return nil
}

func (p *SQLFSPlugin) Initialize(config map[string]interface{}) error {
	p.config = config

	// Create appropriate backend
	backend, err := CreateBackend(config)
	if err != nil {
		return fmt.Errorf("failed to create backend: %w", err)
	}
	p.backend = backend

	// Create SQLFS instance with the backend
	fs, err := NewSQLFS(backend, config)
	if err != nil {
		return fmt.Errorf("failed to initialize sqlfs: %w", err)
	}
	p.fs = fs

	backendType := "sqlite"
	if bt, ok := config["backend"].(string); ok && bt != "" {
		backendType = bt
	}
	log.Infof("[sqlfs] Initialized with backend: %s", backendType)
	return nil
}

func (p *SQLFSPlugin) GetFileSystem() filesystem.FileSystem {
	return p.fs
}

func (p *SQLFSPlugin) GetReadme() string {
	return getReadme()
}

func (p *SQLFSPlugin) Shutdown() error {
	if p.fs != nil {
		return p.fs.Close()
	}
	return nil
}

// SQLFS implements FileSystem interface using a database backend
type SQLFS struct {
	db         *sql.DB
	backend    DBBackend
	mu         sync.RWMutex
	pluginName string
	listCache  *ListDirCache // cache for directory listings
}

// FileEntry represents a file or directory in the database
type FileEntry struct {
	Path    string
	IsDir   bool
	Mode    uint32
	Size    int64
	ModTime time.Time
	Data    []byte
}

// NewSQLFS creates a new database-backed file system
func NewSQLFS(backend DBBackend, config map[string]interface{}) (*SQLFS, error) {
	db, err := backend.Open(config)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply backend-specific optimizations
	for _, sql := range backend.GetOptimizationSQL() {
		if _, err := db.Exec(sql); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to apply optimization: %w", err)
		}
	}

	// Parse cache configuration
	cacheEnabled := true // enabled by default
	cacheMaxSize := 1000 // default 1000 entries
	cacheTTLSeconds := 5 // default 5 seconds

	if val, ok := config["cache_enabled"].(bool); ok {
		cacheEnabled = val
	}
	if val, ok := config["cache_max_size"].(int); ok && val > 0 {
		cacheMaxSize = val
	}
	if val, ok := config["cache_ttl_seconds"].(int); ok && val > 0 {
		cacheTTLSeconds = val
	}

	fs := &SQLFS{
		db:         db,
		backend:    backend,
		pluginName: PluginName,
		listCache:  NewListDirCache(cacheMaxSize, time.Duration(cacheTTLSeconds)*time.Second, cacheEnabled),
	}

	// Initialize database schema
	if err := fs.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Ensure root directory exists
	if err := fs.ensureRootExists(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	return fs, nil
}

// initSchema creates the database schema
func (fs *SQLFS) initSchema() error {
	for _, sql := range fs.backend.GetInitSQL() {
		if _, err := fs.db.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute init SQL: %w", err)
		}
	}
	return nil
}

// ensureRootExists ensures the root directory exists
func (fs *SQLFS) ensureRootExists() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var exists int
	err := fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = '/'").Scan(&exists)
	if err != nil {
		return err
	}

	if exists == 0 {
		_, err = fs.db.Exec(
			"INSERT INTO files (path, is_dir, mode, size, mod_time, data) VALUES (?, ?, ?, ?, ?, ?)",
			"/", 1, 0755, 0, time.Now().Unix(), nil,
		)
		return err
	}

	return nil
}

// Close closes the database connection
func (fs *SQLFS) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if fs.db != nil {
		return fs.db.Close()
	}
	return nil
}

// getParentPath returns the parent directory path
func getParentPath(path string) string {
	if path == "/" {
		return "/"
	}
	parent := filepath.Dir(path)
	if parent == "." {
		return "/"
	}
	return parent
}

func (fs *SQLFS) Create(path string) error {
	path = filesystem.NormalizePath(path)

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if parent directory exists
	parent := getParentPath(path)
	if parent != "/" {
		var isDir int
		err := fs.db.QueryRow("SELECT is_dir FROM files WHERE path = ?", parent).Scan(&isDir)
		if err == sql.ErrNoRows {
			return filesystem.NewNotFoundError("create", parent)
		} else if err != nil {
			return err
		}
		if isDir == 0 {
			return filesystem.NewNotDirectoryError(parent)
		}
	}

	// Check if file already exists
	var exists int
	err := fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", path).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return filesystem.NewAlreadyExistsError("file", path)
	}

	// Create empty file
	_, err = fs.db.Exec(
		"INSERT INTO files (path, is_dir, mode, size, mod_time, data) VALUES (?, ?, ?, ?, ?, ?)",
		path, 0, 0644, 0, time.Now().Unix(), []byte{},
	)

	// Invalidate parent directory cache
	if err == nil {
		fs.listCache.InvalidateParent(path)
	}

	return err
}

func (fs *SQLFS) Mkdir(path string, perm uint32) error {
	path = filesystem.NormalizePath(path)

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if parent directory exists
	parent := getParentPath(path)
	if parent != "/" {
		var isDir int
		err := fs.db.QueryRow("SELECT is_dir FROM files WHERE path = ?", parent).Scan(&isDir)
		if err == sql.ErrNoRows {
			return filesystem.NewNotFoundError("mkdir", parent)
		} else if err != nil {
			return err
		}
		if isDir == 0 {
			return filesystem.NewNotDirectoryError(parent)
		}
	}

	// Check if directory already exists
	var exists int
	err := fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", path).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return filesystem.NewAlreadyExistsError("directory", path)
	}

	// Create directory
	if perm == 0 {
		perm = 0755
	}
	_, err = fs.db.Exec(
		"INSERT INTO files (path, is_dir, mode, size, mod_time, data) VALUES (?, ?, ?, ?, ?, ?)",
		path, 1, perm, 0, time.Now().Unix(), nil,
	)

	// Invalidate parent directory cache
	if err == nil {
		fs.listCache.InvalidateParent(path)
	}

	return err
}

func (fs *SQLFS) Remove(path string) error {
	path = filesystem.NormalizePath(path)

	if path == "/" {
		return fmt.Errorf("cannot remove root directory")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if file exists and is not a directory
	var isDir int
	err := fs.db.QueryRow("SELECT is_dir FROM files WHERE path = ?", path).Scan(&isDir)
	if err == sql.ErrNoRows {
		return filesystem.NewNotFoundError("remove", path)
	} else if err != nil {
		return err
	}

	if isDir == 1 {
		// Check if directory is empty
		var count int
		err = fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path LIKE ? AND path != ?", path+"/%", path).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("directory not empty: %s", path)
		}
	}

	// Delete file
	_, err = fs.db.Exec("DELETE FROM files WHERE path = ?", path)

	// Invalidate parent directory cache and the path itself if it's a directory
	if err == nil {
		fs.listCache.InvalidateParent(path)
		fs.listCache.Invalidate(path)
	}

	return err
}

func (fs *SQLFS) RemoveAll(path string) error {
	path = filesystem.NormalizePath(path)

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Use batched deletion to avoid long-running transactions and locks
	const batchSize = 1000

	// If path is root, remove all children but not the root itself
	if path == "/" {
		for {
			result, err := fs.db.Exec("DELETE FROM files WHERE path != '/' LIMIT ?", batchSize)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			// If no rows were affected, we're done
			if affected == 0 {
				break
			}
			// If fewer rows than batch size were deleted, we're done
			if affected < int64(batchSize) {
				break
			}
		}
		// Invalidate entire cache
		fs.listCache.InvalidatePrefix("/")
		return nil
	}

	// Delete file and all children in batches
	for {
		result, err := fs.db.Exec("DELETE FROM files WHERE (path = ? OR path LIKE ?) LIMIT ?", path, path+"/%", batchSize)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		// If no rows were affected, we're done
		if affected == 0 {
			break
		}
		// If fewer rows than batch size were deleted, we're done
		if affected < int64(batchSize) {
			break
		}
	}

	// Invalidate cache for the path and all descendants
	fs.listCache.InvalidateParent(path)
	fs.listCache.InvalidatePrefix(path)

	return nil
}

func (fs *SQLFS) Read(path string, offset int64, size int64) ([]byte, error) {
	path = filesystem.NormalizePath(path)

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var isDir int
	var data []byte
	err := fs.db.QueryRow("SELECT is_dir, data FROM files WHERE path = ?", path).Scan(&isDir, &data)
	if err == sql.ErrNoRows {
		return nil, filesystem.NewNotFoundError("read", path)
	} else if err != nil {
		return nil, err
	}

	if isDir == 1 {
		return nil, filesystem.NewInvalidArgumentError("path", path, "is a directory")
	}

	// Apply offset and size
	dataLen := int64(len(data))
	if offset < 0 {
		offset = 0
	}
	if offset >= dataLen {
		return []byte{}, io.EOF
	}

	end := dataLen
	if size >= 0 {
		end = offset + size
		if end > dataLen {
			end = dataLen
		}
	}

	result := data[offset:end]
	if end >= dataLen {
		return result, io.EOF
	}
	return result, nil
}

func (fs *SQLFS) Write(path string, data []byte) ([]byte, error) {
	path = filesystem.NormalizePath(path)

	// Check file size limit
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum limit of %dMB (got %d bytes)", MaxFileSizeMB, len(data))
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if file exists
	var exists int
	var isDir int
	err := fs.db.QueryRow("SELECT COUNT(*), COALESCE(MAX(is_dir), 0) FROM files WHERE path = ?", path).Scan(&exists, &isDir)
	if err != nil {
		return nil, err
	}

	if exists > 0 && isDir == 1 {
		return nil, filesystem.NewInvalidArgumentError("path", path, "is a directory")
	}

	if exists == 0 {
		// File doesn't exist, create it
		parent := getParentPath(path)
		if parent != "/" {
			var parentIsDir int
			err := fs.db.QueryRow("SELECT is_dir FROM files WHERE path = ?", parent).Scan(&parentIsDir)
			if err == sql.ErrNoRows {
				return nil, filesystem.NewNotFoundError("write", parent)
			} else if err != nil {
				return nil, err
			}
			if parentIsDir == 0 {
				return nil, filesystem.NewNotDirectoryError(parent)
			}
		}

		_, err = fs.db.Exec(
			"INSERT INTO files (path, is_dir, mode, size, mod_time, data) VALUES (?, ?, ?, ?, ?, ?)",
			path, 0, 0644, len(data), time.Now().Unix(), data,
		)

		// Invalidate parent directory cache on new file creation
		if err == nil {
			fs.listCache.InvalidateParent(path)
		}
	} else {
		// Update existing file
		_, err = fs.db.Exec(
			"UPDATE files SET data = ?, size = ?, mod_time = ? WHERE path = ?",
			data, len(data), time.Now().Unix(), path,
		)
		// Note: no need to invalidate parent cache on update, only on create/delete
	}

	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("Written %d bytes to %s", len(data), path)), nil
}

func (fs *SQLFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	path = filesystem.NormalizePath(path)

	// Try to get from cache first
	if files, found := fs.listCache.Get(path); found {
		return files, nil
	}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Check if directory exists
	var isDir int
	err := fs.db.QueryRow("SELECT is_dir FROM files WHERE path = ?", path).Scan(&isDir)
	if err == sql.ErrNoRows {
		return nil, filesystem.NewNotFoundError("readdir", path)
	} else if err != nil {
		return nil, err
	}

	if isDir == 0 {
		return nil, filesystem.NewNotDirectoryError(path)
	}

	// Query children
	pattern := path
	if path != "/" {
		pattern = path + "/"
	}

	rows, err := fs.db.Query(
		"SELECT path, is_dir, mode, size, mod_time FROM files WHERE path LIKE ? AND path != ? AND path NOT LIKE ?",
		pattern+"%", path, pattern+"%/%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []filesystem.FileInfo
	for rows.Next() {
		var filePath string
		var isDir int
		var mode uint32
		var size int64
		var modTime int64

		if err := rows.Scan(&filePath, &isDir, &mode, &size, &modTime); err != nil {
			return nil, err
		}

		name := filepath.Base(filePath)
		files = append(files, filesystem.FileInfo{
			Name:    name,
			Size:    size,
			Mode:    mode,
			ModTime: time.Unix(modTime, 0),
			IsDir:   isDir == 1,
			Meta: filesystem.MetaData{
				Name: PluginName,
			},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Cache the result
	fs.listCache.Put(path, files)

	return files, nil
}

func (fs *SQLFS) Stat(path string) (*filesystem.FileInfo, error) {
	path = filesystem.NormalizePath(path)

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var isDir int
	var mode uint32
	var size int64
	var modTime int64

	err := fs.db.QueryRow(
		"SELECT is_dir, mode, size, mod_time FROM files WHERE path = ?",
		path,
	).Scan(&isDir, &mode, &size, &modTime)

	if err == sql.ErrNoRows {
		return nil, filesystem.NewNotFoundError("stat", path)
	} else if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	if path == "/" {
		name = "/"
	}

	return &filesystem.FileInfo{
		Name:    name,
		Size:    size,
		Mode:    mode,
		ModTime: time.Unix(modTime, 0),
		IsDir:   isDir == 1,
		Meta: filesystem.MetaData{
			Name: PluginName,
			Type: fs.backend.GetDriverName(),
		},
	}, nil
}

func (fs *SQLFS) Rename(oldPath, newPath string) error {
	oldPath = filesystem.NormalizePath(oldPath)
	newPath = filesystem.NormalizePath(newPath)

	if oldPath == "/" || newPath == "/" {
		return fmt.Errorf("cannot rename root directory")
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if old path exists
	var exists int
	err := fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", oldPath).Scan(&exists)
	if err != nil {
		return err
	}
	if exists == 0 {
		return filesystem.NewNotFoundError("rename", oldPath)
	}

	// Check if new path already exists
	err = fs.db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", newPath).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return filesystem.NewAlreadyExistsError("file", newPath)
	}

	// Rename file/directory
	_, err = fs.db.Exec("UPDATE files SET path = ? WHERE path = ?", newPath, oldPath)
	if err != nil {
		return err
	}

	// If it's a directory, rename all children
	_, err = fs.db.Exec(
		"UPDATE files SET path = ? || SUBSTR(path, ?) WHERE path LIKE ?",
		newPath, len(oldPath)+1, oldPath+"/%",
	)

	// Invalidate cache for old and new parent directories
	if err == nil {
		fs.listCache.InvalidateParent(oldPath)
		fs.listCache.InvalidateParent(newPath)
		fs.listCache.Invalidate(oldPath)
		fs.listCache.InvalidatePrefix(oldPath)
	}

	return err
}

func (fs *SQLFS) Chmod(path string, mode uint32) error {
	path = filesystem.NormalizePath(path)

	fs.mu.Lock()
	defer fs.mu.Unlock()

	result, err := fs.db.Exec("UPDATE files SET mode = ? WHERE path = ?", mode, path)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return filesystem.NewNotFoundError("chmod", path)
	}

	return nil
}

func (fs *SQLFS) Open(path string) (io.ReadCloser, error) {
	data, err := fs.Read(path, 0, -1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (fs *SQLFS) OpenWrite(path string) (io.WriteCloser, error) {
	return filesystem.NewBufferedWriter(path, fs.Write), nil
}

func getReadme() string {
	return `SQLFS Plugin - Database-backed File System

This plugin provides a persistent file system backed by database storage.

FEATURES:
  - Persistent storage (survives server restarts)
  - Full POSIX-like file system operations
  - Multiple database backends (SQLite, TiDB)
  - Efficient database-backed storage
  - ACID transactions
  - Supports files and directories
  - Maximum file size: 5MB per file

CONFIGURATION:

  SQLite Backend (Local Testing):
  [plugins.sqlfs]
  enabled = true
  path = "/sqlfs"

    [plugins.sqlfs.config]
    backend = "sqlite"  # or "sqlite3"
    db_path = "sqlfs.db"

    # Optional cache settings (enabled by default)
    cache_enabled = true        # Enable/disable directory listing cache
    cache_max_size = 1000       # Maximum number of cached entries (default: 1000)
    cache_ttl_seconds = 5       # Cache entry TTL in seconds (default: 5)

  TiDB Backend (Production):
  [plugins.sqlfs]
  enabled = true
  path = "/sqlfs"

    [plugins.sqlfs.config]
    backend = "tidb"

    # For TiDB Cloud (TLS required):
    user = "3YdGXuXNdAEmP1f.root"
    password = "your_password"
    host = "gateway01.us-west-2.prod.aws.tidbcloud.com"
    port = "4000"
    database = "baas"
    enable_tls = true
    tls_server_name = "gateway01.us-west-2.prod.aws.tidbcloud.com"

    # Or use DSN with TLS:
    # dsn = "user:password@tcp(host:4000)/database?charset=utf8mb4&parseTime=True&tls=tidb"

USAGE:

  Create a directory:
    agfs mkdir /sqlfs/mydir

  Create a file:
    agfs write /sqlfs/mydir/file.txt "Hello, World!"

  Read a file:
    agfs cat /sqlfs/mydir/file.txt

  List directory:
    agfs ls /sqlfs/mydir

  Get file info:
    agfs stat /sqlfs/mydir/file.txt

  Rename file:
    agfs mv /sqlfs/mydir/file.txt /sqlfs/mydir/newfile.txt

  Change permissions:
    agfs chmod 755 /sqlfs/mydir/file.txt

  Remove file:
    agfs rm /sqlfs/mydir/file.txt

  Remove directory (must be empty):
    agfs rm /sqlfs/mydir

  Remove directory recursively:
    agfs rm -r /sqlfs/mydir

EXAMPLES:

  # Create directory structure
  agfs:/> mkdir /sqlfs/data
  agfs:/> mkdir /sqlfs/data/logs

  # Write files
  agfs:/> echo "Configuration data" > /sqlfs/data/config.txt
  agfs:/> echo "Log entry" > /sqlfs/data/logs/app.log

  # Read files
  agfs:/> cat /sqlfs/data/config.txt
  Configuration data

  # List directory
  agfs:/> ls /sqlfs/data
  config.txt
  logs/

ADVANTAGES:
  - Data persists across server restarts
  - Efficient storage with database compression
  - Transaction safety (ACID properties)
  - Query capabilities (can be extended)
  - Backup friendly (single database file)
  - Fast directory listing with LRU cache (improves shell completion)

USE CASES:
  - Persistent configuration storage
  - Log file storage
  - Document management
  - Application data storage
  - Backup and archival
  - Development and testing with persistent data

TECHNICAL DETAILS:
  - Database: SQLite 3 / TiDB (MySQL-compatible)
  - Journal mode: WAL (Write-Ahead Logging) for SQLite
  - Schema: Single table with path, metadata, and blob data
  - Concurrent reads supported
  - Write serialization via mutex
  - Path normalization and validation
  - LRU cache for directory listings (configurable TTL and size)
  - Automatic cache invalidation on modifications

LIMITATIONS:
  - Maximum file size: 5MB per file
  - Not suitable for large files (use MemFS or StreamFS for larger data)
  - Write operations are serialized
  - No file locking mechanism
  - No sparse file support
  - No streaming support (use StreamFS for real-time streaming)
`
}

// Ensure SQLFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*SQLFSPlugin)(nil)
var _ filesystem.FileSystem = (*SQLFS)(nil)
