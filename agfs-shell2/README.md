# agfs-shell2

DO NOT USE IT NOW, UNDER CONSTRUCTION

Experimental agfs shell implementation with Unix-style pipeline support and **AGFS integration** in pure Python.

## Overview

agfs-shell2 is a simple shell that demonstrates Unix pipeline concepts while integrating with the AGFS (Aggregated File System) server. All file operations go through AGFS, allowing you to work with multiple backend filesystems (local, S3, SQL, etc.) through a unified interface.

## Features

- **Unix-style pipelines**: Chain commands with `|` operator
- **I/O Redirection**: Support for `<`, `>`, `>>`, `2>`, `2>>` operators
- **Variables**: Shell variable assignment and expansion (`VAR=value`, `$VAR`, `${VAR}`)
- **Special variables**: `$?` for exit code of last command
- **Command substitution**: Capture command output with `$(command)` or backticks
- **Control flow**: if/then/elif/else/fi statements and for/in/do/done loops
- **Conditional testing**: `test` and `[ ]` commands for file, string, integer, and logical tests
- **Multiline input**: Backslash continuation, unclosed quotes, and bracket matching like bash
- **Directory navigation**: `cd` command with current working directory tracking
- **Relative paths**: Full support for `.`, `..`, and relative file paths
- **Tab completion**: Smart completion for commands and paths (both absolute and relative)
- **AGFS Integration**: All file operations use AGFS server (no local filesystem access)
- **Streaming I/O**: Memory-efficient streaming for large files (8KB chunks)
- **Stream handling**: Full STDIN/STDOUT/STDERR support
- **Built-in commands**: cd, pwd, ls, cat, mkdir, rm, stat, echo, grep, wc, head, tail, sort, uniq, tr, export, env, unset, test, [
- **Interactive REPL**: Interactive shell mode with dynamic prompt showing current directory
- **Script execution**: Support for shebang scripts (`#!/usr/bin/env uv run agfs-shell2`)
- **Non-interactive mode**: Execute commands from command line with `-c` flag
- **Configurable server**: Support for custom AGFS server URL
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
Type 'exit' or 'quit' to exit, 'help' for help

$ echo hello world | cat > /local/greeting.txt
$ cat /local/greeting.txt
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

## Built-in Commands

### File System Commands (AGFS)
- **cd [path]** - Change current directory (supports relative paths: `.`, `..`, etc.)
- **pwd** - Print current working directory
- **ls [-l] [path]** - List directory contents (defaults to current directory, -l for long format)
- **cat [file...]** - Concatenate and print files or stdin
- **mkdir path** - Create directory
- **rm [-r] path** - Remove file or directory
- **stat path** - Display file status and check if file exists

### Text Processing Commands
- **echo [args...]** - Print arguments to stdout
- **grep pattern** - Search for pattern in stdin
- **wc [-l] [-w] [-c]** - Count lines, words, and bytes
- **head [-n count]** - Output first N lines (default 10)
- **tail [-n count]** - Output last N lines (default 10)
- **sort [-r]** - Sort lines (use -r for reverse)
- **uniq** - Remove duplicate adjacent lines
- **tr set1 set2** - Translate characters

### Environment Variables
- **export [VAR=value ...]** - Set or display environment variables
- **env** - Display all environment variables
- **unset VAR [VAR ...]** - Unset environment variables

### Conditional Testing
- **test EXPRESSION** - Evaluate conditional expressions
- **[ EXPRESSION ]** - Alternative syntax for test command

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

### Examples

```bash
# Count files in directory
file_count=$(ls /local | wc -l)
echo "Found $file_count files"

# Process file based on variable
input_file=/local/input.txt
output_file=/local/output.txt
cat $input_file | sort | uniq > $output_file

# Dynamic path construction
base=/local
project=myproject
echo "Project files:" > $base/$project/info.txt
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
- ✅ 20 built-in commands (cd, pwd, ls, cat, mkdir, rm, stat, echo, grep, wc, head, tail, sort, uniq, tr, export, env, unset, test, [)
- ✅ Interactive REPL mode with dynamic prompt
- ✅ Script file execution (shebang support)
- ✅ Non-interactive command execution (-c flag)

The implementation uses in-memory buffers for streams, making it suitable for learning but not for production use.
