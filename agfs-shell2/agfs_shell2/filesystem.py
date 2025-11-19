"""AGFS File System abstraction layer"""

from typing import Optional
from pyagfs import AGFSClient, AGFSClientError


class AGFSFileSystem:
    """Abstraction layer for AGFS file system operations"""

    def __init__(self, server_url: str = "http://localhost:8080", timeout: int = 5):
        """
        Initialize AGFS file system

        Args:
            server_url: AGFS server URL (default: http://localhost:8080)
            timeout: Request timeout in seconds (default: 5)
        """
        self.server_url = server_url
        self.client = AGFSClient(server_url, timeout=timeout)
        self._connected = False

    def check_connection(self) -> bool:
        """Check if AGFS server is accessible"""
        if self._connected:
            return True

        try:
            self.client.health()
            self._connected = True
            return True
        except AGFSClientError:
            return False

    def read_file(self, path: str) -> bytes:
        """
        Read file content from AGFS

        Args:
            path: File path in AGFS

        Returns:
            File content as bytes

        Raises:
            AGFSClientError: If file cannot be read
        """
        try:
            return self.client.cat(path)
        except AGFSClientError as e:
            raise AGFSClientError(f"{path}: {str(e)}")

    def write_file(self, path: str, data: bytes, append: bool = False) -> None:
        """
        Write data to file in AGFS

        Args:
            path: File path in AGFS
            data: Data to write
            append: If True, append to file; if False, overwrite

        Raises:
            AGFSClientError: If file cannot be written
        """
        try:
            if append:
                # Read existing content, append new data, then write
                try:
                    existing = self.client.cat(path)
                    data = existing + data
                except AGFSClientError:
                    # File doesn't exist, just write new data
                    pass

            # Use max_retries=0 for shell operations (fail fast)
            self.client.write(path, data, max_retries=0)
        except AGFSClientError as e:
            raise AGFSClientError(f"{path}: {str(e)}")

    def file_exists(self, path: str) -> bool:
        """
        Check if file exists in AGFS

        Args:
            path: File path in AGFS

        Returns:
            True if file exists, False otherwise
        """
        try:
            self.client.stat(path)
            return True
        except AGFSClientError:
            return False

    def is_directory(self, path: str) -> bool:
        """
        Check if path is a directory

        Args:
            path: Path in AGFS

        Returns:
            True if path is a directory, False otherwise
        """
        try:
            info = self.client.stat(path)
            # Check if it's a directory based on mode or isDir field
            return info.get('isDir', False)
        except AGFSClientError:
            return False

    def list_directory(self, path: str):
        """
        List directory contents

        Args:
            path: Directory path in AGFS

        Returns:
            List of file info dicts

        Raises:
            AGFSClientError: If directory cannot be listed
        """
        try:
            return self.client.ls(path)
        except AGFSClientError as e:
            raise AGFSClientError(f"{path}: {str(e)}")

    def get_error_message(self, error: Exception) -> str:
        """
        Get user-friendly error message

        Args:
            error: Exception object

        Returns:
            Formatted error message
        """
        if isinstance(error, AGFSClientError):
            msg = str(error)
            if "Connection refused" in msg:
                return f"AGFS server not running at {self.server_url}"
            return msg
        return str(error)
