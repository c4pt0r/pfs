"""REPL Command Handlers"""

import os
import re
import shlex
import sys
from abc import ABC, abstractmethod
from typing import List, Optional

import requests
from rich.console import Console

from . import cli_commands
from pyagfs import AGFSClientError

console = Console()


class PipelineCommand(ABC):
    """Base class for commands that support pipeline operations.

    Pipeline-aware commands can:
    - Accept input from previous commands in a pipeline (pipe_input)
    - Generate output for next commands in a pipeline
    - Support output redirection (> and >>)
    """

    def __init__(self, handler):
        """Initialize with reference to CommandHandler.

        Args:
            handler: CommandHandler instance for accessing client and utilities
        """
        self.handler = handler

    @abstractmethod
    def execute(
        self, args: List[str], pipe_input: Optional[bytes] = None, is_last: bool = False
    ) -> Optional[bytes]:
        """Execute the command with optional piped input.

        Args:
            args: Command arguments (not including the command name itself)
            pipe_input: Input bytes from previous command in pipeline (None if no input)
            is_last: Whether this is the last command in pipeline (affects output behavior)

        Returns:
            bytes: Output to pass to next command in pipeline
            None: If this is the last command or command failed
        """
        pass

    def supports_pipe_input(self) -> bool:
        """Whether this command can accept input from a pipe."""
        return True

    def supports_pipe_output(self) -> bool:
        """Whether this command can output to a pipe."""
        return True

    def supports_redirection(self) -> bool:
        """Whether this command supports output redirection (> and >>)."""
        return True


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
            "digest": self.cmd_digest,
            # Utility commands
            "clear": self.cmd_clear,
            "exit": self.cmd_exit,
            "quit": self.cmd_exit,
        }

        # Registry of pipeline-aware commands (used in pipeline execution)
        self.pipeline_commands = {
            "tee": TeeCommand(self),
            "echo": EchoCommand(self),
            "cat": CatCommand(self),
            "grep": GrepCommand(self),
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

        This is the new unified implementation using the PipelineCommand abstraction.

        Args:
            cmd_str: Command string to execute
            pipe_input: Input bytes from previous command in pipeline
            is_last: Whether this is the last command in the pipeline

        Returns:
            bytes: Output of the command, or None if command failed
        """
        try:
            # Parse command string to check for redirections
            redirect_chain = self._parse_redirections(cmd_str)

            # Extract command name and arguments
            if redirect_chain:
                # Command has redirections, parse from redirect_chain
                command_str = redirect_chain["command"]
                redirects = redirect_chain["redirects"]
                parts = shlex.split(command_str)
            else:
                # No redirections, parse directly
                parts = shlex.split(cmd_str)
                redirects = None

            if not parts:
                return None

            cmd = parts[0].lower()
            args = parts[1:]

            # Check if command is in pipeline registry
            if cmd in self.pipeline_commands:
                pipeline_cmd = self.pipeline_commands[cmd]

                # Execute the command to get output
                output = pipeline_cmd.execute(args, pipe_input, is_last=(is_last and not redirects))

                # If there are redirections, apply them to the output
                if redirects and output:
                    return self._apply_redirections(output, redirects, is_last)
                else:
                    return output
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

    def _apply_redirections(
        self, content: bytes, redirects: List[dict], is_last: bool = False
    ) -> Optional[bytes]:
        """Apply a chain of redirections to content.

        This is the unified redirection handler used by all commands.

        Args:
            content: The content to redirect (as bytes)
            redirects: List of redirect specs [{'path': str, 'append': bool}, ...]
            is_last: Whether this is the last command in pipeline

        Returns:
            bytes: Final output after all redirections (for pipeline continuation)
            None: If this is the last command or an error occurred
        """
        if not content:
            return None

        current_content = content

        for i, redirect in enumerate(redirects):
            path = self._resolve_path(redirect["path"])
            append = redirect["append"]

            # Handle append mode
            if append:
                try:
                    existing = self.client.cat(path)
                    write_content = existing + current_content
                except Exception:
                    # File doesn't exist, just use current content
                    write_content = current_content
            else:
                write_content = current_content

            # Write and get response
            try:
                response = self.client.write(path, write_content)

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
                        # Not the last redirect, but no response - can't continue chain
                        console.print(
                            f"[red]Error: Write to {path} succeeded but returned no response. Cannot continue chain.[/red]",
                            highlight=False,
                        )
                        console.print(
                            f"[red]Chain stopped at redirect {i + 1}/{len(redirects)}[/red]",
                            highlight=False,
                        )
                        return None
                    else:
                        # Last redirect, no response is OK
                        current_content = write_content

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

        # Return final content for pipeline continuation, or None if last command
        if is_last:
            return None
        else:
            return current_content

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

    def _expand_wildcards(self, path: str, include_dirs: bool = True) -> List[str]:
        """Expand wildcards in path to list of matching files/directories.

        Args:
            path: Path that may contain wildcards (* or ?)
            include_dirs: Whether to include directories in results (default: True)

        Returns:
            List of absolute paths matching the pattern.
            If no wildcards, returns [path].
            If wildcards but no matches, returns empty list.
        """
        import fnmatch

        # Resolve relative paths first (always, even without wildcards)
        resolved_path = self._resolve_path(path)

        # Check if path contains wildcards
        if '*' not in path and '?' not in path:
            # No wildcards, return resolved path
            return [resolved_path]

        # Split into directory and pattern parts
        dir_path = os.path.dirname(resolved_path)
        file_pattern = os.path.basename(resolved_path)

        # If directory part also has wildcards, we don't support that (yet)
        if '*' in dir_path or '?' in dir_path:
            console.print(
                f"[yellow]Warning: Wildcards in directory path not supported: {dir_path}[/yellow]",
                highlight=False
            )
            return []

        try:
            # List directory
            files = self.client.ls(dir_path or '/')

            # Filter files matching the pattern
            matched_paths = []
            for file_info in files:
                if fnmatch.fnmatch(file_info['name'], file_pattern):
                    # Include based on type
                    if include_dirs or not file_info.get('isDir', False):
                        full_path = os.path.join(dir_path or '/', file_info['name'])
                        matched_paths.append(full_path)

            return sorted(matched_paths)

        except Exception as e:
            console.print(
                f"[yellow]Warning: Failed to expand wildcards in {path}: {e}[/yellow]",
                highlight=False
            )
            return []

    def _expand_patterns(
        self,
        patterns: List[str],
        include_dirs: bool = True,
        on_no_match: str = "error",
        cmd_name: str = "command"
    ) -> Optional[List[str]]:
        """Expand multiple wildcard patterns with consistent error handling.

        Args:
            patterns: List of patterns that may contain wildcards
            include_dirs: Whether to include directories in results
            on_no_match: How to handle no matches: "error" (return None), "skip" (continue), "warn" (warning + continue)
            cmd_name: Command name for error messages

        Returns:
            List of all matching paths, or None if on_no_match="error" and no matches found
        """
        all_matches = []

        for pattern in patterns:
            expanded = self._expand_wildcards(pattern, include_dirs=include_dirs)

            if not expanded:
                # Handle no match based on policy
                if on_no_match == "error":
                    console.print(f"[yellow]{cmd_name}: no match: {pattern}[/yellow]", highlight=False)
                    return None
                elif on_no_match == "skip":
                    continue
                elif on_no_match == "warn":
                    console.print(f"[yellow]{cmd_name}: no match: {pattern}[/yellow]", highlight=False)
                    continue
            else:
                all_matches.extend(expanded)

        return all_matches

    def _wrap_cli_command(
        self,
        cli_func,
        args: List[str],
        min_args: int = 1,
        usage: str = "",
        resolve_paths: bool = True,
        path_indices: Optional[List[int]] = None,
    ) -> bool:
        """Generic wrapper for cli_commands functions to eliminate code duplication.

        This method handles common patterns:
        - Argument validation
        - Path resolution
        - Error handling
        - Delegation to cli_commands

        Args:
            cli_func: Function from cli_commands module to call
            args: Command arguments from user
            min_args: Minimum number of arguments required
            usage: Usage string to display if arguments are insufficient
            resolve_paths: Whether to resolve relative paths to absolute
            path_indices: Indices of arguments that are paths (default: [0])

        Returns:
            bool: True to continue REPL, False to exit

        Example:
            def cmd_mkdir(self, args):
                return self._wrap_cli_command(
                    cli_commands.cmd_mkdir, args,
                    min_args=1, usage="mkdir <directory>"
                )
        """
        # Validate arguments
        if len(args) < min_args:
            if usage:
                console.print(f"Usage: {usage}", highlight=False)
            else:
                # Try to infer usage from function name
                cmd_name = cli_func.__name__.replace('cmd_', '')
                console.print(f"Usage: {cmd_name} <arguments>", highlight=False)
            return True

        # Resolve paths if requested
        if resolve_paths:
            if path_indices is None:
                path_indices = [0]  # Default: first argument is a path

            resolved_args = list(args)  # Create a copy
            for idx in path_indices:
                if idx < len(resolved_args):
                    resolved_args[idx] = self._resolve_path(resolved_args[idx])
            args = resolved_args

        # Call the cli_commands function
        try:
            cli_func(self.client, *args)
        except Exception as e:
            # Extract command name for error message
            cmd_name = cli_func.__name__.replace('cmd_', '')
            error_path = args[0] if args else "unknown"
            console.print(self._format_error(cmd_name, error_path, e), highlight=False)

        return True

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
            # Find where the first redirection starts
            first_redirect_idx = None
            for i, arg in enumerate(args):
                if arg == ">" or arg == ">>":
                    first_redirect_idx = i
                    break

            if first_redirect_idx is not None:
                # Get content before any redirection
                args_before_redirect = args[:first_redirect_idx]
                redirect_part = args[first_redirect_idx:]

                # Get the initial content from the command
                try:
                    result = content_getter(args_before_redirect)
                    if result is None:
                        return True
                    content, source_path = result
                except Exception as e:
                    console.print(
                        self._format_error(cmd_name, "", e),
                        highlight=False,
                    )
                    return True

                # Build command string for parsing redirections
                quoted_args = [shlex.quote(arg) for arg in args_before_redirect]
                cmd_str = (
                    f"{cmd_name} {' '.join(quoted_args)} {' '.join(redirect_part)}"
                )

                # Parse and execute as redirect chain using unified system
                redirect_chain = self._parse_redirections(cmd_str)
                if redirect_chain and redirect_chain.get('redirects'):
                    self._apply_redirections(content, redirect_chain['redirects'], is_last=True)
                    return True

        # Check for >> (append) redirection
        if ">>" in args:
            idx = args.index(">>")
            if idx + 1 < len(args):
                dest_path = self._resolve_path(args[idx + 1])
                source_path = None
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
                source_path = None
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
        # Handle AGFSClientError - already formatted nicely
        if isinstance(error, AGFSClientError):
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
            ("  upload [-r] <local> <agfs>", "Upload local file/dir to AGFS"),
            ("  download [-r] <agfs> <local>", "Download AGFS file/dir to local"),
            (
                "  tailf [-n N] <file>",
                "Show last N lines, then follow to EOF on changes",
            ),
            (
                "  grep [-r] [-i] [-c] [--stream] <pattern> <path>",
                "Search for pattern in files (regex supported)",
            ),
            (
                "  digest [-a algorithm] <file>",
                "Calculate file digest (xxh3 or md5)",
            ),
            ("", ""),
            ("Plugin Management", ""),
            ("  mounts", "List mounted plugins"),
            ("  mount <fstype> <path> [k=v ...]", "Mount plugin dynamically"),
            ("  unmount <path>", "Unmount plugin"),
            ("  plugins", "Show mounted plugins"),
            (
                "  plugins load <lib|url|pfs://...>",
                "Load external plugin from file, HTTP(S), or pfs:// URL",
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
            (
                "  cat /s3fs/aws/hello | tee -a /local/access.log > /queuefs/tasks/enqueue",
                "Read S3 file, append to log, enqueue to task queue",
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
            ("  cat video.mp4 | agfs write --stream /mnt/streamfs/video", ""),
            ("  agfs cat --stream /mnt/streamfs/video | ffplay -", ""),
            (
                "  ffmpeg -i in.mp4 -f mpegts - | agfs write --stream /mnt/streamfs/live",
                "",
            ),
        ]

        console.print("\nAGFS CLI Commands\n", highlight=False)
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
        """List directory contents (supports wildcards)"""
        # Filter out flags from args
        paths = [arg for arg in args if not arg.startswith("-")]

        if not paths:
            # No arguments, list current directory
            path = self.current_path
            try:
                cli_commands.cmd_ls(self.client, path)
            except Exception as e:
                console.print(self._format_error("ls", path, e), highlight=False)
            return True

        # Check if path contains wildcards
        pattern = paths[0]
        if '*' in pattern or '?' in pattern:
            # Expand wildcards and list matching files
            expanded = self._expand_wildcards(pattern, include_dirs=True)
            if not expanded:
                console.print(f"[yellow]ls: no match: {pattern}[/yellow]", highlight=False)
                return True

            # Show just the list of matching files (like Unix ls)
            # Group by directory vs file for better display
            files = []
            dirs = []
            for path in expanded:
                try:
                    stat = self.client.stat(path)
                    if stat.get('isDir'):
                        dirs.append((path, stat))
                    else:
                        files.append((path, stat))
                except Exception as e:
                    console.print(self._format_error("ls", path, e), highlight=False)

            # Display files first (just like regular ls does)
            for path, stat in files + dirs:
                # Format like regular ls output
                mode = stat.get('mode', 0)
                size = stat.get('size', 0)
                mtime = stat.get('modTime', '')
                is_dir = stat.get('isDir', False)

                # Format permissions using cli_commands helper
                perms = cli_commands.format_permissions(mode, is_dir)

                # Format timestamp
                if mtime:
                    from datetime import datetime
                    try:
                        dt = datetime.fromisoformat(mtime.replace('Z', '+00:00'))
                        time_str = dt.strftime('%Y-%m-%d %H:%M:%S')
                    except:
                        time_str = mtime[:19] if len(mtime) >= 19 else mtime
                else:
                    time_str = ' ' * 19

                # Get just the filename
                filename = os.path.basename(path.rstrip('/'))

                # Print in ls format
                console.print(f"{perms} {size:8} {time_str} {filename}", highlight=False)
        else:
            # No wildcards, normal ls
            path = self._resolve_path(pattern)
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
        """Display file contents (supports wildcards, > and >> redirection and --stream)"""
        if not args:
            console.print(
                "Usage: cat [--stream] <file> [> output] [>> output]", highlight=False
            )
            console.print("       cat <file1> [file2...]", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
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

        # Handle redirection if present (note: redirection doesn't work with --stream or wildcards)
        if self._handle_redirection(args, cat_content_getter, "cat"):
            if stream:
                console.print(
                    "cat: --stream cannot be used with redirection", highlight=False
                )
            return True

        # Expand wildcards in all file arguments
        all_files = self._expand_patterns(args, include_dirs=False, on_no_match="error", cmd_name="cat")
        if all_files is None:
            return True

        # Cat each file
        for i, path in enumerate(all_files):
            try:
                if len(all_files) > 1:
                    # Print separator for multiple files
                    if i > 0:
                        console.print("", highlight=False)
                    console.print(f"==> {path} <==", highlight=False)
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
                    "[yellow]Use: cat file.dat | agfs write --stream /mnt/streamfs/stream[/yellow]",
                    highlight=False,
                )
                console.print(
                    "[yellow]Or use shell redirection: command | agfs write --stream <path>[/yellow]",
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
        return self._wrap_cli_command(
            cli_commands.cmd_mkdir, args,
            min_args=1, usage="mkdir <directory>"
        )

    def cmd_rm(self, args: List[str]) -> bool:
        """Remove file or directory (supports wildcards)"""
        if not args:
            console.print("Usage: rm <path> [-r]", highlight=False)
            console.print("       rm <path1> [path2...] [-r]", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
            return True

        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]
        if not path_args:
            console.print("Usage: rm <path> [-r]", highlight=False)
            return True

        # Expand wildcards in all path arguments
        all_paths = self._expand_patterns(path_args, include_dirs=True, on_no_match="skip", cmd_name="rm")
        if not all_paths:
            return True

        # Remove each path
        for path in all_paths:
            try:
                cli_commands.cmd_rm(self.client, path, recursive)
                console.print(f"  removed '{path}'", highlight=False)
            except Exception as e:
                console.print(self._format_error("rm", path, e), highlight=False)

        return True

    def cmd_touch(self, args: List[str]) -> bool:
        """Create empty file"""
        return self._wrap_cli_command(
            cli_commands.cmd_touch, args,
            min_args=1, usage="touch <file>"
        )

    def cmd_stat(self, args: List[str]) -> bool:
        """Show file/directory information (supports wildcards)"""
        if not args:
            console.print("Usage: stat <path>", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
            return True

        # Expand wildcards
        all_paths = self._expand_patterns(args, include_dirs=True, on_no_match="error", cmd_name="stat")
        if all_paths is None:
            return True

        # Show stat for each path
        for i, path in enumerate(all_paths):
            if len(all_paths) > 1 and i > 0:
                console.print("", highlight=False)
            try:
                cli_commands.cmd_stat(self.client, path)
            except Exception as e:
                console.print(self._format_error("stat", path, e), highlight=False)

        return True

    def cmd_cp(self, args: List[str]) -> bool:
        """Copy file or directory (supports wildcards)"""
        if len(args) < 2:
            console.print("Usage: cp [-r] <source> <destination>", highlight=False)
            console.print("       cp [-r] <source1> [source2...] <destination>", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print("Usage: cp [-r] <source> <destination>", highlight=False)
            return True

        # Destination is always the last argument
        dst = self._resolve_path(path_args[-1])

        # All other arguments are sources (may contain wildcards)
        source_patterns = path_args[:-1]

        # Expand wildcards in all source patterns
        all_sources = self._expand_patterns(source_patterns, include_dirs=True, on_no_match="error", cmd_name="cp")
        if all_sources is None:
            return True

        # If multiple sources, destination must be a directory
        if len(all_sources) > 1:
            try:
                dst_stat = self.client.stat(dst)
                if not dst_stat.get("isDir"):
                    console.print(f"cp: target '{path_args[-1]}' is not a directory", highlight=False)
                    return True
            except:
                console.print(f"cp: target '{path_args[-1]}' does not exist", highlight=False)
                return True

        # Copy each source
        for src in all_sources:
            try:
                src_stat = self.client.stat(src)
            except Exception as e:
                console.print(self._format_error("cp", src, e), highlight=False)
                continue

            # Check if source is a directory
            if src_stat.get("isDir"):
                if not recursive:
                    console.print(
                        f"cp: {src}: is a directory (not copied, use -r)", highlight=False
                    )
                    continue
                # Recursive copy of directory
                self._copy_directory(src, dst, src)
                continue

            # Source is a file
            try:
                # Determine final destination path
                final_dst = dst
                try:
                    dst_stat = self.client.stat(dst)
                    if dst_stat.get("isDir"):
                        # Destination is a directory, append source filename
                        src_filename = os.path.basename(src.rstrip("/"))
                        final_dst = f"{dst.rstrip('/')}/{src_filename}"
                except:
                    # Destination doesn't exist, use as-is
                    pass

                # Read source file and write to destination
                content = self.client.cat(src)
                self.client.write(final_dst, content)
                console.print(f"  {src} -> {final_dst}", highlight=False)
            except Exception as e:
                console.print(self._format_error("cp", src, e), highlight=False)

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
        """Move/rename file (supports wildcards)"""
        if len(args) < 2:
            console.print("Usage: mv <source> <destination>", highlight=False)
            console.print("       mv <source1> [source2...] <destination>", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
            return True

        # Destination is always the last argument
        dst = self._resolve_path(args[-1])

        # All other arguments are sources (may contain wildcards)
        source_patterns = args[:-1]

        # Expand wildcards in all source patterns
        all_sources = self._expand_patterns(source_patterns, include_dirs=True, on_no_match="error", cmd_name="mv")
        if all_sources is None:
            return True

        # If multiple sources, destination must be a directory
        if len(all_sources) > 1:
            try:
                dst_stat = self.client.stat(dst)
                if not dst_stat.get("isDir"):
                    console.print(f"mv: target '{args[-1]}' is not a directory", highlight=False)
                    return True
            except:
                console.print(f"mv: target '{args[-1]}' does not exist", highlight=False)
                return True

        # Move each source
        for src in all_sources:
            try:
                # Determine final destination path
                final_dst = dst
                try:
                    dst_stat = self.client.stat(dst)
                    if dst_stat.get("isDir"):
                        # Destination is a directory, append source filename
                        src_filename = os.path.basename(src.rstrip("/"))
                        final_dst = f"{dst.rstrip('/')}/{src_filename}"
                except:
                    # Destination doesn't exist, use as-is (only for single file)
                    pass

                cli_commands.cmd_mv(self.client, src, final_dst)
                console.print(f"  {src} -> {final_dst}", highlight=False)
            except Exception as e:
                console.print(self._format_error("mv", src, e), highlight=False)

        return True

    def cmd_chmod(self, args: List[str]) -> bool:
        """Change file permissions (supports wildcards)"""
        if len(args) < 2:
            console.print("Usage: chmod <mode> <path> [path2...]", highlight=False)
            console.print("Supports wildcards: *.txt, file?.log, etc.", highlight=False)
            return True

        try:
            mode = int(args[0], 8)
        except ValueError:
            console.print(f"chmod: invalid mode: '{args[0]}'", highlight=False)
            return True

        # Expand wildcards for all path arguments
        all_paths = self._expand_patterns(args[1:], include_dirs=True, on_no_match="skip", cmd_name="chmod")
        if not all_paths:
            return True

        # Apply chmod to each path
        for path in all_paths:
            try:
                cli_commands.cmd_chmod(self.client, mode, path)
                console.print(f"  changed mode of '{path}' to {oct(mode)[2:]}", highlight=False)
            except Exception as e:
                console.print(self._format_error("chmod", path, e), highlight=False)
        return True

    def cmd_mounts(self, args: List[str]) -> bool:
        """List mounted plugins"""
        return self._wrap_cli_command(
            cli_commands.cmd_mounts, args,
            min_args=0, resolve_paths=False
        )

    def cmd_unmount(self, args: List[str]) -> bool:
        """Unmount a plugin"""
        return self._wrap_cli_command(
            cli_commands.cmd_unmount, args,
            min_args=1, usage="unmount <path>",
            resolve_paths=False  # Mount paths don't need resolution
        )

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
                console.print("Usage: plugins load <library_path|url|pfs://...>", highlight=False)
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
                console.print(
                    "  plugins load pfs://s3fs/aws/plugin.wasm",
                    highlight=False,
                )
                console.print(
                    "  plugins load pfs:///mnt/plugins/myplugin.so",
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
                "  plugins load <library_path|url|pfs://...>   - Load external plugin",
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
        """Upload local file or directory to AGFS"""
        if len(args) < 2:
            console.print("Usage: upload [-r] <local_path> <agfs_path>", highlight=False)
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print("Usage: upload [-r] <local_path> <agfs_path>", highlight=False)
            return True

        local_path = path_args[0]
        agfs_path = self._resolve_path(path_args[1])

        try:
            cli_commands.cmd_upload(self.client, local_path, agfs_path, recursive)
        except Exception as e:
            console.print(f"upload: {e}", highlight=False)
        return True

    def cmd_download(self, args: List[str]) -> bool:
        """Download file or directory from AGFS to local filesystem"""
        if len(args) < 2:
            console.print(
                "Usage: download [-r] <agfs_path> <local_path>", highlight=False
            )
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print(
                "Usage: download [-r] <agfs_path> <local_path>", highlight=False
            )
            return True

        agfs_path = self._resolve_path(path_args[0])
        local_path = path_args[1]

        try:
            cli_commands.cmd_download(self.client, agfs_path, local_path, recursive)
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
        console.print("  cat file.txt | tee -a log.txt > output.txt", highlight=False)
        return True

    # Old pipeline methods removed - now using PipelineCommand abstraction
    # See TeeCommand, EchoCommand, CatCommand, GrepCommand classes at end of file

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

    def cmd_digest(self, args: List[str]) -> bool:
        """Calculate the digest of a file

        Usage:
            digest [-a ALGORITHM] PATH

        Options:
            -a, --algorithm    Hash algorithm to use: xxh3 (default) or md5

        Examples:
            digest /local/file.txt              # Using xxh3 (default)
            digest -a md5 /local/file.txt       # Using md5
            digest --algorithm xxh3 /data.bin   # Explicit algorithm
            digest xxh3 /data.txt > /output     # Redirect output to file
        """
        if not args:
            console.print("Usage: digest [-a ALGORITHM] PATH", highlight=False)
            console.print("       digest [--algorithm ALGORITHM] PATH", highlight=False)
            console.print("\nAlgorithms: xxh3 (default), md5", highlight=False)
            return True

        # Define content getter for redirection support
        def digest_content_getter(pre_redirect_args):
            # Parse arguments before redirection
            algorithm = "xxh3"  # Default
            path_arg = None

            i = 0
            while i < len(pre_redirect_args):
                if pre_redirect_args[i] in ["-a", "--algorithm"] and i + 1 < len(pre_redirect_args):
                    algorithm = pre_redirect_args[i + 1]
                    if algorithm not in ["xxh3", "md5"]:
                        console.print(
                            f"digest: unsupported algorithm: '{algorithm}' (supported: xxh3, md5)",
                            highlight=False
                        )
                        return None
                    i += 2
                elif not pre_redirect_args[i].startswith("-"):
                    path_arg = pre_redirect_args[i]
                    i += 1
                else:
                    console.print(f"digest: unknown option: '{pre_redirect_args[i]}'", highlight=False)
                    return None

            if not path_arg:
                console.print("Usage: digest [-a ALGORITHM] PATH", highlight=False)
                return None

            source_path = self._resolve_path(path_arg)

            # Calculate digest
            result = self.client.digest(source_path, algorithm)

            # Format output as text for redirection
            output = f"Algorithm: {result['algorithm']}\nPath: {result['path']}\nDigest: {result['digest']}\n"
            return (output.encode(), source_path)

        # Handle redirection if present
        if self._handle_redirection(args, digest_content_getter, "digest"):
            return True

        # Parse arguments (no redirection case)
        algorithm = "xxh3"  # Default
        path_arg = None

        i = 0
        while i < len(args):
            if args[i] in ["-a", "--algorithm"] and i + 1 < len(args):
                algorithm = args[i + 1]
                if algorithm not in ["xxh3", "md5"]:
                    console.print(
                        f"digest: unsupported algorithm: '{algorithm}' (supported: xxh3, md5)",
                        highlight=False
                    )
                    return True
                i += 2
            elif not args[i].startswith("-"):
                path_arg = args[i]
                i += 1
            else:
                console.print(f"digest: unknown option: '{args[i]}'", highlight=False)
                return True

        if not path_arg:
            console.print("Usage: digest [-a ALGORITHM] PATH", highlight=False)
            return True

        path = self._resolve_path(path_arg)

        try:
            cli_commands.cmd_digest(self.client, path, algorithm)
        except Exception as e:
            console.print(self._format_error("digest", path, e), highlight=False)

        return True

    def cmd_clear(self, args: List[str]) -> bool:
        """Clear screen"""
        console.clear()
        return True

    def cmd_exit(self, args: List[str]) -> bool:
        """Exit REPL"""
        console.print("Goodbye!", highlight=False)
        return False


# ============================================================================
# Pipeline Command Implementations
# ============================================================================


class TeeCommand(PipelineCommand):
    """Tee command - write input to file(s) and pass through to output."""

    def execute(
        self, args: List[str], pipe_input: Optional[bytes] = None, is_last: bool = False
    ) -> Optional[bytes]:
        """Execute tee command.

        Args:
            args: [-a] file1 [file2 ...]
            pipe_input: Input from previous command
            is_last: Whether this is last in pipeline

        Returns:
            bytes: Pass-through of input (or redirect response if redirected)
        """
        if pipe_input is None:
            console.print("[red]tee: no input provided[/red]", highlight=False)
            return None

        # Parse arguments for -a flag and file paths
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

        # Write to all specified files
        for file_arg in file_paths:
            try:
                file_path = self.handler._resolve_path(file_arg)

                if append_mode:
                    # Append mode: read existing content and append
                    try:
                        existing_content = self.handler.client.cat(file_path)
                        new_content = existing_content + pipe_input
                    except:
                        # File doesn't exist, just use pipe_input
                        new_content = pipe_input
                    self.handler.client.write(file_path, new_content)
                else:
                    # Write mode: overwrite file
                    self.handler.client.write(file_path, pipe_input)

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


class EchoCommand(PipelineCommand):
    """Echo command - output text."""

    def execute(
        self, args: List[str], pipe_input: Optional[bytes] = None, is_last: bool = False
    ) -> Optional[bytes]:
        """Execute echo command.

        Args:
            args: Text to echo
            pipe_input: Ignored (echo doesn't use piped input)
            is_last: Whether this is last in pipeline

        Returns:
            bytes: The echoed text as bytes
        """
        text = " ".join(args)
        output = (text + "\n").encode()

        if is_last:
            print(text)
            return None
        else:
            return output

    def supports_pipe_input(self) -> bool:
        """Echo doesn't use piped input."""
        return False


class CatCommand(PipelineCommand):
    """Cat command - display file contents or pass through piped input (supports wildcards)."""

    def execute(
        self, args: List[str], pipe_input: Optional[bytes] = None, is_last: bool = False
    ) -> Optional[bytes]:
        """Execute cat command.

        Args:
            args: [file] - file path (optional if piped input provided, supports wildcards)
            pipe_input: Input from previous command
            is_last: Whether this is last in pipeline

        Returns:
            bytes: File contents or piped input
        """
        # If no args and we have piped input, just pass it through
        if not args and pipe_input is not None:
            if is_last:
                sys.stdout.buffer.write(pipe_input)
                if pipe_input and not pipe_input.endswith(b"\n"):
                    sys.stdout.buffer.write(b"\n")
                sys.stdout.buffer.flush()
                return None
            else:
                return pipe_input

        # Read from file(s)
        if not args:
            console.print("[red]cat: missing file argument[/red]", highlight=False)
            return None

        # Expand wildcards in all file arguments
        all_files = self.handler._expand_patterns(args, include_dirs=False, on_no_match="error", cmd_name="cat")
        if all_files is None:
            return None

        # Read and concatenate all files
        all_content = b""
        for path in all_files:
            try:
                content = self.handler.client.cat(path)
                all_content += content
            except Exception as e:
                console.print(f"[red]cat: {path}: {e}[/red]", highlight=False)
                return None

        if is_last:
            sys.stdout.buffer.write(all_content)
            if all_content and not all_content.endswith(b"\n"):
                sys.stdout.buffer.write(b"\n")
            sys.stdout.buffer.flush()
            return None
        else:
            return all_content


class GrepCommand(PipelineCommand):
    """Grep command - filter input by pattern."""

    def execute(
        self, args: List[str], pipe_input: Optional[bytes] = None, is_last: bool = False
    ) -> Optional[bytes]:
        """Execute grep command.

        Args:
            args: [-i] [-c] [-v] pattern
            pipe_input: Input from previous command
            is_last: Whether this is last in pipeline

        Returns:
            bytes: Filtered output
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

        # Decode input
        try:
            text = pipe_input.decode("utf-8")
        except UnicodeDecodeError:
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
            if invert_match:
                matches = not matches
            if matches:
                matching_lines.append(line)

        # Handle output
        if count_only:
            count_str = f"{len(matching_lines)}\n"
            if is_last:
                print(len(matching_lines))
                return None
            else:
                return count_str.encode("utf-8")
        else:
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
