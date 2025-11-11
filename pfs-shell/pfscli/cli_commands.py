import json
import os
import signal
import sys
import time
from datetime import datetime
from typing import Optional
from rich.console import Console

console = Console()

def format_permissions(mode: int, is_dir: bool) -> str:
    """Format permissions in Unix style"""
    result = "d" if is_dir else "-"
    result += "r" if mode & 0o400 else "-"
    result += "w" if mode & 0o200 else "-"
    result += "x" if mode & 0o100 else "-"
    result += "r" if mode & 0o040 else "-"
    result += "w" if mode & 0o020 else "-"
    result += "x" if mode & 0o010 else "-"
    result += "r" if mode & 0o004 else "-"
    result += "w" if mode & 0o002 else "-"
    result += "x" if mode & 0o001 else "-"
    return result


def cmd_ls(client, path: str):
    """List directory contents"""
    files = []
    try:
        files = client.ls(path)
        if not files:
            return
    except Exception as e:
        console.print(f"[red]ls: {path}: {e}[/red]", highlight=False)
        return
    # Separate directories and files
    dirs = [f for f in files if f.get("isDir", False)]
    regular_files = [f for f in files if not f.get("isDir", False)]

    # Sort directories and files by modification time (newest first)
    def parse_time(f):
        try:
            return datetime.fromisoformat(f.get("modTime", "").replace("Z", "+00:00"))
        except:
            return datetime.min

    dirs.sort(key=parse_time, reverse=True)
    regular_files.sort(key=parse_time, reverse=True)

    # Combine: directories first, then files
    sorted_files = dirs + regular_files

    for f in sorted_files:
        mode = f.get("mode", 0)
        is_dir = f.get("isDir", False)
        size = f.get("size", 0)
        mtime_str = f.get("modTime", "")
        name = f.get("name", "")

        # Format time to show only up to seconds (remove nanoseconds)
        try:
            dt = datetime.fromisoformat(mtime_str.replace("Z", "+00:00"))
            mtime_display = dt.strftime("%Y-%m-%d %H:%M:%S")
        except:
            mtime_display = mtime_str

        mode_str = format_permissions(mode, is_dir)
        # Highlight directories in blue and files in default color
        if is_dir:
            name_display = f"[bold cyan]{name}/[/bold cyan]"
        else:
            name_display = name

        # Print with highlight disabled for mode, size, and time
        console.print(f"{mode_str} {size:>8} {mtime_display} ", end="", highlight=False)
        # Print name with highlight enabled (for directories)
        console.print(name_display, highlight=False)


def cmd_cat(client, path: str, stream: bool = False):
    """Display file contents (supports both text and binary)

    Args:
        path: File path to read
        stream: Enable streaming mode for continuous reads (default: False)
    """
    if stream:
        # Streaming mode - read chunks continuously
        try:
            response = client.cat(path, stream=True)
            for chunk in response.iter_content(chunk_size=8192):
                if chunk:
                    sys.stdout.buffer.write(chunk)
                    sys.stdout.buffer.flush()
        except KeyboardInterrupt:
            console.print("\n[yellow]Stream interrupted[/yellow]", highlight=False)
        except Exception as e:
            console.print(f"cat: {path}: {e}", highlight=False)
    else:
        # Normal mode - read all at once
        content = client.cat(path)

        # Write raw bytes to stdout
        sys.stdout.buffer.write(content)

        # If content doesn't end with newline, add one for proper prompt display
        if content and not content.endswith(b'\n'):
            sys.stdout.buffer.write(b'\n')

        sys.stdout.buffer.flush()


def cmd_tail(client, path: str, lines: int = 10):
    """Display last N lines of a file

    Args:
        path: File path to tail
        lines: Number of lines to display (default: 10)
    """
    # Get file info
    stat_info = client.stat(path)
    if stat_info.get('isDir'):
        console.print(f"tail: {path}: Is a directory", highlight=False)
        return

    file_size = stat_info['size']

    if file_size == 0:
        return

    # Read last chunk to get last N lines
    # We read a larger chunk to ensure we get enough lines
    chunk_size = min(8192, file_size)
    offset = file_size - chunk_size

    data = client.cat(path, offset=offset, size=chunk_size)

    # Try to decode as UTF-8 and split into lines
    try:
        text = data.decode('utf-8')
        all_lines = text.splitlines()

        # Print last N lines
        for line in all_lines[-lines:]:
            print(line)
    except UnicodeDecodeError:
        # For binary files, just output the raw bytes
        sys.stdout.buffer.write(data)
        sys.stdout.buffer.flush()


