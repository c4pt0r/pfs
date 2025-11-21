package sqlfs2

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/config"
	log "github.com/sirupsen/logrus"
)

const (
	PluginName = "sqlfs2"
)

// SQLFS2Plugin provides a SQL interface through file system operations
// Directory structure: /sqlfs2/<dbName>/<tableName>/{schema, execute, query}
type SQLFS2Plugin struct {
	db      *sql.DB
	backend Backend
	config  map[string]interface{}
}

// NewSQLFS2Plugin creates a new SQLFS2 plugin
func NewSQLFS2Plugin() *SQLFS2Plugin {
	return &SQLFS2Plugin{}
}

func (p *SQLFS2Plugin) Name() string {
	return PluginName
}

func (p *SQLFS2Plugin) Validate(cfg map[string]interface{}) error {
	allowedKeys := []string{"backend", "db_path", "dsn", "user", "password", "host", "port", "database",
		"enable_tls", "tls_server_name", "tls_skip_verify", "mount_path"}
	if err := config.ValidateOnlyKnownKeys(cfg, allowedKeys); err != nil {
		return err
	}

	// Validate backend type
	backendType := config.GetStringConfig(cfg, "backend", "sqlite")
	validBackends := map[string]bool{
		"sqlite":  true,
		"sqlite3": true,
		"mysql":   true,
		"tidb":    true,
	}
	if !validBackends[backendType] {
		return fmt.Errorf("unsupported database backend: %s (valid options: sqlite, sqlite3, mysql, tidb)", backendType)
	}

	// Validate optional string parameters
	for _, key := range []string{"db_path", "dsn", "user", "password", "host", "database", "tls_server_name"} {
		if err := config.ValidateStringType(cfg, key); err != nil {
			return err
		}
	}

	// Validate optional integer parameters
	for _, key := range []string{"port"} {
		if err := config.ValidateIntType(cfg, key); err != nil {
			return err
		}
	}

	// Validate optional boolean parameters
	for _, key := range []string{"enable_tls", "tls_skip_verify"} {
		if err := config.ValidateBoolType(cfg, key); err != nil {
			return err
		}
	}

	return nil
}

func (p *SQLFS2Plugin) Initialize(cfg map[string]interface{}) error {
	p.config = cfg

	backendType := config.GetStringConfig(cfg, "backend", "sqlite")

	// Create backend instance
	backend := newBackend(backendType)
	if backend == nil {
		return fmt.Errorf("unsupported backend: %s", backendType)
	}
	p.backend = backend

	// Initialize database connection using the backend
	db, err := backend.Initialize(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize %s backend: %w", backendType, err)
	}
	p.db = db

	log.Infof("[sqlfs2] Initialized with backend: %s", backendType)
	return nil
}

func (p *SQLFS2Plugin) GetFileSystem() filesystem.FileSystem {
	return &sqlfs2FS{plugin: p}
}

func (p *SQLFS2Plugin) GetReadme() string {
	return getReadme()
}

func (p *SQLFS2Plugin) Shutdown() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// sqlfs2FS implements the FileSystem interface for SQL operations
type sqlfs2FS struct {
	plugin *SQLFS2Plugin
}

// parsePath parses a path like /dbName/tableName/operation into components
func (fs *sqlfs2FS) parsePath(path string) (dbName, tableName, operation string, err error) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || path == "" {
		// Root directory
		return "", "", "", nil
	}

	if len(parts) == 1 {
		// Database level: /dbName
		return parts[0], "", "", nil
	}

	if len(parts) == 2 {
		// Table level: /dbName/tableName
		return parts[0], parts[1], "", nil
	}

	if len(parts) == 3 {
		// Operation level: /dbName/tableName/operation
		return parts[0], parts[1], parts[2], nil
	}

	return "", "", "", fmt.Errorf("invalid path: %s", path)
}

// tableExists checks if a table exists in the specified database
func (fs *sqlfs2FS) tableExists(dbName, tableName string) (bool, error) {
	if dbName == "" || tableName == "" {
		return false, fmt.Errorf("dbName and tableName must not be empty")
	}

	tables, err := fs.plugin.backend.ListTables(fs.plugin.db, dbName)
	if err != nil {
		return false, err
	}

	for _, t := range tables {
		if t == tableName {
			return true, nil
		}
	}

	return false, nil
}

