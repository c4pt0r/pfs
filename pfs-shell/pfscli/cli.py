"""Main CLI Entry Point"""

import sys
import click
from prompt_toolkit import PromptSession
from prompt_toolkit.history import FileHistory
from prompt_toolkit.auto_suggest import AutoSuggestFromHistory
from prompt_toolkit.completion import Completer, Completion
from rich.console import Console
from rich.panel import Panel
from rich.text import Text
import os
import tempfile

from .client import PFSClient, PFSClientError
from .commands import CommandHandler
from . import cli_commands

console = Console()


class PFSCompleter(Completer):
    """Custom completer for PFS commands and file paths"""

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
    client = PFSClient(api_base_url)

    # Test connection
    try:
        client.health()
        console.print(r"""    ____  ___________
   / __ \/ ____/ ___/
  / /_/ / /_   \__ \
 / ____/ __/  ___/ /
/_/   /_/    /____/""")
        console.print()
        console.print(f"Connected to pfs server at {api_base_url}")
        console.print("press 'help' or '?' for help")
        print()
    except Exception as e:
        console.print(f"Failed to connect to {api_base_url}\n{e}")
        sys.exit(1)

    # Initialize command handler
    handler = CommandHandler(client)

    # Setup custom completer with file path support
    completer = PFSCompleter(handler)

    # Setup history file with fallback to temp file
    default_history = "~/.pfscli_history"
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
            mode="w", delete=False, suffix="_pfscli_history"
        )
        temp_history_path = temp_history.name
        temp_history.close()
        console.print(
            f"[yellow]Warning: Cannot use {history_path}, using temporary history file[/yellow]"
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
            prompt_text = f"pfs:{handler.current_path}> "

            # Read command
            line = session.prompt(prompt_text)

            # Execute command
            if not handler.execute(line):
                break

        except KeyboardInterrupt:
            console.print("\nUse 'exit' or 'quit' to leave")
            continue
        except EOFError:
            console.print("\nGoodbye!")
            break
        except Exception as e:
            console.print(f"Unexpected error: {e}")


@click.group()
@click.version_option(version="1.0.0", prog_name="pfs")
@click.option(
    "--pfs-api-baseurl",
    default="http://localhost:8080/api/v1",
    help="PFS API base URL",
    show_default=True,
)
@click.pass_context
def main(ctx, pfs_api_baseurl):
    """PFS CLI - Client for PFS (Plugin-based File System) Server"""
    ctx.ensure_object(dict)
    ctx.obj["api_base_url"] = pfs_api_baseurl
    ctx.obj["client"] = PFSClient(pfs_api_baseurl)


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
    except PFSClientError as e:
        console.print(f"ls: {e}")
    except Exception as e:
        console.print(f"ls: {path}: {e}")


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
    except PFSClientError as e:
        console.print(f"tree: {e}")
    except Exception as e:
        console.print(f"tree: {path}: {e}")


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
        console.print(f"cat: {path}: {e}")


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
        console.print(f"tail: {path}: {e}")


@main.command()
@click.argument("path")
@click.pass_context
def mkdir(ctx, path):
    """Create directory"""
    try:
        cli_commands.cmd_mkdir(ctx.obj["client"], path)
    except Exception as e:
        console.print(f"mkdir: {path}: {e}")


@main.command()
@click.argument("path")
@click.option("-r", "--recursive", is_flag=True, help="Remove directories recursively")
@click.pass_context
def rm(ctx, path, recursive):
    """Remove file or directory"""
    try:
        cli_commands.cmd_rm(ctx.obj["client"], path, recursive)
    except Exception as e:
        console.print(f"rm: {path}: {e}")


@main.command()
@click.argument("path")
@click.pass_context
def touch(ctx, path):
    """Create empty file"""
    try:
        cli_commands.cmd_touch(ctx.obj["client"], path)
    except Exception as e:
        console.print(f"touch: {path}: {e}")


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
                    "write: content is required (or use --stream to read from stdin)"
                )
                return
            cli_commands.cmd_write(ctx.obj["client"], path, " ".join(content))
    except Exception as e:
        console.print(f"write: {path}: {e}")


@main.command()
@click.argument("path")
@click.pass_context
def stat(ctx, path):
    """Show file/directory information"""
    try:
        cli_commands.cmd_stat(ctx.obj["client"], path)
    except Exception as e:
        console.print(f"stat: {path}: {e}")


@main.command()
@click.argument("source")
@click.argument("destination")
@click.pass_context
def cp(ctx, source, destination):
    """Copy file"""
    try:
        cli_commands.cmd_cp(ctx.obj["client"], source, destination)
    except Exception as e:
        console.print(f"cp: {source}: {e}")


@main.command()
@click.argument("source")
@click.argument("destination")
@click.pass_context
def mv(ctx, source, destination):
    """Move/rename file"""
    try:
        cli_commands.cmd_mv(ctx.obj["client"], source, destination)
    except Exception as e:
        console.print(f"mv: {source}: {e}")


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
        console.print(f"chmod: invalid mode: '{mode}'")
    except Exception as e:
        console.print(f"chmod: {path}: {e}")


@main.command()
@click.argument("local_path")
@click.argument("pfs_path")
@click.option("-r", "--recursive", is_flag=True, help="Upload directory recursively")
@click.pass_context
def upload(ctx, local_path, pfs_path, recursive):
    """Upload local file or directory to PFS"""
    try:
        cli_commands.cmd_upload(ctx.obj["client"], local_path, pfs_path, recursive)
    except Exception as e:
        console.print(f"upload: {e}")


@main.command()
@click.argument("pfs_path")
@click.argument("local_path")
@click.option("-r", "--recursive", is_flag=True, help="Download directory recursively")
@click.pass_context
def download(ctx, pfs_path, local_path, recursive):
    """Download file or directory from PFS to local filesystem"""
    try:
        cli_commands.cmd_download(ctx.obj["client"], pfs_path, local_path, recursive)
    except Exception as e:
        console.print(f"download: {e}")


@main.command()
@click.pass_context
def mounts(ctx):
    """List mounted plugins"""
    try:
        cli_commands.cmd_mounts(ctx.obj["client"])
    except Exception as e:
        console.print(f"mounts: {e}")


@main.command()
@click.argument("fstype")
@click.argument("path")
@click.argument("config_args", nargs=-1)
@click.pass_context
def mount(ctx, fstype, path, config_args):
    """Mount a plugin dynamically

    \b
    Examples:
      pfs mount memfs /test/mem
      pfs mount sqlfs /test/db backend=sqlite db_path=/tmp/test.db
      pfs mount s3fs /s3 bucket=my-bucket region=us-west-1
    """
    try:
        cli_commands.cmd_mount(ctx.obj["client"], fstype, path, list(config_args))
    except Exception as e:
        console.print(f"mount: {e}")


@main.command()
@click.argument("path")
@click.pass_context
def unmount(ctx, path):
    """Unmount a plugin"""
    try:
        cli_commands.cmd_unmount(ctx.obj["client"], path)
    except Exception as e:
        console.print(f"unmount: {e}")


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
        console.print(f"tailf: {path}: {e}")


if __name__ == "__main__":
    main()
