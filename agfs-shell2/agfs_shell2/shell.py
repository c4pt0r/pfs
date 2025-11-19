"""Shell implementation with REPL and command execution"""

import sys
import os
from typing import Optional
from .parser import CommandParser
from .pipeline import Pipeline
from .process import Process
from .streams import InputStream, OutputStream, ErrorStream
from .builtins import get_builtin
from .filesystem import AGFSFileSystem
from pyagfs import AGFSClientError


class Shell:
    """Simple shell with pipeline support"""

    def __init__(self, server_url: str = "http://localhost:8080"):
        self.parser = CommandParser()
        self.running = True
        self.filesystem = AGFSFileSystem(server_url)
        self.server_url = server_url
        self.cwd = '/'  # Current working directory

    def resolve_path(self, path: str) -> str:
        """
        Resolve a relative or absolute path to an absolute path

        Args:
            path: Path to resolve (can be relative or absolute)

        Returns:
            Absolute path
        """
        if not path:
            return self.cwd

        # Already absolute
        if path.startswith('/'):
            # Normalize the path (remove redundant slashes, handle . and ..)
            return os.path.normpath(path)

        # Relative path - join with cwd
        full_path = os.path.join(self.cwd, path)
        # Normalize to handle . and ..
        return os.path.normpath(full_path)

    def execute(self, command_line: str, stdin_data: Optional[bytes] = None) -> int:
        """
        Execute a command line (possibly with pipelines and redirections)

        Args:
            command_line: Command string to execute
            stdin_data: Optional stdin data to provide to first command

        Returns:
            Exit code of the pipeline
        """
        # Parse the command line with redirections
        commands, redirections = self.parser.parse_command_line(command_line)

        if not commands:
            return 0

        # Special handling for cd command (must be a single command, not in pipeline)
        if len(commands) == 1 and commands[0][0] == 'cd':
            cmd, args = commands[0]
            # Resolve target path
            target = args[0] if args else '/'
            resolved_path = self.resolve_path(target)

            # Verify the directory exists
            try:
                entries = self.filesystem.list_directory(resolved_path)
                # Successfully listed - it's a valid directory
                self.cwd = resolved_path
                return 0
            except Exception as e:
                error_msg = str(e)
                if "No such file or directory" in error_msg or "not found" in error_msg.lower():
                    sys.stderr.write(f"cd: {target}: No such file or directory\n")
                else:
                    sys.stderr.write(f"cd: {target}: {error_msg}\n")
                return 1

        # Resolve paths in redirections
        if 'stdin' in redirections:
            input_file = self.resolve_path(redirections['stdin'])
            try:
                # Use AGFS to read input file
                stdin_data = self.filesystem.read_file(input_file)
            except AGFSClientError as e:
                error_msg = self.filesystem.get_error_message(e)
                sys.stderr.write(f"shell: {error_msg}\n")
                return 1
            except Exception as e:
                sys.stderr.write(f"shell: {input_file}: {str(e)}\n")
                return 1

        # Build processes for each command
        processes = []
        for i, (cmd, args) in enumerate(commands):
            # Get the executor for this command
            executor = get_builtin(cmd)

            # Resolve relative paths in arguments (for file-related commands)
            # Commands that typically take file paths: cat, ls, mkdir, rm
            if cmd in ('cat', 'ls', 'mkdir', 'rm'):
                resolved_args = []
                for arg in args:
                    # Skip flags (starting with -)
                    if arg.startswith('-'):
                        resolved_args.append(arg)
                    else:
                        # Resolve path
                        resolved_args.append(self.resolve_path(arg))
                args = resolved_args

            # Create streams
            if i == 0 and stdin_data is not None:
                stdin = InputStream.from_bytes(stdin_data)
            else:
                stdin = InputStream.from_bytes(b'')

            stdout = OutputStream.to_buffer()
            stderr = ErrorStream.to_buffer()

            # Create process with filesystem and cwd
            process = Process(
                command=cmd,
                args=args,
                stdin=stdin,
                stdout=stdout,
                stderr=stderr,
                executor=executor,
                filesystem=self.filesystem
            )
            # Pass cwd to process for pwd command
            process.cwd = self.cwd
            processes.append(process)

        # Create and execute pipeline
        pipeline = Pipeline(processes)
        exit_code = pipeline.execute()

        # Get results
        stdout_data = pipeline.get_stdout()
        stderr_data = pipeline.get_stderr()

        # Handle output redirection (>)
        if 'stdout' in redirections:
            output_file = self.resolve_path(redirections['stdout'])
            mode = redirections.get('stdout_mode', 'write')
            append = (mode == 'append')
            try:
                # Use AGFS to write output file
                self.filesystem.write_file(output_file, stdout_data, append=append)
            except AGFSClientError as e:
                error_msg = self.filesystem.get_error_message(e)
                sys.stderr.write(f"shell: {error_msg}\n")
                return 1
            except Exception as e:
                sys.stderr.write(f"shell: {output_file}: {str(e)}\n")
                return 1
        else:
            # Output to stdout if no redirection
            if stdout_data:
                sys.stdout.buffer.write(stdout_data)
                sys.stdout.buffer.flush()

        # Handle error redirection (2>)
        if 'stderr' in redirections:
            error_file = self.resolve_path(redirections['stderr'])
            mode = redirections.get('stderr_mode', 'write')
            append = (mode == 'append')
            try:
                # Use AGFS to write error file
                self.filesystem.write_file(error_file, stderr_data, append=append)
            except AGFSClientError as e:
                error_msg = self.filesystem.get_error_message(e)
                sys.stderr.write(f"shell: {error_msg}\n")
                return 1
            except Exception as e:
                sys.stderr.write(f"shell: {error_file}: {str(e)}\n")
                return 1
        else:
            # Output to stderr if no redirection
            if stderr_data:
                sys.stderr.buffer.write(stderr_data)
                sys.stderr.buffer.flush()

        return exit_code

    def repl(self):
        """Run interactive REPL"""
        print("agfs-shell2 v0.1.0")
        print(f"Connected to AGFS server at {self.server_url}")

        # Check server connection
        if not self.filesystem.check_connection():
            print(f"⚠ Warning: Cannot connect to AGFS server at {self.server_url}")
            print("  Make sure the server is running. File operations will fail.")

        print("Type 'exit' or 'quit' to exit, 'help' for help")
        print()

        # Setup tab completion
        try:
            import readline
            from .completer import ShellCompleter

            completer = ShellCompleter(self.filesystem)
            # Pass shell reference to completer for cwd
            completer.shell = self
            readline.set_completer(completer.complete)

            # Different binding for libedit (macOS) vs GNU readline (Linux)
            if 'libedit' in readline.__doc__:
                # macOS/BSD libedit
                readline.parse_and_bind("bind ^I rl_complete")
            else:
                # GNU readline
                readline.parse_and_bind("tab: complete")

            # Configure readline to use space and special chars as delimiters
            # This allows path completion to work properly
            readline.set_completer_delims(' \t\n;|&<>()')
        except ImportError:
            # readline not available (e.g., on Windows without pyreadline)
            pass

        while self.running:
            try:
                # Read command
                try:
                    # Show current directory in prompt
                    prompt = f"{self.cwd}> " if self.cwd != '/' else "/> "
                    command = input(prompt)
                except EOFError:
                    print()
                    break

                command = command.strip()

                # Handle special commands
                if command in ('exit', 'quit'):
                    break
                elif command == 'help':
                    self.show_help()
                    continue
                elif not command:
                    continue

                # Execute command
                try:
                    self.execute(command)
                except KeyboardInterrupt:
                    print("^C")
                    continue
                except Exception as e:
                    print(f"Error: {e}", file=sys.stderr)

            except KeyboardInterrupt:
                print()
                continue

        print("Goodbye!")

    def show_help(self):
        """Show help message"""
        help_text = """
agfs-shell2 - Experimental shell with AGFS integration

File System Commands (AGFS):
  cd [path]              - Change current directory (supports relative paths)
  pwd                    - Print current working directory
  ls [path]              - List directory contents (defaults to cwd)
  mkdir path             - Create directory
  rm [-r] path           - Remove file or directory
  cat [file...]          - Read and concatenate files

Text Processing Commands:
  echo [args...]         - Print arguments to stdout
  grep pattern           - Search for pattern in stdin
  wc [-l] [-w] [-c]      - Count lines, words, and bytes
  head [-n count]        - Output first N lines (default 10)
  tail [-n count]        - Output last N lines (default 10)
  sort [-r]              - Sort lines (use -r for reverse)
  uniq                   - Remove duplicate adjacent lines
  tr set1 set2           - Translate characters

Pipeline Syntax:
  command1 | command2 | command3

Redirection Operators:
  < file                 - Read input from AGFS file
  > file                 - Write output to AGFS file (overwrite)
  >> file                - Append output to AGFS file
  2> file                - Write stderr to AGFS file
  2>> file               - Append stderr to AGFS file

Path Resolution:
  - Absolute paths start with / (e.g., /local/file.txt)
  - Relative paths are resolved from current directory (e.g., file.txt, ../dir)
  - Special: . (current dir), .. (parent dir)
  - Tab completion works for both absolute and relative paths

Examples:
  $ cd /local/mydir      # Change to /local/mydir
  $ pwd                  # Shows: /local/mydir
  $ ls                   # List current directory
  $ cat file.txt         # Read file from current directory
  $ cd subdir            # Change to /local/mydir/subdir
  $ cd ..                # Go back to /local/mydir
  $ cd                   # Go to root (/)
  $ echo "test" > data.txt        # Create file in current directory
  $ cat /local/data.txt | grep "error" > errors.txt

Special Commands:
  help                   - Show this help
  exit, quit             - Exit the shell

Note: All file operations use AGFS. Paths like /local/, /s3fs/, /sqlfs/
      refer to different AGFS filesystem backends.
"""
        print(help_text)
