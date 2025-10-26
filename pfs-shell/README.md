# PFS Shell

Interactive command-line client for PFS (Plugin-based File System) Server

## Features

- **Interactive REPL**: Command-line interface with auto-completion and history
- **Unix-like Commands**: ls, cd, cat, mkdir, rm, mv, chmod, cp, etc.
- **File Streaming**: Stream file contents for video/audio playback and continuous data feeds
- **Upload/Download**: Transfer files between local filesystem and PFS
- **Directory Tree**: Visualize filesystem structure with tree command
- **File Monitoring**: Follow file changes with tailf, watch commands repeatedly
- **Dynamic Mounting**: Mount and unmount plugins at runtime
- **Redirection Support**: Use `>` and `>>` operators like Unix shell
- **Path Completion**: Tab completion for commands and file paths
- **Command History**: Persistent history across sessions

## Installation

```bash
cd pfs-shell
uv run pfs --help
```

## Quick Start

### Start the PFS Server

```bash
cd ../pfs-server
make
./build/pfs-server -c config.yaml
```

### Run PFS Shell

```bash
# Interactive shell (default server: http://localhost:8080/api/v1)
uv run pfs sh

# Connect to custom server
uv run pfs --pfs-api-baseurl http://localhost:9000/api/v1 sh

# Direct command execution (no REPL)
uv run pfs ls /
uv run pfs cat /path/to/file
uv run pfs mkdir /mydir
```

## Usage

### Interactive Shell

```bash
$ uv run pfs sh
Connected to pfs server at http://localhost:8080/api/v1
press 'help' or '?' for help

pfs:/> ls
drwxr-xr-x      512 2025-01-15 10:30:45 mnt/
drwxr-xr-x      512 2025-01-15 10:30:45 tmp/

pfs:/> cd mnt
pfs:/mnt> pwd
/mnt

pfs:/mnt> help
# Shows all available commands
```

### File System Commands

#### Directory Operations

```bash
# List directory (long format with permissions, size, timestamp)
pfs:/> ls
pfs:/> ls /mnt

# Change directory
pfs:/> cd /mnt
pfs:/mnt> cd queue
pfs:/mnt/queue> cd ..
pfs:/mnt> cd /

# Print working directory
pfs:/> pwd
/

# Display directory tree
pfs:/> tree
pfs:/> tree /mnt
pfs:/> tree -L 2 /mnt    # Limit depth to 2 levels

# Create directory
pfs:/> mkdir /mydir
pfs:/> mkdir /path/to/nested/dir
```

#### File Operations

```bash
# Read file content
pfs:/> cat /path/to/file.txt
pfs:/> cat --stream /mnt/streamfs/video    # Streaming mode for continuous data

# Display last N lines
pfs:/> tail /var/log/app.log
pfs:/> tail -n 20 /var/log/app.log

# Follow file changes (like tail -f)
pfs:/> tailf /var/log/app.log
pfs:/> tailf -n 50 /var/log/app.log

# Write content to file
pfs:/> write /tmp/test.txt "Hello World"
pfs:/> echo "Hello World" > /tmp/test.txt
pfs:/> echo "More content" >> /tmp/test.txt    # Append

# Create empty file
pfs:/> touch /tmp/newfile.txt

# Get file info
pfs:/> stat /path/to/file
  File: file.txt
  Type: File
  Size: 1024
  Mode: 644
  Modified: 2025-01-15T10:30:45Z

# Copy file/directory
pfs:/> cp /source.txt /dest.txt
pfs:/> cp -r /sourcedir /destdir

# Move/rename
pfs:/> mv /old.txt /new.txt
pfs:/> mv /file.txt /other/location/

# Remove file/directory
pfs:/> rm /tmp/test.txt
pfs:/> rm -r /mydir

# Change permissions
pfs:/> chmod 644 /path/to/file
pfs:/> chmod 755 /path/to/script.sh
```

#### File Transfer

```bash
# Upload local file to PFS
pfs:/> upload /local/file.txt /pfs/file.txt
pfs:/> upload -r /local/dir /pfs/dir    # Recursive directory upload

# Download from PFS to local
pfs:/> download /pfs/file.txt /local/file.txt
pfs:/> download -r /pfs/dir /local/dir    # Recursive directory download
```

### Redirection

