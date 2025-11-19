# AGFS Shell

Interactive command-line client for AGFS (Plugin-based File System) Server

## Features

- **Interactive REPL**: Command-line interface with auto-completion and history
- **Unix-like Commands**: ls, cd, cat, mkdir, rm, mv, chmod, cp, etc.
- **Pipeline Support**: Chain commands together with `|` operator for data flow
- **Chained Redirection**: Chain multiple redirections for data transformation (e.g., `echo 'query' > file1 > file2`)
- **Heredoc Support**: Multi-line input with `<< EOF` syntax for writing complex content
- **Tee Command**: Split data flow to multiple destinations while passing through (`echo 'hello' | tee output`)
- **File Streaming**: Stream file contents for video/audio playback and continuous data feeds
- **Upload/Download**: Transfer files between local filesystem and AGFS
- **Directory Tree**: Visualize filesystem structure with tree command
- **File Monitoring**: Follow file changes with tailf, watch commands repeatedly
- **Dynamic Mounting**: Mount and unmount plugins at runtime
- **Redirection Support**: Use `>` and `>>` operators like Unix shell
- **Path Completion**: Tab completion for commands and file paths
- **Command History**: Persistent history across sessions

## Installation

```bash
cd agfs-shell
uv run agfs --help
```

## Quick Start

### Start the AGFS Server

```bash
cd ../agfs-server
make
./build/agfs-server -c config.yaml
``

```bash
# Interactive shell (default server: http://localhost:8080/api/v1)
uv run agfs sh

# Connect to custom server
uv run agfs --agfs-api-baseurl http://localhost:9000/api/v1 sh

# Direct command execution (no REPL)
uv run agfs ls /
uv run agfs cat /path/to/file
uv run agfs mkdir /mydir
```

## Usage

### Interactive Shell

```bash
$ uv run agfs sh
Connected to agfs server at http://localhost:8080/api/v1
press 'help' or '?' for help

agfs:/> ls
drwxr-xr-x      512 2025-01-15 10:30:45 mnt/
drwxr-xr-x      512 2025-01-15 10:30:45 tmp/

agfs:/> cd mnt
agfs:/mnt> pwd
/mnt

agfs:/mnt> help
# Shows all available commands
```

### File System Commands

#### Directory Operations

```bash
# List directory (long format with permissions, size, timestamp)
agfs:/> ls
agfs:/> ls /mnt

# Change directory
agfs:/> cd /mnt
agfs:/mnt> cd queue
agfs:/mnt/queue> cd ..
agfs:/mnt> cd /

# Print working directory
agfs:/> pwd
/

# Display directory tree
agfs:/> tree
agfs:/> tree /mnt
agfs:/> tree -L 2 /mnt    # Limit depth to 2 levels

# Create directory
agfs:/> mkdir /mydir
agfs:/> mkdir /path/to/nested/dir
```

#### File Operations

```bash
# Read file content
agfs:/> cat /path/to/file.txt
agfs:/> cat --stream /mnt/streamfs/video    # Streaming mode for continuous data

# Display last N lines
agfs:/> tail /var/log/app.log
agfs:/> tail -n 20 /var/log/app.log

# Follow file changes (like tail -f)
agfs:/> tailf /var/log/app.log
agfs:/> tailf -n 50 /var/log/app.log

# Write content to file
agfs:/> write /tmp/test.txt "Hello World"
agfs:/> echo "Hello World" > /tmp/test.txt
agfs:/> echo "More content" >> /tmp/test.txt    # Append

# Create empty file
agfs:/> touch /tmp/newfile.txt

# Get file info
agfs:/> stat /path/to/file
  File: file.txt
  Type: File
  Size: 1024
  Mode: 644
  Modified: 2025-01-15T10:30:45Z

# Copy file/directory
agfs:/> cp /source.txt /dest.txt
agfs:/> cp -r /sourcedir /destdir

# Move/rename
agfs:/> mv /old.txt /new.txt
agfs:/> mv /file.txt /other/location/

# Remove file/directory
agfs:/> rm /tmp/test.txt
agfs:/> rm -r /mydir

# Change permissions
agfs:/> chmod 644 /path/to/file
agfs:/> chmod 755 /path/to/script.sh
```

#### File Transfer

```bash
# Upload local file to AGFS
agfs:/> upload /local/file.txt /agfs/file.txt
agfs:/> upload -r /local/dir /agfs/dir    # Recursive directory upload

