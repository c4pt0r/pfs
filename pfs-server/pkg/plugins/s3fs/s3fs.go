package s3fs

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin/config"
	log "github.com/sirupsen/logrus"
)

const (
	PluginName = "s3fs"
)

// S3FS implements FileSystem interface using AWS S3 as backend
type S3FS struct {
	client     *S3Client
	mu         sync.RWMutex
	pluginName string
}

// NewS3FS creates a new S3-backed file system
func NewS3FS(cfg S3Config) (*S3FS, error) {
	client, err := NewS3Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	return &S3FS{
		client:     client,
		pluginName: PluginName,
	}, nil
}

// normalizePath normalizes a path
func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}

func (fs *S3FS) Create(path string) error {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if file already exists
	exists, err := fs.client.ObjectExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}
	if exists {
		return fmt.Errorf("file already exists: %s", path)
	}

	// Check if parent directory exists
	parent := getParentPath(path)
	if parent != "" {
		dirExists, err := fs.client.DirectoryExists(ctx, parent)
		if err != nil {
			return fmt.Errorf("failed to check parent directory: %w", err)
		}
		if !dirExists {
			return fmt.Errorf("parent directory does not exist: %s", parent)
		}
	}

	// Create empty file
	err = fs.client.PutObject(ctx, path, []byte{})
	if err != nil {
		return err
	}

	return nil
}

func (fs *S3FS) Mkdir(path string, perm uint32) error {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if directory already exists
	exists, err := fs.client.DirectoryExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if directory exists: %w", err)
	}
	if exists {
		return fmt.Errorf("directory already exists: %s", path)
	}

	// Check if parent directory exists
	parent := getParentPath(path)
	if parent != "" {
		dirExists, err := fs.client.DirectoryExists(ctx, parent)
		if err != nil {
			return fmt.Errorf("failed to check parent directory: %w", err)
		}
		if !dirExists {
			return fmt.Errorf("parent directory does not exist: %s", parent)
		}
	}

	// Create directory marker
	return fs.client.CreateDirectory(ctx, path)
}

func (fs *S3FS) Remove(path string) error {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if it's a file
	exists, err := fs.client.ObjectExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}

	if exists {
		// It's a file, delete it
		return fs.client.DeleteObject(ctx, path)
	}

	// Check if it's a directory
	dirExists, err := fs.client.DirectoryExists(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check if directory exists: %w", err)
	}

	if !dirExists {
		return fmt.Errorf("no such file or directory: %s", path)
	}

	// Check if directory is empty
	objects, err := fs.client.ListObjects(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to list directory: %w", err)
	}

	if len(objects) > 0 {
		return fmt.Errorf("directory not empty: %s", path)
	}

	// Delete directory marker
	return fs.client.DeleteObject(ctx, path+"/")
}

func (fs *S3FS) RemoveAll(path string) error {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.client.DeleteDirectory(ctx, path)
}

func (fs *S3FS) Read(path string, offset int64, size int64) ([]byte, error) {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Get the entire object (S3 doesn't support efficient range reads in this simple implementation)
	data, err := fs.client.GetObject(ctx, path)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NotFound") {
			return nil, fmt.Errorf("no such file: %s", path)
		}
		return nil, err
	}

	// Apply range read using common helper
	return plugin.ApplyRangeRead(data, offset, size)
}

func (fs *S3FS) Write(path string, data []byte) ([]byte, error) {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if it's a directory
	dirExists, _ := fs.client.DirectoryExists(ctx, path)
	if dirExists {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	// Check if parent directory exists
	parent := getParentPath(path)
	if parent != "" {
		parentExists, err := fs.client.DirectoryExists(ctx, parent)
		if err != nil {
			return nil, fmt.Errorf("failed to check parent directory: %w", err)
		}
		if !parentExists {
			return nil, fmt.Errorf("parent directory does not exist: %s", parent)
		}
	}

	// Write to S3
	err := fs.client.PutObject(ctx, path, data)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("Written %d bytes to %s", len(data), path)), nil
}

func (fs *S3FS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Check if directory exists
	if path != "" {
		exists, err := fs.client.DirectoryExists(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("failed to check directory: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("no such directory: %s", path)
		}
	}

	// List objects
	objects, err := fs.client.ListObjects(ctx, path)
	if err != nil {
		return nil, err
	}

	var files []filesystem.FileInfo
	for _, obj := range objects {
		files = append(files, filesystem.FileInfo{
			Name:    obj.Key,
			Size:    obj.Size,
			Mode:    0644,
			ModTime: obj.LastModified,
			IsDir:   obj.IsDir,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "s3",
			},
		})
	}

	return files, nil
}

