"""Decorators to reduce code duplication in CLI commands"""

import functools
from rich.console import Console
from pyagfs import AGFSClientError

console = Console()


def agfs_command(command_name=None):
    """
    Decorator that handles common error handling for CLI commands.

    This decorator:
    - Extracts the client from Click context
    - Handles exceptions and formats error messages
    - Reduces boilerplate code in CLI command definitions

    Args:
        command_name: Name to use in error messages (defaults to function name)

    Example:
        @main.command()
        @click.argument("path")
        @click.pass_context
        @agfs_command()
        def mkdir(ctx, path):
            return cli_commands.cmd_mkdir(ctx.obj["client"], path)
    """
    def decorator(func):
        cmd_name = command_name or func.__name__

        @functools.wraps(func)
        def wrapper(ctx, *args, **kwargs):
            try:
                # Extract client from context and pass to the function
                # The function can choose to use it or not
                return func(ctx, *args, **kwargs)
            except AGFSClientError as e:
                # AGFSClient already formats error messages nicely
                console.print(f"{cmd_name}: {e}", highlight=False)
            except Exception as e:
                # For other exceptions, include the first argument if it's a path
                if args:
                    console.print(f"{cmd_name}: {args[0]}: {e}", highlight=False)
                else:
                    console.print(f"{cmd_name}: {e}", highlight=False)

        return wrapper
    return decorator
