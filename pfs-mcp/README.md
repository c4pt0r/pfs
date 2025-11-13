# PFS MCP Server

Model Context Protocol (MCP) server for PFS (Plugin-based File System), enabling AI models to interact with PFS through standardized tools.

## Overview

PFS MCP Server exposes PFS file system operations as MCP tools, allowing AI assistants like Claude to read, write, and manage files in a PFS server through a standardized protocol.

## Features

- **File Operations**: Read, write, create, delete, copy, move files
- **Directory Operations**: List contents, create, remove, copy directories
- **Transfer Operations**: Upload from local filesystem to PFS, download from PFS to local filesystem
- **Search**: Grep with regex pattern matching
- **Plugin Management**: Mount/unmount plugins, list mounts
- **Health Monitoring**: Check server status
- **Notifications**: Send messages via QueueFS

## Installation

### Using uv (recommended)

```bash
# Install from local directory
uv pip install -e .

# Or if installing as dependency
uv pip install pfs-mcp
```

### Using pip

```bash
pip install -e .
```

## Usage

### Starting the Server

The MCP server runs as a stdio server that communicates via JSON-RPC:

```bash
# Using default PFS server (http://localhost:8080)
pfs-mcp

# Using custom PFS server URL
PFS_SERVER_URL=http://myserver:8080 pfs-mcp
```

### Configuration with Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "pfs": {
      "command": "pfs-mcp",
      "env": {
        "PFS_SERVER_URL": "http://localhost:8080"
      }
    }
  }
}
```

Or if using uv:

```json
{
  "mcpServers": {
    "pfs": {
      "command": "uvx",
      "args": ["--from", "/path/to/pfs-mcp", "pfs-mcp"],
      "env": {
        "PFS_SERVER_URL": "http://localhost:8080"
      }
    }
  }
}
```

### Available Tools

Once configured, the following tools are available to AI assistants:

#### File Operations

- `pfs_cat` - Read file content
  ```
  path: File path to read
  offset: Starting offset (optional, default: 0)
  size: Bytes to read (optional, default: -1 for all)
  ```

- `pfs_write` - Write content to file
  ```
  path: File path to write
  content: Content to write
  ```

- `pfs_rm` - Remove file or directory
  ```
  path: Path to remove
  recursive: Remove recursively (optional, default: false)
  ```

- `pfs_stat` - Get file/directory information
  ```
  path: Path to get info about
  ```

- `pfs_mv` - Move or rename file/directory
  ```
  old_path: Source path
  new_path: Destination path
  ```

- `pfs_cp` - Copy file or directory within PFS
  ```
  src: Source path in PFS
  dst: Destination path in PFS
  recursive: Copy directories recursively (optional, default: false)
  stream: Use streaming for large files (optional, default: false)
  ```

- `pfs_upload` - Upload file or directory from local filesystem to PFS
  ```
  local_path: Path to local file or directory
  remote_path: Destination path in PFS
  recursive: Upload directories recursively (optional, default: false)
  stream: Use streaming for large files (optional, default: false)
  ```

- `pfs_download` - Download file or directory from PFS to local filesystem
  ```
  remote_path: Path in PFS
  local_path: Destination path on local filesystem
  recursive: Download directories recursively (optional, default: false)
  stream: Use streaming for large files (optional, default: false)
  ```

#### Directory Operations

- `pfs_ls` - List directory contents
  ```
  path: Directory path (optional, default: /)
  ```

- `pfs_mkdir` - Create directory
  ```
  path: Directory path to create
  mode: Permissions mode (optional, default: 755)
  ```

#### Search Operations

- `pfs_grep` - Search for pattern in files
  ```
  path: Path to search in
  pattern: Regular expression pattern
  recursive: Search recursively (optional, default: false)
  case_insensitive: Case-insensitive search (optional, default: false)
  ```

#### Plugin Management

- `pfs_mounts` - List all mounted plugins

- `pfs_mount` - Mount a plugin
  ```
  fstype: Filesystem type (e.g., 'sqlfs', 'memfs', 's3fs')
  path: Mount path
  config: Plugin configuration (optional)
  ```

- `pfs_unmount` - Unmount a plugin
  ```
  path: Mount path to unmount
  ```

#### Health Check

- `pfs_health` - Check PFS server health status

#### Notification (QueueFS)

- `pfs_notify` - Send notification message via QueueFS
  ```
  queuefs_root: Root path of QueueFS (optional, default: /queuefs)
  to: Target queue name (receiver)
  from: Source queue name (sender)
  data: Message data to send
  ```
  Automatically creates sender and receiver queues if they don't exist.

## Example Usage with AI

Once configured, you can ask Claude (or other MCP-compatible AI assistants) to perform operations like:

- "List all files in the /data directory on PFS"
- "Read the contents of /config/settings.json from PFS"
- "Create a new directory called /logs/2024 in PFS"
- "Copy /data/file.txt to /backup/file.txt in PFS"
- "Upload my local file /tmp/report.pdf to /documents/report.pdf in PFS"
- "Download /logs/app.log from PFS to my local /tmp/app.log"
- "Copy the entire /data directory to /backup/data recursively in PFS"
- "Search for 'error' in all files under /logs recursively"
- "Show me all mounted plugins in PFS"
- "Mount a new memfs plugin at /tmp/cache"
- "Send a notification from 'service-a' to 'service-b' with message 'task completed'"

The AI will use the appropriate MCP tools to interact with your PFS server.

## Environment Variables

- `PFS_SERVER_URL`: PFS server URL (default: `http://localhost:8080`)

## Requirements

- Python >= 3.10
- PFS Server running and accessible
- pypfs SDK
- mcp >= 0.9.0

## Development

### Setup

```bash
# Clone and install in development mode
git clone <repo>
cd pfs-mcp
uv pip install -e .
```

### Testing

Start a PFS server first, then:

```bash
# Test the MCP server manually
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | pfs-mcp
```

## Architecture

```
┌─────────────────┐
│   AI Assistant  │
│   (e.g. Claude) │
└────────┬────────┘
         │ MCP Protocol (JSON-RPC over stdio)
         │
┌────────▼────────┐
│  PFS MCP Server │
│   (pfs-mcp)     │
└────────┬────────┘
         │ HTTP API
         │
┌────────▼────────┐
│   PFS Server    │
│  (Plugin-based  │
│  File System)   │
└─────────────────┘
```

## License

See LICENSE file for details.

## Related Projects

- [PFS](https://github.com/c4pt0r/pfs) - Plugin-based File System
- [pypfs](../pfs-sdk/python) - PFS Python SDK
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP Specification