func (fs *S3FS) Stat(path string) (*filesystem.FileInfo, error) {
	path = normalizePath(path)
	ctx := context.Background()

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Special case for root
	if path == "" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "s3",
				Content: map[string]string{
					"region": fs.client.region,
					"bucket": fs.client.bucket,
					"prefix": fs.client.prefix,
				},
			},
		}, nil
	}

	// Try as file first
	head, err := fs.client.HeadObject(ctx, path)
	if err == nil {
		return &filesystem.FileInfo{
			Name:    filepath.Base(path),
			Size:    aws.ToInt64(head.ContentLength),
			Mode:    0644,
			ModTime: aws.ToTime(head.LastModified),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "s3",
				Content: map[string]string{
					"region": fs.client.region,
					"bucket": fs.client.bucket,
					"prefix": fs.client.prefix,
				},
			},
		}, nil
	}

	// Try as directory
	dirExists, err := fs.client.DirectoryExists(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to check directory: %w", err)
	}

	if dirExists {
		return &filesystem.FileInfo{
			Name:    filepath.Base(path),
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "s3",
				Content: map[string]string{
					"region": fs.client.region,
					"bucket": fs.client.bucket,
					"prefix": fs.client.prefix,
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("no such file or directory: %s", path)
}

func (fs *S3FS) Rename(oldPath, newPath string) error {
	oldPath = normalizePath(oldPath)
	newPath = normalizePath(newPath)
	ctx := context.Background()

	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Check if old path exists
	exists, err := fs.client.ObjectExists(ctx, oldPath)
	if err != nil {
		return fmt.Errorf("failed to check source: %w", err)
	}
	if !exists {
		return fmt.Errorf("no such file or directory: %s", oldPath)
	}

	// Get the object
	data, err := fs.client.GetObject(ctx, oldPath)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}

	// Put to new location
	err = fs.client.PutObject(ctx, newPath, data)
	if err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}

	// Delete old object
	err = fs.client.DeleteObject(ctx, oldPath)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	return nil
}

func (fs *S3FS) Chmod(path string, mode uint32) error {
	// S3 doesn't support Unix permissions
	// This is a no-op for compatibility
	return nil
}