def cmd_mkdir(client, path: str):
    """Create directory"""
    client.mkdir(path)

def cmd_rm(client, path: str, recursive: bool = False):
    """Remove file or directory"""
    client.rm(path, recursive=recursive)

def cmd_touch(client, path: str):
    """Create empty file"""
    client.create(path)

def cmd_write(client, path: str, content: str = None, stream: bool = False):
    """Write content to file

    Args:
        client: PFS client
        path: File path
        content: Content to write (for normal mode)
        stream: Enable streaming mode to read from stdin continuously
    """
    if stream:
        # Streaming mode - read from stdin in binary chunks
        total_bytes = 0
        chunk_count = 0
        should_exit = False
        chunk_size = 65536  # Read 64KB chunks (better for video streaming)

        def signal_handler(sig, frame):
            """Handle Ctrl+C to flush and exit gracefully"""
            nonlocal should_exit
            should_exit = True

        # Register signal handler for Ctrl+C
        signal.signal(signal.SIGINT, signal_handler)

        try:
            console.print(f"[yellow]Streaming to {path}[/yellow]", highlight=False)
            console.print("[dim]Reading binary data from stdin...[/dim]", highlight=False)
            console.print("[dim]Press Ctrl+C to stop[/dim]", highlight=False)
            console.print(highlight=False)

            while not should_exit:
                try:
                    # Read binary chunk from stdin
                    chunk = sys.stdin.buffer.read(chunk_size)

                    if not chunk:
                        # EOF reached
                        break

                    # Write chunk to server
                    msg = client.write(path, chunk)
                    total_bytes += len(chunk)
                    chunk_count += 1

                    # Progress indicator every 100 chunks (~6.4MB)
                    if chunk_count % 100 == 0:
                        mb = total_bytes / (1024 * 1024)
                        console.print(f"[dim]Progress: {mb:.2f} MB sent[/dim]", highlight=False)

                except EOFError:
                    break

            # Final summary
            if total_bytes < 1024:
                console.print(f"\n[green]Streaming complete: {total_bytes} bytes in {chunk_count} chunks[/green]", highlight=False)
            elif total_bytes < 1024 * 1024:
                kb = total_bytes / 1024
                console.print(f"\n[green]Streaming complete: {kb:.2f} KB in {chunk_count} chunks[/green]", highlight=False)
            else:
                mb = total_bytes / (1024 * 1024)
                console.print(f"\n[green]Streaming complete: {mb:.2f} MB in {chunk_count} chunks[/green]", highlight=False)

        except KeyboardInterrupt:
            # This shouldn't be reached due to signal handler, but keep as fallback
            if total_bytes < 1024:
                console.print(f"\n[yellow]Streaming stopped: {total_bytes} bytes written[/yellow]", highlight=False)
            elif total_bytes < 1024 * 1024:
                kb = total_bytes / 1024
                console.print(f"\n[yellow]Streaming stopped: {kb:.2f} KB written[/yellow]", highlight=False)
            else:
                mb = total_bytes / (1024 * 1024)
                console.print(f"\n[yellow]Streaming stopped: {mb:.2f} MB written[/yellow]", highlight=False)
        except Exception as e:
            console.print(f"\n[red]Error during streaming: {e}[/red]", highlight=False)
    else:
        # Normal mode - write content directly
        if content is None:
            console.print("[red]Error: content is required for normal write mode[/red]", highlight=False)
            return

        msg = client.write(path, content.encode())
        if msg:
            print(msg)

def cmd_stat(client, path: str):
    """Show file/directory information"""
    info = client.stat(path)

    file_type = "Directory" if info.get("isDir") else "File"
    console.print(f"  File: {info.get('name', '')}", highlight=False)
    console.print(f"  Type: {file_type}", highlight=False)
    console.print(f"  Size: {info.get('size', 0)}", highlight=False)
    console.print(f"  Mode: {oct(info.get('mode', 0))[2:]}", highlight=False)
    console.print(f"  Modified: {info.get('modTime', '')}", highlight=False)

    meta = info.get("meta", {})
    if meta:
        for key, value in meta.items():
            console.print(f"  Meta.{key}: {value}", highlight=False)


def cmd_cp(client, source: str, destination: str):
    """Copy file"""
    # If destination ends with / or is a directory, append source filename
    if destination.endswith('/'):
        # Destination is explicitly a directory
        source_filename = os.path.basename(source.rstrip('/'))
        destination = os.path.join(destination.rstrip('/'), source_filename)
    else:
        # Check if destination exists and is a directory
        try:
            dest_stat = client.stat(destination)
            if dest_stat.get('isDir', False):
                source_filename = os.path.basename(source.rstrip('/'))
                destination = os.path.join(destination, source_filename)
        except:
            # Destination doesn't exist or stat failed, treat as file
            pass

    content = client.cat(source)
    msg = client.write(destination, content)
    if msg:
        print(msg)


