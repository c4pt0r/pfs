# AGFS Server

A Plugin-based RESTful file system server with a powerful plugin architecture that exposes services as virtual file systems. Access queues, key-value stores, databases, and more through simple file operations.

Highly inspired by Plan9

## Features

- **Plugin Architecture**: Mount multiple filesystems and services at different paths
- **External Plugin Support**: Load plugins from dynamic libraries (.so/.dylib/.dll) without recompiling
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
  - **ProxyFS** - Federation/proxy to remote AGFS servers
  - **S3FS** - Amazon S3 as a file system
  - **LocalFS** - Mount local directories into AGFS
  - **HTTAGFS** - HTTP file server for any AGFS path

## Quick Start

### Build and Run

```bash
# Build
make build

# Run with default config (port 8080)
./build/agfs-server

# Run with custom config
./build/agfs-server -c config.yaml

# Run on different port
./build/agfs-server -addr :9000

```

### Using the AGFS Shell

The easiest way to interact with AGFS Server:

```bash
cd ../agfs-shell
uv run agfssh

# Or use direct commands
uv run agfsls /
uv run agfscat /queuefs/size
```

## Architecture

```
┌───────────────────────────────────────────────────────────────────┐
│                          AGFS Server                              │
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
│  │         HTTAGFS - HTTP File Server (:9000)           │     │   │
│  │      /httagfs-*  (serves any AGFS path via HTTP)     ├─────┼───┼──→ Browser
│  └──────────────────────────────────────────────────────┘     │   │    curl
└───────────────────────────────────────────────────────────────┼───┘
                                                                │
                       HTTP Federation (ProxyFS)                │
                                    ↓                           │
                         ┌─────────────────────┐                │
                         │  Remote AGFS Server │ ←──────────────┘
                         └─────────────────────┘
```

### Plugin System

- **Plugin Interface**: All plugins implement `FileSystem` interface
- **Built-in Plugins**: Go plugins compiled into the server
- **External Plugins**: Support both native libraries and WebAssembly modules
  - **Native** (.so/.dylib/.dll): Maximum performance, platform-specific
  - **WASM** (.wasm): Cross-platform, sandboxed execution
- **Smart Type Detection**: Automatic plugin type detection via file magic numbers
- **Multi-Language Support**: Write plugins in C, C++, Rust, or any language with C ABI / WASM support
- **Mount Points**: Plugins can be mounted at any path
- **Multi-Instance**: Same plugin type can run multiple instances (e.g., multiple databases)
- **Dynamic Control**: Load/unload/mount/unmount plugins at runtime via API
- **Configuration**: YAML-based configuration with JSON parameter passing to plugins
- **Zero Cgo**: Native plugins use purego for FFI (no C compiler needed for Go code)

## Configuration

### Basic Configuration

```yaml
server:
  address: ":8080"
  log_level: info  # debug, info, warn, error

# External plugins (optional)
external_plugins:
  enabled: true
  plugin_dir: "./plugins"        # Auto-load all plugins from this directory
  auto_load: true                # Enable auto-loading
  plugin_paths:                  # Specific plugins to load (native or WASM)
    - "./examples/hellofs-c/hellofs-c.dylib"      # Native plugin
    - "./examples/hellofs-wasm/hellofs-wasm.wasm" # WASM plugin

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
        welcome: "Hello from AGFS!"
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
| `GET` | `/plugins` | List loaded external plugins | - |
| `POST` | `/plugins/load` | Load external plugin | `{"library_path": "..."}` |
| `POST` | `/plugins/unload` | Unload external plugin | `{"library_path": "..."}` |

### Health Check

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Server health check |

## Built-in Plugins

### QueueFS - Message Queue

Exposes multiple message queues through virtual files. Each queue is represented as a directory with control files for queue operations.

**Features:**
- Multiple independent queues in one instance
- Nested queue directories for organization
- FIFO (First-In-First-Out) message ordering
- JSON-formatted message output with ID and timestamp
- Non-blocking operations (dequeue returns empty object when queue is empty)
- Thread-safe concurrent access
- **Pluggable backends**: Memory (default), SQLite, TiDB/MySQL
- **Persistent storage**: SQLite and TiDB backends survive server restarts
- **Poll offset tracking**: Peek file's modTime reflects latest enqueued message timestamp
- **TLS support**: Secure connections to TiDB Cloud and MySQL

**File Structure:**
```
/queuefs/
├── README              (plugin documentation)
└── <queue_name>/       (each queue is a directory)
    ├── enqueue         (write-only: add message to queue)
    ├── dequeue         (read-only: remove and return first message)
    ├── peek            (read-only: view first message without removing)
    ├── size            (read-only: get queue size)
    └── clear           (write-only: remove all messages)
