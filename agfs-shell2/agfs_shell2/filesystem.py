"""AGFS File System abstraction layer"""

from typing import Optional, Iterator, Union, BinaryIO
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

    def read_file(self, path: str, offset: int = 0, size: int = -1, stream: bool = False) -> Union[bytes, Iterator[bytes]]:
        """
        Read file content from AGFS

        Args:
            path: File path in AGFS
            offset: Starting byte offset (default: 0)
            size: Number of bytes to read, -1 for all (default: -1)
            stream: If True, return iterator for streaming; if False, return all content

        Returns:
            If stream=False: File content as bytes
            If stream=True: Iterator yielding chunks of bytes

        Raises:
            AGFSClientError: If file cannot be read
        """
        try:
            if stream:
                # Return streaming response iterator
                response = self.client.cat(path, offset=offset, size=size, stream=True)
                return response.iter_content(chunk_size=8192)
            else:
                # Return all content at once
                return self.client.cat(path, offset=offset, size=size)
        except AGFSClientError as e:
            # SDK error already includes path, don't duplicate it
            raise AGFSClientError(str(e))

    def write_file(self, path: str, data: Union[bytes, Iterator[bytes], BinaryIO], append: bool = False) -> None:
        """
        Write data to file in AGFS

        Args:
            path: File path in AGFS
            data: Data to write (bytes, iterator of bytes, or file-like object)
            append: If True, append to file; if False, overwrite

        Raises:
            AGFSClientError: If file cannot be written
        """
        try:
            if append:
                # For append mode, we need to read existing content first
                # This means we can't stream directly, need to collect all data
                try:
                    existing = self.client.cat(path)
                except AGFSClientError:
                    # File doesn't exist, just write new data
                    existing = b''

                # Collect data if it's streaming
                if hasattr(data, '__iter__') and not isinstance(data, (bytes, bytearray)):
                    chunks = [existing]
                    for chunk in data:
                        chunks.append(chunk)
                    data = b''.join(chunks)
                elif hasattr(data, 'read'):
                    # File-like object
                    data = existing + data.read()
                else:
                    data = existing + data

            # Write to AGFS - SDK now supports streaming data directly
            # Use max_retries=0 for shell operations (fail fast)
            self.client.write(path, data, max_retries=0)
        except AGFSClientError as e:
            # SDK error already includes path, don't duplicate it
            raise AGFSClientError(str(e))

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
            # SDK error already includes path, don't duplicate it
            raise AGFSClientError(str(e))

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