func (fs *sqlfs2FS) Read(path string, offset int64, size int64) ([]byte, error) {
	dbName, tableName, operation, err := fs.parsePath(path)
	if err != nil {
		return nil, err
	}

	// Only support reading schema file
	if operation == "schema" {
		if dbName == "" || tableName == "" {
			return nil, fmt.Errorf("invalid path for schema: %s", path)
		}

		// Get table schema using backend
		createTableStmt, err := fs.plugin.backend.GetTableSchema(fs.plugin.db, dbName, tableName)
		if err != nil {
			return nil, err
		}

		data := []byte(createTableStmt + "\n")
		return plugin.ApplyRangeRead(data, offset, size)
	}

	// Support reading count file
	if operation == "count" {
		if dbName == "" || tableName == "" {
			return nil, fmt.Errorf("invalid path for count: %s", path)
		}

		// Switch to database if needed
		if err := fs.plugin.backend.SwitchDatabase(fs.plugin.db, dbName); err != nil {
			return nil, err
		}

		// Execute count query
		sqlStmt := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", dbName, tableName)
		var count int64
		err := fs.plugin.db.QueryRow(sqlStmt).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("count query error: %w", err)
		}

		data := []byte(fmt.Sprintf("%d\n", count))
		return plugin.ApplyRangeRead(data, offset, size)
	}

	if operation == "query" || operation == "execute" || operation == "insert_json" {
		return nil, fmt.Errorf("%s is write-only", operation)
	}

	// Directory read - return error indicating this is a directory
	return nil, filesystem.NewInvalidArgumentError("path", path, "is a directory")
}

