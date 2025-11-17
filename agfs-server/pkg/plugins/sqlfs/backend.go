package sqlfs

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql" // MySQL/TiDB driver
	log "github.com/sirupsen/logrus"
)

// DBBackend defines the interface for different database backends
type DBBackend interface {
	// Open opens a connection to the database
	Open(config map[string]interface{}) (*sql.DB, error)

	// GetDriverName returns the driver name (e.g., "sqlite3", "mysql")
	GetDriverName() string

	// GetInitSQL returns the SQL statements to initialize the schema
	GetInitSQL() []string

	// SupportsTxIsolation returns whether the backend supports transaction isolation levels
	SupportsTxIsolation() bool

	// GetOptimizationSQL returns SQL statements for optimization (e.g., PRAGMA for SQLite)
	GetOptimizationSQL() []string
}

// SQLiteBackend implements DBBackend for SQLite
type SQLiteBackend struct{}

func NewSQLiteBackend() *SQLiteBackend {
	return &SQLiteBackend{}
}

func (b *SQLiteBackend) GetDriverName() string {
	return "sqlite3"
}

func (b *SQLiteBackend) Open(config map[string]interface{}) (*sql.DB, error) {
	dbPath := "sqlfs.db" // default
	if path, ok := config["db_path"].(string); ok && path != "" {
		dbPath = path
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	return db, nil
}

func (b *SQLiteBackend) GetInitSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY,
			is_dir INTEGER NOT NULL,
			mode INTEGER NOT NULL,
			size INTEGER NOT NULL,
			mod_time INTEGER NOT NULL,
			data BLOB
		)`,
		`CREATE INDEX IF NOT EXISTS idx_parent ON files(path)`,
	}
}

func (b *SQLiteBackend) GetOptimizationSQL() []string {
	return []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000", // 64MB cache
	}
}

func (b *SQLiteBackend) SupportsTxIsolation() bool {
	return false
}

// TiDBBackend implements DBBackend for TiDB
type TiDBBackend struct{}

func NewTiDBBackend() *TiDBBackend {
	return &TiDBBackend{}
}

func (b *TiDBBackend) GetDriverName() string {
	return "mysql"
}

func (b *TiDBBackend) Open(config map[string]interface{}) (*sql.DB, error) {
	// Check if DSN contains tls parameter
	dsnStr := getStringConfig(config, "dsn", "")
	dsnHasTLS := strings.Contains(dsnStr, "tls=")

	// Register TLS configuration if needed
	enableTLS := getBoolConfig(config, "enable_tls", false) || dsnHasTLS
	tlsConfigName := "tidb"

	if enableTLS {
		// Get TLS configuration
		serverName := getStringConfig(config, "tls_server_name", "")

		// If no explicit server name, try to extract from DSN or host
		if serverName == "" {
			if dsnStr != "" {
				// Extract host from DSN: user:pass@tcp(host:port)/db
				re := regexp.MustCompile(`@tcp\(([^:]+):\d+\)`)
				if matches := re.FindStringSubmatch(dsnStr); len(matches) > 1 {
					serverName = matches[1]
				}
			} else {
				// Use host config
				serverName = getStringConfig(config, "host", "")
			}
		}

		skipVerify := getBoolConfig(config, "tls_skip_verify", false)

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if serverName != "" {
			tlsConfig.ServerName = serverName
		}

		if skipVerify {
			tlsConfig.InsecureSkipVerify = true
			log.Warn("[sqlfs] TLS certificate verification is disabled (insecure)")
		}

		// Register TLS config with MySQL driver
		if err := mysql.RegisterTLSConfig(tlsConfigName, tlsConfig); err != nil {
			log.Warnf("[sqlfs] Failed to register TLS config (may already exist): %v", err)
		}
	}

	// Parse TiDB connection string
	// Format: user:password@tcp(host:port)/database
	dsn := ""

	if dsnStr, ok := config["dsn"].(string); ok && dsnStr != "" {
		dsn = dsnStr
	} else {
		// Build DSN from individual components
		user := getStringConfig(config, "user", "root")
		password := getStringConfig(config, "password", "")
		host := getStringConfig(config, "host", "127.0.0.1")
		port := getStringConfig(config, "port", "4000")
		database := getStringConfig(config, "database", "sqlfs")

		// Build base DSN
		if password != "" {
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True",
				user, password, host, port, database)
		} else {
			dsn = fmt.Sprintf("%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True",
				user, host, port, database)
		}

		// Add TLS parameter if enabled
		if enableTLS {
			dsn += fmt.Sprintf("&tls=%s", tlsConfigName)
		}
	}

	log.Infof("[sqlfs] Connecting to TiDB (TLS: %v)", enableTLS)

	// Extract database name to create it if needed
	dbName := extractDatabaseName(dsn, getStringConfig(config, "database", ""))

	// First, try to connect without database to create it if needed
	if dbName != "" {
		dsnWithoutDB := removeDatabaseFromDSN(dsn)
		if dsnWithoutDB != dsn {
			tempDB, err := sql.Open("mysql", dsnWithoutDB)
			defer tempDB.Close()
			if err == nil {
				// Try to create database if it doesn't exist
				_, err = tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName))
				if err != nil {
					log.Errorf("[sqlfs] Failed to create database '%s': %v", dbName, err)
					return nil, err
				}
			}
		}
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open TiDB database: %w", err)
	}

	// Set connection pool parameters
	// TODO: make it configurable
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	return db, nil
}

// extractDatabaseName extracts database name from DSN or config
func extractDatabaseName(dsn string, configDB string) string {
	if dsn != "" {
		// Extract from DSN: ...)/database?...
		re := regexp.MustCompile(`\)/([^?]+)`)
		if matches := re.FindStringSubmatch(dsn); len(matches) > 1 {
			return matches[1]
		}
	}
	return configDB
}

// removeDatabaseFromDSN removes database name from DSN
func removeDatabaseFromDSN(dsn string) string {
	// Replace )/database? with )/?
	re := regexp.MustCompile(`\)/[^?]+(\?|$)`)
	return re.ReplaceAllString(dsn, ")/$1")
}

func (b *TiDBBackend) GetInitSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS files (
			path VARCHAR(3072) PRIMARY KEY,
			is_dir TINYINT NOT NULL,
			mode INT UNSIGNED NOT NULL,
			size BIGINT NOT NULL,
			mod_time BIGINT NOT NULL,
			data LONGBLOB,
			INDEX idx_parent (path(200))
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
}

func (b *TiDBBackend) GetOptimizationSQL() []string {
	// TiDB doesn't need special optimization SQL
	return []string{}
}

func (b *TiDBBackend) SupportsTxIsolation() bool {
	return true
}

// getStringConfig retrieves a string value from config map with default
func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

// getBoolConfig retrieves a boolean value from config map with default
func getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

// CreateBackend creates the appropriate backend based on configuration
func CreateBackend(config map[string]interface{}) (DBBackend, error) {
	backendType := getStringConfig(config, "backend", "sqlite")

	switch backendType {
	case "sqlite", "sqlite3":
		return NewSQLiteBackend(), nil
	case "tidb", "mysql":
		return NewTiDBBackend(), nil
	default:
		return nil, fmt.Errorf("unsupported database backend: %s", backendType)
	}
}