```bash
# Write file content to another file
pfs:/> cat /source.txt > /dest.txt

# Append file content
pfs:/> cat /file1.txt >> /combined.txt
pfs:/> cat /file2.txt >> /combined.txt

# Echo to file
pfs:/> echo "Hello World" > /greeting.txt
pfs:/> echo "More text" >> /greeting.txt
```

### Plugin Management

```bash
# List mounted plugins
pfs:/> mounts
memfs on /mnt/mem (plugin: memfs)
queuefs on /mnt/queue (plugin: queuefs)
kvfs on /mnt/kv (plugin: kvfs)
sqlfs on /mnt/sqlfs (plugin: sqlfs, backend=sqlite, db_path=/tmp/pfs.db)
s3fs on /mnt/s3 (plugin: s3fs, bucket=my-bucket, region=us-west-1)

pfs:/> plugins    # Alias for mounts

# Mount a plugin dynamically
pfs:/> mount memfs /test/mem
pfs:/> mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db
pfs:/> mount s3fs /s3 bucket=my-bucket region=us-west-1 access_key_id=xxx secret_access_key=***

# Unmount a plugin
pfs:/> unmount /test/mem
```

### Queue Operations (via QueueFS)

```bash
# Enqueue a message
pfs:/> echo "My first message" > /mnt/queue/enqueue
pfs:/> echo "Another message" > /mnt/queue/enqueue

# Dequeue a message
pfs:/> cat /mnt/queue/dequeue
{"id":"1736936445000000000","data":"My first message","timestamp":"2025-01-15T10:30:45Z"}

# Peek at next message (without removing)
pfs:/> cat /mnt/queue/peek

# Check queue size
pfs:/> cat /mnt/queue/size
2

# Clear the queue
pfs:/> echo "" > /mnt/queue/clear
```

### KV Store Operations (via KVFS)

```bash
# Set a key-value pair
pfs:/> echo "alice" > /mnt/kv/keys/username
pfs:/> echo "alice@example.com" > /mnt/kv/keys/email

# Get a value
pfs:/> cat /mnt/kv/keys/username
alice

# List all keys
pfs:/> ls /mnt/kv/keys
-rw-r--r--        5 2025-01-15 10:30:45 username
-rw-r--r--       18 2025-01-15 10:30:45 email
-rw-r--r--       12 2025-01-15 10:30:45 welcome

# Delete a key
pfs:/> rm /mnt/kv/keys/email
```

### Watch Command

Execute commands repeatedly at fixed intervals:

```bash
# Watch directory listing (refresh every 2 seconds)
pfs:/> watch ls /mnt/queue

# Watch with custom interval
pfs:/> watch -n 1 cat /mnt/queue/size
pfs:/> watch -n 0.5 stat /tmp/file.txt

# Watch server uptime
pfs:/> watch -n 5 cat /mnt/serverinfo/uptime
```

### Utility Commands

```bash
# Show help
pfs:/> help
pfs:/> ?

# Clear screen
pfs:/> clear

# Exit
pfs:/> exit
pfs:/> quit
```

## Direct Command Execution

Run commands directly without entering the REPL:

```bash
# File operations
uv run pfs ls /
uv run pfs tree /mnt
uv run pfs cat /path/to/file
uv run pfs write /path/to/file "content here"
uv run pfs mkdir /new/directory
uv run pfs rm /path/to/file
uv run pfs rm -r /directory

# File transfer
uv run pfs upload /local/file.txt /pfs/file.txt
uv run pfs upload -r /local/dir /pfs/dir
uv run pfs download /pfs/file.txt /local/file.txt
uv run pfs download -r /pfs/dir /local/dir

# Plugin management
uv run pfs mounts
uv run pfs mount memfs /test/mem
uv run pfs unmount /test/mem

# Streaming (must be used outside REPL)
cat video.mp4 | uv run pfs write --stream /mnt/streamfs/video
uv run pfs cat --stream /mnt/streamfs/video | ffplay -
ffmpeg -i input.mp4 -f mpegts - | uv run pfs write --stream /mnt/streamfs/live

# Tail and follow
uv run pfs tail /var/log/app.log
uv run pfs tail -n 50 /var/log/app.log
uv run pfs tailf /var/log/app.log

# With custom API URL
uv run pfs --pfs-api-baseurl http://remote:8080/api/v1 ls /
```

