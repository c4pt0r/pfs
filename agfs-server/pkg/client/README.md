# AGFS Go Client

A Go client library for interacting with AGFS (Plugin-based File System) HTTP API.

## Overview

This package provides a complete Go client implementation for the AGFS HTTP API, enabling programmatic access to all file system operations exposed by a AGFS server.

## Features

- **Full API Coverage**: Support for all AGFS file system operations
- **Type-Safe**: Strongly-typed Go interfaces
- **Error Handling**: Comprehensive error reporting with HTTP status codes
- **Configurable**: Customizable HTTP client and timeouts
- **Easy to Use**: Simple, intuitive API

## Installation

```go
import "github.com/c4pt0r/agfs/agfs-server/pkg/client"
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/c4pt0r/agfs/agfs-server/pkg/client"
)

func main() {
    // Create a new client
    // Note: baseURL should include the API version path
    c := client.NewClient("http://localhost:8080/api/v1")

    // Create a file
    err := c.Create("/test.txt")
    if err != nil {
        panic(err)
    }

    // Write data to file
    _, err = c.Write("/test.txt", []byte("Hello, AGFS!"))
    if err != nil {
        panic(err)
    }

    // Read file content
    data, err := c.Read("/test.txt")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data)) // Output: Hello, AGFS!

    // List directory
    files, err := c.ReadDir("/")
    if err != nil {
        panic(err)
    }
    for _, f := range files {
        fmt.Printf("%s (%d bytes)\n", f.Name, f.Size)
    }
}
```

## API Reference

### Client Creation

```go
// Create client with default HTTP client (30s timeout)
client := client.NewClient("http://localhost:8080/api/v1")

// Create client with custom HTTP client
httpClient := &http.Client{Timeout: 60 * time.Second}
client := client.NewClientWithHTTPClient("http://localhost:8080/api/v1", httpClient)
```

**Important**: The `baseURL` parameter should include the API version path (e.g., `/api/v1`).

### File Operations

#### Create
```go
err := client.Create("/path/to/file")
```

#### Read
```go
data, err := client.Read("/path/to/file")
```

#### Write
```go
response, err := client.Write("/path/to/file", []byte("content"))
```

#### Remove
```go
// Remove single file or empty directory
err := client.Remove("/path/to/file")

// Remove recursively
err := client.RemoveAll("/path/to/directory")
```

### Directory Operations

#### Create Directory
```go
err := client.Mkdir("/path/to/dir", 0755)
```

#### List Directory
```go
files, err := client.ReadDir("/path/to/dir")
for _, file := range files {
    fmt.Printf("%s - IsDir: %v, Size: %d\n", file.Name, file.IsDir, file.Size)
}
```

### File Information

#### Stat
```go
info, err := client.Stat("/path/to/file")
fmt.Printf("Name: %s\n", info.Name)
fmt.Printf("Size: %d\n", info.Size)
fmt.Printf("Mode: %o\n", info.Mode)
fmt.Printf("IsDir: %v\n", info.IsDir)
fmt.Printf("ModTime: %s\n", info.ModTime)
```

### File System Operations

#### Rename/Move
```go
err := client.Rename("/old/path", "/new/path")
```

#### Change Permissions
```go
err := client.Chmod("/path/to/file", 0644)
```

### Health Check

```go
err := client.Health()
if err != nil {
    fmt.Println("Server is unhealthy:", err)
} else {
    fmt.Println("Server is healthy")
}
```

## Data Types

### FileInfo

```go
type FileInfo struct {
    Name    string
    Size    int64
    Mode    uint32
    ModTime time.Time
    IsDir   bool
    Meta    map[string]string
}
```

## Error Handling

All client methods return errors that include HTTP status codes and server error messages:

```go
data, err := client.Read("/nonexistent")
if err != nil {
    // Error format: "HTTP 404: file not found"
    fmt.Println(err)
}
```

## Advanced Usage

### Custom HTTP Client

For custom timeout, transport, or other HTTP client configurations:

```go
httpClient := &http.Client{
    Timeout: 2 * time.Minute,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}

client := client.NewClientWithHTTPClient("http://localhost:8080/api/v1", httpClient)
```

### Working with Plugins

The client works seamlessly with all AGFS plugins:

```go
client := client.NewClient("http://localhost:8080/api/v1")

// Queue plugin
_, err := client.Write("/mnt/queue/enqueue", []byte("task-123"))

// KV store plugin
_, err = client.Write("/mnt/kv/keys/username", []byte("alice"))
data, err := client.Read("/mnt/kv/keys/username")

// Memory filesystem
err = client.Mkdir("/mnt/memfs/memfs", 0755)
```

## Testing

Run the test suite:

```bash
go test ./pkg/client -v
```

## Examples

See `client_test.go` for comprehensive usage examples.

## License

Apache License 2.0
