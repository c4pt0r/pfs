"""Main CLI Entry Point"""

import sys
import os
import tempfile
import click
from prompt_toolkit import PromptSession
from prompt_toolkit.history import FileHistory
from prompt_toolkit.auto_suggest import AutoSuggestFromHistory
from prompt_toolkit.completion import Completer, Completion
from rich.console import Console

from pyagfs import AGFSClient, AGFSClientError
from .commands import CommandHandler
from . import cli_commands
from .version import get_version_string, __version__
from .decorators import agfs_command

console = Console()


class AGFSCompleter(Completer):
    """Custom completer for AGFS commands and file paths"""

    def __init__(self, handler):
        self.handler = handler
        self.command_names = list(handler.commands.keys())

    def get_completions(self, document, complete_event):
        text = document.text_before_cursor
        words = text.split()

        # If we're at the start or only typing the command
        if len(words) == 0 or (len(words) == 1 and not text.endswith(" ")):
            # Complete command names
            word = words[0] if words else ""
            for cmd in self.command_names:
                if cmd.startswith(word.lower()):
                    yield Completion(cmd, start_position=-len(word))

        # If we're typing arguments (file paths)
        else:
            # Get the current word being typed
            if text.endswith(" "):
                current_word = ""
            else:
                current_word = words[-1] if words else ""

            # Determine the directory to list
            if "/" in current_word:
                # Absolute or relative path with directory component
                last_slash = current_word.rfind("/")
                dir_part = current_word[: last_slash + 1]
                file_part = current_word[last_slash + 1 :]

                # Resolve the directory path
                if dir_part.startswith("/"):
                    list_path = dir_part.rstrip("/") or "/"
                else:
                    if self.handler.current_path == "/":
                        list_path = "/" + dir_part.rstrip("/")
                    else:
                        list_path = (
                            self.handler.current_path + "/" + dir_part.rstrip("/")
                        )
            else:
                # No slash, complete from current directory
                dir_part = ""
                file_part = current_word
                list_path = self.handler.current_path

            # Get directory listing
            try:
                files = self.handler.client.ls(list_path)
                for f in files:
                    name = f.get("name", "")
                    is_dir = f.get("isDir", False)

                    # Filter based on what user has typed
                    if name.startswith(file_part):
                        # Add trailing slash for directories
                        display_name = name + "/" if is_dir else name
                        completion_text = dir_part + display_name

                        yield Completion(
                            completion_text,
                            start_position=-len(current_word),
                            display=display_name,
                        )
            except:
                # If we can't list the directory, just skip completion
                pass


