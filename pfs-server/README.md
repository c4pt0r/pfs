# PFS Server

A Plugin-based RESTful file system server with a powerful plugin architecture that exposes services as virtual file systems. Access queues, key-value stores, databases, and more through simple file operations.

Highly inspired by Plan9

## Features

- **Plugin Architecture**: Mount multiple filesystems and services at different paths
- **Unified API**: Single HTTP API for all file operations across all plugins
- **Dynamic Mounting**: Add/remove plugins at runtime without restarting
- **Configuration-based**: YAML configuration supports both single and multi-instance plugins
- **Built-in Plugins (Examples)**:
  - **ServerInfoFS** - Server information and metadata
  - **MemFS** - In-memory file system for fast temporary storage
  - **QueueFS** - Message queue exposed as files
  - **KVFS** - Key-value store as a virtual filesystem
  - **StreamFS** - Streaming data with multiple readers
  - **HelloFS** - Simple example plugin
  - **SQLFS** - Database-backed file system (SQLite/TiDB)
  - **ProxyFS** - Federation/proxy to remote PFS servers
  - **S3FS** - Amazon S3 as a file system
  - **LocalFS** - Mount local directories into PFS
  - **HTTPFS** - HTTP file server for any PFS path

## Quick Start

### Build and Run

```bash
# Build
make build

# Run with default config (port 8080)
./build/pfs-server

# Run with custom config
./build/pfs-server -c config.yaml

# Run on different port
./build/pfs-server -addr :9000

```

### Using the PFS Shell

The easiest way to interact with PFS Server:

```bash
cd ../pfs-shell
uv run pfs sh

# Or use direct commands
uv run pfs ls /
uv run pfs cat /queuefs/size
```

## Architecture

```
┌───────────────────────────────────────────────────────────────────┐
│                          PFS Server                               │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                RESTful API (/api/v1/*)                      │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              ↓                                    │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                      MountableFS                            │  │
│  │          (Plugin Mount Management & Routing)                │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                              ↓                                    │
│         ┌────────────────────┴────────────────────┐               │
│         ↓                    ↓                    ↓               │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐        │
│  │   MemFS     │      │  QueueFS    │      │  ProxyFS    │        │
│  │  /memfs     │      │  /queuefs   │      │ /proxyfs/*  ├────┐   │
│  └─────────────┘      └─────────────┘      └─────────────┘    │   │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐    │   │
│  │    KVFS     │      │  StreamFS   │      │    S3FS     │    │   │
│  │   /kvfs     │      │ /streamfs   │      │  /s3fs/*    │    │   │
│  └─────────────┘      └─────────────┘      └─────────────┘    │   │
│  ┌─────────────┐      ┌─────────────┐      ┌─────────────┐    │   │
│  │   SQLFS     │      │ ServerInfo  │      │  LocalFS    │    │   │
│  │  /sqlfs     │      │/serverinfo  │      │  /local     │    │   │
│  └─────────────┘      └─────────────┘      └─────────────┘    │   │
│                                                               │   │
│  ┌──────────────────────────────────────────────────────┐     │   │
│  │         HTTPFS - HTTP File Server (:9000)            │     │   │
│  │      /httpfs-*  (serves any PFS path via HTTP)       ├─────┼───┼──→ Browser
│  └──────────────────────────────────────────────────────┘     │   │    curl
└───────────────────────────────────────────────────────────────┼───┘
                                                                │
                       HTTP Federation (ProxyFS)                │
                                    ↓                           │
                         ┌─────────────────────┐                │
                         │  Remote PFS Server  │ ←──────────────┘
                         └─────────────────────┘
```

### Plugin System

- **Plugin Interface**: All plugins implement `FileSystem` interface
- **Mount Points**: Plugins can be mounted at any path
- **Multi-Instance**: Same plugin type can run multiple instances (e.g., multiple databases)
- **Dynamic Control**: Mount/unmount plugins at runtime via API
- **Configuration**: YAML-based configuration with support for arrays

## Configuration

### Basic Configuration

```yaml
server:
  address: ":8080"
  log_level: info  # debug, info, warn, error

plugins:
  # Single instance plugin
  memfs:
    enabled: true
    path: /memfs
    config:
      init_dirs:
        - /home
        - /tmp

  queuefs:
    enabled: true
    path: /queuefs
    config: {}

  kvfs:
    enabled: true
    path: /kvfs
    config:
      initial_data:
        welcome: "Hello from PFS!"
```

### Multi-Instance Configuration

