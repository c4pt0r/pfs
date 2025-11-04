"""PFS Server API Client"""

import requests
from typing import List, Dict, Any, Optional
from requests.exceptions import ConnectionError, Timeout, RequestException

class PFSClientError(Exception):
    """Custom exception for PFS client errors"""
    pass

class PFSClient:
    """Client for interacting with PFS (Plugin-based File System) Server API"""

    def __init__(self, api_base_url, timeout=10):
        """
        Initialize PFS client.

        Args:
            api_base_url: Full API base URL including version, e.g., "http://localhost:8080/api/v1"
            timeout: Request timeout in seconds (default: 10)
        """
        self.api_base = api_base_url.rstrip("/")
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
                # Try to get error message from JSON response
                try:
                    error_data = e.response.json()
                    error_msg = error_data.get("error", "")
                    if error_msg:
                        raise PFSClientError(error_msg)
                except:
                    pass

                # Handle specific status codes
                if status_code == 404:
                    raise PFSClientError("No such file or directory")
                elif status_code == 403:
                    raise PFSClientError("Permission denied")
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
        response = self.session.post(
            f"{self.api_base}/files",
            params={"path": path},
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def mkdir(self, path: str, mode: str = "755") -> Dict[str, Any]:
        """Create a directory"""
        response = self.session.post(
            f"{self.api_base}/directories",
            params={"path": path, "mode": mode},
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def rm(self, path: str, recursive: bool = False) -> Dict[str, Any]:
        """Remove a file or directory"""
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
        response = self.session.post(
            f"{self.api_base}/rename",
            params={"path": old_path},
            json={"newPath": new_path},
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def chmod(self, path: str, mode: int) -> Dict[str, Any]:
        """Change file permissions"""
        response = self.session.post(
            f"{self.api_base}/chmod",
            params={"path": path},
            json={"mode": mode},
            timeout=self.timeout
        )
        response.raise_for_status()
        return response.json()

    def mounts(self) -> List[Dict[str, Any]]:
        """List all mounted plugins"""
        response = self.session.get(f"{self.api_base}/mounts", timeout=self.timeout)
        response.raise_for_status()
        data = response.json()
        return data.get("mounts", [])

    def mount(self, fstype: str, path: str, config: Dict[str, Any]) -> Dict[str, Any]:
        """Mount a plugin dynamically

        Args:
            fstype: Filesystem type (e.g., 'sqlfs', 's3fs', 'memfs')
            path: Mount path
            config: Plugin configuration as dictionary

        Returns:
            Response with message
        """
        response = self.session.post(
            f"{self.api_base}/mount",
            json={"fstype": fstype, "path": path, "config": config},
            timeout=self.timeout
        )
        if not response.ok:
            # Try to extract error message from response body
            try:
                error_data = response.json()
                error_msg = error_data.get("error", str(response.status_code))
            except:
                error_msg = response.text or str(response.status_code)
            raise Exception(error_msg)
        return response.json()

    def unmount(self, path: str) -> Dict[str, Any]:
        """Unmount a plugin"""
        response = self.session.post(
            f"{self.api_base}/unmount",
            json={"path": path},
            timeout=self.timeout
        )
        if not response.ok:
            # Try to extract error message from response body
            try:
                error_data = response.json()
                error_msg = error_data.get("error", str(response.status_code))
            except:
                error_msg = response.text or str(response.status_code)
            raise Exception(error_msg)
        return response.json()