func (fs *sqlfs2FS) Write(path string, data []byte) ([]byte, error) {
	dbName, tableName, operation, err := fs.parsePath(path)
	if err != nil {
		return nil, err
	}

	if operation == "" {
		return nil, fmt.Errorf("cannot write to directory: %s", path)
	}

	if operation == "schema" || operation == "count" {
		return nil, fmt.Errorf("%s is read-only", operation)
	}

	// Switch to database if needed
	if err := fs.plugin.backend.SwitchDatabase(fs.plugin.db, dbName); err != nil {
		return nil, err
	}

	// Handle insert_json operation
	if operation == "insert_json" {
		if dbName == "" || tableName == "" {
			return nil, fmt.Errorf("invalid path for insert_json: %s", path)
		}

		// Get table columns
		columns, err := fs.plugin.backend.GetTableColumns(fs.plugin.db, dbName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get table columns: %w", err)
		}

		if len(columns) == 0 {
			return nil, fmt.Errorf("no columns found for table %s", tableName)
		}

		// Build column names list
		columnNames := make([]string, len(columns))
		for i, col := range columns {
			columnNames[i] = col.Name
		}

		// Try to detect stream mode (NDJSON - Newline Delimited JSON)
		// Stream mode: each line is a complete JSON object
		dataStr := string(data)

		// Don't trim the entire string first, split directly
		lines := strings.Split(dataStr, "\n")

		// Filter out empty lines for counting
		nonEmptyLines := 0
		firstNonEmptyIdx := -1
		for i, line := range lines {
			if strings.TrimSpace(line) != "" {
				nonEmptyLines++
				if firstNonEmptyIdx == -1 {
					firstNonEmptyIdx = i
				}
			}
		}

		isStreamMode := false
		if nonEmptyLines > 1 && firstNonEmptyIdx >= 0 {
			// Check if first non-empty line is a valid JSON object (not array)
			var testObj map[string]interface{}
			firstLine := strings.TrimSpace(lines[firstNonEmptyIdx])
			if err := json.Unmarshal([]byte(firstLine), &testObj); err == nil {
				// Also check it's not a JSON array
				isStreamMode = true
			}
		}

		var records []map[string]interface{}
		var streamErrors []string

		if isStreamMode {
			// Stream mode: parse line by line (NDJSON format)
			for lineNum, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var record map[string]interface{}
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					streamErrors = append(streamErrors, fmt.Sprintf("line %d: %v", lineNum+1, err))
					continue
				}
				records = append(records, record)
			}
		} else {
			// Normal mode: parse as single JSON or JSON array
			var jsonData interface{}
			if err := json.Unmarshal(data, &jsonData); err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}

			// Handle both single object and array of objects
			switch v := jsonData.(type) {
			case map[string]interface{}:
				// Single JSON object
				records = append(records, v)
			case []interface{}:
				// Array of JSON objects
				for i, item := range v {
					if record, ok := item.(map[string]interface{}); ok {
						records = append(records, record)
					} else {
						return nil, fmt.Errorf("element at index %d is not a JSON object", i)
					}
				}
			default:
				return nil, fmt.Errorf("JSON must be an object or array of objects")
			}
		}

		if len(records) == 0 {
			if len(streamErrors) > 0 {
				return nil, fmt.Errorf("no valid records found. Errors: %s", strings.Join(streamErrors, "; "))
			}
			return nil, fmt.Errorf("no records to insert")
		}

		// Execute inserts
		insertedCount := 0
		failedCount := 0
		var insertErrors []string

		for idx, record := range records {
			// Build values list matching column order
			values := make([]interface{}, len(columnNames))
			for i, colName := range columnNames {
				if val, ok := record[colName]; ok {
					values[i] = val
				} else {
					values[i] = nil
				}
			}

			// Build INSERT statement
			placeholders := make([]string, len(columnNames))
			for i := range placeholders {
				placeholders[i] = "?"
			}

			insertSQL := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
				dbName, tableName,
				strings.Join(columnNames, ", "),
				strings.Join(placeholders, ", "))

			// Execute insert
			_, err := fs.plugin.db.Exec(insertSQL, values...)
			if err != nil {
				failedCount++
				insertErrors = append(insertErrors, fmt.Sprintf("record %d: %v", idx+1, err))
				// In stream mode, continue on error; in normal mode, fail fast
				if !isStreamMode {
					return nil, fmt.Errorf("insert error at record %d: %w", idx+1, err)
				}
				continue
			}
			insertedCount++
		}

		// Return success response
		response := map[string]interface{}{
			"inserted_count": insertedCount,
			"mode":           "normal",
		}

		if isStreamMode {
			response["mode"] = "stream"
			response["total_lines"] = len(lines)
			if failedCount > 0 {
				response["failed_count"] = failedCount
				response["errors"] = insertErrors
			}
			if len(streamErrors) > 0 {
				response["parse_errors"] = streamErrors
			}
		}

		output, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return output, nil
	}

	sqlStmt := strings.TrimSpace(string(data))
	if sqlStmt == "" {
		return nil, fmt.Errorf("empty SQL statement")
	}

	if operation == "query" {
		// Execute SELECT queries
		rows, err := fs.plugin.db.Query(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("query error: %w", err)
		}
		defer rows.Close()

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("failed to get columns: %w", err)
		}

		// Collect results
		var results []map[string]interface{}
		for rows.Next() {
			// Create a slice of interface{} to hold each column value
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return nil, fmt.Errorf("failed to scan row: %w", err)
			}

			// Create a map for this row
			rowMap := make(map[string]interface{})
			for i, colName := range columns {
				val := values[i]
				// Convert []byte to string for better JSON representation
				if b, ok := val.([]byte); ok {
					rowMap[colName] = string(b)
				} else {
					rowMap[colName] = val
				}
			}
			results = append(results, rowMap)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("rows iteration error: %w", err)
		}

		// Format as JSON
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal results: %w", err)
		}

		return output, nil

	} else if operation == "execute" {
		// Execute DML statements (INSERT, UPDATE, DELETE)
		result, err := fs.plugin.db.Exec(sqlStmt)
		if err != nil {
			return nil, fmt.Errorf("execution error: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		lastInsertId, _ := result.LastInsertId()

		response := map[string]interface{}{
			"rows_affected": rowsAffected,
		}
		if lastInsertId > 0 {
			response["last_insert_id"] = lastInsertId
		}

		output, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return output, nil
	}

	return nil, fmt.Errorf("unknown operation: %s", operation)
}

func (fs *sqlfs2FS) Create(path string) error {
	return fmt.Errorf("operation not supported: create")
}

func (fs *sqlfs2FS) Mkdir(path string, perm uint32) error {
	return fmt.Errorf("operation not supported: mkdir")
}

func (fs *sqlfs2FS) Remove(path string) error {
	return fmt.Errorf("operation not supported: remove")
}

func (fs *sqlfs2FS) RemoveAll(path string) error {
	dbName, tableName, operation, err := fs.parsePath(path)
	if err != nil {
		return err
	}

	// Support removing database (DROP DATABASE)
	// Path should be /dbName
	if dbName != "" && tableName == "" && operation == "" {
		// Execute DROP DATABASE
		sqlStmt := fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)
		_, err := fs.plugin.db.Exec(sqlStmt)
		if err != nil {
			return fmt.Errorf("failed to drop database: %w", err)
		}

		log.Infof("[sqlfs2] Dropped database: %s", dbName)
		return nil
	}

	// Support removing tables (DROP TABLE)
	// Path should be /dbName/tableName
	if dbName != "" && tableName != "" && operation == "" {
		// Switch to database if needed
		if err := fs.plugin.backend.SwitchDatabase(fs.plugin.db, dbName); err != nil {
			return err
		}

		// Execute DROP TABLE
		sqlStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", dbName, tableName)
		_, err := fs.plugin.db.Exec(sqlStmt)
		if err != nil {
			return fmt.Errorf("failed to drop table: %w", err)
		}

		log.Infof("[sqlfs2] Dropped table: %s.%s", dbName, tableName)
		return nil
	}

	return fmt.Errorf("operation not supported: can only remove databases or tables")
}