```yaml
plugins:
  # Multiple SQLFS instances
  sqlfs:
    - name: local
      enabled: true
      path: /sqlfs
      config:
        backend: sqlite
        db_path: sqlfs.db
        cache_enabled: true

    - name: production
      enabled: true
      path: /sqlfs_prod
      config:
        backend: tidb
        dsn: "user:pass@tcp(host:4000)/db"

  # Multiple ProxyFS instances (federation)
  proxyfs:
    - name: server1
      enabled: true
      path: /remote/server1
      config:
        base_url: "http://server1.example.com:8080/api/v1"

    - name: server2
      enabled: true
      path: /remote/server2
      config:
        base_url: "http://server2.example.com:8080/api/v1"

  # Multiple LocalFS instances
  localfs_home:
    enabled: true
    path: /home
    config:
      local_dir: /Users/username

  localfs_data:
    enabled: true
    path: /data
    config:
      local_dir: /var/data
```

See [config.example.yaml](config.example.yaml) for complete examples.

## API Reference

All endpoints are prefixed with `/api/v1/`.

### File Operations

| Method | Endpoint | Description | Query Parameters |
|--------|----------|-------------|------------------|
| `POST` | `/files` | Create empty file | `path` |
| `GET` | `/files` | Read file | `path`, `offset` (optional), `size` (optional), `stream` (optional) |
| `PUT` | `/files` | Write file | `path` |
| `DELETE` | `/files` | Delete file | `path`, `recursive` (optional) |
| `GET` | `/stat` | Get file info | `path` |

### Directory Operations

| Method | Endpoint | Description | Query Parameters |
|--------|----------|-------------|------------------|
| `POST` | `/directories` | Create directory | `path`, `mode` (optional) |
| `GET` | `/directories` | List directory | `path` |

### File Management

| Method | Endpoint | Description | Body |
|--------|----------|-------------|------|
| `POST` | `/rename` | Rename/move | `{"newPath": "..."}` |
| `POST` | `/chmod` | Change permissions | `{"mode": 0644}` |

### Plugin Management

| Method | Endpoint | Description | Body |
|--------|----------|-------------|------|
| `GET` | `/mounts` | List mounted plugins | - |
| `POST` | `/mount` | Mount plugin | `{"fstype": "...", "path": "...", "config": {...}}` |
| `POST` | `/unmount` | Unmount plugin | `{"path": "..."}` |

### Health Check

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Server health check |

## Built-in Plugins

### QueueFS - Message Queue

Exposes a message queue through virtual files:

**File Structure:**
```
/queuefs/
├── enqueue  (write to enqueue)
├── dequeue  (read to dequeue)
├── peek     (read without removing)
├── size     (read queue size)
└── clear    (write to clear)
```

**Examples:**
```bash
# Enqueue message
pfs:/> echo "Task 1" > /queuefs/enqueue

# Dequeue message
pfs:/> cat /queuefs/dequeue
{"id":"1736936445000000000","data":"Task 1","timestamp":"2025-01-15T10:30:45Z"}

# Check size
pfs:/> cat /queuefs/size
5

# Peek at next
pfs:/> cat /queuefs/peek

# Clear queue
pfs:/> echo "" > /queuefs/clear
```

**cURL Examples:**
```bash
# Enqueue
curl -X PUT "http://localhost:8080/api/v1/files?path=/queuefs/enqueue" -d "My message"

# Dequeue
curl "http://localhost:8080/api/v1/files?path=/queuefs/dequeue"

# Size
curl "http://localhost:8080/api/v1/files?path=/queuefs/size"
```

### KVFS - Key-Value Store

Key-value store where each key is a file:

**File Structure:**
```
/kvfs/
└── keys/
    ├── key1
    ├── key2
    └── ...
```

**Examples:**
```bash
# Set key
pfs:/> echo "alice" > /kvfs/keys/username
pfs:/> echo "alice@example.com" > /kvfs/keys/email

# Get key
pfs:/> cat /kvfs/keys/username
alice

# List keys
pfs:/> ls /kvfs/keys

# Delete key
pfs:/> rm /kvfs/keys/username
```

**cURL Examples:**
```bash
# Set
curl -X PUT "http://localhost:8080/api/v1/files?path=/kvfs/keys/mykey" -d "myvalue"

# Get
curl "http://localhost:8080/api/v1/files?path=/kvfs/keys/mykey"

# List
curl "http://localhost:8080/api/v1/directories?path=/kvfs/keys"

# Delete
curl -X DELETE "http://localhost:8080/api/v1/files?path=/kvfs/keys/mykey"
```