def cmd_mv(client, source: str, destination: str):
    """Move/rename file"""
    # If destination ends with / or is a directory, append source filename
    if destination.endswith('/'):
        # Destination is explicitly a directory
        source_filename = os.path.basename(source.rstrip('/'))
        destination = os.path.join(destination.rstrip('/'), source_filename)
    else:
        # Check if destination exists and is a directory
        try:
            dest_stat = client.stat(destination)
            if dest_stat.get('isDir', False):
                source_filename = os.path.basename(source.rstrip('/'))
                destination = os.path.join(destination, source_filename)
        except:
            # Destination doesn't exist or stat failed, treat as file
            pass

    client.mv(source, destination)


def cmd_chmod(client, mode: int, path: str):
    """Change file permissions"""
    client.chmod(path, mode)


def cmd_mounts(client):
    """List mounted plugins in Unix-style format"""
    mounts_list = client.mounts()

    if not mounts_list:
        console.print("No plugins mounted", highlight=False)
        return

    # Print mounts in Unix mount style: <fstype> on <mountpoint> (options...)
    for mount in mounts_list:
        path = mount.get("path", "")
        plugin = mount.get("pluginName", "")
        config = mount.get("config", {})

        # Build options string from config
        options = []
        for key, value in config.items():
            # Hide sensitive keys
            if key in ["secret_access_key", "password", "token"]:
                options.append(f"{key}=***")
            else:
                # Convert value to string, truncate if too long
                value_str = str(value)
                if len(value_str) > 50:
                    value_str = value_str[:47] + "..."
                options.append(f"{key}={value_str}")

        # Format output line
        if options:
            options_str = ", ".join(options)
            console.print(f"{plugin} on {path} (plugin: {plugin}, {options_str})", highlight=False)
        else:
            console.print(f"{plugin} on {path} (plugin: {plugin})", highlight=False)


def cmd_unmount(client, path: str):
    """Unmount a plugin"""
    client.unmount(path)
    console.print(f"Unmounted plugin at {path}", highlight=False)


def cmd_load_plugin(client, library_path: str):
    """Load an external plugin from a shared library or HTTP(S) URL

    Args:
        client: PFS client
        library_path: Path to the shared library (.so/.dylib/.dll) or HTTP(S) URL
    """
    result = client.load_plugin(library_path)
    plugin_name = result.get("plugin_name", "unknown")
    console.print(f"[green]Loaded external plugin:[/green] [bold cyan]{plugin_name}[/bold cyan]", highlight=False)
    console.print(f"  Library: {library_path}", highlight=False)


def cmd_unload_plugin(client, library_path: str):
    """Unload an external plugin

    Args:
        client: PFS client
        library_path: Path to the shared library
    """
    client.unload_plugin(library_path)
    console.print(f"[green]Unloaded external plugin:[/green] {library_path}", highlight=False)


def cmd_list_plugins(client):
    """List all loaded external plugins

    Args:
        client: PFS client
    """
    plugins = client.list_plugins()

    if not plugins:
        console.print("No external plugins loaded", highlight=False)
        return

    console.print(f"[bold]Loaded External Plugins:[/bold] ({len(plugins)})", highlight=False)
    for plugin_path in plugins:
        # Extract just the filename for display
        filename = os.path.basename(plugin_path)
        console.print(f"  [cyan]{filename}[/cyan]", highlight=False)
        console.print(f"    {plugin_path}", highlight=False)


def cmd_mount(client, fstype: str, path: str, config_args: list):
    """Mount a plugin dynamically

    Args:
        client: PFS client
        fstype: Filesystem type (e.g., 'memfs', 's3fs', 'sqlfs')
        path: Mount path
        config_args: List of config parameters in format ["key=value", ...]
    """
    # Parse config arguments
    config = {}
    for arg in config_args:
        if "=" not in arg:
            console.print(f"[red]Invalid config parameter: {arg}[/red]", highlight=False)
            console.print("Config parameters must be in format key=value", highlight=False)
            return

        key, value = arg.split("=", 1)

        # Try to parse value as JSON types
        # Attempt to convert to int
        try:
            config[key] = int(value)
            continue
        except ValueError:
            pass

        # Attempt to convert to float
        try:
            config[key] = float(value)
            continue
        except ValueError:
            pass

        # Check for boolean
        if value.lower() in ("true", "false"):
            config[key] = value.lower() == "true"
            continue

        # Otherwise, keep as string
        config[key] = value

    try:
        result = client.mount(fstype, path, config)
        if result.get("message"):
            console.print(f"  {result['message']}", highlight=False)
    except Exception as e:
        console.print(f"[red]Failed to mount {fstype} at {path}: {e}[/red]", highlight=False)


