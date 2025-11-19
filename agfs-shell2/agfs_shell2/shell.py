"""Shell implementation with REPL and command execution"""

import sys
import os
from typing import Optional
from .parser import CommandParser
from .pipeline import Pipeline
from .process import Process
from .streams import InputStream, OutputStream, ErrorStream
from .builtins import get_builtin


class Shell:
    """Simple shell with pipeline support"""

    def __init__(self):
        self.parser = CommandParser()
        self.running = True

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
                with open(input_file, 'rb') as f:
                    stdin_data = f.read()
            except FileNotFoundError:
                sys.stderr.write(f"shell: {input_file}: No such file or directory\n")
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

            # Create process
            process = Process(
                command=cmd,
                args=args,
                stdin=stdin,
                stdout=stdout,
                stderr=stderr,
                executor=executor
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
            try:
                open_mode = 'ab' if mode == 'append' else 'wb'
                with open(output_file, open_mode) as f:
                    f.write(stdout_data)
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
            try:
                open_mode = 'ab' if mode == 'append' else 'wb'
                with open(error_file, open_mode) as f:
                    f.write(stderr_data)
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
agfs-shell2 - Experimental shell with Unix pipeline support

Built-in Commands:
  echo [args...]         - Print arguments to stdout
  cat [file...]          - Concatenate and print files or stdin
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
  < file                 - Read input from file
  > file                 - Write output to file (overwrite)
  >> file                - Append output to file
  2> file                - Write stderr to file
  2>> file               - Append stderr to file

Examples:
  $ echo "hello world" | grep "hello"
  $ echo "hello" | tr 'h' 'H'
  $ cat < input.txt
  $ echo "hello world" > output.txt
  $ cat file.txt | grep pattern > results.txt
  $ echo "line1" >> output.txt

Special Commands:
  help                   - Show this help
  exit, quit             - Exit the shell
"""
        print(help_text)