func (fs *sqlfs2FS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	dbName, tableName, operation, err := fs.parsePath(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Root directory: list databases
	if dbName == "" {
		dbNames, err := fs.plugin.backend.ListDatabases(fs.plugin.db)
		if err != nil {
			return nil, err
		}

		var databases []filesystem.FileInfo
		for _, name := range dbNames {
			databases = append(databases, filesystem.FileInfo{
				Name:    name,
				Size:    0,
				Mode:    0755,
				ModTime: now,
				IsDir:   true,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "database"},
			})
		}
		return databases, nil
	}

	// Database level: list tables
	if tableName == "" {
		tableNames, err := fs.plugin.backend.ListTables(fs.plugin.db, dbName)
		if err != nil {
			return nil, err
		}

		var tables []filesystem.FileInfo
		for _, name := range tableNames {
			tables = append(tables, filesystem.FileInfo{
				Name:    name,
				Size:    0,
				Mode:    0755,
				ModTime: now,
				IsDir:   true,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "table"},
			})
		}
		return tables, nil
	}

	// Table level: list operations (schema, execute, query, count, insert_json)
	if operation == "" {
		// Check if table exists
		exists, err := fs.tableExists(dbName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to check table existence: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("table '%s.%s' does not exist", dbName, tableName)
		}

		return []filesystem.FileInfo{
			{
				Name:    "schema",
				Size:    0,
				Mode:    0444, // read-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
			},
			{
				Name:    "count",
				Size:    0,
				Mode:    0444, // read-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
			},
			{
				Name:    "query",
				Size:    0,
				Mode:    0222, // write-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
			},
			{
				Name:    "execute",
				Size:    0,
				Mode:    0222, // write-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
			},
			{
				Name:    "insert_json",
				Size:    0,
				Mode:    0222, // write-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
			},
		}, nil
	}

	return nil, fmt.Errorf("not a directory: %s", path)
}

