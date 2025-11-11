"""PFS Server API Client"""

import requests
from typing import List, Dict, Any, Optional
from requests.exceptions import ConnectionError, Timeout, RequestException

from .exceptions import PFSClientError


class PFSClient:
    """Client for interacting with PFS (Plugin-based File System) Server API"""

    def __init__(self, api_base_url="http://localhost:8080", timeout=10):
        """
        Initialize PFS client.

        Args:
            api_base_url: API base URL. Can be either full URL with "/api/v1" or just the base.
                         If "/api/v1" is not present, it will be automatically appended.
                         e.g., "http://localhost:8080" or "http://localhost:8080/api/v1"
            timeout: Request timeout in seconds (default: 10)
        """
        api_base_url = api_base_url.rstrip("/")
        # Auto-append /api/v1 if not present
        if not api_base_url.endswith("/api/v1"):
            api_base_url = api_base_url + "/api/v1"
        self.api_base = api_base_url
        self.session = requests.Session()
        self.timeout = timeout

    def _handle_request_error(self, e: Exception, operation: str = "request") -> None:
        """Convert request exceptions to user-friendly error messages"""
        if isinstance(e, ConnectionError):
            # Extract host and port from the error message
            url_parts = self.api_base.split("://")
            if len(url_parts) > 1:
                host_port = url_parts[1].split("/")[0]
            else:
                host_port = "server"
            raise PFSClientError(f"Connection refused - server not running at {host_port}")
        elif isinstance(e, Timeout):
            raise PFSClientError(f"Request timeout after {self.timeout}s")
        elif isinstance(e, requests.exceptions.HTTPError):
            # Extract useful error information from response
            if hasattr(e, 'response') and e.response is not None:
                status_code = e.response.status_code
                # Try to get error message from JSON response first (priority)
                try:
                    error_data = e.response.json()
                    error_msg = error_data.get("error", "")
                    if error_msg:
                        # Use the server's detailed error message
                        raise PFSClientError(error_msg)
                except (ValueError, KeyError, TypeError):
                    # If JSON parsing fails, fall through to generic status code messages
                    pass
                except PFSClientError:
                    # Re-raise our own error
                    raise

                # Fallback to generic messages based on status codes
                if status_code == 404:
                    raise PFSClientError("No such file or directory")
                elif status_code == 403:
                    raise PFSClientError("Permission denied")
                elif status_code == 409:
                    raise PFSClientError("Resource already exists")
                elif status_code == 500:
                    raise PFSClientError("Internal server error")
                elif status_code == 502:
                    raise PFSClientError("Bad Gateway - backend service unavailable")
                else:
                    raise PFSClientError(f"HTTP error {status_code}")
            else:
                raise PFSClientError("HTTP error")
        else:
            # For other exceptions, re-raise with simplified message
            raise PFSClientError(str(e))

    def health(self) -> Dict[str, Any]:
        """Check server health"""
        response = self.session.get(f"{self.api_base}/health", timeout=self.timeout)
        response.raise_for_status()
        return response.json()

    def ls(self, path: str = "/") -> List[Dict[str, Any]]:
        """List directory contents"""
        try:
            response = self.session.get(
                f"{self.api_base}/directories",
                params={"path": path},
                timeout=self.timeout
            )
            response.raise_for_status()
            data = response.json()
            files = data.get("files")
            return files if files is not None else []
        except Exception as e:
            self._handle_request_error(e)

    def cat(self, path: str, offset: int = 0, size: int = -1, stream: bool = False):
        """Read file content with optional offset and size

        Args:
            path: File path
            offset: Starting position (default: 0)
            size: Number of bytes to read (default: -1, read all)
            stream: Enable streaming mode for continuous reads (default: False)

        Returns:
            If stream=False: bytes content
            If stream=True: Response object for iteration
        """
        try:
            params = {"path": path}

            if stream:
                params["stream"] = "true"
                # Streaming mode - return response object for iteration
                response = self.session.get(
                    f"{self.api_base}/files",
                    params=params,
                    stream=True,
                    timeout=None  # No timeout for streaming
                )
                response.raise_for_status()
                return response
            else:
                # Normal mode - return content
                if offset > 0:
                    params["offset"] = str(offset)
                if size >= 0:
                    params["size"] = str(size)

                response = self.session.get(
                    f"{self.api_base}/files",
                    params=params,
                    timeout=self.timeout
                )
                response.raise_for_status()
                return response.content
        except Exception as e:
            self._handle_request_error(e)

    def write(self, path: str, data: bytes) -> str:
        """Write data to file and return the response message"""
        try:
            response = self.session.put(
                f"{self.api_base}/files",
                params={"path": path},
                data=data,
                timeout=self.timeout
            )
            response.raise_for_status()
            result = response.json()
            return result.get("message", "OK")
        except Exception as e:
            self._handle_request_error(e)

    def create(self, path: str) -> Dict[str, Any]:
        """Create a new file"""
        try:
            response = self.session.post(
                f"{self.api_base}/files",
                params={"path": path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def mkdir(self, path: str, mode: str = "755") -> Dict[str, Any]:
        """Create a directory"""
        try:
            response = self.session.post(
                f"{self.api_base}/directories",
                params={"path": path, "mode": mode},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def rm(self, path: str, recursive: bool = False) -> Dict[str, Any]:
        """Remove a file or directory"""
        try:
            params = {"path": path}
            if recursive:
                params["recursive"] = "true"
            response = self.session.delete(
                f"{self.api_base}/files",
                params=params,
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def stat(self, path: str) -> Dict[str, Any]:
        """Get file/directory information"""
        try:
            response = self.session.get(
                f"{self.api_base}/stat",
                params={"path": path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def mv(self, old_path: str, new_path: str) -> Dict[str, Any]:
        """Rename/move a file or directory"""
        try:
            response = self.session.post(
                f"{self.api_base}/rename",
                params={"path": old_path},
                json={"newPath": new_path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def chmod(self, path: str, mode: int) -> Dict[str, Any]:
        """Change file permissions"""
        try:
            response = self.session.post(
                f"{self.api_base}/chmod",
                params={"path": path},
                json={"mode": mode},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def mounts(self) -> List[Dict[str, Any]]:
        """List all mounted plugins"""
        try:
            response = self.session.get(f"{self.api_base}/mounts", timeout=self.timeout)
            response.raise_for_status()
            data = response.json()
            return data.get("mounts", [])
        except Exception as e:
            self._handle_request_error(e)

    def mount(self, fstype: str, path: str, config: Dict[str, Any]) -> Dict[str, Any]:
        """Mount a plugin dynamically

        Args:
            fstype: Filesystem type (e.g., 'sqlfs', 's3fs', 'memfs')
            path: Mount path
            config: Plugin configuration as dictionary

        Returns:
            Response with message
        """
        try:
            response = self.session.post(
                f"{self.api_base}/mount",
                json={"fstype": fstype, "path": path, "config": config},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def unmount(self, path: str) -> Dict[str, Any]:
        """Unmount a plugin"""
        try:
            response = self.session.post(
                f"{self.api_base}/unmount",
                json={"path": path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def load_plugin(self, library_path: str) -> Dict[str, Any]:
        """Load an external plugin from a shared library or HTTP(S) URL

        Args:
            library_path: Path to the shared library (.so/.dylib/.dll) or HTTP(S) URL

        Returns:
            Response with message and plugin name
        """
        try:
            response = self.session.post(
                f"{self.api_base}/plugins/load",
                json={"library_path": library_path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def unload_plugin(self, library_path: str) -> Dict[str, Any]:
        """Unload an external plugin

        Args:
            library_path: Path to the shared library

        Returns:
            Response with message
        """
        try:
            response = self.session.post(
                f"{self.api_base}/plugins/unload",
                json={"library_path": library_path},
                timeout=self.timeout
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def list_plugins(self) -> List[str]:
        """List all loaded external plugins

        Returns:
            List of plugin library paths
        """
        try:
            response = self.session.get(
                f"{self.api_base}/plugins",
                timeout=self.timeout
            )
            response.raise_for_status()
            data = response.json()
            return data.get("loaded_plugins", [])
        except Exception as e:
            self._handle_request_error(e)

    def grep(self, path: str, pattern: str, recursive: bool = False, case_insensitive: bool = False, stream: bool = False):
        """Search for a pattern in files using regular expressions

        Args:
            path: Path to file or directory to search
            pattern: Regular expression pattern to search for
            recursive: Whether to search recursively in directories (default: False)
            case_insensitive: Whether to perform case-insensitive matching (default: False)
            stream: Whether to stream results as NDJSON (default: False)

        Returns:
            If stream=False: Dict with 'matches' (list of match objects) and 'count'
            If stream=True: Iterator yielding match dicts and a final summary dict

        Example (non-stream):
            >>> result = client.grep("/local/test-grep", "error", recursive=True)
            >>> print(result['count'])
            2

        Example (stream):
            >>> for item in client.grep("/local/test-grep", "error", recursive=True, stream=True):
            ...     if item.get('type') == 'summary':
            ...         print(f"Total: {item['count']}")
            ...     else:
            ...         print(f"{item['file']}:{item['line']}: {item['content']}")
        """
        try:
            response = self.session.post(
                f"{self.api_base}/grep",
                json={
                    "path": path,
                    "pattern": pattern,
                    "recursive": recursive,
                    "case_insensitive": case_insensitive,
                    "stream": stream
                },
                timeout=None if stream else self.timeout,
                stream=stream
            )
            response.raise_for_status()

            if stream:
                # Return iterator for streaming results
                return self._parse_ndjson_stream(response)
            else:
                # Return complete result
                return response.json()
        except Exception as e:
            self._handle_request_error(e)

    def _parse_ndjson_stream(self, response):
        """Parse NDJSON streaming response line by line"""
        import json
        for line in response.iter_lines():
            if line:
                try:
                    yield json.loads(line)
                except json.JSONDecodeError as e:
                    # Skip malformed lines
                    continue
