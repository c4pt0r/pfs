"""Configuration management for agfs-shell2"""

import os


class Config:
    """Configuration for AGFS shell"""

    def __init__(self):
        # Default AGFS server URL
        self.server_url = os.getenv('AGFS_SERVER_URL', 'http://localhost:8080')

    @classmethod
    def from_env(cls):
        """Create configuration from environment variables"""
        return cls()

    @classmethod
    def from_args(cls, server_url: str = None):
        """Create configuration from command line arguments"""
        config = cls()
        if server_url:
            config.server_url = server_url
        return config

    def __repr__(self):
        return f"Config(server_url={self.server_url})"