# Download from AGFS to local
agfs:/> download /agfs/file.txt /local/file.txt
agfs:/> download -r /agfs/dir /local/dir    # Recursive directory download
```

### Redirection

```bash
# Write file content to another file
agfs:/> cat /source.txt > /dest.txt

# Append file content
agfs:/> cat /file1.txt >> /combined.txt
agfs:/> cat /file2.txt >> /combined.txt

# Echo to file
agfs:/> echo "Hello World" > /greeting.txt
agfs:/> echo "More text" >> /greeting.txt
```

### Heredoc (Multi-line Input)

Use heredoc syntax to write multi-line content to files (bash-compatible syntax):

```bash
# Write multi-line JSON using heredoc
agfs:/> cat << EOF > /sqlfs2/mydb/users/insert_json
> {"id": 1, "name": "Alice", "email": "alice@example.com"}
> {"id": 2, "name": "Bob", "email": "bob@example.com"}
> {"id": 3, "name": "Carol", "email": "carol@example.com"}
> EOF

# Append multi-line content
agfs:/> cat << END >> /data/config.json
> {
>   "timeout": 30,
>   "retries": 3
> }
> END

# Write script content
agfs:/> cat << SCRIPT > /scripts/process.sh
> #!/bin/bash
> echo "Processing data..."
> cat input.txt | grep "pattern" > output.txt
> SCRIPT
```

**How Heredoc Works:**
1. Type the command with `<< DELIMITER` followed by `>` or `>>` and the target file
2. Enter your multi-line content **line by line** (press Enter after each line)
3. Type the delimiter on a line by itself to finish
4. Content is automatically written to the specified file

**⚠️ IMPORTANT: Heredoc and Pasting**

Heredoc in REPL mode is designed for **interactive line-by-line input**, not for pasting multi-line content.

**Why pasting doesn't work:**
- When you paste multi-line heredoc, all lines (including the delimiter) are read as content
- This is a limitation of how REPL processes input buffer

**✅ Recommended alternatives for pasting:**

1. **Use `upload` command** (Best for files):
   ```bash
   # Create a local file with your content
   $ cat > /tmp/data.ndjson
   {"id":1,"name":"test1"}
   {"id":2,"name":"test2"}

   # Upload it
   agfs:/> upload /tmp/data.ndjson /sqlfs2/mydb/users/insert_json
   ```

2. **Use `echo` with multi-line string** (For small content):
   ```bash
   agfs:/> echo '{"id":1,"name":"test1"}
   {"id":2,"name":"test2"}' > /sqlfs2/mydb/users/insert_json
   ```

3. **Type heredoc line by line** (Don't paste):
   ```bash
   agfs:/> cat << EOF > /sqlfs2/mydb/users/insert_json
   > {"id":1,"name":"test1"}    # Type this line manually
   > {"id":2,"name":"test2"}    # Type this line manually
   > EOF                        # Type this line manually
   ```

**Supported Features:**
- Works with both `>` (write) and `>>` (append) operators
- Any alphanumeric delimiter (e.g., `EOF`, `END`, `DELIMITER`)
- Press Ctrl+C to cancel heredoc input
- Automatically handles NDJSON format for streaming data
- Bash-compatible syntax: `cat << EOF > file`

### Pipelines and Chained Redirections

AGFS Shell supports Unix-like pipelines and powerful chained redirections for data transformation workflows.

#### Pipelines

Chain multiple commands together using the `|` operator:

```bash
# Basic pipeline
agfs:/> cat /data.txt | tee /backup.txt

# Multiple stages
agfs:/> cat /log.txt | tee /log-backup.txt | tee /log-archive.txt

