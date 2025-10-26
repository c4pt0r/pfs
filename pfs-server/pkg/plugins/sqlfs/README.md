SQLFS Plugin - Database-backed File System

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
    pfs mkdir /sqlfs/mydir

  Create a file:
    pfs write /sqlfs/mydir/file.txt "Hello, World!"

  Read a file:
    pfs cat /sqlfs/mydir/file.txt

  List directory:
    pfs ls /sqlfs/mydir

  Get file info:
    pfs stat /sqlfs/mydir/file.txt

  Rename file:
    pfs mv /sqlfs/mydir/file.txt /sqlfs/mydir/newfile.txt

  Change permissions:
    pfs chmod 755 /sqlfs/mydir/file.txt

  Remove file:
    pfs rm /sqlfs/mydir/file.txt

  Remove directory (must be empty):
    pfs rm /sqlfs/mydir

  Remove directory recursively:
    pfs rm -r /sqlfs/mydir

EXAMPLES:

  # Create directory structure
  pfs:/> mkdir /sqlfs/data
  pfs:/> mkdir /sqlfs/data/logs

  # Write files
  pfs:/> echo "Configuration data" > /sqlfs/data/config.txt
  pfs:/> echo "Log entry" > /sqlfs/data/logs/app.log

  # Read files
  pfs:/> cat /sqlfs/data/config.txt
  Configuration data

  # List directory
  pfs:/> ls /sqlfs/data
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

## License

Apache License 2.0