### StreamFS - Streaming Data

Supports streaming reads/writes with multiple concurrent readers:

**Features:**
- Write streaming data
- Multiple readers can read the same stream
- Configurable buffer size
- Ring buffer prevents memory overflow

**Examples:**
```bash
# Stream video to server
cat video.mp4 | pfs write --stream /streamfs/live

# Stream from server (multiple clients can read simultaneously)
pfs cat --stream /streamfs/live | ffplay -

# Live transcoding
ffmpeg -i input.mp4 -f mpegts - | pfs write --stream /streamfs/channel1
pfs cat --stream /streamfs/channel1 | ffplay -
```

**Configuration:**
```yaml
streamfs:
  enabled: true
  path: /streamfs
  config:
    buffer_size: "10MB"  # Ring buffer size per stream
```

### SQLFS - Database-backed File System

Store files in SQL databases (SQLite or TiDB):

**Features:**
- Persistent file storage
- Transaction support
- Metadata caching
- Multi-instance support

**Configuration:**
```yaml
sqlfs:
  - name: local
    enabled: true
    path: /sqlfs
    config:
      backend: sqlite
      db_path: sqlfs.db
      cache_enabled: true
      cache_max_size: 1000
      cache_ttl_seconds: 5

  - name: production
    enabled: true
    path: /sqlfs_prod
    config:
      backend: tidb
      dsn: "user:pass@tcp(host:4000)/db?charset=utf8mb4"
      cache_enabled: true
```

**Examples:**
```bash
pfs:/> write /sqlfs/data/config.json '{"key": "value"}'
pfs:/> cat /sqlfs/data/config.json
pfs:/> write /sqlfs/local.txt "local data"
pfs:/> write /sqlfs_prod/prod.txt "production data"
```

### ProxyFS - Federation/Remote Access

Access remote PFS servers as local mount points:

**Features:**
- Transparent proxying to remote servers
- Multi-server federation
- Supports all file operations
- Client-side caching (optional)

**Configuration:**
```yaml
proxyfs:
  - name: remote1
    enabled: true
    path: /remote/server1
    config:
      base_url: "http://server1.local:8080/api/v1"

  - name: remote2
    enabled: true
    path: /remote/server2
    config:
      base_url: "http://server2.local:8080/api/v1"
```

**Examples:**
```bash
# Access remote server files
pfs:/> ls /remote/server1
pfs:/> cat /remote/server1/data/file.txt
pfs:/> write /remote/server2/upload.txt "data"

# Federation: copy between servers
pfs:/> cp /remote/server1/file.txt /remote/server2/file.txt
```

### S3FS - Amazon S3 File System

Access S3 buckets as file systems:

**Configuration:**
```yaml
s3fs:
  - name: mybucket
    enabled: true
    path: /s3/mybucket
    config:
      region: us-west-1
      bucket: my-bucket-name
      access_key_id: AKIAXXXXXXXX
      secret_access_key: secretkey
      prefix: pfs/  # Optional: prefix all keys
```

**Examples:**
```bash
# Upload to S3
pfs:/> write /s3/mybucket/data.txt "content"
pfs:/> upload /local/file.pdf /s3/mybucket/documents/file.pdf

# Download from S3
pfs:/> cat /s3/mybucket/data.txt
pfs:/> download /s3/mybucket/documents/file.pdf /local/file.pdf

# List S3 objects
pfs:/> ls /s3/mybucket
```

### LocalFS - Local File System Mount

Mount local directories into PFS for direct access:

**Configuration:**
```yaml
localfs:
  enabled: true
  path: /local
  config:
    local_dir: /path/to/local/directory  # Path to mount

# Multiple local mounts
localfs_home:
  enabled: true
  path: /home
  config:
    local_dir: /Users/username

localfs_data:
  enabled: true
  path: /data
  config:
    local_dir: /var/data
```

**Features:**
- Direct access to local file system
- No data copying - files are accessed in place
- Preserves file permissions and timestamps
- Supports all standard file operations
- Efficient for large files

**Examples:**
```bash
# List local directory
pfs:/> ls /local

# Read local file
pfs:/> cat /local/config.txt

# Write to local file
pfs:/> echo "data" > /local/output.txt

# Create directory
pfs:/> mkdir /local/newdir

# Copy from memfs to local
pfs:/> cp /memfs/temp.txt /local/backup.txt
```