```

**Configuration:**

```yaml
# Memory backend (default) - Fast, non-persistent
queuefs:
  enabled: true
  path: /queuefs
  config:
    backend: memory  # or omit for default

# SQLite backend - Persistent, file-based
queuefs:
  enabled: true
  path: /queuefs
  config:
    backend: sqlite
    db_path: queue.db

# TiDB/MySQL backend (local) - Persistent, distributed
queuefs:
  enabled: true
  path: /queuefs
  config:
    backend: tidb     # or "mysql"
    host: 127.0.0.1
    port: 4000
    user: root
    password: ""
    database: queuedb

# TiDB Cloud backend - Persistent, cloud-hosted with TLS
queuefs:
  enabled: true
  path: /queuefs
  config:
    backend: tidb
    user: 3YdGXuXNdAEmP1f.root
    password: your_password
    host: gateway01.us-west-2.prod.aws.tidbcloud.com
    port: 4000
    database: queuedb
    enable_tls: true
    tls_server_name: gateway01.us-west-2.prod.aws.tidbcloud.com
    # tls_skip_verify: false  # optional, for testing only
```

**Basic Usage:**

```bash
# Create a queue
agfs:/> mkdir /queuefs/tasks

# Enqueue messages
agfs:/> echo "Process order #123" > /queuefs/tasks/enqueue
agfs:/> echo "Send email to user" > /queuefs/tasks/enqueue
agfs:/> echo "Update inventory" > /queuefs/tasks/enqueue

# Check queue size
agfs:/> cat /queuefs/tasks/size
3

# Peek at next message (without removing)
agfs:/> cat /queuefs/tasks/peek
{"id":"1736936445000000000","data":"Process order #123","timestamp":"2025-01-15T10:30:45Z"}

# Dequeue messages (removes from queue)
agfs:/> cat /queuefs/tasks/dequeue
{"id":"1736936445000000000","data":"Process order #123","timestamp":"2025-01-15T10:30:45Z"}

agfs:/> cat /queuefs/tasks/size
2

# Clear all remaining messages
agfs:/> echo "" > /queuefs/tasks/clear

# Delete the queue
agfs:/> rm -rf /queuefs/tasks
```

**Multi-Queue Usage:**

```bash
# Create multiple queues for different purposes
agfs:/> mkdir /queuefs/orders
agfs:/> mkdir /queuefs/notifications
agfs:/> mkdir /queuefs/logs

# Each queue operates independently
agfs:/> echo "order-456" > /queuefs/orders/enqueue
agfs:/> echo "User logged in" > /queuefs/notifications/enqueue
agfs:/> echo "Database connected" > /queuefs/logs/enqueue

# List all queues
agfs:/> ls /queuefs/
README  orders  notifications  logs

# Process specific queues
agfs:/> cat /queuefs/orders/dequeue
{"id":"1736936450000000000","data":"order-456","timestamp":"2025-01-15T10:30:50Z"}
```

**Nested Queues:**

```bash
# Create hierarchical queue organization
agfs:/> mkdir -p /queuefs/logs/errors
agfs:/> mkdir -p /queuefs/logs/warnings
agfs:/> mkdir -p /queuefs/logs/info

# Use nested queues
agfs:/> echo "Connection timeout" > /queuefs/logs/errors/enqueue
agfs:/> echo "Slow query detected" > /queuefs/logs/warnings/enqueue
agfs:/> echo "Service started" > /queuefs/logs/info/enqueue

# List queue hierarchy
agfs:/> ls /queuefs/logs/
errors  warnings  info

# Process nested queues
agfs:/> cat /queuefs/logs/errors/dequeue
{"id":"1736936460000000000","data":"Connection timeout","timestamp":"2025-01-15T10:31:00Z"}
```

**Empty Queue Behavior:**

```bash
# Reading from empty queue returns empty JSON object (no error)
agfs:/> cat /queuefs/tasks/dequeue
{}

