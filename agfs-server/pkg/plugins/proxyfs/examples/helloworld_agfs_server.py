#!/usr/bin/env python3
"""
HelloWorld AGFS Server - A simple Python implementation of AGFS HTTP API

This server implements a minimal read-only file system with a single file:
  /hello.txt -> "Hello, World!"

It can be used with ProxyFS to demonstrate remote file system access.

Usage:
    python3 helloworld_agfs_server.py [--port PORT]

Example:
    # Start the server
    python3 helloworld_agfs_server.py --port 9090
"""

from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import json
import argparse
from datetime import datetime, timezone


class HelloWorldFileSystem:
    """A simple in-memory read-only file system"""

    def __init__(self):
        # Define our simple file system structure
        now = datetime.now(timezone.utc).isoformat()
        self.files = {
            "/": {
                "name": "/",
                "isDir": True,
                "size": 0,
                "mode": 0o755,
                "modTime": now,
                "meta": {"type": "directory"}
            },
            "/hello.txt": {
                "name": "hello.txt",
                "isDir": False,
                "size": 14,
                "mode": 0o644,
                "modTime": now,
                "content": b"Hello, World!\n",
                "meta": {"type": "file", "description": "A friendly greeting"}
            },
            "/README.md": {
                "name": "README.md",
                "isDir": False,
                "size": 0,
                "mode": 0o444,
                "modTime": now,
                "content": b"""# HelloWorld FileSystem

This is a simple read-only file system implemented in Python.

## Files

- `/hello.txt` - A simple greeting message
- `/README.md` - This file

## Features

- Read-only access
- Compatible with AGFS HTTP API
- Can be mounted via ProxyFS

## Try it!

```bash
cat /hello.txt
```
""",
                "meta": {"type": "markdown"}
            }
        }

    def list_directory(self, path):
        """List directory contents"""
        if path not in self.files:
            raise FileNotFoundError(f"No such directory: {path}")

        if not self.files[path]["isDir"]:
            raise NotADirectoryError(f"Not a directory: {path}")

        # Return all files in root directory
        if path == "/":
            return [
                {
                    "name": info["name"],
                    "size": info["size"],
                    "mode": info["mode"],
                    "modTime": info["modTime"],
                    "isDir": info["isDir"],
                    "meta": info.get("meta", {})
                }
                for p, info in self.files.items()
                if p != "/" and not info["isDir"]
            ]
        return []

    def read_file(self, path, offset=0, size=-1):
        """Read file content with optional offset and size

        Args:
            path: File path
            offset: Starting position (default: 0)
            size: Number of bytes to read (-1 means read all)

        Returns:
            tuple: (data, is_eof) where is_eof indicates if we reached end of file
        """
        if path not in self.files:
            raise FileNotFoundError(f"No such file: {path}")

        if self.files[path]["isDir"]:
            raise IsADirectoryError(f"Is a directory: {path}")

        content = self.files[path]["content"]
        content_len = len(content)

        # Validate offset
        if offset < 0:
            offset = 0
        if offset >= content_len:
            return b"", True  # EOF

        # Calculate end position
        if size < 0:
            # Read all remaining data
            end = content_len
        else:
            end = offset + size
            if end > content_len:
                end = content_len

        # Extract the range
        result = content[offset:end]

        # Check if we reached EOF
        is_eof = (end >= content_len)

        return result, is_eof

    def stat(self, path):
        """Get file/directory information"""
        if path not in self.files:
            raise FileNotFoundError(f"No such file or directory: {path}")

        info = self.files[path]
        return {
            "name": info["name"],
            "size": info["size"],
            "mode": info["mode"],
            "modTime": info["modTime"],
            "isDir": info["isDir"],
            "meta": info.get("meta", {})
        }


class PFSRequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler implementing AGFS API"""

    # Class-level file system instance
    fs = HelloWorldFileSystem()

    def _send_json_response(self, status_code, data):
        """Send JSON response"""
        self.send_response(status_code)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(data).encode('utf-8'))

    def _send_binary_response(self, status_code, data):
        """Send binary response"""
        self.send_response(status_code)
        self.send_header('Content-Type', 'application/octet-stream')
        self.end_headers()
        self.wfile.write(data)

    def _send_error_response(self, status_code, error_message):
        """Send error response"""
        self._send_json_response(status_code, {"error": error_message})

    def _get_path_param(self):
        """Extract path parameter from query string"""
        parsed_url = urlparse(self.path)
        query_params = parse_qs(parsed_url.query)
        path = query_params.get('path', ['/'])[0]
        return path

    def _get_offset_size_params(self):
        """Extract offset and size parameters from query string"""
        parsed_url = urlparse(self.path)
        query_params = parse_qs(parsed_url.query)

        offset = 0
        size = -1

        if 'offset' in query_params:
            try:
                offset = int(query_params['offset'][0])
            except (ValueError, IndexError):
                pass

        if 'size' in query_params:
            try:
                size = int(query_params['size'][0])
            except (ValueError, IndexError):
                pass

        return offset, size

    def do_GET(self):
        """Handle GET requests"""
        parsed_url = urlparse(self.path)
        endpoint = parsed_url.path

        try:
            # Health check
            if endpoint == '/api/v1/health':
                self._send_json_response(200, {"status": "healthy"})
                return

            # Read file
            elif endpoint == '/api/v1/files':
                path = self._get_path_param()
                offset, size = self._get_offset_size_params()
                try:
                    content, is_eof = self.fs.read_file(path, offset, size)
                    self._send_binary_response(200, content)
                except FileNotFoundError as e:
                    self._send_error_response(404, str(e))
                except IsADirectoryError as e:
                    self._send_error_response(400, str(e))
                return

            # List directory
            elif endpoint == '/api/v1/directories':
                path = self._get_path_param()
                try:
                    files = self.fs.list_directory(path)
                    self._send_json_response(200, {"files": files})
                except FileNotFoundError as e:
                    self._send_error_response(404, str(e))
                except NotADirectoryError as e:
                    self._send_error_response(400, str(e))
                return

            # Stat
            elif endpoint == '/api/v1/stat':
                path = self._get_path_param()
                try:
                    info = self.fs.stat(path)
                    self._send_json_response(200, info)
                except FileNotFoundError as e:
                    self._send_error_response(404, str(e))
                return

            else:
                self._send_error_response(404, f"Endpoint not found: {endpoint}")

        except Exception as e:
            self._send_error_response(500, f"Internal server error: {str(e)}")

    def do_POST(self):
        """Handle POST requests - not supported in read-only FS"""
        self._send_error_response(403, "This is a read-only file system")

    def do_PUT(self):
        """Handle PUT requests - not supported in read-only FS"""
        self._send_error_response(403, "This is a read-only file system")

    def do_DELETE(self):
        """Handle DELETE requests - not supported in read-only FS"""
        self._send_error_response(403, "This is a read-only file system")

    def log_message(self, format, *args):
        """Override to customize logging"""
        print(f"[{self.log_date_time_string()}] {format % args}")


def main():
    parser = argparse.ArgumentParser(
        description='HelloWorld AGFS Server - A simple read-only file system'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=9091,
        help='Port to listen on (default: 9090)'
    )
    parser.add_argument(
        '--host',
        type=str,
        default='0.0.0.0',
        help='Host to bind to (default: 0.0.0.0)'
    )
    args = parser.parse_args()

    server_address = (args.host, args.port)
    httpd = HTTPServer(server_address, PFSRequestHandler)

    print(f"""
╔═══════════════════════════════════════════════════════════════╗
║          HelloWorld FS Server                                 ║
║          A simple read-only mock agfs http service             ║
╚═══════════════════════════════════════════════════════════════╝

Server running at: http://{args.host}:{args.port}
API base URL:      http://localhost:{args.port}/api/v1

Files available:
  /hello.txt   - A friendly greeting
  /README.md   - Documentation

Press Ctrl+C to stop the server.
""")

    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n\nShutting down server...")
        httpd.shutdown()
        print("Server stopped.")


if __name__ == '__main__':
    main()
