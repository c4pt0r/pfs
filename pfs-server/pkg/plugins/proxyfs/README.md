# ProxyFS Plugin

A PFS plugin that transparently proxies all file system operations to a remote PFS HTTP API server.

## Overview

ProxyFS enables PFS federation by allowing one PFS instance to mount and access file systems from another remote PFS server. All file operations are forwarded over HTTP to the remote server, making it possible to build distributed file system architectures.

## Features

- **Transparent Proxying**: All file system operations forwarded to remote PFS server
- **Full API Compatibility**: Supports all PFS file system operations
- **Health Checking**: Automatic connection validation on initialization
- **Hot Reload**: Reload proxy connection without restarting server
- **Configurable**: Remote server URL configurable via plugin config
- **Federation**: Build distributed PFS architectures

## Installation

The ProxyFS plugin is built into the PFS server. Simply import and mount it:

```go
import "github.com/c4pt0r/pfs-server/pkg/plugins/proxyfs"
```

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/c4pt0r/pfs-server/pkg/mountablefs"
    "github.com/c4pt0r/pfs-server/pkg/plugins/proxyfs"
)

func main() {
    // Create a mountable file system
    mfs := mountablefs.NewMountableFS()

    // Create and mount ProxyFS plugin
    plugin := proxyfs.NewProxyFSPlugin("http://remote-server:8080/api/v1")
    err := plugin.Initialize(nil)
    if err != nil {
        panic(err)
    }

    // Mount at /remote
    mfs.Mount("/remote", plugin)

    // Now all operations under /remote are forwarded to remote server
}
```

### With Configuration

```go
plugin := proxyfs.NewProxyFSPlugin("")

config := map[string]interface{}{
    "base_url": "http://remote-server:8080/api/v1",
}

err := plugin.Initialize(config)
if err != nil {
    panic(err)
}

mfs.Mount("/remote", plugin)
```

## Configuration

The plugin accepts the following configuration parameters:

| Parameter | Type   | Description                                    | Example                            |
|-----------|--------|------------------------------------------------|------------------------------------|
| base_url  | string | Full URL to remote PFS API including version  | `http://remote:8080/api/v1`       |

**Important**: The `base_url` must include the API version path (e.g., `/api/v1`).

## Usage Examples

Once mounted, the ProxyFS behaves like any other PFS plugin:

### Via pfs shell

```bash
# All operations are executed on the remote server
pfs:/> mkdir /remote/memfs
pfs:/> echo "hello" > /remote/memfs/file.txt
pfs:/> cat /remote/memfs/file.txt
hello
pfs:/> ls /remote/memfs
file.txt

# Hot reload the proxy connection
pfs:/> echo '' > /remote/reload
ProxyFS reloaded successfully
```

### Via API

```bash
# Create directory on remote server
curl -X POST "http://localhost:8080/api/v1/directories?path=/remote/memfs"

# Write file on remote server
curl -X PUT "http://localhost:8080/api/v1/files?path=/remote/memfs/file.txt" \
  -d "hello"

# Read file from remote server
curl "http://localhost:8080/api/v1/files?path=/remote/memfs/file.txt"
```

### Programmatic Access

```go
// Get the file system from plugin
fs := plugin.GetFileSystem()

// All operations are proxied to remote server
err := fs.Mkdir("/memfs", 0755)
_, err = fs.Write("/memfs/file.txt", []byte("content"))
data, err := fs.Read("/memfs/file.txt")
files, err := fs.ReadDir("/memfs")
```

## Architecture

```
┌─────────────────────────────────────┐
│     Local PFS Server (Port 8080)    │
│                                     │
│  ┌──────────────────────────────┐  │
│  │    MountableFS               │  │
│  │                              │  │
│  │  /remote → ProxyFS       │  │
│  │                 ↓            │  │
│  │              HTTP Client     │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
              ↓ HTTP
┌─────────────────────────────────────┐
│   Remote PFS Server (Port 9090)     │
│                                     │
│  ┌──────────────────────────────┐  │
│  │    HTTP API Handler          │  │
│  │           ↓                  │  │
│  │    Actual File System        │  │
│  │    (MemFS, QueueFS, etc.)    │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
```

## Hot Reload Feature

ProxyFS includes a special `/reload` virtual file that allows you to reload the connection to the remote server without restarting.

### When to Use Hot Reload

- Remote server was restarted
- Network connection was interrupted
- Connection pool needs refreshing
- Switching between backend servers

### Usage

```bash
# Via CLI
pfs:/> echo '' > /proxyfs/reload
ProxyFS reloaded successfully

# Via API
curl -X PUT "http://localhost:8080/api/v1/files?path=/proxyfs/reload" -d ""

# Check reload file info
pfs:/> stat /proxyfs/reload
File: reload
Type: File
Mode: 200  (write-only)
Meta.type: control
Meta.description: Write to this file to reload proxy connection
```

### How It Works

1. Writing to `/reload` triggers the reload mechanism
2. A new HTTP client is created with the same base URL
3. Health check is performed to verify the new connection
4. If successful, the old client is replaced
5. All subsequent requests use the new connection

### Reload Process

```go
// Internal reload process
func (pfs *ProxyFS) Reload() error {
    // Create new client
    pfs.client = client.NewClient(pfs.baseURL)

    // Test connection
    if err := pfs.client.Health(); err != nil {
        return fmt.Errorf("failed to connect after reload: %w", err)
    }

    return nil
}
```

## Use Cases

### 1. Remote File System Access

Access files on a remote PFS server as if they were local:

