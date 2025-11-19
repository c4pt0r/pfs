# agfs-shell2 (WIP)

DO NOT USE IT NOW, UNDER CONSTRUCTION

Experimental agfs shell implementation with Unix-style pipeline support in pure Python.

## Features

- **Unix-style pipelines**: Chain commands with `|` operator
- **I/O Redirection**: Support for `<`, `>`, `>>`, `2>`, `2>>` operators
- **Stream handling**: Full STDIN/STDOUT/STDERR support
- **Built-in commands**: echo, cat, grep, wc, head, tail, sort, uniq, tr
- **Interactive REPL**: Interactive shell mode
- **Non-interactive mode**: Execute commands from command line

## Architecture

The shell is built with several key components:

- **Streams** (`streams.py`): InputStream, OutputStream, ErrorStream classes wrapping file descriptors
- **Process** (`process.py`): Represents a single command with stdin/stdout/stderr
- **Pipeline** (`pipeline.py`): Chains multiple processes together
- **Parser** (`parser.py`): Parses command strings into pipeline components
- **Builtins** (`builtins.py`): Implementation of built-in commands
- **Shell** (`shell.py`): Main shell with REPL

## Installation

```bash
cd agfs-shell2
uv sync
```

## Usage

### Interactive REPL Mode

```bash
uv run agfs-shell2
```

```
agfs-shell2 v0.1.0
Type 'exit' or 'quit' to exit, 'help' for help

$ echo hello world
hello world
$ echo "apple\nbanana\napple" | sort | uniq
apple
banana
```

### Non-Interactive Mode

```bash
# Execute a single command
uv run agfs-shell2 "echo hello world"

# Use with shell pipes
echo "test data" | uv run agfs-shell2 "cat | grep test"

# Complex pipelines
printf "line1\nline2\nline3\n" | uv run agfs-shell2 "cat | wc -l"
```

## Built-in Commands

- **echo [args...]** - Print arguments to stdout
- **cat [file...]** - Concatenate and print files or stdin
- **grep pattern** - Search for pattern in stdin
- **wc [-l] [-w] [-c]** - Count lines, words, and bytes
- **head [-n count]** - Output first N lines (default 10)
- **tail [-n count]** - Output last N lines (default 10)
- **sort [-r]** - Sort lines (use -r for reverse)
- **uniq** - Remove duplicate adjacent lines
- **tr set1 set2** - Translate characters

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

### Redirection Examples

```bash
# Output redirection (overwrite)
uv run agfs-shell2 "echo 'hello world' > output.txt"

# Output redirection (append)
uv run agfs-shell2 "echo 'line 1' > file.txt"
uv run agfs-shell2 "echo 'line 2' >> file.txt"

# Input redirection
echo "test data" > input.txt
uv run agfs-shell2 "cat < input.txt"
uv run agfs-shell2 "wc -l < input.txt"

# Combine input and output redirection
uv run agfs-shell2 "cat < input.txt | grep test > output.txt"

# Pipeline with output redirection
uv run agfs-shell2 "echo hello world | tr ' ' ',' > result.txt"

# Complex: input + pipeline + output
printf "apple\nbanana\napple\n" > items.txt
uv run agfs-shell2 "cat < items.txt | sort | uniq > unique_items.txt"
```

## Project Structure

```
agfs-shell2/
├── agfs_shell2/
│   ├── __init__.py      # Package initialization
│   ├── streams.py       # Stream classes (InputStream, OutputStream, ErrorStream)
│   ├── process.py       # Process class for command execution
│   ├── pipeline.py      # Pipeline class for chaining processes
│   ├── parser.py        # Command line parser
│   ├── builtins.py      # Built-in command implementations
│   ├── shell.py         # Shell with REPL
│   └── cli.py           # CLI entry point
├── pyproject.toml       # Project configuration
├── examples.sh          # Example commands
└── README.md           # This file
```

## Design Notes

This is an experimental/educational project demonstrating:

1. **Stream abstraction**: How Unix treats everything as a file/stream
2. **Process composition**: How simple commands can be composed into complex operations
3. **Pipeline execution**: How stdout of one process becomes stdin of the next
4. **I/O Redirection**: Unix-style file redirection with `<`, `>`, and `>>`
5. **Python implementation**: Pure Python implementation without subprocess module

### Features Implemented

- ✅ Unix-style pipelines (`|`)
- ✅ Input redirection (`<`)
- ✅ Output redirection (`>`)
- ✅ Append redirection (`>>`)
- ✅ Error redirection (`2>`, `2>>`)
- ✅ Combining pipelines with redirections
- ✅ 9 built-in commands (echo, cat, grep, wc, head, tail, sort, uniq, tr)
- ✅ Interactive REPL mode
- ✅ Non-interactive command execution

The implementation uses in-memory buffers for streams, making it suitable for learning but not for production use.