func (fs *sqlfs2FS) Stat(path string) (*filesystem.FileInfo, error) {
	dbName, tableName, operation, err := fs.parsePath(path)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Root directory
	if dbName == "" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: now,
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName},
		}, nil
	}

	// Database directory
	if tableName == "" {
		return &filesystem.FileInfo{
			Name:    dbName,
			Size:    0,
			Mode:    0755,
			ModTime: now,
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "database"},
		}, nil
	}

	// Table directory
	if operation == "" {
		// Check if table exists
		exists, err := fs.tableExists(dbName, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to check table existence: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("table '%s.%s' does not exist", dbName, tableName)
		}

		return &filesystem.FileInfo{
			Name:    tableName,
			Size:    0,
			Mode:    0755,
			ModTime: now,
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "table"},
		}, nil
	}

	// Operation files
	mode := uint32(0644)
	if operation == "schema" || operation == "count" {
		mode = 0444 // read-only
	} else if operation == "query" || operation == "execute" || operation == "insert_json" {
		mode = 0222 // write-only
	}

	return &filesystem.FileInfo{
		Name:    operation,
		Size:    0,
		Mode:    mode,
		ModTime: now,
		IsDir:   false,
		Meta:    filesystem.MetaData{Name: PluginName, Type: "operation"},
	}, nil
}

func (fs *sqlfs2FS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("operation not supported: rename")
}

func (fs *sqlfs2FS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("operation not supported: chmod")
}

