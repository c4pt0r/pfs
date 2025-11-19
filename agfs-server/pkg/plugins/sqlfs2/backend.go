package sqlfs2

import "database/sql"

// Backend defines the interface for different database backends
type Backend interface {
	// Initialize creates and returns a database connection
	Initialize(cfg map[string]interface{}) (*sql.DB, error)

	// GetTableSchema retrieves the CREATE TABLE statement for a table
	GetTableSchema(db *sql.DB, dbName, tableName string) (string, error)

	// ListDatabases returns a list of all databases
	ListDatabases(db *sql.DB) ([]string, error)

	// ListTables returns a list of all tables in a database
	ListTables(db *sql.DB, dbName string) ([]string, error)

	// SwitchDatabase switches to the specified database (no-op for SQLite)
	SwitchDatabase(db *sql.DB, dbName string) error

	// GetTableColumns retrieves column names and types for a table
	GetTableColumns(db *sql.DB, dbName, tableName string) ([]ColumnInfo, error)

	// Name returns the backend name
	Name() string
}

// ColumnInfo contains information about a table column
type ColumnInfo struct {
	Name string
	Type string
}

// newBackend creates a backend instance based on the backend type
func newBackend(backendType string) Backend {
	switch backendType {
	case "sqlite", "sqlite3":
		return &SQLiteBackend{}
	case "mysql":
		return &MySQLBackend{}
	case "tidb":
		return &TiDBBackend{}
	default:
		return nil
	}
}