func (fs *S3FS) Open(path string) (io.ReadCloser, error) {
	data, err := fs.Read(path, 0, -1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (fs *S3FS) OpenWrite(path string) (io.WriteCloser, error) {
	return &s3fsWriter{fs: fs, path: path}, nil
}

type s3fsWriter struct {
	fs   *S3FS
	path string
	buf  []byte
}

func (w *s3fsWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *s3fsWriter) Close() error {
	_, err := w.fs.Write(w.path, w.buf)
	return err
}

// S3FSPlugin wraps S3FS as a plugin
type S3FSPlugin struct {
	fs     *S3FS
	config map[string]interface{}
}

// NewS3FSPlugin creates a new S3FS plugin
func NewS3FSPlugin() *S3FSPlugin {
	return &S3FSPlugin{}
}

func (p *S3FSPlugin) Name() string {
	return PluginName
}

func (p *S3FSPlugin) Validate(cfg map[string]interface{}) error {
	// Check for unknown parameters
	allowedKeys := []string{"bucket", "region", "access_key_id", "secret_access_key", "endpoint", "prefix", "disable_ssl", "mount_path"}
	if err := config.ValidateOnlyKnownKeys(cfg, allowedKeys); err != nil {
		return err
	}

	// Validate bucket (required)
	if _, err := config.RequireString(cfg, "bucket"); err != nil {
		return err
	}

	// Validate optional string parameters
	for _, key := range []string{"region", "access_key_id", "secret_access_key", "endpoint", "prefix"} {
		if err := config.ValidateStringType(cfg, key); err != nil {
			return err
		}
	}

	// Validate disable_ssl (optional boolean)
	if err := config.ValidateBoolType(cfg, "disable_ssl"); err != nil {
		return err
	}

	return nil
}

func (p *S3FSPlugin) Initialize(config map[string]interface{}) error {
	p.config = config

	// Parse configuration
	cfg := S3Config{
		Region: getStringConfig(config, "region", "us-east-1"),
		Bucket: getStringConfig(config, "bucket", ""),
		AccessKeyID: getStringConfig(config, "access_key_id", ""),
		SecretAccessKey: getStringConfig(config, "secret_access_key", ""),
		Endpoint: getStringConfig(config, "endpoint", ""),
		Prefix: getStringConfig(config, "prefix", ""),
		DisableSSL: getBoolConfig(config, "disable_ssl", false),
	}

	if cfg.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	// Create S3FS instance
	fs, err := NewS3FS(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize s3fs: %w", err)
	}
	p.fs = fs

	log.Infof("[s3fs] Initialized with bucket: %s, region: %s", cfg.Bucket, cfg.Region)
	return nil
}

func (p *S3FSPlugin) GetFileSystem() filesystem.FileSystem {
	return p.fs
}

func (p *S3FSPlugin) GetReadme() string {
	return getReadme()
}

func (p *S3FSPlugin) Shutdown() error {
	return nil
}

func getReadme() string {
	return `S3FS Plugin - AWS S3-backed File System

This plugin provides a file system backed by AWS S3 object storage.

FEATURES:
  - Store files and directories in AWS S3
  - Support for S3-compatible services (MinIO, LocalStack, etc.)
  - Full POSIX-like file system operations
  - Automatic directory handling
  - Optional key prefix for namespace isolation

CONFIGURATION:

  AWS S3:
  [plugins.s3fs]
  enabled = true
  path = "/s3fs"

    [plugins.s3fs.config]
    region = "us-east-1"
    bucket = "my-bucket"
    access_key_id = "AKIAIOSFODNN7EXAMPLE"
    secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    prefix = "pfs/"  # Optional: all keys will be prefixed with this

  S3-Compatible Service (MinIO, LocalStack):
  [plugins.s3fs]
  enabled = true
  path = "/s3fs"

    [plugins.s3fs.config]
    region = "us-east-1"
    bucket = "my-bucket"
    access_key_id = "minioadmin"
    secret_access_key = "minioadmin"
    endpoint = "http://localhost:9000"
    disable_ssl = true

  Multiple S3 Buckets:
  [plugins.s3fs_prod]
  enabled = true
  path = "/s3/prod"

    [plugins.s3fs_prod.config]
    region = "us-east-1"
    bucket = "production-bucket"
    access_key_id = "..."
    secret_access_key = "..."

  [plugins.s3fs_dev]
  enabled = true
  path = "/s3/dev"

    [plugins.s3fs_dev.config]
    region = "us-west-2"
    bucket = "development-bucket"
    access_key_id = "..."
    secret_access_key = "..."

USAGE:

  Create a directory:
    pfs mkdir /s3fs/data

  Create a file:
    pfs write /s3fs/data/file.txt "Hello, S3!"

  Read a file:
    pfs cat /s3fs/data/file.txt

  List directory:
    pfs ls /s3fs/data

  Remove file:
    pfs rm /s3fs/data/file.txt

  Remove directory (must be empty):
    pfs rm /s3fs/data

  Remove directory recursively:
    pfs rm -r /s3fs/data

EXAMPLES:

  # Basic file operations
  pfs:/> mkdir /s3fs/documents
  pfs:/> echo "Important data" > /s3fs/documents/report.txt
  pfs:/> cat /s3fs/documents/report.txt
  Important data

  # List contents
  pfs:/> ls /s3fs/documents
  report.txt

  # Move/rename
  pfs:/> mv /s3fs/documents/report.txt /s3fs/documents/report-2024.txt

NOTES:
  - S3 doesn't have real directories; they are simulated with "/" in object keys
  - Large files may take time to upload/download
  - Permissions (chmod) are not supported by S3
  - Atomic operations are limited by S3's eventual consistency model

USE CASES:
  - Cloud-native file storage
  - Backup and archival
  - Sharing files across distributed systems
  - Cost-effective long-term storage
  - Integration with AWS services

ADVANTAGES:
  - Unlimited storage capacity
  - High durability (99.999999999%)
  - Geographic redundancy
  - Pay-per-use pricing
  - Versioning and lifecycle policies (via S3 bucket settings)
`
}

// Helper functions
func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

func getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

// Ensure S3FSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*S3FSPlugin)(nil)
var _ filesystem.FileSystem = (*S3FS)(nil)