## Command Reference

### File System Operations

| Command | Description | Example |
|---------|-------------|---------|
| `ls [path]` | List directory in long format | `ls /mnt` |
| `tree [path] [-L depth]` | Display directory tree | `tree -L 2 /mnt` |
| `cd <path>` | Change directory | `cd /home` |
| `pwd` | Print working directory | `pwd` |
| `cat <file>` | Display file contents | `cat /tmp/file.txt` |
| `cat --stream <file>` | Stream file contents | `cat --stream /video` |
| `tail [-n N] <file>` | Display last N lines | `tail -n 20 /log` |
| `tailf [-n N] <file>` | Follow file changes | `tailf /log` |
| `write <file> <content>` | Write to file | `write /f.txt hello` |
| `write --stream <file>` | Stream write from stdin | `cat video \| pfs write --stream /f` |
| `echo <content> > <file>` | Write to file | `echo hello > /f.txt` |
| `echo <content> >> <file>` | Append to file | `echo more >> /f.txt` |
| `mkdir <dir>` | Create directory | `mkdir /mydir` |
| `rm [-r] <path>` | Remove file/directory | `rm -r /mydir` |
| `touch <file>` | Create empty file | `touch /new.txt` |
| `stat <path>` | Show file info | `stat /file.txt` |
| `cp [-r] <src> <dst>` | Copy file/directory | `cp -r /src /dst` |
| `mv <src> <dst>` | Move/rename | `mv old.txt new.txt` |
| `chmod <mode> <path>` | Change permissions | `chmod 644 file.txt` |

### File Transfer

| Command | Description | Example |
|---------|-------------|---------|
| `upload [-r] <local> <pfs>` | Upload to PFS | `upload -r /local /pfs` |
| `download [-r] <pfs> <local>` | Download from PFS | `download -r /pfs /local` |

### Plugin Management

| Command | Description | Example |
|---------|-------------|---------|
| `mounts` | List mounted plugins | `mounts` |
| `mount <type> <path> [k=v...]` | Mount plugin | `mount memfs /test` |
| `unmount <path>` | Unmount plugin | `unmount /test` |
| `plugins` | Alias for mounts | `plugins` |

### Monitoring

| Command | Description | Example |
|---------|-------------|---------|
| `watch [-n sec] <cmd> [args]` | Execute command repeatedly | `watch -n 1 ls /queue` |

### Utility Commands

| Command | Description |
|---------|-------------|
| `help`, `?` | Show help |
| `clear` | Clear screen |
| `exit`, `quit` | Exit REPL |

## Examples

### Example 1: File Operations

```bash
$ uv run pfs sh
Connected to pfs server at http://localhost:8080/api/v1

pfs:/> mkdir /demo
pfs:/> cd /demo
pfs:/demo> write hello.txt "Hello from PFS CLI!"
pfs:/demo> cat hello.txt
Hello from PFS CLI!
pfs:/demo> ls
-rw-r--r--       20 2025-01-15 10:30:45 hello.txt
pfs:/demo> stat hello.txt
  File: hello.txt
  Type: File
  Size: 20
  Mode: 644
  Modified: 2025-01-15T10:30:45Z
```

### Example 2: Queue Operations

```bash
pfs:/> echo "First task" > /mnt/queue/enqueue
pfs:/> echo "Second task" > /mnt/queue/enqueue
pfs:/> cat /mnt/queue/size
2
pfs:/> cat /mnt/queue/dequeue
{"id":"1736936445000000000","data":"First task","timestamp":"2025-01-15T10:30:45Z"}
pfs:/> cat /mnt/queue/size
1
```

### Example 3: KV Store Operations

```bash
pfs:/> echo "Alice" > /mnt/kv/keys/user:1:name
pfs:/> echo "alice@example.com" > /mnt/kv/keys/user:1:email
pfs:/> echo "Bob" > /mnt/kv/keys/user:2:name

pfs:/> ls /mnt/kv/keys
-rw-r--r--        5 2025-01-15 10:30:45 user:1:name
-rw-r--r--       18 2025-01-15 10:30:45 user:1:email
-rw-r--r--        3 2025-01-15 10:30:45 user:2:name
-rw-r--r--       12 2025-01-15 10:30:45 welcome

pfs:/> cat /mnt/kv/keys/user:1:name
Alice

pfs:/> rm /mnt/kv/keys/user:2:name
```

