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

        # Handle input redirection (<)
        if 'stdin' in redirections:
            input_file = redirections['stdin']
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

            # Create streams
            if i == 0 and stdin_data is not None:
                stdin = InputStream.from_bytes(stdin_data)
            else:
                stdin = InputStream.from_bytes(b'')

            stdout = OutputStream.to_buffer()
            stderr = ErrorStream.to_buffer()

            # Create process with filesystem
            process = Process(
                command=cmd,
                args=args,
                stdin=stdin,
                stdout=stdout,
                stderr=stderr,
                executor=executor,
                filesystem=self.filesystem
            )
            processes.append(process)

        # Create and execute pipeline
        pipeline = Pipeline(processes)
        exit_code = pipeline.execute()

        # Get results
        stdout_data = pipeline.get_stdout()
        stderr_data = pipeline.get_stderr()

        # Handle output redirection (>)
        if 'stdout' in redirections:
            output_file = redirections['stdout']
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
            error_file = redirections['stderr']
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

        while self.running:
            try:
                # Read command
                try:
                    command = input("$ ")
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
  ls [path]              - List directory contents
  pwd                    - Print working directory (always /)
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

Examples (All paths are AGFS paths):
  $ ls /local
  $ echo "hello world" > /local/greeting.txt
  $ cat /local/greeting.txt
  $ cat /local/data.txt | grep "error" > /local/errors.txt
  $ mkdir /local/mydir
  $ rm -r /local/mydir

Special Commands:
  help                   - Show this help
  exit, quit             - Exit the shell

Note: All file operations use AGFS. Paths like /local/, /s3fs/, /sqlfs/
      refer to different AGFS filesystem backends.
"""
        print(help_text)
