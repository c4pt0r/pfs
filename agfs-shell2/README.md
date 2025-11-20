# agfs-shell2

DO NOT USE IT NOW, UNDER CONSTRUCTION

Experimental agfs shell implementation with Unix-style pipeline support and **AGFS integration** in pure Python.

## Overview

agfs-shell2 is a simple shell that demonstrates Unix pipeline concepts while integrating with the AGFS (Aggregated File System) server. All file operations go through AGFS, allowing you to work with multiple backend filesystems (local, S3, SQL, etc.) through a unified interface.

## Features

- **Unix-style pipelines**: Chain commands with `|` operator
- **I/O Redirection**: Support for `<`, `>`, `>>`, `2>`, `2>>` operators
- **Heredoc**: Multi-line input with `<<` operator (variable expansion and literal modes)
- **Glob expansion**: Wildcard patterns (`*.txt`, `file?.dat`, `[abc]`, etc.)
- **Variables**: Shell variable assignment and expansion (`VAR=value`, `$VAR`, `${VAR}`)
- **Special variables**: `$?` for exit code of last command
- **Command substitution**: Capture command output with `$(command)` or backticks
- **Control flow**: if/then/elif/else/fi statements and for/in/do/done loops
- **Conditional testing**: `test` and `[ ]` commands for file, string, integer, and logical tests
- **Multiline input**: Backslash continuation, unclosed quotes, and bracket matching like bash
- **Directory navigation**: `cd` command with current working directory tracking
- **Relative paths**: Full support for `.`, `..`, and relative file paths
- **Tab completion**: Smart completion for commands and paths (both absolute and relative)
- **Command history**: Persistent command history with navigation (↑/↓ arrows)
- **AGFS Integration**: All file operations use AGFS server (no local filesystem access)
- **File transfer**: Upload/download files between local filesystem and AGFS
- **Streaming I/O**: Memory-efficient streaming for large files (8KB chunks)
- **Stream handling**: Full STDIN/STDOUT/STDERR support
- **Built-in commands**: 24 commands including file operations, text processing, JSON handling, and control flow
  - File ops: cd, pwd, ls, cat, mkdir, rm, stat, cp, upload, download
  - Text processing: echo, grep, jq, wc, head, tail, sort, uniq, tr
  - Variables: export, env, unset
  - Testing: test, [
  - Utilities: sleep
- **Interactive REPL**: Interactive shell mode with dynamic prompt showing current directory
- **Script execution**: Support for shebang scripts (`#!/usr/bin/env uv run agfs-shell2`)
- **Non-interactive mode**: Execute commands from command line with `-c` flag
- **Configurable server**: Support for custom AGFS server URL and timeout
- **Rich output**: Colorized and formatted output using Rich library

## Prerequisites

**AGFS Server must be running!**

Start the AGFS server before using agfs-shell2:

```bash
# Option 1: Run from source
cd agfs-server
go run main.go

# Option 2: Use Docker
docker run -p 8080:8080 c4pt0r/agfs-server:latest
```

## Installation

```bash
cd agfs-shell2
uv sync
```

## Architecture

The shell is built with several key components:

- **Streams** (`streams.py`): InputStream, OutputStream, ErrorStream classes
- **Process** (`process.py`): Represents a single command with stdin/stdout/stderr and filesystem access
- **Pipeline** (`pipeline.py`): Chains multiple processes together
- **Parser** (`parser.py`): Parses command strings into pipeline components
- **Builtins** (`builtins.py`): Implementation of built-in commands (uses AGFS for file I/O)
- **FileSystem** (`filesystem.py`): AGFS abstraction layer using pyagfs SDK
- **Shell** (`shell.py`): Main shell with REPL and AGFS integration
- **Config** (`config.py`): Configuration management for server URL

## Usage

### Configure Server (Optional)

By default, agfs-shell2 connects to `http://localhost:8080`. You can configure a different server:

```bash
# Via command line argument
uv run agfs-shell2 --agfs-api-url http://192.168.1.100:8080

# Via environment variable (AGFS_API_URL is preferred)
export AGFS_API_URL=http://192.168.1.100:8080
uv run agfs-shell2

# Backward compatibility with AGFS_SERVER_URL
export AGFS_SERVER_URL=http://192.168.1.100:8080
uv run agfs-shell2
```

### Interactive REPL Mode

```bash
uv run agfs-shell2
```

```
agfs-shell2 v0.1.0
Connected to AGFS server at http://localhost:8080
Type 'help' for help, Ctrl+D or 'exit' to quit

agfs:/> echo hello world | cat > /local/greeting.txt
agfs:/> cat /local/greeting.txt
hello world
```

If the server is not running, you'll see a warning:
```
⚠ Warning: Cannot connect to AGFS server at http://localhost:8080
  Make sure the server is running. File operations will fail.
```

### Non-Interactive Mode

#### Using `-c` flag (recommended)

```bash
# Execute a command string
uv run agfs-shell2 -c "echo 'hello world' > /local/test.txt"

# Read from AGFS
uv run agfs-shell2 -c "cat /local/test.txt"

# Use with shell pipes
echo "test data" | uv run agfs-shell2 -c "cat | grep test > /local/results.txt"

# Complex pipelines with AGFS
uv run agfs-shell2 -c "cat /local/input.txt | sort | uniq > /local/output.txt"

# Multiple commands in one script
uv run agfs-shell2 -c "ls / | grep local"
```

#### Using positional arguments (also works)

```bash
# Write to AGFS
uv run agfs-shell2 "echo 'hello world' > /local/test.txt"

# Read from AGFS
uv run agfs-shell2 "cat /local/test.txt"

# Without quotes (splits on spaces)
uv run agfs-shell2 echo hello world
```

## Interactive Features

agfs-shell2 provides a rich interactive experience with several productivity features:

### Command History

The shell automatically saves your command history across sessions:

- **History File**: Commands are stored in `~/.agfs_shell_history` (configurable via `HISTFILE` variable)
- **History Length**: Up to 1000 commands are saved
- **Automatic Loading**: History is loaded when the shell starts
- **Automatic Saving**: History is saved when you exit the shell

**Custom history file location**:
```bash
# Check current history file
agfs:/> env | grep HISTFILE
HISTFILE=/Users/user/.agfs_shell_history

# Change history file location during session
agfs:/> export HISTFILE=/tmp/my_history.txt
agfs:/> # All future commands will be saved to /tmp/my_history.txt

# Use project-specific history
agfs:/> export HISTFILE=~/projects/myapp/.agfs_history
agfs:/> # History now saved to project directory
```

**Navigation**:
- Press **↑** (Up Arrow) to navigate to previous commands
- Press **↓** (Down Arrow) to navigate to newer commands

**Example workflow**:
```bash
agfs:/> echo "test 1"
agfs:/> ls /local
agfs:/> pwd
agfs:/> # Press ↑ to get "pwd"
agfs:/> # Press ↑↑ to get "ls /local"
```

### Tab Completion

The shell supports intelligent tab completion:

- **Command Completion**: Press Tab to complete command names
- **Path Completion**: Press Tab to complete file and directory paths
- **AGFS Integration**: Tab completion works with AGFS filesystem

**Examples**:
```bash
agfs:/> ec<Tab>       # Completes to "echo"
agfs:/> cat /lo<Tab>  # Completes to "/local/"
agfs:/> ls /local/te<Tab>  # Completes to "/local/test.txt" (if it exists)
```

### Multiline Editing

The shell supports multiline commands:

- **Control Structures**: `if`, `for` automatically trigger multiline mode
- **Line Continuation**: Use `\` at end of line to continue
- **Here Documents**: Use `<<` for multiline input

**Keyboard Shortcuts** (via readline):
- **Ctrl-A**: Move to beginning of line
- **Ctrl-E**: Move to end of line
- **Ctrl-K**: Delete from cursor to end of line
- **Ctrl-U**: Delete from cursor to beginning of line
- **Ctrl-W**: Delete word before cursor
- **Ctrl-L**: Clear screen (if supported)
- **Ctrl-D**: Exit shell (when line is empty)
- **Ctrl-C**: Cancel current input

## Built-in Commands

### File System Commands (AGFS)
- **cd [path]** - Change current directory (supports relative paths: `.`, `..`, etc.)
- **pwd** - Print current working directory
- **ls [-l] [path]** - List directory contents with color highlighting
  - Directories shown in **blue**
  - `-l` for long format with permissions, size, and timestamp
  - Defaults to current directory
- **cat [file...]** - Concatenate and print files or stdin
- **mkdir path** - Create directory
- **rm [-r] path** - Remove file or directory
- **stat path** - Display file status and check if file exists
- **cp [-r] source dest** - Copy files between local filesystem and AGFS
  - Use `local:path` prefix for local filesystem paths
  - Supports recursive directory copy with `-r` flag
- **upload [-r] local_path agfs_path** - Upload files/directories from local to AGFS
- **download [-r] agfs_path local_path** - Download files/directories from AGFS to local

### Text Processing Commands
- **echo [args...]** - Print arguments to stdout
- **grep [OPTIONS] PATTERN [FILE...]** - Search for patterns in files or stdin
- **wc [-l] [-w] [-c]** - Count lines, words, and bytes
- **head [-n count]** - Output first N lines (default 10)
- **tail [-n count]** - Output last N lines (default 10)
- **sort [-r]** - Sort lines (use -r for reverse)
- **uniq** - Remove duplicate adjacent lines
- **tr set1 set2** - Translate characters

### Pattern Matching with grep

agfs-shell2 includes a powerful **grep** command for searching text:

```bash
# Basic search
echo 'hello world' | grep hello

# Search in files
grep 'error' /local/app.log

# Case-insensitive search (-i)
grep -i 'ERROR' /local/app.log

# Show line numbers (-n)
grep -n 'function' /local/code.py

# Count matches (-c)
grep -c 'TODO' /local/*.py

# Invert match (-v) - show non-matching lines
grep -v 'debug' /local/app.log

# Show only filenames (-l)
grep -l 'import' /local/*.py

# Multiple files (auto-shows filenames)
grep 'pattern' /local/file1.txt /local/file2.txt

# Regular expressions
grep '^error' /local/app.log          # Lines starting with 'error'
grep 'error$' /local/app.log          # Lines ending with 'error'
grep 'er.or' /local/app.log           # Any character between 'er' and 'or'

# Combine options
grep -in 'error' /local/app.log       # Ignore case + line numbers
grep -vc 'comment' /local/code.py     # Count non-matching lines
```

**Supported options:**
- `-i` - Ignore case distinctions
- `-v` - Invert match (select non-matching lines)
- `-n` - Print line numbers with output
- `-c` - Count matching lines
- `-l` - Print only names of files with matches
- `-h` - Suppress filename prefix (default for single file)
- `-H` - Print filename prefix (default for multiple files)

### Environment Variables
- **export [VAR=value ...]** - Set or display environment variables
- **env** - Display all environment variables
- **unset VAR [VAR ...]** - Unset environment variables

### Utility Commands

**sleep** - Pause execution for specified seconds (supports decimal values)

```bash
# Basic sleep
> sleep 1
> echo "Done"

# Sleep with decimal seconds
> sleep 0.5

# Use in loops for rate limiting
> for i in 1 2 3 4 5; do
    echo "Processing item $i"
    sleep 1
  done

# Delay between commands
> echo "Starting backup..." && sleep 2 && echo "Backup started"

# Rate-limited file processing
> for file in /local/data/*.txt; do
    echo "Processing $file"
    cat $file | grep "ERROR" >> /local/all_errors.txt
    sleep 0.1  # Small delay between files
  done

# Progress indicator with sleep
> for i in 1 2 3 4 5; do
    echo -n "."
    sleep 1
  done
> echo " Complete!"
```

### JSON Processing with jq

agfs-shell2 includes a built-in **jq** command for processing JSON data. This allows you to query, filter, and transform JSON files stored in AGFS.

```bash
# Basic JSON formatting
echo '{"name":"Alice","age":30}' | jq .

# Extract specific field
cat data.json | jq .name

# Array iteration
echo '{"users":["Alice","Bob"]}' | jq '.users[]'

# Nested field access
jq '.data.items[0].value' file.json

# Select from array
echo '[1,2,3,4,5]' | jq '.[]'

# Get object keys
echo '{"x":1,"y":2}' | jq 'keys'

# Filter array elements
cat users.json | jq '.[] | select(.active == true)'

# Transform and map
echo '[{"id":1},{"id":2}]' | jq '.[] | .id'
```

**Supported operations**:
- `.` - Identity (pretty-print)
- `.field` - Field access
- `.field.nested` - Nested access
- `.[index]` - Array indexing
- `.[]` - Array iteration
- `keys` - Get object keys
- `length` - Get array/object length
- `select()` - Filter elements
- And most standard jq operations

**Requirements**: The `jq` library must be installed:
```bash
uv pip install jq
```

### Conditional Testing
- **test EXPRESSION** - Evaluate conditional expressions
- **[ EXPRESSION ]** - Alternative syntax for test command

## Advanced Shell Features

### Heredoc (Here Documents)

Heredoc allows you to write multi-line text directly in the shell, useful for creating files with multiple lines or passing multi-line input to commands.

**Variable-expanding heredoc** (variables like `$VAR` are expanded):
```bash
# Create a config file with variable expansion
> export APP_NAME="MyApp"
> export VERSION="1.0.0"
> cat << EOF > /local/config.txt
Application: $APP_NAME
Version: $VERSION
Created: $(date)
EOF

> cat /local/config.txt
Application: MyApp
Version: 1.0.0
Created: Wed Nov 20 12:00:00 2024
```

**Literal heredoc** (no variable expansion, use quoted delimiter):
```bash
# Create a script with literal $ signs
> cat << 'EOF' > /local/script.sh
#!/bin/bash
echo "The price is $100"
VAR="literal"
echo $VAR
EOF

> cat /local/script.sh
#!/bin/bash
echo "The price is $100"
VAR="literal"
echo $VAR
```

**Heredoc with commands**:
```bash
# Create JSON file with heredoc
> cat << EOF | jq . > /local/data.json
{
  "name": "test",
  "items": [1, 2, 3]
}
EOF

# Multi-line SQL query
> cat << EOF > /local/query.sql
SELECT *
FROM users
WHERE active = true
  AND created_at > '2024-01-01'
ORDER BY name;
EOF
```

### Script Files

agfs-shell2 can execute script files with full support for variables, control flow, and all shell features.

**Create a script file**:
```bash
# Create process_logs.sh
cat << 'EOF' > process_logs.sh
#!/usr/bin/env uv run agfs-shell2

# Process log files and extract errors
LOG_DIR=/local/logs
OUTPUT=/local/error_summary.txt

echo "Processing logs in $LOG_DIR..." > $OUTPUT
echo "================================" >> $OUTPUT

# Count errors in each log file
for logfile in $LOG_DIR/*.log
do
  echo "Checking $logfile..." >&2
  error_count=$(cat $logfile | grep -i error | wc -l)

  if [ $error_count -gt 0 ]
  then
    echo "$logfile: $error_count errors" >> $OUTPUT
    cat $logfile | grep -i error >> $OUTPUT
  else
    echo "$logfile: OK" >> $OUTPUT
  fi
done

echo "Done! Summary written to $OUTPUT" >&2
EOF

# Make it executable and run
chmod +x process_logs.sh
./process_logs.sh
```

**Script with functions and complex logic**:
```bash
cat << 'EOF' > backup_system.sh
#!/usr/bin/env uv run agfs-shell2

# Backup system - copies important files to backup location
BACKUP_DIR=/local/backups
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_PATH=$BACKUP_DIR/backup_$TIMESTAMP

# Create backup directory
mkdir $BACKUP_PATH

# List of directories to backup
DIRS="/local/data /local/config /local/logs"

for dir in $DIRS
do
  if [ -d $dir ]
  then
    echo "Backing up $dir..."
    cp -r $dir $BACKUP_PATH/
  else
    echo "Warning: $dir not found, skipping"
  fi
done

# Create backup manifest
cat << MANIFEST > $BACKUP_PATH/manifest.txt
Backup created: $TIMESTAMP
Directories backed up:
$DIRS
MANIFEST

echo "Backup completed: $BACKUP_PATH"
EOF
```

**Script with error handling**:
```bash
cat << 'EOF' > data_pipeline.sh
#!/usr/bin/env uv run agfs-shell2

# Data processing pipeline with error handling
INPUT_FILE=/local/raw_data.json
PROCESSED=/local/processed_data.json
ERROR_LOG=/local/pipeline_errors.log

# Clear error log
echo "Pipeline started at $(date)" > $ERROR_LOG

# Check if input file exists
if [ ! -f $INPUT_FILE ]
then
  echo "ERROR: Input file $INPUT_FILE not found" >> $ERROR_LOG
  exit 1
fi

# Process JSON data
cat $INPUT_FILE | jq '.items[] | select(.active == true)' > $PROCESSED

if [ $? -eq 0 ]
then
  echo "SUCCESS: Processed $(cat $PROCESSED | wc -l) items" >> $ERROR_LOG
  exit 0
else
  echo "ERROR: jq processing failed" >> $ERROR_LOG
  exit 1
fi
EOF
```

## Glob Expansion

agfs-shell2 supports glob patterns for filename expansion, similar to bash:

```bash
# Match all .txt files in /local
cat /local/*.txt

# Match files with single character
ls /local/file?.dat

# Character class - match file1, file2, file3
echo /local/file[123].txt

# Range - match files a through z
rm /local/test[a-z].log
```

### Supported Patterns

- **`*`** - Matches any string (including empty string)
  - `*.txt` matches `file.txt`, `data.txt`, `test.txt`
  - `/local/*` matches all files in `/local`

- **`?`** - Matches exactly one character
  - `file?.txt` matches `file1.txt`, `fileA.txt` but not `file12.txt`

- **`[...]`** - Matches any character in the set
  - `file[123].txt` matches `file1.txt`, `file2.txt`, `file3.txt`
  - `test[a-z].dat` matches `testa.dat`, `testb.dat`, ..., `testz.dat`

### How It Works

1. Glob patterns are expanded **after** variable expansion
2. If no files match the pattern, the literal pattern is kept
3. Matches are sorted alphabetically
4. Works with any command that accepts file arguments

**Practical glob examples**:
```bash
# Process multiple log files
> for file in /local/*.log; do
    echo "Processing $file"
    cat $file | grep ERROR
  done

# Backup with wildcards
> mkdir /local/backup
> for src in /local/data*.txt; do
    cp $src /local/backup/
  done

# Count lines in all Python files
> wc -l /local/src/*.py

# Delete all temporary files
> rm /local/tmp/*.tmp

# Process files matching specific pattern
> for file in /local/report_[0-9][0-9][0-9][0-9].txt; do
    echo "Processing report: $file"
    cat $file | grep "Total"
  done

# Combine glob with grep
> grep "ERROR" /local/logs/app_*.log

# Process files by extension
> for json in /local/data/*.json; do
    echo "Validating $json"
    cat $json | jq . > /dev/null && echo "OK" || echo "Invalid JSON"
  done
```

**Glob with variables**:
```bash
# Use variables in glob patterns
> LOG_DIR=/local/logs
> PATTERN="*.log"
> for file in $LOG_DIR/$PATTERN; do
    cat $file | grep -i error
  done

# Dynamic pattern matching
> export FILE_PREFIX="data"
> export FILE_EXT="csv"
> ls /local/${FILE_PREFIX}*.${FILE_EXT}
```

## Variables and Command Substitution

agfs-shell2 supports shell variables and command substitution, allowing you to capture command output and reuse it.

### Special Variables

- **$?** - Exit code of the last executed command (0 for success, non-zero for failure)

```bash
# Check the exit code of the last command
echo "test"
echo $?  # Prints: 0

# Use in conditionals
[ "a" = "a" ]
if [ $? -eq 0 ]; then
  echo "Test succeeded"
fi
```

### Variable Assignment

```bash
# Simple variable assignment
name=Alice
path=/local/data.txt

# Variable assignment with command substitution
content=$(cat /local/file.txt)
count=$(echo "hello world" | wc -w)
files=`ls /local`  # backtick syntax also supported
```

### Variable Expansion

```bash
# Simple variable expansion
echo $name

# Braced variable expansion (recommended)
echo ${name}

# Using variables in commands
file=/local/test.txt
cat $file
```

### Command Substitution

Command substitution allows you to use the output of a command as part of another command:

```bash
# Using $() syntax (recommended)
line_count=$(wc -l < /local/data.txt)
echo "File has $line_count lines"

# Using backticks (also supported)
current_dir=`pwd`
echo "Current directory: $current_dir"

# Nested command substitution
file_size=$(stat $(echo /local/test.txt))

# With pipelines
error_count=$(cat /local/log.txt | grep ERROR | wc -l)
echo "Found $error_count errors"
```

### Comprehensive Variable Examples

```bash
# Basic variable usage
> NAME="Alice"
> echo "Hello, $NAME!"
Hello, Alice!

# Variables in paths
> DATA_DIR=/local/data
> LOG_FILE=$DATA_DIR/app.log
> echo "Starting..." > $LOG_FILE

# Count files in directory
> file_count=$(ls /local | wc -l)
> echo "Found $file_count files"
Found 5 files

# Process file based on variable
> input_file=/local/input.txt
> output_file=/local/output.txt
> cat $input_file | sort | uniq > $output_file

# Dynamic path construction
> base=/local
> project=myproject
> mkdir -p $base/$project
> echo "Project files:" > $base/$project/info.txt

# Use command output in variables
> today=$(date +%Y-%m-%d)
> backup_file=/local/backup_$today.tar
> echo "Creating backup: $backup_file"

# Combining variables and command substitution
> LOG_DIR=/local/logs
> error_count=$(cat $LOG_DIR/*.log | grep -i error | wc -l)
> if [ $error_count -gt 0 ]; then
    echo "Found $error_count errors in logs"
  fi

# Variable in loops
> FILES="file1.txt file2.txt file3.txt"
> for file in $FILES; do
    echo "Processing: $file"
    cat /local/$file | wc -l
  done

# Build complex commands with variables
> SEARCH_TERM="error"
> INPUT_DIR=/local/logs
> OUTPUT_FILE=/local/error_report.txt
> cat $INPUT_DIR/*.log | grep -i "$SEARCH_TERM" > $OUTPUT_FILE
> echo "Found $(cat $OUTPUT_FILE | wc -l) occurrences of '$SEARCH_TERM'"

# Environment variables
> export DATABASE_URL="postgres://localhost/mydb"
> export LOG_LEVEL="debug"
> env | grep -E "(DATABASE|LOG)"
DATABASE_URL=postgres://localhost/mydb
LOG_LEVEL=debug
```

## Control Flow (if/then/else/fi)

agfs-shell2 supports bash-style if statements for conditional execution.

### Syntax

```bash
# Basic if/then/fi
if condition; then
  commands
fi

# If/else
if condition; then
  commands
else
  commands
fi

# If/elif/else
if condition1; then
  commands
elif condition2; then
  commands
else
  commands
fi

# Single-line syntax
if condition; then command; fi
if condition; then command1; else command2; fi
```

### Conditional Tests

The `test` command (or `[ ]` syntax) is used to evaluate conditions:

**File tests:**
- `-f FILE` - True if file exists and is a regular file
- `-d FILE` - True if file exists and is a directory
- `-e FILE` - True if file exists

**String tests:**
- `-z STRING` - True if string is empty
- `-n STRING` - True if string is not empty
- `STRING1 = STRING2` - True if strings are equal
- `STRING1 != STRING2` - True if strings are not equal

**Integer tests:**
- `INT1 -eq INT2` - True if integers are equal
- `INT1 -ne INT2` - True if integers are not equal
- `INT1 -gt INT2` - True if INT1 is greater than INT2
- `INT1 -lt INT2` - True if INT1 is less than INT2
- `INT1 -ge INT2` - True if INT1 is greater than or equal to INT2
- `INT1 -le INT2` - True if INT1 is less than or equal to INT2

**Logical operators:**
- `! EXPR` - True if expression is false (negation)
- `EXPR -a EXPR` - True if both expressions are true (AND)
- `EXPR -o EXPR` - True if either expression is true (OR)

### Examples

```bash
# String comparison
if [ "hello" = "hello" ]; then
  echo "Strings match"
fi

# File existence check
if [ -f /local/data.txt ]; then
  echo "File exists"
  cat /local/data.txt
else
  echo "File not found"
fi

# Directory check
if [ -d /local/mydir ]; then
  echo "Directory exists"
fi

# String emptiness test
name=""
if [ -z "$name" ]; then
  echo "Name is empty"
fi

# Negation
if [ ! -f /local/file.txt ]; then
  echo "File does not exist"
fi

# Multi-line if statement (in interactive mode)
if [ -f /local/input.txt ]
then
  content=$(cat /local/input.txt)
  if [ -n "$content" ]
  then
    echo "Processing file..."
    cat /local/input.txt | sort > /local/output.txt
  fi
fi

# elif chain
status="running"
if [ "$status" = "stopped" ]; then
  echo "Service is stopped"
elif [ "$status" = "running" ]; then
  echo "Service is running"
else
  echo "Unknown status"
fi

# Using $? (exit code)
echo "Running command..."
if [ $? -eq 0 ]; then
  echo "Command succeeded"
else
  echo "Command failed with exit code: $?"
fi

# Integer comparisons
count=10
if [ $count -gt 5 ]; then
  echo "Count is greater than 5"
fi

if [ $count -le 100 ]; then
  echo "Count is within limit"
fi
```

**Real-world if/else examples**:
```bash
# Validate and process file
> INPUT=/local/data.json
> if [ -f $INPUT ]; then
    echo "File exists, processing..."
    LINES=$(cat $INPUT | jq '.items | length')
    if [ $LINES -gt 0 ]; then
      echo "Processing $LINES items"
      cat $INPUT | jq '.items[]' > /local/processed.json
    else
      echo "File is empty"
    fi
  else
    echo "ERROR: File $INPUT not found"
    exit 1
  fi

# Conditional backup
> SOURCE=/local/important.txt
> BACKUP=/local/backup/important.txt
> if [ -f $SOURCE ]; then
    if [ -f $BACKUP ]; then
      echo "Backup already exists, creating timestamped backup"
      TIMESTAMP=$(date +%Y%m%d_%H%M%S)
      cp $SOURCE /local/backup/important_$TIMESTAMP.txt
    else
      echo "Creating initial backup"
      cp $SOURCE $BACKUP
    fi
  fi

# Check file size and act accordingly
> LOG_FILE=/local/app.log
> if [ -f $LOG_FILE ]; then
    SIZE=$(stat $LOG_FILE | grep Size | cut -d: -f2)
    if [ $SIZE -gt 10000000 ]; then
      echo "Log file too large, archiving..."
      cp $LOG_FILE /local/archive/app_$(date +%Y%m%d).log
      echo "" > $LOG_FILE
    fi
  fi

# Multi-condition checks
> STATUS="running"
> ERROR_COUNT=$(cat /local/errors.log | wc -l)
> if [ "$STATUS" = "running" -a $ERROR_COUNT -eq 0 ]; then
    echo "System healthy"
  elif [ "$STATUS" = "running" -a $ERROR_COUNT -gt 0 ]; then
    echo "System running with $ERROR_COUNT errors"
  else
    echo "System not running"
  fi
```

## For Loops (for/in/do/done)

agfs-shell2 supports bash-style for loops to iterate over lists of items.

### Syntax

```bash
# Basic for loop
for var in item1 item2 item3; do
  commands
done

# Multi-line syntax
for var in item1 item2 item3
do
  commands
done
```

### Examples

```bash
# Loop over numbers
for i in 1 2 3 4 5; do
  echo "Number: $i"
done

# Loop over words
for fruit in apple banana cherry; do
  echo "Fruit: $fruit"
done

# Loop with command substitution
for file in $(ls /local); do
  echo "File: $file"
done

# Loop with variable expansion
items="red green blue"
for color in $(echo $items); do
  echo "Color: $color"
done

# Using loop variable in commands
for name in Alice Bob Charlie; do
  echo "Hello, $name!"
  if [ "$name" = "Bob" ]; then
    echo "  Special greeting for Bob"
  fi
done
```

**Real-world for loop examples**:
```bash
# Process all log files and extract errors
> for logfile in /local/logs/*.log; do
    echo "=== Processing $logfile ==="
    error_count=$(cat $logfile | grep -i error | wc -l)
    warning_count=$(cat $logfile | grep -i warning | wc -l)
    echo "Errors: $error_count, Warnings: $warning_count"

    if [ $error_count -gt 0 ]; then
      cat $logfile | grep -i error > /local/errors_$(basename $logfile)
    fi
  done

# Batch file conversion
> for jsonfile in /local/data/*.json; do
    echo "Converting $jsonfile..."
    basename=$(basename $jsonfile .json)
    cat $jsonfile | jq -r '.[] | [.name, .value] | @csv' > /local/csv/$basename.csv
    echo "Created /local/csv/$basename.csv"
  done

# Create multiple backup copies
> SOURCE=/local/important.txt
> for i in 1 2 3 4 5; do
    BACKUP=/local/backup/important_copy_$i.txt
    cp $SOURCE $BACKUP
    echo "Created backup $i: $BACKUP"
  done

# Process files with different operations based on extension
> for file in /local/data/*; do
    if echo $file | grep -q ".json$"; then
      echo "Validating JSON: $file"
      cat $file | jq . > /dev/null && echo "  ✓ Valid" || echo "  ✗ Invalid"
    elif echo $file | grep -q ".txt$"; then
      echo "Counting lines: $file"
      echo "  $(cat $file | wc -l) lines"
    else
      echo "Unknown type: $file"
    fi
  done

# Parallel-style processing with status tracking
> TOTAL=0
> SUCCESS=0
> FAILED=0
> for file in /local/input/*.dat; do
    TOTAL=$((TOTAL + 1))
    echo "Processing file $TOTAL: $file"

    if cat $file | sort > /local/output/$(basename $file); then
      SUCCESS=$((SUCCESS + 1))
      echo "  ✓ Success"
    else
      FAILED=$((FAILED + 1))
      echo "  ✗ Failed"
    fi
  done
> echo "Summary: Total=$TOTAL, Success=$SUCCESS, Failed=$FAILED"

# Nested loops
> for dir in /local/projects/*; do
    if [ -d $dir ]; then
      echo "Project: $(basename $dir)"
      for file in $dir/*.txt; do
        if [ -f $file ]; then
          lines=$(cat $file | wc -l)
          echo "  - $(basename $file): $lines lines"
        fi
      done
    fi
  done

# Loop with command substitution
> SERVERS="server1 server2 server3"
> for server in $SERVERS; do
    LOG_FILE=/local/logs/${server}.log
    if [ -f $LOG_FILE ]; then
      echo "=== $server ==="
      cat $LOG_FILE | tail -n 5
    fi
  done
```

## Path Support

agfs-shell2 supports both absolute and relative paths:

- **Absolute paths**: Start with `/` (e.g., `/local/file.txt`, `/s3fs/bucket/data.csv`)
- **Relative paths**: Resolved from current directory (e.g., `file.txt`, `../parent/file.txt`)
- **Special paths**: `.` (current directory), `..` (parent directory)
- **Tab completion**: Works for both absolute and relative paths

The shell prompt shows your current directory (e.g., `/local/project >`)

## Examples

Run the examples script:

```bash
./examples.sh
./test_redirections.sh  # Test redirection features
```

### Pipeline Examples

```bash
# Basic pipeline
uv run agfs-shell2 "echo hello world | grep hello"

# Word count
uv run agfs-shell2 "echo hello world | wc -w"

# Character translation
uv run agfs-shell2 "echo hello | tr h H"

# Sort and unique
printf "apple\nbanana\napple\ncherry\n" | uv run agfs-shell2 "cat | sort | uniq"

# Complex pipeline
printf "apple pie\nbanana split\napple juice\ncherry pie\n" | \
  uv run agfs-shell2 "cat | grep pie | sort | wc -l"
```

### AGFS File Operations

All file operations automatically use AGFS paths. AGFS paths typically start with a mount point like `/local/`, `/s3fs/`, `/sqlfs/`, etc.

```bash
# Write to local filesystem via AGFS
uv run agfs-shell2 "echo 'Hello AGFS!' > /local/hello.txt"

# Read from AGFS
uv run agfs-shell2 "cat /local/hello.txt"

# Append to AGFS file
uv run agfs-shell2 "echo 'Line 2' >> /local/hello.txt"

# Input redirection from AGFS
uv run agfs-shell2 "wc -l < /local/hello.txt"

# Cross-filesystem operations (if you have multiple mounts)
uv run agfs-shell2 "cat /local/data.txt > /s3fs/backup.txt"
uv run agfs-shell2 "cat /sqlfs/query_results.txt | grep ERROR > /local/errors.txt"

# Complex pipeline with AGFS
uv run agfs-shell2 "cat /local/access.log | grep 404 | sort | uniq > /local/404_urls.txt"
```

### Using cd and Relative Paths

Interactive mode with directory navigation:

```bash
$ uv run agfs-shell2

agfs-shell2 v0.1.0
Connected to AGFS server at http://localhost:8080
Type 'exit' or 'quit' to exit, 'help' for help

> pwd
/

> cd /local/project

/local/project > pwd
/local/project

/local/project > ls
README.md
src/
tests/

/local/project > cat README.md
This is my project

/local/project > cd src

/local/project/src > ls
main.py
utils.py

/local/project/src > cat main.py
def main():
    print("Hello World")

/local/project/src > cd ../tests

/local/project/tests > pwd
/local/project/tests

/local/project/tests > cd

> pwd
/
```

Using relative paths in commands:

```bash
# After cd /local/project
/local/project > echo "new file" > data.txt        # Creates /local/project/data.txt
/local/project > cat data.txt                      # Reads from current directory
/local/project > cat src/main.py                   # Relative path to subdirectory
/local/project > cat ../other_project/file.txt    # Relative path to parent
```

### File Transfer Examples

```bash
# Upload local file to AGFS
> upload ~/Documents/report.pdf /local/backup/

# Upload directory recursively
> upload -r ~/Projects/myapp /local/projects/

# Download from AGFS to local
> download /local/data.json ~/Downloads/

# Download directory recursively
> download -r /local/logs ~/backup/logs/

# Copy within AGFS
> cp /local/file.txt /local/backup/file.txt

# Copy from local to AGFS using cp
> cp local:~/data.csv /local/imports/data.csv

# Copy from AGFS to local using cp
> cp /local/report.txt local:~/Desktop/report.txt

# Copy directory recursively
> cp -r /local/project /local/backup/project
```

### Testing

Run the integration tests (requires AGFS server):

```bash
./test_agfs_integration.sh
```

This will test:
- Writing files to AGFS
- Reading files from AGFS
- Append operations
- Input/output redirections
- Complex pipelines

## Project Structure

```
agfs-shell2/
├── agfs_shell2/
│   ├── __init__.py      # Package initialization
│   ├── streams.py       # Stream classes (InputStream, OutputStream, ErrorStream)
│   ├── process.py       # Process class for command execution with filesystem access
│   ├── pipeline.py      # Pipeline class for chaining processes
│   ├── parser.py        # Command line parser with redirection support
│   ├── builtins.py      # Built-in command implementations (AGFS-aware)
│   ├── filesystem.py    # AGFS filesystem abstraction layer
│   ├── config.py        # Configuration management
│   ├── shell.py         # Shell with REPL, cd support, and AGFS integration
│   ├── completer.py     # Tab completion for commands and paths
│   └── cli.py           # CLI entry point with argument parsing
├── pyproject.toml       # Project configuration (with pyagfs dependency)
├── examples.sh          # Example commands
├── test_redirections.sh # Redirection tests
├── test_agfs_integration.sh  # AGFS integration tests
├── test_cd_relative.sh  # cd and relative path tests
├── demo_cd_relative.sh  # Demo of cd and relative path features
└── README.md           # This file
```

## Design Notes

This is an experimental/educational project demonstrating:

1. **Stream abstraction**: How Unix treats everything as a file/stream
2. **Process composition**: How simple commands can be composed into complex operations
3. **Pipeline execution**: How stdout of one process becomes stdin of the next
4. **I/O Redirection**: Unix-style file redirection with `<`, `>`, and `>>`
5. **Variables and substitution**: Shell variable expansion, command substitution, and special variables ($?)
6. **Control flow**: Conditional execution (if/then/elif/else/fi) and loops (for/in/do/done)
7. **Conditional testing**: File, string, and integer tests using test and [ ] commands
8. **Directory navigation**: Working directory concept with relative path resolution
9. **Tab completion**: Interactive command and path completion using readline
10. **AGFS Integration**: How to build applications using distributed/pluggable filesystems
11. **Python implementation**: Pure Python implementation without subprocess module

### Key Design Decisions

- **No local filesystem access**: All file operations go through AGFS, demonstrating how to build cloud-native tools
- **In-memory pipeline buffers**: Pipeline data flows through memory buffers, not temporary files
- **Synchronous execution**: Processes execute sequentially for simplicity (not true parallel execution)
- **AGFS path model**: Paths like `/local/file.txt`, `/s3fs/bucket/file.txt` show filesystem plugin architecture
- **Current working directory**: Tracked in shell state, allowing navigation within AGFS filesystem hierarchy
- **Path resolution**: Both absolute and relative paths supported, with `.` and `..` handling

### Features Implemented

- ✅ Unix-style pipelines (`|`)
- ✅ Input redirection (`<`)
- ✅ Output redirection (`>`)
- ✅ Append redirection (`>>`)
- ✅ Error redirection (`2>`, `2>>`)
- ✅ Combining pipelines with redirections
- ✅ Shell variables (`VAR=value`, `$VAR`, `${VAR}`)
- ✅ Special variables (`$?` for exit code)
- ✅ Command substitution (`$(command)`, backticks)
- ✅ Environment variable management (`export`, `env`, `unset`)
- ✅ Control flow (`if/then/elif/else/fi` statements and `for/in/do/done` loops)
- ✅ Conditional testing (`test` and `[ ]` commands with file, string, integer, and logical tests)
- ✅ Directory navigation (`cd` command)
- ✅ Relative path support (`.`, `..`, relative files)
- ✅ Tab completion for commands and paths
- ✅ 24 built-in commands (cd, pwd, ls, cat, mkdir, rm, stat, cp, upload, download, echo, grep, jq, wc, head, tail, sort, uniq, tr, export, env, unset, test, [, sleep)
- ✅ Interactive REPL mode with dynamic prompt
- ✅ Script file execution (shebang support)
- ✅ Non-interactive command execution (-c flag)

The implementation uses in-memory buffers for streams, making it suitable for learning but not for production use.