# Tee with append mode
agfs:/> echo "new entry" | tee -a /file1.txt /file2.txt /file3.txt
```

#### Chained Redirections

Chain multiple redirections to transform data through multiple AGFS files. This is especially powerful with dynamic AGFS filesystems that transform data on write:

```bash
# Example: Query database and save to S3
agfs:/sqlfs/mydb> echo 'select * from feeds limit 100' > query > /s3fs/aws/backup.json
```

**How Chained Redirection Works:**
1. Initial command generates content (e.g., `echo 'select * from users'`)
2. Content is written to first file (e.g., `/sqlfs/query`)
3. If the write returns a response (e.g., query results), that response becomes input for the next redirection
4. The process continues through all redirections in the chain
5. Final result is silently saved (no console output)

**Use Cases:**
- Query SQL databases and save results to S3/file storage
- Transform data through multiple processing stages
- Chain API calls where each response feeds the next request
- Multi-step data pipelines within the filesystem

#### Tee Command

Split data to multiple destinations while passing it through:

```bash
# Write to one file and continue in pipeline
agfs:/> echo "important data" | tee /backup.txt

# Write to multiple files
agfs:/> cat /source.txt | tee /dest1.txt /dest2.txt /dest3.txt

# Append mode with -a flag
agfs:/> echo "log entry" | tee -a /logs/app.log /logs/all.log
```

### Plugin Management

```bash
# List mounted plugins
agfs:/> mounts
memfs on /mnt/mem (plugin: memfs)
queuefs on /mnt/queue (plugin: queuefs)
kvfs on /mnt/kv (plugin: kvfs)
sqlfs on /mnt/sqlfs (plugin: sqlfs, backend=sqlite, db_path=/tmp/agfs.db)
s3fs on /mnt/s3 (plugin: s3fs, bucket=my-bucket, region=us-west-1)

agfs:/> plugins    # Alias for mounts

# Mount a plugin dynamically
agfs:/> mount memfs /test/mem
agfs:/> mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db
agfs:/> mount s3fs /s3 bucket=my-bucket region=us-west-1 access_key_id=xxx secret_access_key=***

# Unmount a plugin
agfs:/> unmount /test/mem
```

### Queue Operations (via QueueFS)

```bash
# Enqueue a message
agfs:/> echo "My first message" > /mnt/queue/enqueue
agfs:/> echo "Another message" > /mnt/queue/enqueue

# Dequeue a message
agfs:/> cat /mnt/queue/dequeue
{"id":"1736936445000000000","data":"My first message","timestamp":"2025-01-15T10:30:45Z"}

# Peek at next message (without removing)
agfs:/> cat /mnt/queue/peek

# Check queue size
agfs:/> cat /mnt/queue/size
2

# Clear the queue
agfs:/> echo "" > /mnt/queue/clear
```

### KV Store Operations (via KVFS)

```bash
# Set a key-value pair
agfs:/> echo "alice" > /mnt/kv/keys/username
agfs:/> echo "alice@example.com" > /mnt/kv/keys/email

# Get a value
agfs:/> cat /mnt/kv/keys/username
alice

# List all keys
agfs:/> ls /mnt/kv/keys
-rw-r--r--        5 2025-01-15 10:30:45 username
-rw-r--r--       18 2025-01-15 10:30:45 email
-rw-r--r--       12 2025-01-15 10:30:45 welcome

# Delete a key
agfs:/> rm /mnt/kv/keys/email
```

### Watch Command

Execute commands repeatedly at fixed intervals:

```bash
# Watch directory listing (refresh every 2 seconds)
agfs:/> watch ls /mnt/queue

# Watch with custom interval
agfs:/> watch -n 1 cat /mnt/queue/size
agfs:/> watch -n 0.5 stat /tmp/file.txt

# Watch server uptime
agfs:/> watch -n 5 cat /mnt/serverinfo/uptime
```

### Utility Commands

```bash
# Show help
agfs:/> help
agfs:/> ?

# Clear screen
agfs:/> clear

# Exit
agfs:/> exit
agfs:/> quit
```

## Direct Command Execution

Run commands directly without entering the REPL:

```bash
# File operations
uv run agfs ls /
uv run agfs tree /mnt
uv run agfs cat /path/to/file
uv run agfs write /path/to/file "content here"
uv run agfs mkdir /new/directory
uv run agfs rm /path/to/file
uv run agfs rm -r /directory

# File transfer
uv run agfs upload /local/file.txt /agfs/file.txt
uv run agfs upload -r /local/dir /agfs/dir
uv run agfs download /agfs/file.txt /local/file.txt
uv run agfs download -r /agfs/dir /local/dir

# Plugin management
uv run agfs mounts
uv run agfs mount memfs /test/mem
uv run agfs unmount /test/mem