def start_repl(api_base_url: str):
    """Start interactive REPL session"""
    # Initialize client
    client = AGFSClient(api_base_url)

    # Test connection
    try:
        health_info = client.health()
        print(r"""    ___   _____________ _____
   /   | / ____/ ____// ___/
  / /| |/ / __/ /_   \__ \
 / ___ / /_/ / __/  ___/ /
/_/  |_\____/_/    /____/
""")
        console.print(f"[dim]Client: {get_version_string()}[/dim]", highlight=False)

        # Display server version info if available
        server_version = health_info.get("version", "unknown")
        server_commit = health_info.get("gitCommit", "unknown")
        server_build_time = health_info.get("buildTime", "unknown")
        console.print(f"[dim]Server: version={server_version}, commit={server_commit}, build={server_build_time}[/dim]", highlight=False)

        console.print(f"Connected to agfs server at {api_base_url}", highlight=False)
        console.print("press 'help' or '?' for help", highlight=False)
        print()
    except Exception as e:
        console.print(f"Failed to connect to {api_base_url}\n{e}", highlight=False)
        sys.exit(1)

    # Initialize command handler
    handler = CommandHandler(client)

    # Setup custom completer with file path support
    completer = AGFSCompleter(handler)

    # Setup history file with fallback to temp file
    default_history = "~/.agfscli_history"
    history_path = os.path.expanduser(default_history)

    # Try to create/access the default history file
    try:
        # Try to create the file or ensure it's writable
        os.makedirs(os.path.dirname(history_path), exist_ok=True)
        with open(history_path, "a"):
            pass  # Just test if we can open for append
        history = FileHistory(history_path)
    except (OSError, IOError, PermissionError) as e:
        # If default history file fails, use a temp file
        temp_history = tempfile.NamedTemporaryFile(
            mode="w", delete=False, suffix="_agfscli_history"
        )
        temp_history_path = temp_history.name
        temp_history.close()
        console.print(
            f"[yellow]Warning: Cannot use {history_path}, using temporary history file[/yellow]",
            highlight=False
        )
        history = FileHistory(temp_history_path)

    # Setup prompt session
    session = PromptSession(
        history=history,
        auto_suggest=AutoSuggestFromHistory(),
        completer=completer,
        complete_while_typing=True,
    )

    # REPL loop
    while True:
        try:
            # Get prompt
            prompt_text = f"agfs:{handler.current_path}> "

            # Read command
            line = session.prompt(prompt_text)

            # Execute command
            if not handler.execute(line):
                break

        except KeyboardInterrupt:
            console.print("\nUse 'exit' or 'quit' to leave", highlight=False)
            continue
        except EOFError:
            console.print("\nGoodbye!", highlight=False)
            break
        except Exception as e:
            console.print(f"Unexpected error: {e}", highlight=False)


@click.group()
@click.version_option(version=get_version_string(), prog_name="agfs")
@click.option(
    "--agfs-api-baseurl",
    default=lambda: os.environ.get("AGFS_API_URL", "http://localhost:8080/api/v1"),
    help="AGFS API base URL (can also set via AGFS_API_URL environment variable)",
    show_default="http://localhost:8080/api/v1",
)
@click.pass_context
def main(ctx, agfs_api_baseurl):
    """AGFS CLI - Client for AGFS (Agent File System) Server"""
    ctx.ensure_object(dict)
    ctx.obj["api_base_url"] = agfs_api_baseurl
    ctx.obj["client"] = AGFSClient(agfs_api_baseurl)


@main.command()
@click.pass_context
def sh(ctx):
    """Start interactive REPL shell"""
    start_repl(ctx.obj["api_base_url"])


@main.command()
@click.pass_context
def shell(ctx):
    """Start interactive REPL shell (alias for sh)"""
    start_repl(ctx.obj["api_base_url"])


@main.command()
@click.argument("path", default="/")
@click.pass_context
def ls(ctx, path):
    """List directory contents"""
    try:
        cli_commands.cmd_ls(ctx.obj["client"], path)
    except AGFSClientError as e:
        console.print(f"ls: {e}", highlight=False)
    except Exception as e:
        console.print(f"ls: {path}: {e}", highlight=False)