agfs:/> cat /queuefs/tasks/peek
{}

agfs:/> cat /queuefs/tasks/size
0
```

**Poll Offset Tracking:**

The `peek` file's modification time (`modTime`) reflects the timestamp of the most recently enqueued message. This enables efficient polling by checking the file's `modTime` to detect new messages without reading the queue.

```bash
# Check if new messages arrived since last poll
agfs:/> stat /queuefs/tasks/peek
# ModTime: 2025-01-15T10:30:45Z  (timestamp of last enqueued message)

# If modTime changed, there's a new message
# This is useful for implementing efficient poll-based consumers
```

**cURL Examples:**

```bash
# Create queue
curl -X POST "http://localhost:8080/api/v1/directories?path=/queuefs/myqueue"

# Enqueue messages
curl -X PUT "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/enqueue" \
  -d "First message"

curl -X PUT "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/enqueue" \
  -d "Second message"

# Check size
curl "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/size"
# Output: 2

# Peek at next message
curl "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/peek"
# Output: {"id":"...","data":"First message","timestamp":"..."}

# Check peek file stat for poll offset
curl "http://localhost:8080/api/v1/stat?path=/queuefs/myqueue/peek"
# Returns FileInfo with modTime reflecting last enqueued message

# Dequeue message
curl "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/dequeue"
# Output: {"id":"...","data":"First message","timestamp":"..."}

# Clear queue
curl -X PUT "http://localhost:8080/api/v1/files?path=/queuefs/myqueue/clear" -d ""

# Delete queue
curl -X DELETE "http://localhost:8080/api/v1/files?path=/queuefs/myqueue&recursive=true"
```

**Use Cases:**
1. **Task Queues**: Background job processing, async task execution
2. **Event Processing**: Event-driven architectures, message passing
3. **Log Aggregation**: Centralized logging with categorized queues
4. **Workflow Orchestration**: Multi-step processes with queue-based state
5. **Rate Limiting**: Queue-based request buffering and throttling
6. **Microservices Communication**: Simple message broker between services
7. **Distributed Processing**: TiDB backend for multi-server queue sharing
8. **Persistent Queues**: SQLite/TiDB for durable message storage

**Message Format:**

All dequeued/peeked messages are returned as JSON:
```json
{
  "id": "1736936445000000000",       // Unique message ID (nanosecond timestamp)
  "data": "message content",          // Original message text
  "timestamp": "2025-01-15T10:30:45Z" // ISO 8601 timestamp
}
```

**Backend Comparison:**

| Backend | Persistence | Performance | Use Case |
|---------|-------------|-------------|----------|
| **Memory** | ✗ (lost on restart) | Fastest | Development, temporary queues, caching |
| **SQLite** | ✓ (file-based) | Fast | Single server, moderate load, local persistence |
| **TiDB/MySQL** | ✓ (distributed) | Good | Production, distributed systems, high availability |

**Performance Notes:**
- Thread-safe for concurrent producers/consumers
- O(1) enqueue, O(1) dequeue operations
- Memory backend: Lowest latency, no I/O overhead
- SQLite backend: Good for single-server deployments
- TiDB backend: Horizontally scalable, ACID compliant, suitable for distributed systems

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
agfs:/> echo "alice" > /kvfs/keys/username
agfs:/> echo "alice@example.com" > /kvfs/keys/email

# Get key
agfs:/> cat /kvfs/keys/username
alice

# List keys
agfs:/> ls /kvfs/keys

# Delete key
agfs:/> rm /kvfs/keys/username
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
cat video.mp4 | agfs write --stream /streamfs/live

# Stream from server (multiple clients can read simultaneously)
agfs cat --stream /streamfs/live | ffplay -

# Live transcoding
ffmpeg -i input.mp4 -f mpegts - | agfs write --stream /streamfs/channel1
agfs cat --stream /streamfs/channel1 | ffplay -
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
agfs:/> write /sqlfs/data/config.json '{"key": "value"}'
agfs:/> cat /sqlfs/data/config.json
agfs:/> write /sqlfs/local.txt "local data"
agfs:/> write /sqlfs_prod/prod.txt "production data"
```