# Streaming (must be used outside REPL)
cat video.mp4 | uv run agfs write --stream /mnt/streamfs/video
uv run agfs cat --stream /mnt/streamfs/video | ffplay -
ffmpeg -i input.mp4 -f mpegts - | uv run agfs write --stream /mnt/streamfs/live

# Tail and follow
uv run agfs tail /var/log/app.log
uv run agfs tail -n 50 /var/log/app.log
uv run agfs tailf /var/log/app.log

# With custom API URL
uv run agfs --agfs-api-baseurl http://remote:8080/api/v1 ls /
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
| `write --stream <file>` | Stream write from stdin | `cat video \| agfs write --stream /f` |
| `echo <content> > <file>` | Write to file | `echo hello > /f.txt` |
| `echo <content> >> <file>` | Append to file | `echo more >> /f.txt` |
| `tee [-a] <file> [files...]` | Write stdin to file(s) and stdout | `echo hi \| tee f1.txt f2.txt` |
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
| `upload [-r] <local> <agfs>` | Upload to AGFS | `upload -r /local /agfs` |
| `download [-r] <agfs> <local>` | Download from AGFS | `download -r /agfs /local` |

### Plugin Management

| Command | Description | Example |
|---------|-------------|---------|
| `mounts` | List mounted plugins | `mounts` |
| `mount <type> <path> [k=v...]` | Mount plugin | `mount memfs /test` |
| `unmount <path>` | Unmount plugin | `unmount /test` |
| `plugins` | Alias for mounts | `plugins` |

### Pipelines and Redirections

| Operator | Description | Example |
|---------|-------------|---------|
| `\|` | Pipe output to next command | `cat file.txt \| tee backup.txt` |
| `>` | Redirect output to file (overwrite) | `echo hello > file.txt` |
| `>>` | Redirect output to file (append) | `echo world >> file.txt` |
| `> file1 > file2` | Chained redirection | `echo 'query' > /sqlfs/q > /s3/result.json` |

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
$ uv run agfs sh
Connected to agfs server at http://localhost:8080/api/v1

agfs:/> mkdir /demo
agfs:/> cd /demo
agfs:/demo> write hello.txt "Hello from AGFS CLI!"
agfs:/demo> cat hello.txt
Hello from AGFS CLI!
agfs:/demo> ls
-rw-r--r--       20 2025-01-15 10:30:45 hello.txt
agfs:/demo> stat hello.txt
  File: hello.txt
  Type: File
  Size: 20
  Mode: 644
  Modified: 2025-01-15T10:30:45Z
```

### Example 2: Queue Operations

```bash
agfs:/> echo "First task" > /mnt/queue/enqueue
agfs:/> echo "Second task" > /mnt/queue/enqueue
agfs:/> cat /mnt/queue/size
2
agfs:/> cat /mnt/queue/dequeue
{"id":"1736936445000000000","data":"First task","timestamp":"2025-01-15T10:30:45Z"}
agfs:/> cat /mnt/queue/size
1
```

### Example 3: KV Store Operations

```bash
agfs:/> echo "Alice" > /mnt/kv/keys/user:1:name
agfs:/> echo "alice@example.com" > /mnt/kv/keys/user:1:email
agfs:/> echo "Bob" > /mnt/kv/keys/user:2:name

agfs:/> ls /mnt/kv/keys
-rw-r--r--        5 2025-01-15 10:30:45 user:1:name
-rw-r--r--       18 2025-01-15 10:30:45 user:1:email
-rw-r--r--        3 2025-01-15 10:30:45 user:2:name
-rw-r--r--       12 2025-01-15 10:30:45 welcome

agfs:/> cat /mnt/kv/keys/user:1:name
Alice

agfs:/> rm /mnt/kv/keys/user:2:name
```

### Example 4: Streaming Video

```bash
# Upload video (outside REPL)
$ cat video.mp4 | uv run agfs write --stream /mnt/streamfs/video
Streaming to /mnt/streamfs/video
Reading binary data from stdin...
Progress: 6.40 MB sent
Progress: 12.80 MB sent
✓ Streaming complete: 15.32 MB in 240 chunks

# Play video from AGFS
$ uv run agfs cat --stream /mnt/streamfs/video | ffplay -