def cmd_upload(client, local_path: str, pfs_path: str, recursive: bool = False):
    """Upload local file or directory to PFS"""
    # Check if local path exists
    if not os.path.exists(local_path):
        console.print(f"upload: {local_path}: No such file or directory", highlight=False)
        return

    # Handle directory upload
    if os.path.isdir(local_path):
        if not recursive:
            console.print(f"upload: {local_path}: Is a directory (use -r for recursive upload)", highlight=False)
            return

        # Get the local directory name
        local_dir_name = os.path.basename(os.path.normpath(local_path))

        # Check if remote path exists and is a directory
        try:
            stat_info = client.stat(pfs_path)
            if stat_info.get('isDir'):
                # Remote path exists and is a directory
                # Create subdirectory with same name as local directory
                pfs_path = f"{pfs_path.rstrip('/')}/{local_dir_name}"
                console.print(f"Target is a directory, uploading to {pfs_path}", highlight=False)
        except Exception:
            # Remote path doesn't exist, that's fine
            pass

        _upload_directory(client, local_path, pfs_path)
    else:
        # Upload single file
        # Check if remote destination is a directory
        try:
            stat_info = client.stat(pfs_path)
            if stat_info.get('isDir'):
                # Remote path is a directory, append local filename
                local_filename = os.path.basename(local_path)
                pfs_path = f"{pfs_path.rstrip('/')}/{local_filename}"
                console.print(f"Target is a directory, uploading to {pfs_path}", highlight=False)
        except Exception:
            # Remote path doesn't exist or stat failed, use as-is
            pass

        _upload_file(client, local_path, pfs_path)

def _upload_file(client, local_path: str, pfs_path: str):
    """Upload a single file"""
    try:
        with open(local_path, 'rb') as f:
            content = f.read()

        # Upload to PFS
        msg = client.write(pfs_path, content)

        # Show success message with file size
        size = len(content)
        if msg:
            print(f"Uploaded {local_path} -> {pfs_path} ({size} bytes): {msg}")
        else:
            print(f"Uploaded {local_path} -> {pfs_path} ({size} bytes)")
    except Exception as e:
        console.print(f"upload: {local_path}: {e}", highlight=False)


def _upload_directory(client, local_dir: str, pfs_dir: str):
    """Upload a directory recursively"""
    # Create the destination directory
    try:
        client.mkdir(pfs_dir)
        console.print(f"Created directory {pfs_dir}", highlight=False)
    except Exception as e:
        # Directory might already exist, continue
        pass

    total_files = 0
    total_bytes = 0

    # Walk through the local directory
    for root, dirs, files in os.walk(local_dir):
        # Calculate relative path from local_dir
        rel_path = os.path.relpath(root, local_dir)

        # Calculate PFS path
        if rel_path == '.':
            current_pfs_dir = pfs_dir
        else:
            # Convert Windows paths to Unix-style
            rel_path = rel_path.replace('\\', '/')
            current_pfs_dir = f"{pfs_dir}/{rel_path}"

        # Create subdirectories
        for dir_name in dirs:
            subdir_pfs_path = f"{current_pfs_dir}/{dir_name}"
            try:
                client.mkdir(subdir_pfs_path)
                console.print(f"Created directory {subdir_pfs_path}", highlight=False)
            except Exception as e:
                # Directory might already exist, continue
                pass

        # Upload files
        for file_name in files:
            local_file_path = os.path.join(root, file_name)
            pfs_file_path = f"{current_pfs_dir}/{file_name}"

            try:
                with open(local_file_path, 'rb') as f:
                    content = f.read()

                msg = client.write(pfs_file_path, content)
                size = len(content)
                if msg:
                    print(f"  {local_file_path} -> {pfs_file_path} ({size} bytes): {msg}")
                else:
                    print(f"  {local_file_path} -> {pfs_file_path} ({size} bytes)")

                total_files += 1
                total_bytes += size
            except Exception as e:
                console.print(f"upload: {local_file_path}: {e}", highlight=False)

    console.print(f"\nUploaded {total_files} files, {total_bytes} bytes total", highlight=False)