**cURL Examples:**
```bash
# List directory
curl "http://localhost:8080/api/v1/directories?path=/local"

# Read file
curl "http://localhost:8080/api/v1/files?path=/local/file.txt"

# Write file
curl -X PUT "http://localhost:8080/api/v1/files?path=/local/output.txt" -d "content"

# Create directory
curl -X POST "http://localhost:8080/api/v1/directories?path=/local/newdir"
```

**Use Cases:**
- Access local configuration files
- Process local data files
- Integrate with existing file-based workflows
- Development and testing with local data
- Backup and sync operations

### HTTPFS - HTTP File Server

Serve any PFS path via HTTP, similar to `python3 -m http.server`:

**Features:**
- Serve any PFS filesystem (memfs, queuefs, s3fs, etc.) via HTTP
- Browse directories and download files in web browser
- README files display inline instead of downloading
- Virtual status file for monitoring each instance
- Dynamic mounting - create/remove HTTP servers at runtime
- Multiple instances serving different content

**Configuration:**
```yaml
httpfs:
  # Serve memfs on port 9000
  - name: httpfs-memfs
    enabled: true
    path: /httpfs-memfs
    config:
      pfs_path: /memfs        # PFS path to serve
      http_port: "9000"       # HTTP server port

  # Serve queuefs on port 9001
  - name: httpfs-queue
    enabled: false
    path: /httpfs-queue
    config:
      pfs_path: /queuefs
      http_port: "9001"

  # Serve S3 content on port 9002
  - name: httpfs-s3
    enabled: false
    path: /httpfs-s3
    config:
      pfs_path: /s3fs/mybucket
      http_port: "9002"
```

**Static Examples:**
```bash
# Upload files to memfs
pfs:/> write /memfs/report.pdf < report.pdf
pfs:/> write /memfs/README.md "# Project\n\nDocumentation here"

# Files are now accessible via HTTP
# Browser: http://localhost:9000/
# CLI: curl http://localhost:9000/report.pdf
```

**Dynamic Mounting Examples:**
```bash
# Create temporary HTTP server
pfs:/> mount httpfs /temp-http pfs_path=/memfs http_port=10000

# Check instance status
pfs:/> cat /temp-http
# Output:
# HTTPFS Instance Status
# ======================
# Virtual Path:    /temp-http
# PFS Source Path: /memfs
# HTTP Port:       10000
# Server Status:   Running
# Uptime:          5m30s
# ...

# Access via HTTP
# Browser: http://localhost:10000/
# CLI: curl http://localhost:10000/

# Remove when done
pfs:/> unmount /temp-http
```

**cURL Examples:**
```bash
# Dynamic mount
curl -X POST http://localhost:8080/api/v1/mount \
  -H "Content-Type: application/json" \
  -d '{
    "fstype": "httpfs",
    "path": "/my-http",
    "config": {
      "pfs_path": "/memfs",
      "http_port": "10000"
    }
  }'

# Check status
curl "http://localhost:8080/api/v1/files?path=/my-http"

# Access HTTP server
curl http://localhost:10000/
curl http://localhost:10000/file.txt

# Unmount
curl -X POST http://localhost:8080/api/v1/unmount \
  -H "Content-Type: application/json" \
  -d '{"path": "/my-http"}'
```

**Use Cases:**
- Temporary file sharing
- Multi-environment documentation (dev/staging/prod on different ports)
- Browse S3 buckets via HTTP
- Monitor queue contents in browser
- Quick file distribution without setting up separate web servers
- Development and debugging - visualize PFS content

**Special Features:**

1. **README Display**: README files (README, README.md, README.txt) display inline in browser instead of downloading

2. **Virtual Status File**: Each instance has a status file showing:
   - Virtual mount path
   - Source PFS path
   - HTTP port and endpoint
   - Server status and uptime
   - Access instructions

3. **Multiple Instances**: Run multiple HTTP servers simultaneously, each serving different content on different ports

**Multi-Instance Example:**
```bash
# Serve different content on different ports
pfs:/> mount httpfs /docs pfs_path=/memfs/docs http_port=8001
pfs:/> mount httpfs /images pfs_path=/memfs/images http_port=8002
pfs:/> mount httpfs /s3-public pfs_path=/s3fs/public http_port=8003

# Now you have:
# http://localhost:8001/ -> Documentation
# http://localhost:8002/ -> Images
# http://localhost:8003/ -> S3 public files
```

### MemFS - In-Memory File System

Fast in-memory storage for temporary files:

**Configuration:**
```yaml
memfs:
  enabled: true
  path: /memfs
  config:
    init_dirs:
      - /home
      - /tmp
      - /var
```