### Example 4: Streaming Video

```bash
# Upload video (outside REPL)
$ cat video.mp4 | uv run pfs write --stream /mnt/streamfs/video
Streaming to /mnt/streamfs/video
Reading binary data from stdin...
Progress: 6.40 MB sent
Progress: 12.80 MB sent
✓ Streaming complete: 15.32 MB in 240 chunks

# Play video from PFS
$ uv run pfs cat --stream /mnt/streamfs/video | ffplay -

# Live streaming with ffmpeg
$ ffmpeg -i input.mp4 -f mpegts - | uv run pfs write --stream /mnt/streamfs/live
```

### Example 5: Directory Tree

```bash
pfs:/> tree /mnt
/mnt
├── queue/
│   ├── enqueue [0B]
│   ├── dequeue [0B]
│   ├── peek [0B]
│   ├── size [0B]
│   └── clear [0B]
├── kv/
│   └── keys/
│       ├── username [5B]
│       ├── email [18B]
│       └── welcome [12B]
└── mem/
    ├── file1.txt [1.2K]
    └── file2.txt [3.4K]

3 directories, 10 files
```

### Example 6: Upload/Download Files

```bash
# Upload single file
pfs:/> upload /home/user/document.pdf /mnt/storage/document.pdf
Uploaded /home/user/document.pdf -> /mnt/storage/document.pdf (524288 bytes)

# Upload directory recursively
pfs:/> upload -r /home/user/photos /mnt/storage/photos
Created directory /mnt/storage/photos
  /home/user/photos/img1.jpg -> /mnt/storage/photos/img1.jpg (204800 bytes)
  /home/user/photos/img2.jpg -> /mnt/storage/photos/img2.jpg (307200 bytes)
Uploaded 2 files, 512000 bytes total

# Download file
pfs:/> download /mnt/storage/document.pdf /home/user/Downloads/document.pdf
Downloaded /mnt/storage/document.pdf -> /home/user/Downloads/document.pdf (524288 bytes)

# Download directory recursively
pfs:/> download -r /mnt/storage/photos /home/user/Downloads/photos
Created directory /home/user/Downloads/photos
  /mnt/storage/photos/img1.jpg -> /home/user/Downloads/photos/img1.jpg (204800 bytes)
  /mnt/storage/photos/img2.jpg -> /home/user/Downloads/photos/img2.jpg (307200 bytes)
Downloaded 2 files, 512000 bytes total
```

### Example 7: Watch Command

```bash
# Monitor queue size in real-time
pfs:/> watch -n 1 cat /mnt/queue/size
Every 1s: cat /mnt/queue/size    2025-01-15 10:30:45

2

# Watch directory changes
pfs:/> watch -n 2 ls /mnt/queue
Every 2s: ls /mnt/queue    2025-01-15 10:30:45

-rw-r--r--        0 2025-01-15 10:30:45 enqueue
-rw-r--r--        0 2025-01-15 10:30:45 dequeue
-rw-r--r--        0 2025-01-15 10:30:45 peek
-rw-r--r--        0 2025-01-15 10:30:45 size
-rw-r--r--        0 2025-01-15 10:30:45 clear
```

## Advanced Features

### Path Completion

Press `Tab` to auto-complete commands and file paths:

```bash
pfs:/> en<Tab>
pfs:/> echo    # Completes to 'echo'

pfs:/> cat /mnt/qu<Tab>
pfs:/> cat /mnt/queue/    # Completes path
```

### Command History

Use `↑` and `↓` arrow keys to navigate command history. History is saved to `~/.pfscli_history`.

### Redirection Operators

```bash
# Write (overwrite)
pfs:/> cat /source.txt > /dest.txt
pfs:/> echo "Hello" > /greeting.txt

# Append
pfs:/> cat /file1.txt >> /combined.txt
pfs:/> echo "More" >> /greeting.txt
```

### Streaming Mode

Streaming mode is designed for continuous data feeds (video, audio, logs):

