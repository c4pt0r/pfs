"""Command metadata and decorator system for agfs-shell"""

from functools import wraps
from typing import Optional, Set, Callable


class CommandMetadata:
    """Store and manage command metadata"""

    _registry = {}

    @classmethod
    def register(cls, func: Callable, **metadata) -> Callable:
        """
        Register a command with its metadata

        Args:
            func: The command function
            **metadata: Command metadata (needs_path_resolution, supports_streaming, etc.)

        Returns:
            The original function (for decorator chaining)
        """
        # Extract command name from function name (cmd_cat -> cat)
        cmd_name = func.__name__.replace('cmd_', '')
        cls._registry[cmd_name] = metadata
        return func

    @classmethod
    def get_metadata(cls, command_name: str) -> dict:
        """
        Get metadata for a command

        Args:
            command_name: Name of the command

        Returns:
            Dictionary of metadata, or empty dict if command not found
        """
        return cls._registry.get(command_name, {})

    @classmethod
    def needs_path_resolution(cls, command_name: str) -> bool:
        """Check if command needs path resolution for its arguments"""
        return cls.get_metadata(command_name).get('needs_path_resolution', False)

    @classmethod
    def supports_streaming(cls, command_name: str) -> bool:
        """Check if command supports streaming I/O"""
        return cls.get_metadata(command_name).get('supports_streaming', False)

    @classmethod
    def no_pipeline(cls, command_name: str) -> bool:
        """Check if command cannot be used in pipelines"""
        return cls.get_metadata(command_name).get('no_pipeline', False)

    @classmethod
    def changes_cwd(cls, command_name: str) -> bool:
        """Check if command changes the current working directory"""
        return cls.get_metadata(command_name).get('changes_cwd', False)

    @classmethod
    def get_path_arg_indices(cls, command_name: str) -> Optional[Set[int]]:
        """
        Get indices of arguments that should be treated as paths

        Returns:
            Set of argument indices, or None if all non-flag args are paths
        """
        return cls.get_metadata(command_name).get('path_arg_indices', None)

    @classmethod
    def all_commands(cls) -> list:
        """Get list of all registered command names"""
        return list(cls._registry.keys())

    @classmethod
    def get_commands_with_feature(cls, feature: str) -> list:
        """
        Get list of commands that have a specific feature enabled

        Args:
            feature: Feature name (e.g., 'needs_path_resolution', 'supports_streaming')

        Returns:
            List of command names with that feature
        """
        return [
            cmd_name for cmd_name, metadata in cls._registry.items()
            if metadata.get(feature, False)
        ]


def command(
    name: Optional[str] = None,
    needs_path_resolution: bool = False,
    supports_streaming: bool = False,
    no_pipeline: bool = False,
    changes_cwd: bool = False,
    path_arg_indices: Optional[Set[int]] = None
):
    """
    Decorator to register a command with metadata

    Args:
        name: Command name (defaults to function name without 'cmd_' prefix)
        needs_path_resolution: Whether command arguments need path resolution
        supports_streaming: Whether command supports streaming I/O
        no_pipeline: Whether command cannot be used in pipelines
        changes_cwd: Whether command changes current working directory
        path_arg_indices: Set of argument indices that are paths (None = all non-flag args)

    Example:
        @command(needs_path_resolution=True, supports_streaming=True)
        def cmd_cat(process):
            '''Read and concatenate files'''
            # implementation...
    """
    def decorator(func: Callable) -> Callable:
        cmd_name = name or func.__name__.replace('cmd_', '')

        metadata = {
            'needs_path_resolution': needs_path_resolution,
            'supports_streaming': supports_streaming,
            'no_pipeline': no_pipeline,
            'changes_cwd': changes_cwd,
            'path_arg_indices': path_arg_indices,
        }

        CommandMetadata.register(func, **metadata)

        @wraps(func)
        def wrapper(*args, **kwargs):
            return func(*args, **kwargs)

        return wrapper

    return decorator
