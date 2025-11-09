"""REPL Command Handlers"""

import shlex
import os
from typing import List
from rich.console import Console
import requests

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

    def _handle_redirection(self, args: List[str], content_getter, cmd_name: str) -> bool:
        """
        Handle > and >> redirection for commands.

        Args:
            args: Command arguments
            content_getter: Function that returns (content: bytes, source_path: str) or None
            cmd_name: Command name for error messages

        Returns:
            True if redirection was handled, False otherwise
        """
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

                    # Read existing destination content and append
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
                    console.print(self._format_error(cmd_name, source_path or dest_path, e))
            else:
                console.print(f"{cmd_name}: syntax error near unexpected token `newline'")
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
                    console.print(self._format_error(cmd_name, source_path or dest_path, e), highlight=False)
            else:
                console.print(f"{cmd_name}: syntax error near unexpected token `newline'", highlight=False)
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
            ("  tailf [-n N] <file>", "Show last N lines, then follow to EOF on changes"),
            ("", ""),
            ("Plugin Management", ""),
            ("  mounts", "List mounted plugins"),
            ("  mount <fstype> <path> [k=v ...]", "Mount plugin dynamically"),
            ("  unmount <path>", "Unmount plugin"),
            ("  plugins", "Show mounted plugins"),
            ("  plugins load <lib|url>", "Load external plugin from file or HTTP(S) URL"),
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
            ("Streaming Examples (outside REPL)", ""),
            ("  cat video.mp4 | pfs write --stream /mnt/streamfs/video", ""),
            ("  pfs cat --stream /mnt/streamfs/video | ffplay -", ""),
            ("  ffmpeg -i in.mp4 -f mpegts - | pfs write --stream /mnt/streamfs/live", ""),
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
                    console.print(f"tree: invalid depth: '{args[i + 1]}'", highlight=False)
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
            console.print("Usage: cat [--stream] <file> [> output] [>> output]", highlight=False)
            return True

        # Check for --stream flag
        stream = "--stream" in args
        args = [arg for arg in args if arg != "--stream"]

        if not args:
            console.print("Usage: cat [--stream] <file> [> output] [>> output]", highlight=False)
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
                console.print("cat: --stream cannot be used with redirection", highlight=False)
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
                    console.print(f"tail: invalid number of lines: '{args[i + 1]}'", highlight=False)
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
            console.print("       write --stream <file>   # Read from stdin", highlight=False)
            console.print("       write <file> <content>  # Write content directly", highlight=False)
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
                console.print("[yellow]Note: --stream is not available in REPL mode[/yellow]", highlight=False)
                console.print("[yellow]Use: cat file.dat | pfs write --stream /mnt/streamfs/stream[/yellow]", highlight=False)
                console.print("[yellow]Or use shell redirection: command | pfs write --stream <path>[/yellow]", highlight=False)
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
            console.print(self._format_error('stat', path, e), highlight=False)
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
        if src_stat.get('isDir'):
            if not recursive:
                console.print(f"cp: {path_args[0]}: is a directory (not copied)", highlight=False)
                return True
            # Recursive copy of directory
            self._copy_directory(src, dst, path_args[0])
            return True

        # Source is a file
        try:
            # Check if destination is a directory
            try:
                dst_stat = self.client.stat(dst)
                if dst_stat.get('isDir'):
                    # Destination is a directory, append source filename
                    import os
                    src_filename = os.path.basename(src.rstrip('/'))
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
        import os

        # Get the source directory name
        src_dir_name = os.path.basename(src_dir.rstrip('/'))

        # Check if destination exists and is a directory
        try:
            dst_stat = self.client.stat(dst_dir)
            if dst_stat.get('isDir'):
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
                file_name = file_info.get('name', '')
                is_dir = file_info.get('isDir', False)

                src_path = f"{current_src.rstrip('/')}/{file_name}"
                dst_path = f"{current_dst.rstrip('/')}/{file_name}"

                if is_dir:
                    # Create subdirectory and add to queue
                    try:
                        self.client.mkdir(dst_path)
                        console.print(f"Created directory {dst_path}/", highlight=False)
                        queue.append((src_path, dst_path))
                    except Exception as e:
                        console.print(self._format_error("cp", src_path, e), highlight=False)
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
                        console.print(self._format_error("cp", src_path, e), highlight=False)

        # Print summary
        if total_files > 0:
            if total_bytes < 1024:
                console.print(f"\nCopied {total_files} files, {total_bytes} bytes total", highlight=False)
            elif total_bytes < 1024 * 1024:
                kb = total_bytes / 1024
                console.print(f"\nCopied {total_files} files, {kb:.2f} KB total", highlight=False)
            else:
                mb = total_bytes / (1024 * 1024)
                console.print(f"\nCopied {total_files} files, {mb:.2f} MB total", highlight=False)

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
            console.print("Usage: mount <fstype> <path> [key=value ...]", highlight=False)
            console.print("\nExamples:", highlight=False)
            console.print("  mount memfs /test/mem", highlight=False)
            console.print("  mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db", highlight=False)
            console.print("  mount s3fs /test/s3 bucket=my-bucket region=us-west-1 access_key_id=xxx secret_access_key=yyy", highlight=False)
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
                console.print("  plugins load ./examples/plugins/hellofs-c/hellofs-c.dylib", highlight=False)
                console.print("  plugins load http://example.com/plugins/myplugin.so", highlight=False)
                console.print("  plugins load https://example.com/plugins/myplugin.dylib", highlight=False)
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
                console.print(f"[red]Error unloading plugin: {e}[/red]", highlight=False)
            return True

        elif subcommand == "list":
            try:
                cli_commands.cmd_list_plugins(self.client)
            except Exception as e:
                console.print(f"[red]Error listing plugins: {e}[/red]", highlight=False)
            return True

        else:
            console.print(f"[red]Unknown subcommand: {subcommand}[/red]", highlight=False)
            console.print("\nUsage:", highlight=False)
            console.print("  plugins           - List mounted plugins", highlight=False)
            console.print("  plugins load <library_path>   - Load external plugin", highlight=False)
            console.print("  plugins unload <library_path> - Unload external plugin", highlight=False)
            console.print("  plugins list      - List loaded external plugins", highlight=False)
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
            console.print("Usage: download [-r] <pfs_path> <local_path>", highlight=False)
            return True

        # Check for -r flag
        recursive = "-r" in args or "--recursive" in args
        path_args = [arg for arg in args if not arg.startswith("-")]

        if len(path_args) < 2:
            console.print("Usage: download [-r] <pfs_path> <local_path>", highlight=False)
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
                    console.print(f"tailf: invalid number of lines: '{args[i + 1]}'", highlight=False)
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
            console.print("Usage: watch [-n seconds] <command> [args...]", highlight=False)
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
                    console.print("[red]Error: interval must be positive[/red]", highlight=False)
                    return True
                command_start_idx = 2
            except ValueError:
                console.print(f"[red]Error: invalid interval: {args[1]}[/red]", highlight=False)
                return True

        # Get command and its arguments
        if command_start_idx >= len(args):
            console.print("[red]Error: no command specified[/red]", highlight=False)
            return True

        cmd_name = args[command_start_idx].lower()
        cmd_args = args[command_start_idx + 1:]

        # Map command names to functions
        command_map = {
            "ls": cli_commands.cmd_ls,
            "cat": cli_commands.cmd_cat,
            "stat": cli_commands.cmd_stat,
            "mounts": cli_commands.cmd_mounts,
        }

        if cmd_name not in command_map:
            console.print(f"[red]Error: command '{cmd_name}' not supported in watch[/red]", highlight=False)
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

    def cmd_clear(self, args: List[str]) -> bool:
        """Clear screen"""
        console.clear()
        return True

    def cmd_exit(self, args: List[str]) -> bool:
        """Exit REPL"""
        console.print("Goodbye!", highlight=False)
        return False