def _get_unique_filename(filepath: str) -> str:
    """Get a unique filename by adding .1, .2, etc. if file already exists

    Args:
        filepath: Original file path

    Returns:
        Unique file path that doesn't exist
    """
    if not os.path.exists(filepath):
        return filepath

    # File exists, find a unique name
    counter = 1
    while True:
        new_filepath = f"{filepath}.{counter}"
        if not os.path.exists(new_filepath):
            return new_filepath
        counter += 1


def cmd_download(client, pfs_path: str, local_path: str, recursive: bool = False):
    """Download file or directory from PFS to local filesystem"""
    # Get info about the PFS path
    try:
        stat_info = client.stat(pfs_path)
    except Exception as e:
        console.print(f"download: {pfs_path}: {e}", highlight=False)
        return

    if stat_info.get('isDir'):
        # Download directory
        if not recursive:
            console.print(f"download: {pfs_path}: Is a directory (use -r for recursive download)", highlight=False)
            return

        # Get the PFS directory name
        pfs_dir_name = os.path.basename(pfs_path.rstrip('/'))

        # Check if local path exists and is a directory
        if os.path.exists(local_path):
            if os.path.isdir(local_path):
                # Local path exists and is a directory
                # Create subdirectory with same name as PFS directory
                local_path = os.path.join(local_path, pfs_dir_name)
                console.print(f"Target is a directory, downloading to {local_path}", highlight=False)
            else:
                console.print(f"download: {local_path}: File exists (not a directory)", highlight=False)
                return

        _download_directory(client, pfs_path, local_path)
    else:
        # Download single file
        # If local_path is a directory, use the PFS filename
        if os.path.isdir(local_path):
            pfs_filename = os.path.basename(pfs_path)
            local_path = os.path.join(local_path, pfs_filename)
            # Find a unique filename if it already exists
            local_path = _get_unique_filename(local_path)

        _download_file(client, pfs_path, local_path)


def _download_file(client, pfs_path: str, local_path: str):
    """Download a single file"""
    try:
        # Download from PFS
        content = client.cat(pfs_path)

        # Create parent directory if needed
        local_dir = os.path.dirname(local_path)
        if local_dir and not os.path.exists(local_dir):
            os.makedirs(local_dir)

        # Write to local file
        with open(local_path, 'wb') as f:
            f.write(content)

        # Show success message with file size
        size = len(content)
        print(f"Downloaded {pfs_path} -> {local_path} ({size} bytes)")
    except Exception as e:
        console.print(f"download: {pfs_path}: {e}", highlight=False)


def _download_directory(client, pfs_dir: str, local_dir: str):
    """Download a directory recursively"""
    # Create the destination directory
    try:
        os.makedirs(local_dir, exist_ok=True)
        console.print(f"Created directory {local_dir}", highlight=False)
    except Exception as e:
        console.print(f"download: {local_dir}: {e}", highlight=False)
        return

    total_files = 0
    total_bytes = 0

    # Queue for BFS traversal
    from collections import deque
    queue = deque([(pfs_dir, local_dir)])

    while queue:
        current_pfs_dir, current_local_dir = queue.popleft()

        # List directory contents
        try:
            files = client.ls(current_pfs_dir)
        except Exception as e:
            console.print(f"download: {current_pfs_dir}: {e}", highlight=False)
            continue

        for file_info in files:
            file_name = file_info.get('name', '')
            is_dir = file_info.get('isDir', False)

            pfs_file_path = f"{current_pfs_dir.rstrip('/')}/{file_name}"
            local_file_path = os.path.join(current_local_dir, file_name)

            if is_dir:
                # Create subdirectory and add to queue
                try:
                    os.makedirs(local_file_path, exist_ok=True)
                    console.print(f"Created directory {local_file_path}", highlight=False)
                    queue.append((pfs_file_path, local_file_path))
                except Exception as e:
                    console.print(f"download: {local_file_path}: {e}", highlight=False)
            else:
                # Download file
                try:
                    content = client.cat(pfs_file_path)

                    with open(local_file_path, 'wb') as f:
                        f.write(content)

                    size = len(content)
                    print(f"  {pfs_file_path} -> {local_file_path} ({size} bytes)")

                    total_files += 1
                    total_bytes += size
                except Exception as e:
                    console.print(f"download: {pfs_file_path}: {e}", highlight=False)

    console.print(f"\nDownloaded {total_files} files, {total_bytes} bytes total", highlight=False)


