# pypfs - PFS Python SDK

Python SDK for interacting with PFS (Plugin-based File System) Server API.

## Installation

```bash
pip install pypfs
```

For local development:

```bash
pip install -e .
```

## Quick Start

```python
from pypfs import PFSClient

# Initialize client
client = PFSClient("http://localhost:8080")

# Check server health
health = client.health()
print(health)

# List directory contents
files = client.ls("/")
for file in files:
    print(f"{file['name']} - {'dir' if file['isDir'] else 'file'}")

# Read file content
content = client.cat("/path/to/file.txt")
print(content.decode())

# Write to file
client.write("/path/to/file.txt", b"Hello, PFS!")

# Create directory
client.mkdir("/new/directory")

# Remove file or directory
client.rm("/path/to/file.txt")
client.rm("/path/to/directory", recursive=True)
```

## Advanced Usage

### Streaming Operations

```python
# Stream file content
response = client.cat("/large/file.log", stream=True)
for chunk in response.iter_content(chunk_size=8192):
    process(chunk)

# Stream grep results
for match in client.grep("/logs", "error", recursive=True, stream=True):
    if match.get('type') == 'summary':
        print(f"Total matches: {match['count']}")
    else:
        print(f"{match['file']}:{match['line']}: {match['content']}")
```

### Mount Management

```python
# List mounted plugins
mounts = client.mounts()

# Mount a plugin
client.mount("memfs", "/test/mem", {})
client.mount("sqlfs", "/test/db", {
    "backend": "sqlite",
    "db_path": "/tmp/test.db"
})

# Unmount a plugin
client.unmount("/test/mem")
```

### Plugin Management

```python
# Load external plugin
result = client.load_plugin("./plugins/myplugin.so")
print(result)

# List loaded plugins
plugins = client.list_plugins()
print(plugins)

# Unload plugin
client.unload_plugin("./plugins/myplugin.so")
```

### Search with Grep

```python
# Simple search
result = client.grep("/local/logs", "error")
print(f"Found {result['count']} matches")
for match in result['matches']:
    print(f"{match['file']}:{match['line']}: {match['content']}")

# Recursive case-insensitive search
result = client.grep("/local", "warning|error", recursive=True, case_insensitive=True)
```

### File Operations

```python
# Get file info
info = client.stat("/path/to/file.txt")
print(f"Size: {info['size']}, Mode: {info['mode']}")

# Move/rename file
client.mv("/old/path.txt", "/new/path.txt")

# Change permissions
client.chmod("/path/to/file.txt", 0o644)

# Copy file (read + write)
content = client.cat("/source.txt")
client.write("/destination.txt", content)
```

## Error Handling

```python
from pypfs import PFSClient, PFSClientError

try:
    client = PFSClient("http://localhost:8080")
    content = client.cat("/nonexistent/file.txt")
except PFSClientError as e:
    print(f"Error: {e}")
```

## API Reference

### PFSClient

#### Constructor
- `PFSClient(api_base_url, timeout=10)` - Initialize client with API base URL

#### File Operations
- `ls(path="/")` - List directory contents
- `cat(path, offset=0, size=-1, stream=False)` - Read file content
- `write(path, data)` - Write data to file
- `create(path)` - Create new empty file
- `rm(path, recursive=False)` - Remove file or directory
- `stat(path)` - Get file/directory information
- `mv(old_path, new_path)` - Move/rename file or directory
- `chmod(path, mode)` - Change file permissions

#### Directory Operations
- `mkdir(path, mode="755")` - Create directory

#### Search Operations
- `grep(path, pattern, recursive=False, case_insensitive=False, stream=False)` - Search for pattern in files

#### Mount Operations
- `mounts()` - List all mounted plugins
- `mount(fstype, path, config)` - Mount a plugin dynamically
- `unmount(path)` - Unmount a plugin

#### Plugin Operations
- `list_plugins()` - List all loaded external plugins
- `load_plugin(library_path)` - Load an external plugin
- `unload_plugin(library_path)` - Unload an external plugin

#### Health Check
- `health()` - Check server health

## Development

### Running Tests

```bash
pip install -e ".[dev]"
pytest
```

### Code Formatting

```bash
black pypfs/
ruff check pypfs/
```

## License

See LICENSE file for details.