```bash
# Write streaming data (outside REPL - uses stdin)
cat large_video.mp4 | uv run pfs write --stream /mnt/streamfs/video

# Read streaming data (outside REPL - outputs to stdout)
uv run pfs cat --stream /mnt/streamfs/video | ffplay -

# Live streaming
ffmpeg -i rtsp://camera.local/stream -f mpegts - | \
  uv run pfs write --stream /mnt/streamfs/live
```

**Note**: `--stream` flag should be used outside REPL for stdin/stdout piping.

### Using with Different Servers

```bash
# Default (localhost:8080)
uv run pfs sh

# Custom server
uv run pfs --pfs-api-baseurl http://localhost:9000/api/v1 sh

# Remote server
uv run pfs --pfs-api-baseurl https://pfs.example.com/api/v1 sh

# Using environment variable
export PFS_API_URL=http://localhost:8080/api/v1
uv run pfs sh

# Direct command with custom server
uv run pfs --pfs-api-baseurl http://remote:8080/api/v1 ls /
```

### Recursive Operations

Many commands support `-r` flag for recursive operations:

```bash
# Remove directory recursively
pfs:/> rm -r /mydir

# Copy directory recursively
pfs:/> cp -r /source /destination

# Upload directory recursively
pfs:/> upload -r /local/dir /pfs/dir

# Download directory recursively
pfs:/> download -r /pfs/dir /local/dir
```

## Development

### Project Structure

```
pfs-shell/
├── pfscli/
│   ├── __init__.py
│   ├── cli.py              # Main CLI entry point and command definitions
│   ├── commands.py         # REPL command handlers
│   ├── cli_commands.py     # Core command implementations
│   └── client.py           # PFS API client
├── pyproject.toml          # Project metadata and dependencies
└── README.md
```

### Running from Source

```bash
cd pfs-shell
uv run pfs sh
```

### Adding New Commands

1. Add implementation to `pfscli/cli_commands.py`:
```python
def cmd_mycommand(client, arg1, arg2):
    """My custom command implementation"""
    # Implementation here
    pass
```

2. Add REPL handler to `pfscli/commands.py`:
```python
def cmd_mycommand(self, args: List[str]) -> bool:
    """My custom command"""
    # Parse args, resolve paths
    path = self._resolve_path(args[0])
    try:
        cli_commands.cmd_mycommand(self.client, path)
    except Exception as e:
        console.print(self._format_error("mycommand", path, e))
    return True

# Register in __init__:
self.commands = {
    # ... existing commands
    "mycommand": self.cmd_mycommand,
}
```

3. Add CLI command to `pfscli/cli.py`:
```python
@main.command()
@click.argument("arg1")
@click.pass_context
def mycommand(ctx, arg1):
    """My custom command"""
    try:
        cli_commands.cmd_mycommand(ctx.obj["client"], arg1)
    except Exception as e:
        console.print(f"mycommand: {e}")
```

## Troubleshooting

### Connection Failed

```
Connection refused - server not running at localhost:8080
```

**Solution**: Start the PFS Server first:
```bash
cd ../pfs-server
make
./build/pfs-server -c config.yaml
```

Or check if you're using the correct API base URL:
```bash
# Correct format (with /api/v1)
uv run pfs --pfs-api-baseurl http://localhost:8080/api/v1 sh

# Incorrect format (missing /api/v1)
uv run pfs --pfs-api-baseurl http://localhost:8080 sh  # ❌ Will fail
```

### Command Not Found

```
Unknown command: xyz
Type 'help' for available commands
```

**Solution**: Use `help` to see available commands.

### Plugin Not Mounted

```
ls: /mnt/queue: No such file or directory
```

**Solution**: Check mounted plugins and mount if needed:
```bash
pfs:/> mounts
# If missing:
pfs:/> mount queuefs /mnt/queue
```

### Permission Denied

```
cat: /path/to/file: Permission denied
```

**Solution**: Check file permissions and adjust with chmod:
```bash
pfs:/> stat /path/to/file
pfs:/> chmod 644 /path/to/file
```

### Streaming Not Working in REPL

```
Note: --stream is not available in REPL mode
```

**Solution**: Use streaming commands outside REPL:
```bash
# Correct (outside REPL)
cat video.mp4 | uv run pfs write --stream /mnt/streamfs/video

# Incorrect (inside REPL)
pfs:/> write --stream /file    # ❌ Won't work in REPL
```

## License

Apache 2.0