### ProxyFS - Federation/Remote Access

Access remote AGFS servers as local mount points:

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
agfs:/> ls /remote/server1
agfs:/> cat /remote/server1/data/file.txt
agfs:/> write /remote/server2/upload.txt "data"

# Federation: copy between servers
agfs:/> cp /remote/server1/file.txt /remote/server2/file.txt
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
      prefix: agfs/  # Optional: prefix all keys
```

**Examples:**
```bash
# Upload to S3
agfs:/> write /s3/mybucket/data.txt "content"
agfs:/> upload /local/file.pdf /s3/mybucket/documents/file.pdf

# Download from S3
agfs:/> cat /s3/mybucket/data.txt
agfs:/> download /s3/mybucket/documents/file.pdf /local/file.pdf

# List S3 objects
agfs:/> ls /s3/mybucket
```

### LocalFS - Local File System Mount

Mount local directories into AGFS for direct access:

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
agfs:/> ls /local

# Read local file
agfs:/> cat /local/config.txt

# Write to local file
agfs:/> echo "data" > /local/output.txt

# Create directory
agfs:/> mkdir /local/newdir

# Copy from memfs to local
agfs:/> cp /memfs/temp.txt /local/backup.txt
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

### HTTAGFS - HTTP File Server

Serve any AGFS path via HTTP, similar to `python3 -m http.server`:

**Features:**
- Serve any AGFS filesystem (memfs, queuefs, s3fs, etc.) via HTTP
- Browse directories and download files in web browser
- README files display inline instead of downloading
- Virtual status file for monitoring each instance
- Dynamic mounting - create/remove HTTP servers at runtime
- Multiple instances serving different content

**Configuration:**
```yaml
httpfs:
  # Serve memfs on port 9000
  - name: httagfs-memfs
    enabled: true
    path: /httagfs-memfs
    config:
      agfs_path: /memfs        # AGFS path to serve
      http_port: "9000"       # HTTP server port

  # Serve queuefs on port 9001
  - name: httagfs-queue
    enabled: false
    path: /httagfs-queue
    config:
      agfs_path: /queuefs
      http_port: "9001"

  # Serve S3 content on port 9002
  - name: httagfs-s3
    enabled: false
    path: /httagfs-s3
    config:
      agfs_path: /s3fs/mybucket
      http_port: "9002"
```

**Static Examples:**
```bash
# Upload files to memfs
agfs:/> write /memfs/report.pdf < report.pdf
agfs:/> write /memfs/README.md "# Project\n\nDocumentation here"

# Files are now accessible via HTTP
# Browser: http://localhost:9000/
# CLI: curl http://localhost:9000/report.pdf
```

**Dynamic Mounting Examples:**
```bash
# Create temporary HTTP server
agfs:/> mount httpfs /temp-http agfs_path=/memfs http_port=10000

# Check instance status
agfs:/> cat /temp-http
# Output:
# HTTAGFS Instance Status
# ======================
# Virtual Path:    /temp-http
# AGFS Source Path: /memfs
# HTTP Port:       10000
# Server Status:   Running
# Uptime:          5m30s
# ...

# Access via HTTP
# Browser: http://localhost:10000/
# CLI: curl http://localhost:10000/

# Remove when done
agfs:/> unmount /temp-http
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
      "agfs_path": "/memfs",
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
- Development and debugging - visualize AGFS content

**Special Features:**

1. **README Display**: README files (README, README.md, README.txt) display inline in browser instead of downloading

2. **Virtual Status File**: Each instance has a status file showing:
   - Virtual mount path
   - Source AGFS path
   - HTTP port and endpoint
   - Server status and uptime
   - Access instructions

3. **Multiple Instances**: Run multiple HTTP servers simultaneously, each serving different content on different ports

**Multi-Instance Example:**
```bash
# Serve different content on different ports
agfs:/> mount httpfs /docs agfs_path=/memfs/docs http_port=8001
agfs:/> mount httpfs /images agfs_path=/memfs/images http_port=8002
agfs:/> mount httpfs /s3-public agfs_path=/s3fs/public http_port=8003

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
agfs:/> cat /serverinfofs/version
1.0.0

agfs:/> cat /serverinfofs/uptime
24h30m15s
```