def cmd_tailf(client, path: str, lines: int = 10):
    """Follow file changes, displaying initial lines and then all new content

    First displays the last N lines of the file, then continuously monitors
    for changes. When the file grows, reads from last position to new EOF.

    Args:
        path: File path to follow
        lines: Number of initial lines to display (default: 10)
    """
    try:
        # 1. Get initial file info
        stat_info = client.stat(path)
        if stat_info.get('isDir'):
            console.print(f"tailf: {path}: Is a directory", highlight=False)
            return

        file_size = stat_info['size']

        # 2. Display last N lines initially
        if file_size > 0:
            # Read last chunk to get last N lines
            chunk_size = min(8192, file_size)
            offset = file_size - chunk_size

            data = client.cat(path, offset=offset, size=chunk_size)

            # Try to decode as text for initial display
            try:
                text = data.decode('utf-8')
                all_lines = text.splitlines()

                # Print last N lines
                for line in all_lines[-lines:]:
                    console.print(line, highlight=False)
            except UnicodeDecodeError:
                # Binary data - just output raw bytes
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()

        # 3. Follow mode - read from current EOF to new EOF on each change
        last_offset = file_size
        poll_interval = 1
        idle_count = 0

        while True:
            time.sleep(poll_interval)

            # Check for file changes
            try:
                stat_info = client.stat(path)
                new_size = stat_info['size']
            except Exception as e:
                console.print(f"tailf: {path}: {e}", highlight=False)
                break

            if new_size > last_offset:
                # File grew - read ALL new content from last offset to new EOF
                try:
                    # Read from last position to current EOF (size=-1 means read all to EOF)
                    new_data = client.cat(path, offset=last_offset, size=-1)

                    # Try to output as text, fallback to raw bytes
                    try:
                        text = new_data.decode('utf-8')
                        console.print(text, end='', highlight=False)
                        sys.stdout.flush()
                    except UnicodeDecodeError:
                        # Binary data - output raw bytes
                        sys.stdout.buffer.write(new_data)
                        sys.stdout.buffer.flush()

                    # Update to new EOF position
                    last_offset = new_size
                    idle_count = 0
                    # Speed up polling when file is actively changing
                    poll_interval = max(0.1, poll_interval * 0.8)
                except Exception as e:
                    console.print(f"tailf: {path}: {e}", highlight=False)
                    break
            elif new_size < last_offset:
                # File truncated - restart from beginning
                last_offset = 0
                poll_interval = 0.5
            else:
                # No changes - slow down polling
                idle_count += 1
                if idle_count > 3:
                    poll_interval = min(2.0, poll_interval * 1.2)

    except KeyboardInterrupt:
        console.print("\n", highlight=False)
    except Exception as e:
        # Re-raise to let the CLI command handler add the prefix
        raise


def cmd_watch(client, command_func, args: list, interval: float = 2.0):
    """
    Execute a command repeatedly at fixed intervals

    Args:
        client: PFS client instance
        command_func: The command function to execute (e.g., cmd_ls, cmd_cat)
        args: Arguments to pass to the command
        interval: Interval in seconds between executions (default: 2.0)
    """
    import platform

    try:
        iteration = 0
        while True:
            # Clear screen
            if platform.system() == "Windows":
                os.system("cls")
            else:
                os.system("clear")

            # Display header with timestamp and interval
            now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            header = f"Every {interval}s: {command_func.__name__[4:]} {' '.join(args)}    {now}"
            console.print(f"[bold cyan]{header}[/bold cyan]\n", highlight=False)

            # Execute the command
            try:
                command_func(client, *args)
            except Exception as e:
                console.print(f"[red]Command error: {e}[/red]", highlight=False)

            # Wait for next iteration
            iteration += 1
            time.sleep(interval)

    except KeyboardInterrupt:
        console.print("\n[yellow]Watch stopped[/yellow]", highlight=False)
    except Exception as e:
        console.print(f"\n[red]Watch error: {e}[/red]", highlight=False)
        raise


