"""Configuration management for agfs-shell"""

import os


class Config:
    """Configuration for AGFS shell"""

    def __init__(self):
        # Default AGFS server URL
        # Support both AGFS_API_URL (preferred) and AGFS_SERVER_URL (backward compatibility)
        self.server_url = os.getenv('AGFS_API_URL') or os.getenv('AGFS_SERVER_URL', 'http://localhost:8080')

        # Request timeout in seconds (default: 30)
        # Can be overridden via AGFS_TIMEOUT environment variable
        # Increased default for better support of large file transfers
        timeout_str = os.getenv('AGFS_TIMEOUT', '30')
        try:
            self.timeout = int(timeout_str)
        except ValueError:
            self.timeout = 30

    @classmethod
    def from_env(cls):
        """Create configuration from environment variables"""
        return cls()

    @classmethod
    def from_args(cls, server_url: str = None, timeout: int = None):
        """Create configuration from command line arguments"""
        config = cls()
        if server_url:
            config.server_url = server_url
        if timeout is not None:
            config.timeout = timeout
        return config

    def __repr__(self):
        return f"Config(server_url={self.server_url}, timeout={self.timeout})"