## Dynamic Plugin Management

### Mount Plugin at Runtime

```bash
# Using agfs shell
agfs:/> mount memfs /test/memory
agfs:/> mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db

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
# Using agfs shell
agfs:/> unmount /test/memory

# Using cURL
curl -X POST http://localhost:8080/api/v1/unmount \
  -H "Content-Type: application/json" \
  -d '{"path": "/test/memory"}'
```

### List Mounted Plugins

```bash
# Using agfs shell
agfs:/> mounts

# Using cURL
curl http://localhost:8080/api/v1/mounts
```

## Creating Custom Plugins

AGFS Server supports two types of plugins:

1. **Built-in Plugins**: Go plugins compiled into the server
2. **External Plugins**: Dynamically loaded from shared libraries

### External Plugins (Dynamic Libraries)

External plugins allow you to write plugins in C, C++, Rust, or any language that can produce C-compatible shared libraries, without recompiling the server.

**Quick Start:**

```bash
# 1. Write your plugin in C (see examples/plugins/hellofs-c/)
# 2. Compile to shared library
cd examples/plugins/hellofs-c
make  # Creates hellofs-c.dylib (macOS), .so (Linux), or .dll (Windows)

# 3. Configure server to load it
# config.yaml:
external_plugins:
  enabled: true
  plugin_paths:
    - "./examples/plugins/hellofs-c/hellofs-c.dylib"

# 4. Start server (plugin auto-loads)
./agfs-server -c config.yaml

# 5. Mount and use
curl -X POST http://localhost:8080/api/v1/mount \
  -d '{"fstype": "hellofs-c", "path": "/helloc", "config": {}}'

curl "http://localhost:8080/api/v1/files?path=/helloc/hello"
# Output: Hello from C plugin!
```

**C Plugin API:**

External plugins must implement C-compatible functions:

```c
// Required functions
void* PluginNew();                    // Create plugin instance
const char* PluginName(void* plugin); // Return plugin name

// Optional lifecycle functions
void PluginFree(void* plugin);
const char* PluginValidate(void* plugin, const char* config_json);
const char* PluginInitialize(void* plugin, const char* config_json);
const char* PluginShutdown(void* plugin);
const char* PluginGetReadme(void* plugin);

// Optional filesystem functions
const char* FSRead(void* plugin, const char* path, long long offset, long long size, int* out_len);
FileInfoC* FSStat(void* plugin, const char* path);
FileInfoArray* FSReadDir(void* plugin, const char* path, int* out_count);
const char* FSWrite(void* plugin, const char* path, const char* data, int data_len);
...
```

**Features:**
-  No Cgo required (uses purego for FFI)
-  Cross-platform (macOS, Linux, Windows)
-  Runtime loading/unloading
-  Multi-language support (C, C++, Rust, etc.)
-  Near-native performance
-  Full API documentation available

**Runtime Loading:**

```bash
# Load plugin at runtime
curl -X POST http://localhost:8080/api/v1/plugins/load \
  -d '{"library_path": "./my-plugin.dylib"}'

# List loaded plugins
curl http://localhost:8080/api/v1/plugins

# Mount it
curl -X POST http://localhost:8080/api/v1/mount \
  -d '{"fstype": "my-plugin", "path": "/my", "config": {}}'

# Unmount and unload when done
curl -X POST http://localhost:8080/api/v1/unmount -d '{"path": "/my"}'
curl -X POST http://localhost:8080/api/v1/plugins/unload -d '{"library_path": "./my-plugin.dylib"}'
```

### Built-in Go Plugins

For Go plugins compiled into the server, implement the `ServicePlugin` interface:

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
    "github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
    "github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
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

## External Plugin System