@main.command()
@click.argument("path", default="/")
@click.option(
    "-L",
    "--level",
    "max_depth",
    type=int,
    default=None,
    help="Maximum depth to traverse",
)
@click.pass_context
def tree(ctx, path, max_depth):
    """Display directory tree structure"""
    try:
        cli_commands.cmd_tree(ctx.obj["client"], path, max_depth)
    except AGFSClientError as e:
        console.print(f"tree: {e}", highlight=False)
    except Exception as e:
        console.print(f"tree: {path}: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.option(
    "--stream", is_flag=True, help="Enable streaming mode for continuous reads"
)
@click.pass_context
def cat(ctx, path, stream):
    """Display file contents"""
    try:
        cli_commands.cmd_cat(ctx.obj["client"], path, stream=stream)
    except Exception as e:
        console.print(f"cat: {path}: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.option(
    "-n", "--lines", default=10, help="Number of lines to display", show_default=True
)
@click.pass_context
def tail(ctx, path, lines):
    """Display last N lines of a file"""
    try:
        cli_commands.cmd_tail(ctx.obj["client"], path, lines)
    except Exception as e:
        console.print(f"tail: {path}: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.pass_context
@agfs_command()
def mkdir(ctx, path):
    """Create directory"""
    cli_commands.cmd_mkdir(ctx.obj["client"], path)


@main.command()
@click.argument("path")
@click.option("-r", "--recursive", is_flag=True, help="Remove directories recursively")
@click.pass_context
def rm(ctx, path, recursive):
    """Remove file or directory"""
    try:
        cli_commands.cmd_rm(ctx.obj["client"], path, recursive)
    except Exception as e:
        console.print(f"rm: {path}: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.pass_context
@agfs_command()
def touch(ctx, path):
    """Create empty file"""
    cli_commands.cmd_touch(ctx.obj["client"], path)


@main.command()
@click.argument("path")
@click.argument("content", nargs=-1, required=False)
@click.option("--stream", is_flag=True, help="Enable streaming mode to read from stdin")
@click.pass_context
def write(ctx, path, content, stream):
    """Write content to file or stream from stdin"""
    try:
        if stream:
            # Streaming mode from stdin
            cli_commands.cmd_write(ctx.obj["client"], path, stream=True)
        else:
            # Normal mode with content argument
            if not content:
                console.print(
                    "write: content is required (or use --stream to read from stdin)",
                    highlight=False
                )
                return
            cli_commands.cmd_write(ctx.obj["client"], path, " ".join(content))
    except Exception as e:
        console.print(f"write: {path}: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.pass_context
@agfs_command()
def stat(ctx, path):
    """Show file/directory information"""
    cli_commands.cmd_stat(ctx.obj["client"], path)


@main.command()
@click.argument("source")
@click.argument("destination")
@click.pass_context
@agfs_command("cp")
def cp(ctx, source, destination):
    """Copy file"""
    cli_commands.cmd_cp(ctx.obj["client"], source, destination)


@main.command()
@click.argument("source")
@click.argument("destination")
@click.pass_context
@agfs_command("mv")
def mv(ctx, source, destination):
    """Move/rename file"""
    cli_commands.cmd_mv(ctx.obj["client"], source, destination)


@main.command()
@click.argument("mode")
@click.argument("path")
@click.pass_context
def chmod(ctx, mode, path):
    """Change file permissions"""
    try:
        mode_int = int(mode, 8)
        cli_commands.cmd_chmod(ctx.obj["client"], mode_int, path)
    except ValueError:
        console.print(f"chmod: invalid mode: '{mode}'", highlight=False)
    except Exception as e:
        console.print(f"chmod: {path}: {e}", highlight=False)


@main.command()
@click.argument("local_path")
@click.argument("agfs_path")
@click.option("-r", "--recursive", is_flag=True, help="Upload directory recursively")
@click.pass_context
def upload(ctx, local_path, agfs_path, recursive):
    """Upload local file or directory to AGFS"""
    try:
        cli_commands.cmd_upload(ctx.obj["client"], local_path, agfs_path, recursive)
    except Exception as e:
        console.print(f"upload: {e}", highlight=False)


@main.command()
@click.argument("agfs_path")
@click.argument("local_path")
@click.option("-r", "--recursive", is_flag=True, help="Download directory recursively")
@click.pass_context
def download(ctx, agfs_path, local_path, recursive):
    """Download file or directory from AGFS to local filesystem"""
    try:
        cli_commands.cmd_download(ctx.obj["client"], agfs_path, local_path, recursive)
    except Exception as e:
        console.print(f"download: {e}", highlight=False)


@main.command()
@click.pass_context
@agfs_command()
def mounts(ctx):
    """List mounted plugins"""
    cli_commands.cmd_mounts(ctx.obj["client"])


@main.command()
@click.argument("fstype")
@click.argument("path")
@click.argument("config_args", nargs=-1)
@click.pass_context
def mount(ctx, fstype, path, config_args):
    """Mount a plugin dynamically

    \b
    Examples:
      agfs mount memfs /test/mem
      agfs mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db
      agfs mount s3fs /s3 bucket=my-bucket region=us-west-1
    """
    try:
        cli_commands.cmd_mount(ctx.obj["client"], fstype, path, list(config_args))
    except Exception as e:
        console.print(f"mount: {e}", highlight=False)


@main.command()
@click.argument("path")
@click.pass_context
@agfs_command()
def unmount(ctx, path):
    """Unmount a plugin"""
    cli_commands.cmd_unmount(ctx.obj["client"], path)


@main.command()
@click.argument("path")
@click.option(
    "-n",
    "--lines",
    default=10,
    help="Number of initial lines to display",
    show_default=True,
)
@click.pass_context
def tailf(ctx, path, lines):
    """Follow file changes (displays last N lines, then follows new content to EOF)"""
    try:
        cli_commands.cmd_tailf(ctx.obj["client"], path, lines)
    except Exception as e:
        console.print(f"tailf: {path}: {e}", highlight=False)


@main.group()
@click.pass_context
def plugins(ctx):
    """Plugin management commands"""
    pass


@plugins.command(name="load")
@click.argument("library_path")
@click.pass_context
def plugins_load(ctx, library_path):
    """Load an external plugin from a shared library, HTTP(S) URL, or pfs:// URL

    \b
    Examples:
      agfs plugins load ./examples/plugins/hellofs-c/hellofs-c.dylib
      agfs plugins load http://example.com/plugins/myplugin.so
      agfs plugins load https://example.com/plugins/myplugin.dylib
      agfs plugins load pfs://s3fs/aws/plugin.wasm
      agfs plugins load pfs:///mnt/plugins/myplugin.so
    """
    try:
        cli_commands.cmd_load_plugin(ctx.obj["client"], library_path)
    except Exception as e:
        console.print(f"[red]Error loading plugin: {e}[/red]", highlight=False)


@plugins.command(name="unload")
@click.argument("library_path")
@click.pass_context
def plugins_unload(ctx, library_path):
    """Unload an external plugin"""
    try:
        cli_commands.cmd_unload_plugin(ctx.obj["client"], library_path)
    except Exception as e:
        console.print(f"[red]Error unloading plugin: {e}[/red]", highlight=False)


@plugins.command(name="list")
@click.pass_context
def plugins_list(ctx):
    """List all loaded external plugins"""
    try:
        cli_commands.cmd_list_plugins(ctx.obj["client"])
    except Exception as e:
        console.print(f"[red]Error listing plugins: {e}[/red]", highlight=False)


@main.command()
@click.argument("pattern")
@click.argument("path")
@click.option("-r", "--recursive", is_flag=True, help="Search recursively in directories")
@click.option("-i", "--ignore-case", "case_insensitive", is_flag=True, help="Ignore case distinctions")
@click.option("-c", "--count", "count_only", is_flag=True, help="Only print count of matching lines")
@click.option("--stream", is_flag=True, help="Stream results as they are found (for large searches)")
@click.pass_context
def grep(ctx, pattern, path, recursive, case_insensitive, count_only, stream):
    """Search for PATTERN in files at PATH using regular expressions

    PATH supports wildcards (* and ?) to match multiple files.

    \b
    Examples:
      agfs grep "error" /local/test-grep/file.txt
      agfs grep -i "error|warning" /var/log/*.log
      agfs grep -r "test" /local/test-grep
      agfs grep -i "ERROR" /local/logs
      agfs grep -r -i "warning|error" /local/logs
      agfs grep -c "test" /local/test-grep/*.txt
      agfs grep --stream -r "pattern" /large/directory
    """
    try:
        cli_commands.cmd_grep(
            ctx.obj["client"],
            path,
            pattern,
            recursive=recursive,
            case_insensitive=case_insensitive,
            count_only=count_only,
            stream=stream
        )
    except Exception as e:
        console.print(f"[red]grep: {e}[/red]", highlight=False)


if __name__ == "__main__":
    main()