```go
// Mount remote server's file system
plugin := proxyfs.NewProxyFSPlugin("http://remote:8080/api/v1")
mfs.Mount("/remote", plugin)

// Access remote files locally
data, _ := mfs.Read("/remote/memfs/config.json")
```

### 2. PFS Federation

Build a federated PFS architecture with multiple remote servers:

```go
// Mount multiple remote servers
proxy1 := proxyfs.NewProxyFSPlugin("http://server1:8080/api/v1")
proxy2 := proxyfs.NewProxyFSPlugin("http://server2:8080/api/v1")
proxy3 := proxyfs.NewProxyFSPlugin("http://server3:8080/api/v1")

mfs.Mount("/region-us", proxy1)
mfs.Mount("/region-eu", proxy2)
mfs.Mount("/region-asia", proxy3)
```

### 3. Service Discovery

Access services from remote PFS instances:

```go
// Mount remote queue service
plugin := proxyfs.NewProxyFSPlugin("http://queue-server:8080/api/v1")
mfs.Mount("/remote-queue", plugin)

// Use remote queue
mfs.Write("/remote-queue/queue/enqueue", []byte("task-123"))
```

### 4. Cross-Data Center Access

Access file systems across different data centers:

```go
// DC1
dc1 := proxyfs.NewProxyFSPlugin("http://dc1.example.com/api/v1")
mfs.Mount("/dc1", dc1)

// DC2
dc2 := proxyfs.NewProxyFSPlugin("http://dc2.example.com/api/v1")
mfs.Mount("/dc2", dc2)
```

## Implementation Details

### HTTP Client

ProxyFS uses the PFS Go client library (`pkg/client`) internally, which provides:
- 30-second default timeout
- Automatic error handling
- Type-safe API calls
- Connection pooling

### Error Handling

Errors from the remote server are propagated to the caller with full context:

```go
data, err := fs.Read("/nonexistent")
// Error: HTTP 404: file not found
```

### Health Checking

On initialization, ProxyFS performs a health check to verify connectivity:

```go
err := plugin.Initialize(nil)
// Returns error if remote server is unreachable
```

## Supported Operations

ProxyFS supports all file system operations:

- ✅ Create
- ✅ Read
- ✅ Write
- ✅ Remove / RemoveAll
- ✅ Mkdir
- ✅ ReadDir
- ✅ Stat
- ✅ Rename
- ✅ Chmod
- ✅ Open (ReadCloser)
- ✅ OpenWrite (WriteCloser)

## Performance Considerations

### Network Latency
All operations incur network latency. For latency-sensitive applications, consider:
- Using local caching
- Batching operations
- Deploying ProxyFS servers closer to clients

### Connection Management
The underlying HTTP client uses connection pooling. For high-throughput scenarios:
- Adjust HTTP client transport settings
- Increase MaxIdleConns and MaxIdleConnsPerHost
- Configure appropriate timeouts

### Error Recovery
Network failures are surfaced as errors. Implement retry logic for critical operations:

```go
func readWithRetry(fs filesystem.FileSystem, path string, retries int) ([]byte, error) {
    var err error
    var data []byte
    for i := 0; i < retries; i++ {
        data, err = fs.Read(path)
        if err == nil {
            return data, nil
        }
        time.Sleep(time.Second * time.Duration(i+1))
    }
    return nil, err
}
```

## Testing

Run the test suite:

```bash
go test ./pkg/plugins/proxyfs -v
```

The tests use `httptest` to create mock PFS servers, ensuring reliable testing without external dependencies.

## Security Considerations

### Authentication
ProxyFS currently does not implement authentication. For production use:
- Use TLS/HTTPS for encrypted communication
- Implement authentication at the HTTP client level
- Use network-level security (VPN, private networks)

### Authorization
Authorization is handled by the remote PFS server. Ensure proper access controls are configured on the remote server.

### Network Security
- Use HTTPS in production: `https://remote-server:8443/api/v1`
- Implement mutual TLS for server authentication
- Use firewall rules to restrict access

## Limitations

1. **Synchronous Operations**: All operations are synchronous HTTP calls
2. **No Caching**: No local caching of remote data
3. **Network Dependent**: Requires stable network connectivity
4. **No Streaming**: Large files are loaded entirely into memory

## Future Enhancements

Potential improvements:

- [ ] Local caching for frequently accessed files
- [ ] Streaming support for large files
- [ ] Authentication/authorization support
- [ ] Connection pooling configuration
- [ ] Retry logic with exponential backoff
- [ ] Compression for network transfer
- [ ] Batch operations support

## Example: Complete Setup

```go
package main

import (
    "log"
    "net/http"

    "github.com/c4pt0r/pfs-server/pkg/handlers"
    "github.com/c4pt0r/pfs-server/pkg/mountablefs"
    "github.com/c4pt0r/pfs-server/pkg/plugins/proxyfs"
)

func main() {
    // Create local PFS
    mfs := mountablefs.NewMountableFS()

    // Mount remote PFS servers
    remote1 := proxyfs.NewProxyFSPlugin("http://remote1:8080/api/v1")
    if err := remote1.Initialize(nil); err != nil {
        log.Fatalf("Failed to initialize remote1: %v", err)
    }
    mfs.Mount("/remote1", remote1)

    remote2 := proxyfs.NewProxyFSPlugin("http://remote2:8080/api/v1")
    if err := remote2.Initialize(nil); err != nil {
        log.Fatalf("Failed to initialize remote2: %v", err)
    }
    mfs.Mount("/remote2", remote2)

    // Setup HTTP handlers
    handler := handlers.NewHandler(mfs)
    mux := http.NewServeMux()
    handler.SetupRoutes(mux)

    // Start server
    log.Println("Starting federated PFS server on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## License

Apache License 2.0