**Use Cases:**
- Temporary file storage
- Fast cache
- Development/testing
- Session data

### ServerInfoFS - Server Information

Exposes server metadata as files:

**File Structure:**
```
/serverinfofs/
├── version
├── uptime
└── stats
```

**Examples:**
```bash
pfs:/> cat /serverinfofs/version
1.0.0

pfs:/> cat /serverinfofs/uptime
24h30m15s
```

## Dynamic Plugin Management

### Mount Plugin at Runtime

```bash
# Using pfs shell
pfs:/> mount memfs /test/memory
pfs:/> mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db

# Using cURL
curl -X POST http://localhost:8080/api/v1/mount \
  -H "Content-Type: application/json" \
  -d '{
    "fstype": "memfs",
    "path": "/test/memory",
    "config": {}
  }'
```

### Unmount Plugin

```bash
# Using pfs shell
pfs:/> unmount /test/memory

# Using cURL
curl -X POST http://localhost:8080/api/v1/unmount \
  -H "Content-Type: application/json" \
  -d '{"path": "/test/memory"}'
```

### List Mounted Plugins

```bash
# Using pfs shell
pfs:/> mounts

# Using cURL
curl http://localhost:8080/api/v1/mounts
```

## Creating Custom Plugins

### Plugin Interface

All plugins must implement the `ServicePlugin` interface:

```go
type ServicePlugin interface {
    Name() string
    Initialize(config map[string]interface{}) error
    GetFileSystem() filesystem.FileSystem
    Shutdown() error
}
```

### FileSystem Interface

The returned filesystem must implement:

```go
type FileSystem interface {
    Read(path string) ([]byte, error)
    Write(path string, data []byte) error
    Create(path string) error
    Delete(path string) error
    Stat(path string) (FileInfo, error)
    List(path string) ([]FileInfo, error)
    Mkdir(path string, mode uint32) error
    Rename(oldPath, newPath string) error
    Chmod(path string, mode uint32) error
}
```

### Example Plugin

```go
package myplugin

import (
    "github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
    "github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
)

type MyPlugin struct {
    config map[string]interface{}
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{}
}

func (p *MyPlugin) Name() string {
    return "myplugin"
}

func (p *MyPlugin) Initialize(config map[string]interface{}) error {
    p.config = config
    return nil
}

func (p *MyPlugin) GetFileSystem() filesystem.FileSystem {
    return &MyFS{plugin: p}
}

func (p *MyPlugin) Shutdown() error {
    return nil
}

type MyFS struct {
    plugin *MyPlugin
}

func (fs *MyFS) Read(path string) ([]byte, error) {
    // Implementation
    return []byte("data"), nil
}

// ... implement other FileSystem methods
```

### Register Plugin

Add to `cmd/server/main.go`:

```go
var availablePlugins = map[string]PluginFactory{
    // ... existing plugins
    "myplugin": func(configFile string) plugin.ServicePlugin {
        return myplugin.NewMyPlugin()
    },
}
```

## Project Structure

```
pfs-server/
├── cmd/
│   └── server/
│       └── main.go              # Server entry point
├── pkg/
│   ├── config/
│   │   └── config.go            # YAML configuration
│   ├── filesystem/
│   │   └── filesystem.go        # FileSystem interface
│   ├── mountablefs/
│   │   └── mountablefs.go       # Plugin mount manager
│   ├── plugin/
│   │   ├── plugin.go            # Plugin interfaces
│   │   └── utils.go             # Plugin utilities
│   ├── plugins/
│   │   ├── memfs/               # In-memory FS
│   │   ├── queuefs/             # Message queue
│   │   ├── kvfs/                # Key-value store
│   │   ├── streamfs/            # Streaming
│   │   ├── sqlfs/               # Database-backed FS
│   │   ├── proxyfs/             # Remote proxy
│   │   ├── s3fs/                # Amazon S3
│   │   ├── localfs/             # Local file system mount
│   │   ├── httpfs/              # HTTP file server
│   │   ├── serverinfofs/        # Server info
│   │   └── hellofs/             # Example plugin
│   ├── handlers/
│   │   ├── handlers.go          # HTTP handlers
│   │   └── plugin_handlers.go   # Plugin management
│   └── client/
│       ├── client.go            # Go client library
│       └── client_test.go
├── config.example.yaml          # Example configuration
├── Makefile                     # Build commands
└── go.mod
```

## Development

### Building

```bash
# Build binary
make build

# Run development server
make dev

# Run tests
make test

# Install to $GOPATH/bin
make install
```

## License

Apache License 2.0