AGFS Server supports dynamically loading plugins from shared libraries (.so, .dylib, .dll) at runtime using [purego](https://github.com/ebitengine/purego). This enables:

-  Writing plugins in **any language** (C, C++, Rust, Zig, etc.)
-  **No recompilation** of the server needed
-  **Runtime loading/unloading** of plugins
-  **Zero Cgo** - pure Go FFI implementation
-  **Cross-platform** support (macOS, Linux, Windows)
-  **Near-native performance**

### Architecture

```
┌─────────────────────────────────┐
│   AGFS Server (Go)               │
│                                 │
│  ┌──────────────────────────┐   │
│  │   Plugin Loader          │   │
│  │   (purego FFI)           │   │
│  └──────────────────────────┘   │
└─────────────────────────────────┘
              ↓
┌─────────────────────────────────┐
│  External Plugin                │
│  (.so / .dylib / .dll)          │
│                                 │
│  - C/C++/Rust implementation    │
│  - Exports C-compatible API     │
└─────────────────────────────────┘
```

### Example: HelloFS-C Plugin

A simple read-only filesystem written in C:

** Write Plugin (hellofs.c):**

```c
#include <stdlib.h>
#include <string.h>

typedef struct { char* name; } HelloFSPlugin;

void* PluginNew() {
    HelloFSPlugin* p = malloc(sizeof(HelloFSPlugin));
    p->name = strdup("hellofs-c");
    return p;
}

const char* PluginName(void* plugin) {
    return ((HelloFSPlugin*)plugin)->name;
}

const char* FSRead(void* plugin, const char* path,
                   long long offset, long long size, int* out_len) {
    if (strcmp(path, "/hello") == 0) {
        const char* data = "Hello from C plugin!";
        *out_len = strlen(data);
        return strdup(data);
    }
    *out_len = -1;
    return NULL;
}


**Configure:**

```yaml
external_plugins:
  enabled: true
  plugin_paths:
    - "./hellofs-c.dylib"
```

**Use:**

```bash
# Start server (plugin loads automatically)
./agfs-server

# Mount the plugin
curl -X POST http://localhost:8080/api/v1/mount \
  -d '{"fstype": "hellofs-c", "path": "/helloc", "config": {}}'

# Read file
curl "http://localhost:8080/api/v1/files?path=/helloc/hello"
# Output: Hello from C plugin!
```

### C Plugin API Reference

A full working example (hellofs in Go/Rust/C/WASM) is provided in ./examples

**Required Functions:**

```c
void* PluginNew();                    // Create plugin instance
const char* PluginName(void* plugin); // Return plugin name
```

**Optional Lifecycle:**

```c
void PluginFree(void* plugin);
const char* PluginValidate(void* plugin, const char* config_json);
const char* PluginInitialize(void* plugin, const char* config_json);
const char* PluginShutdown(void* plugin);
const char* PluginGetReadme(void* plugin);
```

**Optional FileSystem Operations:**

```c
const char* FSCreate(void*, const char* path);
const char* FSMkdir(void*, const char* path, unsigned int perm);
const char* FSRemove(void*, const char* path);
const char* FSRemoveAll(void*, const char* path);
const char* FSRead(void*, const char* path, long long offset, long long size, int* out_len);
const char* FSWrite(void*, const char* path, const char* data, int data_len);
FileInfoArray* FSReadDir(void*, const char* path, int* out_count);
FileInfoC* FSStat(void*, const char* path);
const char* FSRename(void*, const char* old_path, const char* new_path);
const char* FSChmod(void*, const char* path, unsigned int mode);
```

**Return Values:**
- Error functions: Return `NULL` on success, error message on failure
- `FSRead`: Returns data pointer, sets `*out_len` to length (or -1 on error)
- `FSStat`/`FSReadDir`: Return C structures (see API docs)

### Configuration Options

```yaml
external_plugins:
  enabled: true                    # Enable external plugin support
  plugin_dir: "./plugins"          # Directory to scan for plugins
  auto_load: true                  # Auto-load all plugins in plugin_dir
  plugin_paths:                    # Specific plugin paths to load
    - "./path/to/plugin1.dylib"
    - "./path/to/plugin2.dylib"
```

### Runtime Plugin Management

**Load Plugin:**
```bash
curl -X POST http://localhost:8080/api/v1/plugins/load \
  -H "Content-Type: application/json" \
  -d '{"library_path": "./my-plugin.dylib"}'
```

**List Loaded Plugins:**
```bash
curl http://localhost:8080/api/v1/plugins
# {"loaded_plugins": ["./my-plugin.dylib", ...]}
```

**Unload Plugin:**
```bash
curl -X POST http://localhost:8080/api/v1/plugins/unload \
  -H "Content-Type: application/json" \
  -d '{"library_path": "./my-plugin.dylib"}'
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
