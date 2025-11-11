"""REPL Command Handlers"""

import os
import re
import shlex
import sys
from typing import List

import requests
from rich.console import Console

from . import cli_commands
from .client import PFSClientError

console = Console()


class CommandHandler:
    """Handler for REPL commands"""

    def __init__(self, client):
        self.client = client
        self.current_path = "/"
        self.commands = {
            "help": self.cmd_help,
            "?": self.cmd_help,
            "ls": self.cmd_ls,
            "tree": self.cmd_tree,
            "cd": self.cmd_cd,
            "pwd": self.cmd_pwd,
            "cat": self.cmd_cat,
            "tail": self.cmd_tail,
            "write": self.cmd_write,
            "echo": self.cmd_echo,
            "mkdir": self.cmd_mkdir,
            "rm": self.cmd_rm,
            "touch": self.cmd_touch,
            "stat": self.cmd_stat,
            "cp": self.cmd_cp,
            "mv": self.cmd_mv,
            "chmod": self.cmd_chmod,
            "upload": self.cmd_upload,
            "download": self.cmd_download,
            "tailf": self.cmd_tailf,
            "mounts": self.cmd_mounts,
            "mount": self.cmd_mount,
            "unmount": self.cmd_unmount,
            "plugins": self.cmd_plugins,
            "watch": self.cmd_watch,
            "tee": self.cmd_tee,
            "grep": self.cmd_grep,
            # Utility commands
            "clear": self.cmd_clear,
            "exit": self.cmd_exit,
            "quit": self.cmd_exit,
        }

    def execute(self, line: str) -> bool:
        """Execute a command. Returns False if should exit."""
        line = line.strip()
        if not line:
            return True

        try:
            # Check for pipe operator
            if "|" in line:
                return self._execute_pipeline(line)

            parts = shlex.split(line)
            cmd = parts[0].lower()
            args = parts[1:]

            if cmd in self.commands:
                return self.commands[cmd](args)
            else:
                console.print(f"[red]Unknown command: {cmd}[/red]", highlight=False)
                console.print("Type 'help' for available commands", highlight=False)
                return True
        except Exception as e:
            console.print(f"[red]Error: {e}[/red]", highlight=False)
            return True

    def _execute_pipeline(self, line: str) -> bool:
        """Execute a pipeline of commands separated by |"""
        # Split by pipe, but respect quotes
        # We'll use a simple approach: split by '|' but handle quotes properly
        commands = []
        current_cmd = []
        in_quotes = False
        quote_char = None

        i = 0
        while i < len(line):
            char = line[i]

            if char in ('"', "'") and (i == 0 or line[i - 1] != "\\"):
                if not in_quotes:
                    in_quotes = True
                    quote_char = char
                elif char == quote_char:
                    in_quotes = False
                    quote_char = None
                current_cmd.append(char)
            elif char == "|" and not in_quotes:
                # Found a pipe separator
                cmd_str = "".join(current_cmd).strip()
                if cmd_str:
                    commands.append(cmd_str)
                current_cmd = []
            else:
                current_cmd.append(char)

            i += 1

        # Don't forget the last command
        cmd_str = "".join(current_cmd).strip()
        if cmd_str:
            commands.append(cmd_str)

        if len(commands) < 2:
            console.print(
                "[red]Error: pipeline requires at least 2 commands[/red]",
                highlight=False,
            )
            return True

        try:
            # Execute the pipeline
            current_input = None

            for i, cmd_str in enumerate(commands):
                is_last = i == len(commands) - 1
                current_input = self._execute_command_with_input(
                    cmd_str, current_input, is_last
                )

                if current_input is None:
                    # Command failed or returned exit signal
                    if isinstance(current_input, bool):
                        return current_input
                    return True

            return True

        except Exception as e:
            console.print(f"[red]Pipeline error: {e}[/red]", highlight=False)
            return True

    def _execute_command_with_input(
        self, cmd_str: str, pipe_input: bytes = None, is_last: bool = False
    ):
        """Execute a command with optional piped input and return its output.

        Args:
            cmd_str: Command string to execute
            pipe_input: Input bytes from previous command in pipeline
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: Output of the command, or None if command failed
        """
        try:
            # Check if command has redirection operators (> or >>)
            # Parse redirections to support chained redirections like: echo 'query' > file1 > file2
            redirect_chain = self._parse_redirections(cmd_str)

            if redirect_chain:
                return self._execute_redirect_chain(redirect_chain, is_last)

            parts = shlex.split(cmd_str)
            if not parts:
                return None

            cmd = parts[0].lower()
            args = parts[1:]

            # Special handling for commands that support piped input
            if cmd == "tee":
                return self._cmd_tee_pipe(args, pipe_input, is_last)
            elif cmd == "cat":
                # For cat without arguments, output the piped input
                if not args and pipe_input is not None:
                    if is_last:
                        sys.stdout.buffer.write(pipe_input)
                        if pipe_input and not pipe_input.endswith(b"\n"):
                            sys.stdout.buffer.write(b"\n")
                        sys.stdout.buffer.flush()
                        return None
                    else:
                        return pipe_input
                else:
                    # Cat with file argument - read the file
                    path = self._resolve_path(args[0])
                    content = self.client.cat(path)
                    if is_last:
                        sys.stdout.buffer.write(content)
                        if content and not content.endswith(b"\n"):
                            sys.stdout.buffer.write(b"\n")
                        sys.stdout.buffer.flush()
                        return None
                    else:
                        return content
            elif cmd == "echo":
                # Echo command - output text
                text = " ".join(args)
                output = (text + "\n").encode()
                if is_last:
                    print(text)
                    return None
                else:
                    return output
            elif cmd == "grep":
                # Grep command - filter piped input
                if pipe_input is None:
                    console.print(
                        f"[red]Error: grep requires piped input when used without path[/red]",
                        highlight=False,
                    )
                    return None
                return self._cmd_grep_pipe(args, pipe_input, is_last)
            else:
                console.print(
                    f"[red]Error: command '{cmd}' does not support piping[/red]",
                    highlight=False,
                )
                return None

        except Exception as e:
            console.print(
                f"[red]Error executing '{cmd_str}': {e}[/red]", highlight=False
            )
            return None

    def _parse_redirections(self, cmd_str: str):
        """Parse command string to extract redirection chain.

        Args:
            cmd_str: Command string that may contain redirections

        Returns:
            dict or None: {'command': str, 'redirects': [{'path': str, 'append': bool}]}
                         Returns None if no redirections found
        """
        # Find all redirection operators (> and >>), respecting quotes
        redirects = []
        current_pos = 0
        in_quotes = False
        quote_char = None
        command_part = []
        i = 0

        while i < len(cmd_str):
            char = cmd_str[i]

            # Handle quotes
            if char in ('"', "'") and (i == 0 or cmd_str[i - 1] != "\\"):
                if not in_quotes:
                    in_quotes = True
                    quote_char = char
                elif char == quote_char:
                    in_quotes = False
                    quote_char = None
                command_part.append(char)
                i += 1
                continue

            # Check for redirection operators outside quotes
            if not in_quotes:
                # Check for >>
                if i < len(cmd_str) - 1 and cmd_str[i : i + 2] == ">>":
                    # Found append redirection
                    cmd_before = "".join(command_part).strip()
                    if not redirects:
                        # First redirection, save command
                        command = cmd_before
                    # Find the path after >>
                    i += 2
                    # Skip whitespace
                    while i < len(cmd_str) and cmd_str[i] in " \t":
                        i += 1
                    # Extract path (until next > or end)
                    path_chars = []
                    while i < len(cmd_str):
                        if cmd_str[i] == ">" and (i == 0 or cmd_str[i - 1] != ">"):
                            break
                        path_chars.append(cmd_str[i])
                        i += 1
                    path = "".join(path_chars).strip()
                    redirects.append({"path": path, "append": True})
                    command_part = []
                    continue
                # Check for single >
                elif cmd_str[i] == ">":
                    # Found write redirection
                    cmd_before = "".join(command_part).strip()
                    if not redirects:
                        # First redirection, save command
                        command = cmd_before
                    # Find the path after >
                    i += 1
                    # Skip whitespace
                    while i < len(cmd_str) and cmd_str[i] in " \t":
                        i += 1
                    # Extract path (until next > or end)
                    path_chars = []
                    while i < len(cmd_str):
                        if cmd_str[i] == ">":
                            break
                        path_chars.append(cmd_str[i])
                        i += 1
                    path = "".join(path_chars).strip()
                    redirects.append({"path": path, "append": False})
                    command_part = []
                    continue

            command_part.append(char)
            i += 1

        if not redirects:
            return None

        return {"command": command, "redirects": redirects}

    def _execute_redirect_chain(self, redirect_chain: dict, is_last: bool = False):
        """Execute a chain of redirections.

        Args:
            redirect_chain: Parsed redirection chain from _parse_redirections
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: Final output, or None if last command
        """
        try:
            command = redirect_chain["command"]
            redirects = redirect_chain["redirects"]

            # Execute the initial command to get content
            parts = shlex.split(command)
            if not parts:
                return None

            cmd = parts[0].lower()
            args = parts[1:]

            # Generate initial content
            if cmd == "echo":
                current_content = (" ".join(args) + "\n").encode()
            else:
                console.print(
                    f"[red]Error: command '{cmd}' not supported in redirection chain[/red]",
                    highlight=False,
                )
                return None

            # Execute each redirection in sequence
            for i, redirect in enumerate(redirects):
                path = self._resolve_path(redirect["path"])
                append = redirect["append"]

                # Handle append mode
                if append:
                    try:
                        existing = self.client.cat(path)
                        write_content = existing + current_content
                    except Exception:
                        write_content = current_content
                else:
                    write_content = current_content

                # Write and get response
                try:
                    response = self.client.write(path, write_content)
                except Exception as write_error:
                    console.print(
                        f"[red]Error writing to {path}: {write_error}[/red]",
                        highlight=False,
                    )
                    if i < len(redirects) - 1:
                        console.print(
                            f"[yellow]Chain stopped at redirect {i + 1}[/yellow]",
                            highlight=False,
                        )
                    return None

                # Convert response to bytes for next iteration
                if response:
                    if isinstance(response, str):
                        current_content = response.encode()
                    elif isinstance(response, bytes):
                        current_content = response
                    else:
                        current_content = str(response).encode()
                else:
                    # No response from write
                    if i < len(redirects) - 1:
                        # Not the last redirect, but no response - we can't continue the chain
                        console.print(
                            f"[red]Error: Write to {path} succeeded but returned no response. Cannot continue chain to next redirect.[/red]",
                            highlight=False,
                        )
                        console.print(
                            f"[red]Chain stopped at redirect {i + 1}/{len(redirects)}[/red]",
                            highlight=False,
                        )
                        return None
                    else:
                        # Last redirect, no response is OK
                        current_content = None

            # Output or return final result
            # For redirect chains, we don't output to stdout (silent execution)
            # Only return content if this is part of a pipeline
            if current_content and not is_last:
                # This redirect chain is part of a pipeline, return content to next command
                return current_content
            else:
                # Silent execution - don't output anything
                return None

        except Exception as e:
            console.print(
                f"[red]Error in redirection chain: {e}[/red]", highlight=False
            )
            return None

    def _execute_with_redirection(
        self, cmd_part: str, dest_path: str, append: bool = False, is_last: bool = False
    ):
        """Execute a command with redirection and return the write response.

        Args:
            cmd_part: The command part before redirection (e.g., "echo 'select 1+1'")
            dest_path: The destination path for redirection
            append: Whether to append (>>) or overwrite (>)
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: Response from write operation, or None if failed
        """
        try:
            # Parse the command
            parts = shlex.split(cmd_part)
            if not parts:
                return None

            cmd = parts[0].lower()
            args = parts[1:]

            # Currently only support echo with redirection in pipeline
            if cmd == "echo":
                content = " ".join(args)
                content_bytes = (content + "\n").encode()
            else:
                console.print(
                    f"[red]Error: command '{cmd}' with redirection not supported in pipeline[/red]",
                    highlight=False,
                )
                return None

            # Resolve the destination path
            resolved_dest = self._resolve_path(dest_path)

            # Handle append mode (Unix standard behavior)
            if append:
                try:
                    existing = self.client.cat(resolved_dest)
                    content_bytes = existing + content_bytes
                except:
                    # File doesn't exist, just use content_bytes
                    pass

            # Write and get response
            response = self.client.write(resolved_dest, content_bytes)

            # Convert response to bytes if it's a string
            if response:
                if isinstance(response, str):
                    response_bytes = response.encode()
                elif isinstance(response, bytes):
                    response_bytes = response
                else:
                    response_bytes = str(response).encode()

                # Output if last command, otherwise return for piping
                if is_last:
                    sys.stdout.buffer.write(response_bytes)
                    if response_bytes and not response_bytes.endswith(b"\n"):
                        sys.stdout.buffer.write(b"\n")
                    sys.stdout.buffer.flush()
                    return None
                else:
                    return response_bytes
            else:
                # No response from write
                return None

        except Exception as e:
            console.print(f"[red]Error in redirection: {e}[/red]", highlight=False)
            return None

    def _normalize_path(self, path: str) -> str:
        """Normalize a path by resolving . and .. components"""
        # Use os.path.normpath to handle .. and . properly
        normalized = os.path.normpath(path)
        # Ensure the path starts with / (Unix-style)
        if not normalized.startswith("/"):
            normalized = "/" + normalized
        # Handle the case where normpath returns just "."
        if normalized == "/.":
            normalized = "/"
        return normalized

    def _resolve_path(self, path: str) -> str:
        """Resolve relative path to absolute path"""
        if path.startswith("/"):
            resolved = path
        elif self.current_path == "/":
            resolved = f"/{path}"
        else:
            resolved = f"{self.current_path}/{path}"
        # Normalize the path to handle .. and .
        return self._normalize_path(resolved)

    def _handle_redirection(
        self, args: List[str], content_getter, cmd_name: str
    ) -> bool:
        """
        Handle > and >> redirection for commands.

        Args:
            args: Command arguments
            content_getter: Function that returns (content: bytes, source_path: str) or None
            cmd_name: Command name for error messages

        Returns:
            True if redirection was handled, False otherwise
        """
        # Check if this is a chained redirection (multiple > or >>)
        redirect_count = args.count(">") + args.count(">>")
        if redirect_count > 1:
            # Construct command string for chained redirection
            # First, get the content before any redirection
            first_redirect_idx = None
            for i, arg in enumerate(args):
                if arg == ">" or arg == ">>":
                    first_redirect_idx = i
                    break

            if first_redirect_idx is not None:
                # Build command string: cmd_name + args_before_redirect + redirections
                args_before_redirect = args[:first_redirect_idx]
                redirect_part = args[first_redirect_idx:]

                # Quote arguments if needed
                quoted_args = [shlex.quote(arg) for arg in args_before_redirect]
                cmd_str = (
                    f"{cmd_name} {' '.join(quoted_args)} {' '.join(redirect_part)}"
                )

                # Parse and execute as redirect chain
                redirect_chain = self._parse_redirections(cmd_str)
                if redirect_chain:
                    self._execute_redirect_chain(redirect_chain, is_last=True)
                    return True

        # Check for >> (append) redirection
        if ">>" in args:
            idx = args.index(">>")
            if idx + 1 < len(args):
                dest_path = self._resolve_path(args[idx + 1])
                try:
                    result = content_getter(args[:idx])
                    if result is None:
                        return True
                    content, source_path = result

                    # Read existing destination content and append (Unix standard behavior)
                    try:
                        existing = self.client.cat(dest_path)
                        new_content = existing + content
                    except:
                        # File doesn't exist, just use source content
                        new_content = content
                    msg = self.client.write(dest_path, new_content)
                    if msg:
                        print(msg)
                except Exception as e:
                    console.print(
                        self._format_error(cmd_name, source_path or dest_path, e)
                    )
            else:
                console.print(
                    f"{cmd_name}: syntax error near unexpected token `newline'"
                )
            return True

        # Check for > (write) redirection
        elif ">" in args:
            idx = args.index(">")
            if idx + 1 < len(args):
                dest_path = self._resolve_path(args[idx + 1])
                try:
                    result = content_getter(args[:idx])
                    if result is None:
                        return True
                    content, source_path = result

                    msg = self.client.write(dest_path, content)
                    if msg:
                        print(msg)
                except Exception as e:
                    console.print(
                        self._format_error(cmd_name, source_path or dest_path, e),
                        highlight=False,
                    )
            else:
                console.print(
                    f"{cmd_name}: syntax error near unexpected token `newline'",
                    highlight=False,
                )
            return True

        return False

    def _format_error(self, cmd: str, path: str, error: Exception) -> str:
        """Format error message in Unix style"""
        # Handle PFSClientError - already formatted nicely
        if isinstance(error, PFSClientError):
            return f"{cmd}: {str(error)}"
        # Handle HTTPError from requests
        elif isinstance(error, requests.exceptions.HTTPError):
            status_code = error.response.status_code
            if status_code == 404:
                return f"{cmd}: {path}: No such file or directory"
            elif status_code == 403:
                return f"{cmd}: {path}: Permission denied"
            elif status_code == 400:
                # Try to extract error message from response
                try:
                    error_data = error.response.json()
                    error_msg = error_data.get("error", str(error))
                    return f"{cmd}: {error_msg}"
                except:
                    return f"{cmd}: {str(error)}"
            elif status_code == 500:
                # For ls command, 500 usually means trying to list a file as a directory
                if cmd == "ls":
                    return f"{cmd}: {path}: Not a directory"
                # Try to extract error message from response
                try:
                    error_data = error.response.json()
                    error_msg = error_data.get("error", "Internal server error")
                    return f"{cmd}: {path}: {error_msg}"
                except:
                    return f"{cmd}: {path}: Internal server error"
            else:
                # Try to extract error message from response
                try:
                    error_data = error.response.json()
                    error_msg = error_data.get("error", str(error))
                    return f"{cmd}: {error_msg}"
                except:
                    return f"{cmd}: {path}: {str(error)}"
        else:
            # Generic error
            return f"{cmd}: {str(error)}"

    def cmd_help(self, args: List[str]) -> bool:
        """Show help information"""
        commands_help = [
            ("", ""),
            ("File System Commands", ""),
            ("  ls [path]", "List directory contents in long format"),
            ("  tree [path] [-L depth]", "Display directory tree structure"),
            ("  cd <path>", "Change current directory"),
            ("  pwd", "Print working directory"),
            ("  cat <file>", "Display file contents"),
            ("  cat --stream <file>", "Stream file contents (for streaming files)"),
            ("  cat <file> > <dest>", "Copy file content to destination"),
            ("  cat <file> >> <dest>", "Append file content to destination"),
            ("  tail [-n N] <file>", "Display last N lines of file (default: 10)"),
            (
                "  echo <text> | tee [-a] <file> [file2...]",
                "Write to file(s) and stdout",
            ),
            ("  write <file> <content>", "Write content to file"),
            ("  write --stream <file>", "Stream write from stdin (use outside REPL)"),
            ("  echo <content> > <file>", "Write content to file"),
            ("  echo <content> >> <file>", "Append content to file"),
            ("  mkdir <dir>", "Create directory"),
            ("  rm [-r] <path>", "Remove file/directory"),
            ("  touch <file>", "Create empty file"),
            ("  stat <path>", "Show file/directory info"),
            ("  cp <src> <dst>", "Copy file"),
            ("  mv <src> <dst>", "Move/rename file"),
            ("  chmod <mode> <path>", "Change permissions (e.g., chmod 755 file)"),
            ("  upload [-r] <local> <pfs>", "Upload local file/dir to PFS"),
            ("  download [-r] <pfs> <local>", "Download PFS file/dir to local"),
            (
                "  tailf [-n N] <file>",
                "Show last N lines, then follow to EOF on changes",
            ),
            (
                "  grep [-r] [-i] [-c] [--stream] <pattern> <path>",
                "Search for pattern in files (regex supported)",
            ),
            ("", ""),
            ("Plugin Management", ""),
            ("  mounts", "List mounted plugins"),
            ("  mount <fstype> <path> [k=v ...]", "Mount plugin dynamically"),
            ("  unmount <path>", "Unmount plugin"),
            ("  plugins", "Show mounted plugins"),
            (
                "  plugins load <lib|url>",
                "Load external plugin from file or HTTP(S) URL",
            ),
            ("  plugins unload <lib>", "Unload external plugin"),
            ("  plugins list", "List loaded external plugins"),
            ("", ""),
            ("Utility Commands", ""),
            ("  clear", "Clear screen"),
            ("  help, ?", "Show this help"),
            ("  exit, quit", "Exit REPL"),
            ("", ""),
            ("Examples", ""),
            ("  echo 'msg' > /mnt/queue/enqueue", "Enqueue a message"),
            ("  cat /mnt/queue/dequeue", "Dequeue a message"),
            ("  cat /mnt/queue/size", "Get queue size"),
            ("  echo 'value' > /mnt/kv/keys/mykey", "Set a key-value"),
            ("  cat /mnt/kv/keys/mykey", "Get a value"),
            ("  ls /mnt/kv/keys", "List all keys"),
            ("", ""),
            ("Pipeline Examples", ""),
            ("  echo 'hello' | tee output.txt", "Write to file and stdout"),
            ("  echo 'world' | tee -a output.txt", "Append to file and stdout"),
            ("  echo 'data' | tee f1.txt f2.txt f3.txt", "Write to multiple files"),
            ("  cat input.txt | tee backup.txt", "Copy file and display"),
            (
                "  echo 'select 1+1' > /sqlfs/query | tee result.txt",
                "Query DB, save and display result",
            ),
            ("", ""),
            ("Chained Redirection Examples", ""),
            (
                "  echo 'select * from users' > query > result.json",
                "Query and chain results",
            ),
            (
                "  echo 'select count(*)' > /sqlfs/q > /s3/backup.txt",
                "Query, save to S3",
            ),
            ("", ""),
            ("Streaming Examples (outside REPL)", ""),
            ("  cat video.mp4 | pfs write --stream /mnt/streamfs/video", ""),
            ("  pfs cat --stream /mnt/streamfs/video | ffplay -", ""),
            (
                "  ffmpeg -i in.mp4 -f mpegts - | pfs write --stream /mnt/streamfs/live",
                "",
            ),
        ]

        console.print("\nPFS CLI Commands\n", highlight=False)
        for cmd, desc in commands_help:
            if not desc:
                # Section header
                console.print(f"[bold]{cmd}[/bold]", highlight=False)
            else:
                # Command with description
                console.print(f"{cmd:<40} {desc}", highlight=False)
        console.print(highlight=False)
        return True

    def cmd_ls(self, args: List[str]) -> bool:
        """List directory contents"""
        # Filter out flags from args
        paths = [arg for arg in args if not arg.startswith("-")]
        path = self._resolve_path(paths[0] if paths else self.current_path)

        try:
            cli_commands.cmd_ls(self.client, path)
        except Exception as e:
            console.print(self._format_error("ls", path, e), highlight=False)
        return True

    def cmd_tree(self, args: List[str]) -> bool:
        """Display directory tree structure"""
        # Parse arguments
        max_depth = None
        path = None

        i = 0
        while i < len(args):
            if args[i] == "-L" and i + 1 < len(args):
                try:
                    max_depth = int(args[i + 1])
                    i += 2
                except ValueError:
                    console.print(
                        f"tree: invalid depth: '{args[i + 1]}'", highlight=False
                    )
                    return True
            elif not args[i].startswith("-"):
                path = args[i]
                i += 1
            else:
                console.print(f"tree: invalid option: '{args[i]}'", highlight=False)
                return True

        # Use current path if no path specified
        if path is None:
            path = self.current_path
        else:
            path = self._resolve_path(path)

        try:
            cli_commands.cmd_tree(self.client, path, max_depth)
        except Exception as e:
            console.print(self._format_error("tree", path, e), highlight=False)
        return True

    def cmd_cd(self, args: List[str]) -> bool:
        """Change directory"""
        if not args:
            self.current_path = "/"
            return True

        path = self._resolve_path(args[0])
        try:
            # Verify path exists and is a directory
            info = self.client.stat(path)
            if info.get("isDir"):
                self.current_path = path
            else:
                console.print(f"cd: not a directory: {path}", highlight=False)
        except Exception as e:
            console.print(f"cd: {path}: No such file or directory", highlight=False)
        return True

    def cmd_pwd(self, args: List[str]) -> bool:
        """Print working directory"""
        console.print(self.current_path, highlight=False)
        return True

    def cmd_cat(self, args: List[str]) -> bool:
        """Display file contents (supports > and >> redirection and --stream)"""
        if not args:
            console.print(
                "Usage: cat [--stream] <file> [> output] [>> output]", highlight=False
            )
            return True

        # Check for --stream flag
        stream = "--stream" in args
        args = [arg for arg in args if arg != "--stream"]

        if not args:
            console.print(
                "Usage: cat [--stream] <file> [> output] [>> output]", highlight=False
            )
            return True

        def cat_content_getter(pre_redirect_args):
            if not pre_redirect_args:
                return None
            source_path = self._resolve_path(pre_redirect_args[0])
            content = self.client.cat(source_path)
            return (content, source_path)

        # Handle redirection if present (note: redirection doesn't work with --stream)
        if self._handle_redirection(args, cat_content_getter, "cat"):
            if stream:
                console.print(
                    "cat: --stream cannot be used with redirection", highlight=False
                )
            return True

        # Normal cat - display to console
        path = self._resolve_path(args[0])
        try:
            cli_commands.cmd_cat(self.client, path, stream=stream)
        except Exception as e:
            console.print(self._format_error("cat", path, e), highlight=False)
        return True

    def cmd_tail(self, args: List[str]) -> bool:
        """Display last N lines of a file"""
        if len(args) < 1:
            console.print("Usage: tail [-n lines] <file>", highlight=False)
            return True

        # Parse arguments
        lines = 10  # Default
        path_arg = None

        i = 0
        while i < len(args):
            if args[i] == "-n" and i + 1 < len(args):
                try:
                    lines = int(args[i + 1])
                    i += 2
                except ValueError:
                    console.print(
                        f"tail: invalid number of lines: '{args[i + 1]}'",
                        highlight=False,
                    )
                    return True
            else:
                path_arg = args[i]
                i += 1

        if not path_arg:
            console.print("Usage: tail [-n lines] <file>", highlight=False)
            return True

        path = self._resolve_path(path_arg)

        try:
            cli_commands.cmd_tail(self.client, path, lines)
        except Exception as e:
            console.print(f"tail: {path_arg}: {e}", highlight=False)
        return True

    def cmd_write(self, args: List[str]) -> bool:
        """Write content to file or stream from stdin (with --stream)"""
        if not args:
            console.print("Usage: write [--stream] <file> [<content>]", highlight=False)
            console.print(
                "       write --stream <file>   # Read from stdin", highlight=False
            )
            console.print(
                "       write <file> <content>  # Write content directly",
                highlight=False,
            )
            return True

        # Check for --stream flag
        stream = "--stream" in args
        args = [arg for arg in args if arg != "--stream"]

        if not args:
            console.print("Usage: write [--stream] <file> [<content>]", highlight=False)
            return True

        path = self._resolve_path(args[0])

        try:
            if stream:
                # Streaming mode - note: this won't work well in REPL since stdin is used for prompt
                console.print(
                    "[yellow]Note: --stream is not available in REPL mode[/yellow]",
                    highlight=False,
                )
                console.print(
                    "[yellow]Use: cat file.dat | pfs write --stream /mnt/streamfs/stream[/yellow]",
                    highlight=False,
                )
                console.print(
                    "[yellow]Or use shell redirection: command | pfs write --stream <path>[/yellow]",
                    highlight=False,
                )
            else:
                # Normal mode
                if len(args) < 2:
                    console.print("Usage: write <file> <content>", highlight=False)
                    return True

                content = " ".join(args[1:])
                msg = self.client.write(path, content.encode())
                if msg:
                    print(msg)
        except Exception as e:
            console.print(self._format_error("write", path, e), highlight=False)
        return True

    def cmd_echo(self, args: List[str]) -> bool:
        """Echo content (can redirect to file with > or append with >>)"""

        def echo_content_getter(pre_redirect_args):
            content = " ".join(pre_redirect_args)
            # Echo always adds newline
            return ((content + "\n").encode(), None)

        # Handle redirection if present
        if self._handle_redirection(args, echo_content_getter, "echo"):
            return True

        # No redirection - print to console
        print(" ".join(args))
        return True

    def cmd_mkdir(self, args: List[str]) -> bool:
        """Create directory"""
        if not args:
            console.print("Usage: mkdir <directory>", highlight=False)
            return True

        path = self._resolve_path(args[0])
        try:
            cli_commands.cmd_mkdir(self.client, path)
        except Exception as e:
            console.print(self._format_error("mkdir", path, e), highlight=False)
        return True

    def cmd_rm(self, args: List[str]) -> bool:
        """Remove file or directory"""
        if not args:
            console.print("Usage: rm <path> [-r]", highlight=False)
            return True

        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]
        if not path_args:
            console.print("Usage: rm <path> [-r]", highlight=False)
            return True

        path = self._resolve_path(path_args[0])
        try:
            cli_commands.cmd_rm(self.client, path, recursive)
        except Exception as e:
            console.print(self._format_error("rm", path, e), highlight=False)
        return True

    def cmd_touch(self, args: List[str]) -> bool:
        """Create empty file"""
        if not args:
            console.print("Usage: touch <file>", highlight=False)
            return True

        path = self._resolve_path(args[0])
        try:
            cli_commands.cmd_touch(self.client, path)
        except Exception as e:
            console.print(self._format_error("touch", path, e), highlight=False)
        return True

    def cmd_stat(self, args: List[str]) -> bool:
        """Show file/directory information"""
        if not args:
            console.print("Usage: stat <path>", highlight=False)
            return True

        path = self._resolve_path(args[0])
        try:
            cli_commands.cmd_stat(self.client, path)
        except Exception as e:
            console.print(self._format_error("stat", path, e), highlight=False)
        return True

    def cmd_cp(self, args: List[str]) -> bool:
        """Copy file or directory"""
        if len(args) < 2:
            console.print("Usage: cp [-r] <source> <destination>", highlight=False)
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print("Usage: cp [-r] <source> <destination>", highlight=False)
            return True

        src = self._resolve_path(path_args[0])
        dst = self._resolve_path(path_args[1])

        # Check if source exists and get its info
        try:
            src_stat = self.client.stat(src)
        except Exception as e:
            console.print(self._format_error("cp", path_args[0], e), highlight=False)
            return True

        # Check if source is a directory
        if src_stat.get("isDir"):
            if not recursive:
                console.print(
                    f"cp: {path_args[0]}: is a directory (not copied)", highlight=False
                )
                return True
            # Recursive copy of directory
            self._copy_directory(src, dst, path_args[0])
            return True

        # Source is a file
        try:
            # Check if destination is a directory
            try:
                dst_stat = self.client.stat(dst)
                if dst_stat.get("isDir"):
                    # Destination is a directory, append source filename
                    src_filename = os.path.basename(src.rstrip("/"))
                    dst = f"{dst.rstrip('/')}/{src_filename}"
            except:
                # Destination doesn't exist, that's fine
                pass

            # Read source file and write to destination
            content = self.client.cat(src)
            self.client.write(dst, content)
        except Exception as e:
            console.print(self._format_error("cp", path_args[0], e), highlight=False)
        return True

    def _copy_directory(self, src_dir: str, dst_dir: str, original_src: str):
        """Copy a directory recursively"""
        # Get the source directory name
        src_dir_name = os.path.basename(src_dir.rstrip("/"))

        # Check if destination exists and is a directory
        try:
            dst_stat = self.client.stat(dst_dir)
            if dst_stat.get("isDir"):
                # Destination is a directory, create subdirectory with source name
                dst_dir = f"{dst_dir.rstrip('/')}/{src_dir_name}"
        except:
            # Destination doesn't exist, that's fine
            pass

        # Create the destination directory
        try:
            self.client.mkdir(dst_dir)
            console.print(f"Created directory {dst_dir}/", highlight=False)
        except Exception as e:
            # Directory might already exist
            pass

        total_files = 0
        total_bytes = 0

        # Queue for BFS traversal
        from collections import deque

        queue = deque([(src_dir, dst_dir)])

        while queue:
            current_src, current_dst = queue.popleft()

            # List directory contents
            try:
                files = self.client.ls(current_src)
            except Exception as e:
                console.print(self._format_error("cp", current_src, e), highlight=False)
                continue

            for file_info in files:
                file_name = file_info.get("name", "")
                is_dir = file_info.get("isDir", False)

                src_path = f"{current_src.rstrip('/')}/{file_name}"
                dst_path = f"{current_dst.rstrip('/')}/{file_name}"

                if is_dir:
                    # Create subdirectory and add to queue
                    try:
                        self.client.mkdir(dst_path)
                        console.print(f"Created directory {dst_path}/", highlight=False)
                        queue.append((src_path, dst_path))
                    except Exception as e:
                        console.print(
                            self._format_error("cp", src_path, e), highlight=False
                        )
                else:
                    # Copy file
                    try:
                        content = self.client.cat(src_path)
                        self.client.write(dst_path, content)

                        size = len(content)
                        total_files += 1
                        total_bytes += size

                        # Print progress for each file
                        if size < 1024:
                            print(f"  {src_path} -> {dst_path} ({size} bytes)")
                        elif size < 1024 * 1024:
                            kb = size / 1024
                            print(f"  {src_path} -> {dst_path} ({kb:.2f} KB)")
                        else:
                            mb = size / (1024 * 1024)
                            print(f"  {src_path} -> {dst_path} ({mb:.2f} MB)")
                    except Exception as e:
                        console.print(
                            self._format_error("cp", src_path, e), highlight=False
                        )

        # Print summary
        if total_files > 0:
            if total_bytes < 1024:
                console.print(
                    f"\nCopied {total_files} files, {total_bytes} bytes total",
                    highlight=False,
                )
            elif total_bytes < 1024 * 1024:
                kb = total_bytes / 1024
                console.print(
                    f"\nCopied {total_files} files, {kb:.2f} KB total", highlight=False
                )
            else:
                mb = total_bytes / (1024 * 1024)
                console.print(
                    f"\nCopied {total_files} files, {mb:.2f} MB total", highlight=False
                )

    def cmd_mv(self, args: List[str]) -> bool:
        """Move/rename file"""
        if len(args) < 2:
            console.print("Usage: mv <source> <destination>", highlight=False)
            return True

        src = self._resolve_path(args[0])
        dst = self._resolve_path(args[1])
        try:
            cli_commands.cmd_mv(self.client, src, dst)
        except Exception as e:
            console.print(self._format_error("mv", src, e), highlight=False)
        return True

    def cmd_chmod(self, args: List[str]) -> bool:
        """Change file permissions"""
        if len(args) < 2:
            console.print("Usage: chmod <mode> <path>", highlight=False)
            return True

        try:
            mode = int(args[0], 8)
            path = self._resolve_path(args[1])
            cli_commands.cmd_chmod(self.client, mode, path)
        except ValueError:
            console.print(f"chmod: invalid mode: '{args[0]}'", highlight=False)
        except Exception as e:
            if len(args) > 1:
                console.print(self._format_error("chmod", args[1], e), highlight=False)
            else:
                console.print(f"chmod: {e}", highlight=False)
        return True

    def cmd_mounts(self, args: List[str]) -> bool:
        """List mounted plugins"""
        try:
            cli_commands.cmd_mounts(self.client)
        except Exception as e:
            console.print(f"mounts: {e}", highlight=False)
        return True

    def cmd_unmount(self, args: List[str]) -> bool:
        """Unmount a plugin"""
        if not args:
            console.print("Usage: unmount <path>", highlight=False)
            return True

        path = args[0]
        try:
            cli_commands.cmd_unmount(self.client, path)
        except Exception as e:
            console.print(f"unmount: {e}", highlight=False)
        return True

    def cmd_mount(self, args: List[str]) -> bool:
        """Mount a plugin dynamically"""
        if len(args) < 2:
            console.print(
                "Usage: mount <fstype> <path> [key=value ...]", highlight=False
            )
            console.print("\nExamples:", highlight=False)
            console.print("  mount memfs /test/mem", highlight=False)
            console.print(
                "  mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db",
                highlight=False,
            )
            console.print(
                "  mount s3fs /test/s3 bucket=my-bucket region=us-west-1 access_key_id=xxx secret_access_key=yyy",
                highlight=False,
            )
            return True

        fstype = args[0]
        path = args[1]
        config_args = args[2:] if len(args) > 2 else []

        try:
            cli_commands.cmd_mount(self.client, fstype, path, config_args)
        except Exception as e:
            console.print(f"mount: {e}", highlight=False)
        return True

    def cmd_plugins(self, args: List[str]) -> bool:
        """Show mounted plugins (alias for mounts) or manage external plugins"""
        # If there are no args, show mounted plugins (alias for mounts)
        if not args:
            return self.cmd_mounts(args)

        # Otherwise, handle plugin subcommands
        subcommand = args[0].lower()

        if subcommand == "load":
            if len(args) < 2:
                console.print("Usage: plugins load <library_path|url>", highlight=False)
                console.print("\nExamples:", highlight=False)
                console.print(
                    "  plugins load ./examples/plugins/hellofs-c/hellofs-c.dylib",
                    highlight=False,
                )
                console.print(
                    "  plugins load http://example.com/plugins/myplugin.so",
                    highlight=False,
                )
                console.print(
                    "  plugins load https://example.com/plugins/myplugin.dylib",
                    highlight=False,
                )
                return True
            library_path = args[1]
            try:
                cli_commands.cmd_load_plugin(self.client, library_path)
            except Exception as e:
                console.print(f"[red]Error loading plugin: {e}[/red]", highlight=False)
            return True

        elif subcommand == "unload":
            if len(args) < 2:
                console.print("Usage: plugins unload <library_path>", highlight=False)
                return True
            library_path = args[1]
            try:
                cli_commands.cmd_unload_plugin(self.client, library_path)
            except Exception as e:
                console.print(
                    f"[red]Error unloading plugin: {e}[/red]", highlight=False
                )
            return True

        elif subcommand == "list":
            try:
                cli_commands.cmd_list_plugins(self.client)
            except Exception as e:
                console.print(f"[red]Error listing plugins: {e}[/red]", highlight=False)
            return True

        else:
            console.print(
                f"[red]Unknown subcommand: {subcommand}[/red]", highlight=False
            )
            console.print("\nUsage:", highlight=False)
            console.print("  plugins           - List mounted plugins", highlight=False)
            console.print(
                "  plugins load <library_path>   - Load external plugin",
                highlight=False,
            )
            console.print(
                "  plugins unload <library_path> - Unload external plugin",
                highlight=False,
            )
            console.print(
                "  plugins list      - List loaded external plugins", highlight=False
            )
            return True

    def cmd_upload(self, args: List[str]) -> bool:
        """Upload local file or directory to PFS"""
        if len(args) < 2:
            console.print("Usage: upload [-r] <local_path> <pfs_path>", highlight=False)
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print("Usage: upload [-r] <local_path> <pfs_path>", highlight=False)
            return True

        local_path = path_args[0]
        pfs_path = self._resolve_path(path_args[1])

        try:
            cli_commands.cmd_upload(self.client, local_path, pfs_path, recursive)
        except Exception as e:
            console.print(f"upload: {e}", highlight=False)
        return True

    def cmd_download(self, args: List[str]) -> bool:
        """Download file or directory from PFS to local filesystem"""
        if len(args) < 2:
            console.print(
                "Usage: download [-r] <pfs_path> <local_path>", highlight=False
            )
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print(
                "Usage: download [-r] <pfs_path> <local_path>", highlight=False
            )
            return True

        pfs_path = self._resolve_path(path_args[0])
        local_path = path_args[1]

        try:
            cli_commands.cmd_download(self.client, pfs_path, local_path, recursive)
        except Exception as e:
            console.print(f"download: {e}", highlight=False)
        return True

    def cmd_tailf(self, args: List[str]) -> bool:
        """Follow file changes (show last N lines, then follow new content to EOF)"""
        if len(args) < 1:
            console.print("Usage: tailf [-n lines] <file>", highlight=False)
            return True

        # Parse arguments
        lines = 10  # Default
        path_arg = None

        i = 0
        while i < len(args):
            if args[i] == "-n" and i + 1 < len(args):
                try:
                    lines = int(args[i + 1])
                    i += 2
                except ValueError:
                    console.print(
                        f"tailf: invalid number of lines: '{args[i + 1]}'",
                        highlight=False,
                    )
                    return True
            else:
                path_arg = args[i]
                i += 1

        if not path_arg:
            console.print("Usage: tailf [-n lines] <file>", highlight=False)
            return True

        path = self._resolve_path(path_arg)

        try:
            cli_commands.cmd_tailf(self.client, path, lines)
        except Exception as e:
            console.print(f"tailf: {path_arg}: {e}", highlight=False)
        return True

    def cmd_watch(self, args: List[str]) -> bool:
        """Watch a command - execute it repeatedly at intervals"""
        if not args:
            console.print(
                "Usage: watch [-n seconds] <command> [args...]", highlight=False
            )
            console.print("Examples:", highlight=False)
            console.print("  watch ls /queuefs", highlight=False)
            console.print("  watch -n 1 cat /serverinfofs/uptime", highlight=False)
            console.print("  watch -n 0.5 stat /memfs/file.txt", highlight=False)
            return True

        # Parse -n option
        interval = 2.0  # Default interval
        command_start_idx = 0

        if args[0] == "-n" and len(args) >= 3:
            try:
                interval = float(args[1])
                if interval <= 0:
                    console.print(
                        "[red]Error: interval must be positive[/red]", highlight=False
                    )
                    return True
                command_start_idx = 2
            except ValueError:
                console.print(
                    f"[red]Error: invalid interval: {args[1]}[/red]", highlight=False
                )
                return True

        # Get command and its arguments
        if command_start_idx >= len(args):
            console.print("[red]Error: no command specified[/red]", highlight=False)
            return True

        cmd_name = args[command_start_idx].lower()
        cmd_args = args[command_start_idx + 1 :]

        # Map command names to functions
        command_map = {
            "ls": cli_commands.cmd_ls,
            "cat": cli_commands.cmd_cat,
            "stat": cli_commands.cmd_stat,
            "mounts": cli_commands.cmd_mounts,
        }

        if cmd_name not in command_map:
            console.print(
                f"[red]Error: command '{cmd_name}' not supported in watch[/red]",
                highlight=False,
            )
            console.print("Supported commands: ls, cat, stat, mounts", highlight=False)
            return True

        # Resolve paths in arguments
        resolved_args = []
        for arg in cmd_args:
            if not arg.startswith("-"):
                resolved_args.append(self._resolve_path(arg))
            else:
                resolved_args.append(arg)

        # Execute watch
        try:
            command_func = command_map[cmd_name]
            cli_commands.cmd_watch(self.client, command_func, resolved_args, interval)
        except Exception as e:
            console.print(f"[red]watch: {e}[/red]", highlight=False)

        return True

    def cmd_tee(self, args: List[str]) -> bool:
        """Tee command - read from stdin and write to file and stdout (REPL mode)"""
        console.print(
            "[yellow]Note: tee command requires piped input[/yellow]", highlight=False
        )
        console.print("Usage:", highlight=False)
        console.print("  echo 'hello' | tee output.txt", highlight=False)
        console.print(
            "  echo 'hello' | tee file1.txt file2.txt file3.txt", highlight=False
        )
        console.print("  echo 'world' | tee -a output.txt", highlight=False)
        console.print("  cat file.txt | tee backup.txt", highlight=False)
        return True

    def _cmd_tee_pipe(self, args: List[str], pipe_input: bytes, is_last: bool):
        """Tee command for pipeline - write input to file(s) and pass through to stdout.

        Args:
            args: List of file paths to write to (supports -a flag for append mode)
            pipe_input: Input bytes from previous command
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: The input bytes (passed through), or None if last command
        """
        if not args:
            console.print("[red]tee: missing file argument[/red]", highlight=False)
            return None

        if pipe_input is None:
            console.print("[red]tee: no input provided[/red]", highlight=False)
            return None

        # Parse arguments to check for -a flag
        append_mode = False
        file_paths = []

        for arg in args:
            if arg == "-a" or arg == "--append":
                append_mode = True
            elif not arg.startswith("-"):
                file_paths.append(arg)
            else:
                console.print(
                    f"[yellow]tee: unknown option: {arg}[/yellow]", highlight=False
                )

        if not file_paths:
            console.print("[red]tee: missing file argument[/red]", highlight=False)
            return None

        # Write to each file specified
        for file_arg in file_paths:
            try:
                file_path = self._resolve_path(file_arg)

                if append_mode:
                    # Append mode: read existing content and append (Unix standard behavior)
                    try:
                        existing_content = self.client.cat(file_path)
                        new_content = existing_content + pipe_input
                    except:
                        # File doesn't exist, just use pipe_input
                        new_content = pipe_input
                    self.client.write(file_path, new_content)
                else:
                    # Write mode: overwrite file
                    self.client.write(file_path, pipe_input)

            except Exception as e:
                console.print(f"[red]tee: {file_arg}: {e}[/red]", highlight=False)
                # Continue to other files even if one fails

        # Output to stdout if this is the last command, otherwise pass through
        if is_last:
            sys.stdout.buffer.write(pipe_input)
            if pipe_input and not pipe_input.endswith(b"\n"):
                sys.stdout.buffer.write(b"\n")
            sys.stdout.buffer.flush()
            return None
        else:
            return pipe_input

    def _cmd_grep_pipe(self, args: List[str], pipe_input: bytes, is_last: bool):
        """Grep command for pipeline - filter input by pattern.

        Args:
            args: Pattern and flags (e.g., ['-i', 'pattern'])
            pipe_input: Input bytes from previous command
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: Filtered output, or None if last command
        """
        if pipe_input is None:
            console.print("[red]grep: no input provided[/red]", highlight=False)
            return None

        # Parse flags
        case_insensitive = False
        count_only = False
        invert_match = False
        remaining_args = []

        for arg in args:
            if arg in ["-i", "--ignore-case"]:
                case_insensitive = True
            elif arg in ["-c", "--count"]:
                count_only = True
            elif arg in ["-v", "--invert-match"]:
                invert_match = True
            elif not arg.startswith("-"):
                remaining_args.append(arg)
            else:
                console.print(
                    f"[yellow]grep: unknown option: {arg}[/yellow]", highlight=False
                )

        if not remaining_args:
            console.print("[red]grep: missing pattern[/red]", highlight=False)
            return None

        pattern = remaining_args[0]

        # Compile regex pattern
        try:
            flags = re.IGNORECASE if case_insensitive else 0
            regex = re.compile(pattern, flags)
        except re.error as e:
            console.print(f"[red]grep: invalid pattern: {e}[/red]", highlight=False)
            return None

        # Decode input and split into lines
        try:
            text = pipe_input.decode("utf-8")
        except UnicodeDecodeError:
            # Try latin-1 as fallback
            try:
                text = pipe_input.decode("latin-1")
            except Exception as e:
                console.print(
                    f"[red]grep: failed to decode input: {e}[/red]", highlight=False
                )
                return None

        lines = text.splitlines(keepends=True)

        # Filter lines
        matching_lines = []
        for line in lines:
            matches = bool(regex.search(line))
            # Apply invert match if specified
            if invert_match:
                matches = not matches
            if matches:
                matching_lines.append(line)

        # Handle output
        if count_only:
            # Count mode: just output the count
            count_str = f"{len(matching_lines)}\n"
            if is_last:
                print(len(matching_lines))
                return None
            else:
                return count_str.encode("utf-8")
        else:
            # Normal mode: output matching lines
            output = "".join(matching_lines)
            if is_last:
                if output:
                    sys.stdout.write(output)
                    if not output.endswith("\n"):
                        sys.stdout.write("\n")
                sys.stdout.flush()
                return None
            else:
                return output.encode("utf-8")

    def cmd_grep(self, args: List[str]) -> bool:
        """Search for pattern in files using regular expressions

        Usage:
            grep [-r] [-i] [-c] [--stream] PATTERN PATH
            ... | grep [-i] [-c] [-v] PATTERN

        Options:
            -r, --recursive      Search recursively in directories (file mode only)
            -i, --ignore-case    Case-insensitive search
            -c, --count          Only print count of matches
            -v, --invert-match   Invert match (select non-matching lines, pipe mode only)
            --stream             Stream results as they are found (file mode only)

        PATH supports wildcards (* and ?):
            *.log              All .log files in current directory
            /var/log/*.log     All .log files in /var/log
            data/*.txt         All .txt files in data directory

        Examples:
            # File mode:
            grep "error" /local/logs/app.log
            grep -i "error|warning" /var/log/*.log
            grep -r "test" /local/test-grep
            grep -i "ERROR" /local/logs
            grep -r -i "warning|error" logs/
            grep --stream -r "pattern" /large/directory

            # Pipe mode (local filtering):
            cat /local/file.txt | grep "error"
            cat /local/logs/app.log | grep -i "error|warning"
            echo "test error" | grep -v "error"
            cat /local/file.txt | grep -c "pattern"
        """
        if not args:
            console.print(
                "Usage: grep [-r] [-i] [-c] [--stream] PATTERN PATH", highlight=False
            )
            console.print("       ... | grep [-i] [-c] [-v] PATTERN", highlight=False)
            return True

        # Parse flags
        recursive = False
        case_insensitive = False
        count_only = False
        stream = False
        remaining_args = []

        for arg in args:
            if arg in ["-r", "--recursive"]:
                recursive = True
            elif arg in ["-i", "--ignore-case"]:
                case_insensitive = True
            elif arg in ["-c", "--count"]:
                count_only = True
            elif arg == "--stream":
                stream = True
            elif not arg.startswith("-"):
                remaining_args.append(arg)
            else:
                console.print(f"grep: unknown option: {arg}", highlight=False)
                return True

        if len(remaining_args) < 2:
            console.print(
                "Usage: grep [-r] [-i] [-c] [--stream] PATTERN PATH", highlight=False
            )
            return True

        pattern = remaining_args[0]
        path = self._resolve_path(remaining_args[1])

        try:
            cli_commands.cmd_grep(
                self.client,
                path,
                pattern,
                recursive=recursive,
                case_insensitive=case_insensitive,
                count_only=count_only,
                stream=stream,
            )
        except Exception as e:
            console.print(self._format_error("grep", path, e), highlight=False)

        return True

    def cmd_clear(self, args: List[str]) -> bool:
        """Clear screen"""
        console.clear()
        return True

    def cmd_exit(self, args: List[str]) -> bool:
        """Exit REPL"""
        console.print("Goodbye!", highlight=False)
        return False