# Live streaming with ffmpeg
$ ffmpeg -i input.mp4 -f mpegts - | uv run agfs write --stream /mnt/streamfs/live
```

### Example 5: Directory Tree

```bash
agfs:/> tree /mnt
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
agfs:/> upload /home/user/document.pdf /mnt/storage/document.pdf
Uploaded /home/user/document.pdf -> /mnt/storage/document.pdf (524288 bytes)

# Upload directory recursively
agfs:/> upload -r /home/user/photos /mnt/storage/photos
Created directory /mnt/storage/photos
  /home/user/photos/img1.jpg -> /mnt/storage/photos/img1.jpg (204800 bytes)
  /home/user/photos/img2.jpg -> /mnt/storage/photos/img2.jpg (307200 bytes)
Uploaded 2 files, 512000 bytes total

# Download file
agfs:/> download /mnt/storage/document.pdf /home/user/Downloads/document.pdf
Downloaded /mnt/storage/document.pdf -> /home/user/Downloads/document.pdf (524288 bytes)

# Download directory recursively
agfs:/> download -r /mnt/storage/photos /home/user/Downloads/photos
Created directory /home/user/Downloads/photos
  /mnt/storage/photos/img1.jpg -> /home/user/Downloads/photos/img1.jpg (204800 bytes)
  /mnt/storage/photos/img2.jpg -> /home/user/Downloads/photos/img2.jpg (307200 bytes)
Downloaded 2 files, 512000 bytes total
```

### Example 7: Watch Command

```bash
# Monitor queue size in real-time
agfs:/> watch -n 1 cat /mnt/queue/size
Every 1s: cat /mnt/queue/size    2025-01-15 10:30:45

2

# Watch directory changes
agfs:/> watch -n 2 ls /mnt/queue
Every 2s: ls /mnt/queue    2025-01-15 10:30:45

-rw-r--r--        0 2025-01-15 10:30:45 enqueue
-rw-r--r--        0 2025-01-15 10:30:45 dequeue
-rw-r--r--        0 2025-01-15 10:30:45 peek
-rw-r--r--        0 2025-01-15 10:30:45 size
-rw-r--r--        0 2025-01-15 10:30:45 clear
```

### Example 8: Pipeline with Tee

```bash
# Save data to multiple files while passing through
agfs:/> echo "Important log entry" | tee /logs/app.log /logs/backup.log
Important log entry

# Verify both files were written
agfs:/> cat /logs/app.log
Important log entry

agfs:/> cat /logs/backup.log
Important log entry

# Append to multiple files
agfs:/> echo "Another entry" | tee -a /logs/app.log /logs/backup.log /logs/archive.log
Another entry

# Chain tee commands
agfs:/> cat /data.json | tee /backup/$(date).json | tee /archive/data.json
{"status": "success", "count": 42}
```

### Example 9: Chained Redirections for Data Transformation

```bash
# Query SQL database and save results to S3 (single command!)
agfs:/sqlfs/mydb> echo 'select * from feeds limit 10' > query > /s3fs/aws/results.json

# The command silently completes. Verify the result:
agfs:/sqlfs/mydb> cat /s3fs/aws/results.json
[
  {
    "id": "41014298",
    "title": "A GIF animation loop, 2015, Duration 1k years",
    "URL": "https://www.aslongaspossible.com/",
    "created_at": "2024-07-19T22:27:09Z"
  },
  ...
]

# Chain multiple transformations
agfs:/> echo "raw data" > /transform/step1 > /transform/step2 > /final/output.txt

# Mix append and overwrite in chain
agfs:/> echo "new entry" >> /accumulator/data > /processor/analyze > /output/report.json
```

**How Example 9 Works:**
1. `echo 'select * from feeds limit 10'` generates the SQL query text
2. First `> query` writes the query to `/sqlfs/mydb/query` file
3. The sqlfs filesystem executes the query and returns JSON results
4. Second `> /s3fs/aws/results.json` writes those results to S3
5. No console output (silent execution)

This enables powerful data pipelines:
- Database → Cloud Storage
- API → Transformation → Storage
- Queue → Processing → Multiple Destinations

## Advanced Features

### Path Completion

Press `Tab` to auto-complete commands and file paths:

```bash
agfs:/> en<Tab>
agfs:/> echo    # Completes to 'echo'