func (fs *sqlfs2FS) Open(path string) (io.ReadCloser, error) {
	data, err := fs.Read(path, 0, -1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (fs *sqlfs2FS) OpenWrite(path string) (io.WriteCloser, error) {
	return filesystem.NewBufferedWriter(path, fs.Write), nil
}

func getReadme() string {
	return `SQLFS2 Plugin - SQL Interface Through File System

This plugin provides a SQL interface through file system operations.

DIRECTORY STRUCTURE:
  /sqlfs2/<dbName>/<tableName>/{schema, count, execute, query, insert_json}

FILES:
  schema      - Read-only file that returns SHOW CREATE TABLE output
  count       - Read-only file that returns SELECT COUNT(*) result
  query       - Write-only file for SELECT queries (returns JSON results)
  execute     - Write-only file for DML statements (INSERT/UPDATE/DELETE)
  insert_json - Write-only file for inserting JSON documents (auto-generates INSERT statements)
                Supports 3 modes (auto-detected):
                1. Single JSON object: {"name": "Alice"}
                2. JSON array: [{"name": "Bob"}, {"name": "Carol"}]
                3. NDJSON stream: {"name": "Alice"}\n{"name": "Bob"}\n (newline-delimited)

CONFIGURATION:

  SQLite Backend:
  [plugins.sqlfs2]
  enabled = true
  path = "/sqlfs2"

    [plugins.sqlfs2.config]
    backend = "sqlite"
    db_path = "sqlfs2.db"

  MySQL Backend:
  [plugins.sqlfs2]
  enabled = true
  path = "/sqlfs2"

    [plugins.sqlfs2.config]
    backend = "mysql"
    host = "localhost"
    port = "3306"
    user = "root"
    password = "password"
    database = "mydb"

  TiDB Backend (Local):
  [plugins.sqlfs2]
  enabled = true
  path = "/sqlfs2"

    [plugins.sqlfs2.config]
    backend = "tidb"
    host = "127.0.0.1"
    port = "4000"
    user = "root"
    password = ""
    database = "test"

  TiDB Cloud Backend (with TLS):
  [plugins.sqlfs2]
  enabled = true
  path = "/sqlfs2"

    [plugins.sqlfs2.config]
    backend = "tidb"
    user = "3YdGXuXNdAEmP1f.root"
    password = "your_password"
    host = "gateway01.us-west-2.prod.aws.tidbcloud.com"
    port = "4000"
    database = "test"
    enable_tls = true
    tls_server_name = "gateway01.us-west-2.prod.aws.tidbcloud.com"

    # Or use DSN with TLS:
    # dsn = "user:password@tcp(host:4000)/database?charset=utf8mb4&parseTime=True&tls=tidb-sqlfs2"

USAGE:

  View table schema:
    cat /sqlfs2/mydb/users/schema

  Get row count:
    cat /sqlfs2/mydb/users/count

  Execute SELECT query:
    echo 'SELECT * FROM users LIMIT 10' > /sqlfs2/mydb/users/query
    # Results are returned as JSON

  Execute INSERT statement:
    echo "INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com')" > /sqlfs2/mydb/users/execute

  Execute UPDATE statement:
    echo "UPDATE users SET name='Bob' WHERE id=1" > /sqlfs2/mydb/users/execute

  Execute DELETE statement:
    echo "DELETE FROM users WHERE id=1" > /sqlfs2/mydb/users/execute

  Insert JSON document (single object):
    echo '{"name": "Alice", "email": "alice@example.com", "age": 30}' > /sqlfs2/mydb/users/insert_json

  Insert JSON array (multiple records):
    echo '[{"name": "Bob", "email": "bob@example.com"}, {"name": "Carol", "email": "carol@example.com"}]' > /sqlfs2/mydb/users/insert_json

  Insert using NDJSON stream mode (auto-detected when input has newline-separated JSON objects):
    cat data.ndjson > /sqlfs2/mydb/users/insert_json
    # data.ndjson format:
    # {"name": "Alice", "email": "alice@example.com"}
    # {"name": "Bob", "email": "bob@example.com"}
    # {"name": "Carol", "email": "carol@example.com"}

  List databases:
    ls /sqlfs2/

  List tables in a database:
    ls /sqlfs2/mydb/

  List operations for a table:
    ls /sqlfs2/mydb/users/

EXAMPLES:

  # Create a test table
  agfs:/> echo "CREATE TABLE IF NOT EXISTS test (id INT, name VARCHAR(100))" > /sqlfs2/main/execute

  # Insert data
  agfs:/> echo "INSERT INTO test VALUES (1, 'Alice')" > /sqlfs2/main/test/execute

  # Query data
  agfs:/> echo "SELECT * FROM test" > /sqlfs2/main/test/query
  [
    {
      "id": 1,
      "name": "Alice"
    }
  ]

  # View schema
  agfs:/> cat /sqlfs2/main/test/schema
  CREATE TABLE test (id INT, name VARCHAR(100))

  # Get row count
  agfs:/> cat /sqlfs2/main/test/count
  1

  # Insert data using JSON (single object)
  agfs:/> echo '{"id": 2, "name": "Bob"}' > /sqlfs2/main/test/insert_json
  {
    "inserted_count": 1
  }

  # Insert multiple records using JSON array
  agfs:/> echo '[{"id": 3, "name": "Carol"}, {"id": 4, "name": "Dave"}]' > /sqlfs2/main/test/insert_json
  {
    "inserted_count": 2,
    "mode": "normal"
  }

  # Insert using NDJSON stream mode (newline-delimited JSON)
  agfs:/> cat <<EOF > /sqlfs2/main/test/insert_json
  {"id": 5, "name": "Eve"}
  {"id": 6, "name": "Frank"}
  {"id": 7, "name": "Grace"}
  EOF
  {
    "inserted_count": 3,
    "mode": "stream",
    "total_lines": 3
  }

  # Verify the inserts
  agfs:/> echo "SELECT * FROM test" > /sqlfs2/main/test/query
  [
    {
      "id": 1,
      "name": "Alice"
    },
    {
      "id": 2,
      "name": "Bob"
    },
    {
      "id": 3,
      "name": "Carol"
    },
    {
      "id": 4,
      "name": "Dave"
    }
  ]

ADVANTAGES:
  - Direct SQL access through file system interface
  - Supports SQLite, MySQL, and TiDB backends
  - JSON output for query results
  - Auto-generate INSERT statements from JSON documents
  - NDJSON streaming support for large data imports
  - Simple and intuitive interface
  - TLS support for secure TiDB Cloud connections

USE CASES:
  - Database exploration and querying
  - Data manipulation through file operations
  - Integration with shell scripts
  - Quick database operations without SQL client
  - Import JSON data without writing SQL INSERT statements
  - Stream large NDJSON files directly into database
`
}

// Ensure SQLFS2Plugin implements ServicePlugin
var _ plugin.ServicePlugin = (*SQLFS2Plugin)(nil)
var _ filesystem.FileSystem = (*sqlfs2FS)(nil)