def cmd_tree(client, path: str = "/", max_depth: Optional[int] = None):
    """Display directory tree structure

    Args:
        path: Root path to display tree from
        max_depth: Maximum depth to traverse (None for unlimited)
    """
    # Detect if we can use Unicode box-drawing characters
    # Try to encode them to check if the terminal supports it
    use_unicode = True
    try:
        # Test if stdout can handle Unicode
        test_chars = "├──└──│"
        if hasattr(sys.stdout, 'encoding') and sys.stdout.encoding:
            test_chars.encode(sys.stdout.encoding)
        else:
            # If no encoding info, assume ASCII
            use_unicode = False
    except (UnicodeEncodeError, AttributeError):
        use_unicode = False

    # Define tree characters based on Unicode support
    if use_unicode:
        BRANCH = "├── "
        LAST = "└── "
        VERTICAL = "│   "
        SPACE = "    "
    else:
        BRANCH = "|-- "
        LAST = "`-- "
        VERTICAL = "|   "
        SPACE = "    "

    # Statistics
    stats = {"dirs": 0, "files": 0}

    def _tree_recursive(current_path: str, prefix: str, depth: int):
        """Recursively display tree structure using DFS"""
        # Check max depth
        if max_depth is not None and depth >= max_depth:
            return

        try:
            # List directory contents
            files = client.ls(current_path)
            if not files:
                return

            # Sort: directories first, then by name
            dirs = sorted([f for f in files if f.get("isDir", False)], key=lambda x: x.get("name", ""))
            regular_files = sorted([f for f in files if not f.get("isDir", False)], key=lambda x: x.get("name", ""))
            all_files = dirs + regular_files

            for i, file_info in enumerate(all_files):
                name = file_info.get("name", "")
                is_dir = file_info.get("isDir", False)
                is_last = (i == len(all_files) - 1)

                # Determine the tree characters
                if is_last:
                    current_prefix = prefix + LAST
                    child_prefix = prefix + SPACE
                else:
                    current_prefix = prefix + BRANCH
                    child_prefix = prefix + VERTICAL

                # Print the entry
                try:
                    if is_dir:
                        console.print(f"{current_prefix}[bold cyan]{name}/[/bold cyan]", highlight=False)
                        stats["dirs"] += 1
                        # Recursively process subdirectory
                        child_path = f"{current_path.rstrip('/')}/{name}"
                        _tree_recursive(child_path, child_prefix, depth + 1)
                    else:
                        size = file_info.get("size", 0)
                        # Format size
                        if size < 1024:
                            size_str = f"{size}B"
                        elif size < 1024 * 1024:
                            size_str = f"{size/1024:.1f}K"
                        elif size < 1024 * 1024 * 1024:
                            size_str = f"{size/(1024*1024):.1f}M"
                        else:
                            size_str = f"{size/(1024*1024*1024):.1f}G"

                        console.print(f"{current_prefix}{name} [{size_str}]", highlight=False)
                        stats["files"] += 1
                except UnicodeEncodeError:
                    # If encoding fails, try printing without formatting
                    print(f"{current_prefix}{name}{'/' if is_dir else ''}")
                    if is_dir:
                        stats["dirs"] += 1
                        child_path = f"{current_path.rstrip('/')}/{name}"
                        _tree_recursive(child_path, child_prefix, depth + 1)
                    else:
                        stats["files"] += 1

        except Exception as e:
            # Print error but continue
            try:
                console.print(f"{prefix}[red]Error reading {current_path}: {e}[/red]", highlight=False)
            except UnicodeEncodeError:
                print(f"{prefix}Error reading {current_path}: {e}")

    # Print the root path
    try:
        console.print(f"[bold cyan]{path}[/bold cyan]", highlight=False)
    except UnicodeEncodeError:
        print(path)

    # Start recursive traversal
    _tree_recursive(path, "", 0)

    # Print statistics
    try:
        console.print(f"\n{stats['dirs']} directories, {stats['files']} files", highlight=False)
    except UnicodeEncodeError:
        print(f"\n{stats['dirs']} directories, {stats['files']} files")

def _grep_multiple_files(client, file_paths: list, pattern: str, case_insensitive: bool, count_only: bool, stream: bool):
    """Helper function to grep multiple files and display combined results

    Args:
        client: PFS client instance
        file_paths: List of file paths to search
        pattern: Regular expression pattern to search for
        case_insensitive: Whether to perform case-insensitive matching
        count_only: Only print the count of matches
        stream: Stream results as they are found
    """
    total_matches = 0

    for file_path in file_paths:
        try:
            if stream:
                # Stream mode for each file
                for item in client.grep(file_path, pattern, recursive=False, case_insensitive=case_insensitive, stream=True):
                    if item.get('type') == 'summary':
                        total_matches += item['count']
                        if 'error' in item:
                            console.print(f"[red]Error in {file_path}: {item['error']}[/red]", highlight=False)
                    else:
                        # Display match
                        if not count_only:
                            try:
                                console.print(
                                    f"[cyan]{item['file']}[/cyan]:[yellow]{item['line']}[/yellow]:{item['content']}",
                                    highlight=False
                                )
                            except UnicodeEncodeError:
                                print(f"{item['file']}:{item['line']}:{item['content']}")
            else:
                # Non-stream mode for each file
                result = client.grep(file_path, pattern, recursive=False, case_insensitive=case_insensitive, stream=False)
                total_matches += result['count']

                if not count_only and result['count'] > 0:
                    for match in result['matches']:
                        try:
                            console.print(
                                f"[cyan]{match['file']}[/cyan]:[yellow]{match['line']}[/yellow]:{match['content']}",
                                highlight=False
                            )
                        except UnicodeEncodeError:
                            print(f"{match['file']}:{match['line']}:{match['content']}")

        except Exception as e:
            console.print(f"[red]grep: {file_path}: {e}[/red]", highlight=False)
            continue

    # Display summary
    if count_only:
        console.print(f"{total_matches}", highlight=False)
    else:
        console.print(f"\n[green]Found {total_matches} match(es) across {len(file_paths)} file(s)[/green]", highlight=False)