agfs:/> cat /mnt/qu<Tab>
agfs:/> cat /mnt/queue/    # Completes path
```

### Command History

Use `↑` and `↓` arrow keys to navigate command history. History is saved to `~/.agfscli_history`.

### Redirection Operators and Pipelines

```bash
# Write (overwrite)
agfs:/> cat /source.txt > /dest.txt
agfs:/> echo "Hello" > /greeting.txt

# Append
agfs:/> cat /file1.txt >> /combined.txt
agfs:/> echo "More" >> /greeting.txt

# Chained redirections (data flows through multiple files)
agfs:/> echo 'select * from users' > /sqlfs/query > /s3fs/backup.json
agfs:/> echo "data" >> /step1 > /step2 >> /final.txt

# Pipelines with tee
agfs:/> echo "log entry" | tee /log1.txt /log2.txt
agfs:/> cat /data.txt | tee /backup.txt | tee /archive.txt

# Pipeline with append
agfs:/> echo "entry" | tee -a /log.txt /backup.log
```

### Streaming Mode

Streaming mode is designed for continuous data feeds (video, audio, logs):

```bash
# Write streaming data (outside REPL - uses stdin)
cat large_video.mp4 | uv run agfs write --stream /mnt/streamfs/video

# Read streaming data (outside REPL - outputs to stdout)
uv run agfs cat --stream /mnt/streamfs/video | ffplay -

# Live streaming
ffmpeg -i rtsp://camera.local/stream -f mpegts - | \
  uv run agfs write --stream /mnt/streamfs/live
```

**Note**: `--stream` flag should be used outside REPL for stdin/stdout piping.

### Using with Different Servers

```bash
# Default (localhost:8080)
uv run agfs sh

# Custom server
uv run agfs --agfs-api-baseurl http://localhost:9000/api/v1 sh

# Remote server
uv run agfs --agfs-api-baseurl https://agfs.example.com/api/v1 sh

# Using environment variable
export AGFS_API_URL=http://localhost:8080/api/v1
uv run agfs sh

# Direct command with custom server
uv run agfs --agfs-api-baseurl http://remote:8080/api/v1 ls /
```

### Recursive Operations

Many commands support `-r` flag for recursive operations:

```bash
# Remove directory recursively
agfs:/> rm -r /mydir

# Copy directory recursively
agfs:/> cp -r /source /destination

# Upload directory recursively
agfs:/> upload -r /local/dir /agfs/dir

# Download directory recursively
agfs:/> download -r /agfs/dir /local/dir
```

## Development

### Project Structure

```
agfs-shell/
├── agfscli/
│   ├── __init__.py
│   ├── cli.py              # Main CLI entry point and command definitions
│   ├── commands.py         # REPL command handlers
│   ├── cli_commands.py     # Core command implementations
│   └── client.py           # AGFS API client
├── pyproject.toml          # Project metadata and dependencies
└── README.md
```

### Running from Source

```bash
cd agfs-shell
uv run agfs sh
```

### Adding New Commands

1. Add implementation to `agfscli/cli_commands.py`:
```python
def cmd_mycommand(client, arg1, arg2):
    """My custom command implementation"""
    # Implementation here
    pass
```

2. Add REPL handler to `agfscli/commands.py`:
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

3. Add CLI command to `agfscli/cli.py`:
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

**Solution**: Start the AGFS Server first:
```bash
cd ../agfs-server
make
./build/agfs-server -c config.yaml
```

Or check if you're using the correct API base URL:
```bash
# Correct format (with /api/v1)
uv run agfs --agfs-api-baseurl http://localhost:8080/api/v1 sh

# Incorrect format (missing /api/v1)
uv run agfs --agfs-api-baseurl http://localhost:8080 sh  # ❌ Will fail
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
agfs:/> mounts
# If missing:
agfs:/> mount queuefs /mnt/queue
```

### Permission Denied

```
cat: /path/to/file: Permission denied
```

**Solution**: Check file permissions and adjust with chmod:
```bash
agfs:/> stat /path/to/file
agfs:/> chmod 644 /path/to/file
```

### Streaming Not Working in REPL

```
Note: --stream is not available in REPL mode
```

**Solution**: Use streaming commands outside REPL:
```bash
# Correct (outside REPL)
cat video.mp4 | uv run agfs write --stream /mnt/streamfs/video

# Incorrect (inside REPL)
agfs:/> write --stream /file    # ❌ Won't work in REPL
```

## License

Apache 2.0