def cmd_grep(client, path: str, pattern: str, recursive: bool = False, case_insensitive: bool = False, count_only: bool = False, stream: bool = False):
    """Search for a pattern in files using regular expressions

    Args:
        client: PFS client instance
        path: Path to file or directory to search (supports wildcards like *.log)
        pattern: Regular expression pattern to search for
        recursive: Whether to search recursively in directories
        case_insensitive: Whether to perform case-insensitive matching
        count_only: Only print the count of matches, not the matches themselves
        stream: Stream results as they are found (useful for large searches)
    """
    import fnmatch

    # Check if path contains wildcards
    if '*' in path or '?' in path:
        # Expand glob pattern
        dir_path = os.path.dirname(path) or '/'
        file_pattern = os.path.basename(path)

        try:
            # List directory
            files = client.ls(dir_path)

            # Filter files matching the pattern
            matched_files = []
            for file_info in files:
                if fnmatch.fnmatch(file_info['name'], file_pattern):
                    if not file_info['isDir']:  # Only include files, not directories
                        matched_files.append(os.path.join(dir_path, file_info['name']))

            if not matched_files:
                console.print(f"[yellow]No files matching pattern: {path}[/yellow]", highlight=False)
                return

            # Search in all matched files
            _grep_multiple_files(client, matched_files, pattern, case_insensitive, count_only, stream)
            return

        except Exception as e:
            console.print(f"[red]grep: failed to expand pattern {path}: {e}[/red]", highlight=False)
            return

    # No wildcards, use normal grep
    try:
        if stream:
            # Stream mode: display results as they come
            match_count = 0
            for item in client.grep(path, pattern, recursive=recursive, case_insensitive=case_insensitive, stream=True):
                if item.get('type') == 'summary':
                    # Final summary
                    if not count_only:
                        console.print(f"\n[green]Found {item['count']} match(es)[/green]", highlight=False)
                    else:
                        console.print(f"{item['count']}", highlight=False)
                    if 'error' in item:
                        console.print(f"[red]Error: {item['error']}[/red]", highlight=False)
                else:
                    # Regular match
                    if not count_only:
                        file_path = item['file']
                        line_num = item['line']
                        content = item['content']

                        # Format output like grep: file:line:content
                        try:
                            console.print(
                                f"[cyan]{file_path}[/cyan]:[yellow]{line_num}[/yellow]:{content}",
                                highlight=False
                            )
                        except UnicodeEncodeError:
                            # Fallback to plain print
                            print(f"{file_path}:{line_num}:{content}")
                    match_count += 1

            if count_only and match_count == 0:
                console.print("0", highlight=False)

        else:
            # Non-stream mode: get all results at once
            result = client.grep(path, pattern, recursive=recursive, case_insensitive=case_insensitive, stream=False)

            if count_only:
                # Only print count
                console.print(f"{result['count']}", highlight=False)
            else:
                # Print all matches
                if result['count'] == 0:
                    console.print("[yellow]No matches found[/yellow]", highlight=False)
                else:
                    for match in result['matches']:
                        file_path = match['file']
                        line_num = match['line']
                        content = match['content']

                        # Format output like grep: file:line:content
                        try:
                            console.print(
                                f"[cyan]{file_path}[/cyan]:[yellow]{line_num}[/yellow]:{content}",
                                highlight=False
                            )
                        except UnicodeEncodeError:
                            # Fallback to plain print
                            print(f"{file_path}:{line_num}:{content}")

                    # Print summary
                    console.print(f"\n[green]Found {result['count']} match(es)[/green]", highlight=False)

    except Exception as e:
        console.print(f"[red]grep: {e}[/red]", highlight=False)
