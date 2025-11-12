#!/usr/bin/env python3
"""PFS MCP Server - Expose PFS operations through Model Context Protocol"""

import json
import logging
from typing import Any, Optional
from mcp.server import Server
from mcp.types import Tool, TextContent, Prompt, PromptMessage
from pypfs import PFSClient, PFSClientError, cp, upload, download

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("pfs-mcp")


class PFSMCPServer:
    """MCP Server for PFS operations"""

    def __init__(self, pfs_url: str = "http://localhost:8080/api/v1"):
        self.server = Server("pfs-mcp")
        self.pfs_url = pfs_url
        self.client: Optional[PFSClient] = None
        self._setup_handlers()

    def _get_client(self) -> PFSClient:
        """Get or create PFS client"""
        if self.client is None:
            self.client = PFSClient(self.pfs_url)
        return self.client

    def _setup_handlers(self):
        """Setup MCP request handlers"""

        @self.server.list_prompts()
        async def list_prompts() -> list[Prompt]:
            """List available prompts"""
            return [
                Prompt(
                    name="pfs_introduction",
                    description="Introduction to PFS (Plugin-based File System) - core concepts and architecture"
                )
            ]

        @self.server.get_prompt()
        async def get_prompt(name: str, arguments: dict[str, str] | None = None) -> PromptMessage:
            """Get prompt content"""
            if name == "pfs_introduction":
                return PromptMessage(
                    role="user",
                    content=TextContent(
                        type="text",
                        text="""# PFS (Plugin-based File System) - Introduction

## Overview
PFS Server is a RESTful file system server inspired by Plan9 that leverages a powerful plugin architecture. It exposes various services—including message queues, key-value stores, databases, and remote systems—through a unified virtual file system interface.

## Core Philosophy
The system follows the Unix philosophy of "everything is a file" but extends it to modern cloud services and data stores. By representing diverse backend services as file hierarchies, PFS provides a consistent, intuitive interface for accessing heterogeneous systems.

## Key Features

### Plugin Architecture
The system allows mounting multiple filesystems and services at different paths, enabling flexible service composition. Each plugin implements the filesystem interface but can represent any kind of backend service.

### External Plugin Support
Plugins load dynamically from:
- Shared libraries (.so on Linux, .dylib on macOS, .dll on Windows)
- WebAssembly modules (.wasm)
- HTTP(S) URLs for remote plugin loading

This enables extending PFS without server recompilation or restart.

### Unified API
A single HTTP REST interface handles operations across all mounted plugins:
- GET /api/v1/files?path=/xxx - Read file content
- PUT /api/v1/files?path=/xxx - Write file content
- GET /api/v1/directories?path=/xxx - List directory
- POST /api/v1/directories?path=/xxx - Create directory
- DELETE /api/v1/files?path=/xxx - Remove file/directory
- GET /api/v1/stat?path=/xxx - Get file info
- POST /api/v1/rename - Move/rename file
- POST /api/v1/grep - Search in files

### Dynamic Management
Plugins can be managed at runtime via API:
- Mount/unmount plugins at any path
- Load/unload external plugins
- Configure multiple instances of the same plugin type
- Query mounted plugins and their configurations

### Multi-Instance Capability
The same plugin type can run multiple independent instances. For example:
- Multiple database connections at /db/users, /db/products, /db/logs
- Multiple S3 buckets at /s3/backup, /s3/public, /s3/archive
- Multiple remote servers federated at /remote/server1, /remote/server2

## Architecture

```
┌─────────────────────────────────────────────┐
│           HTTP REST API (Port 8080)         │
│          /api/v1/files, /directories        │
└───────────────────┬─────────────────────────┘
                    │
         ┌──────────▼──────────┐
         │    MountableFS      │  ← Central router
         │  (Path → Plugin)    │
         └──────────┬──────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
   ┌────▼─────┐          ┌─────▼────┐
   │ Built-in │          │ External │
   │ Plugins  │          │ Plugins  │
   └────┬─────┘          └─────┬────┘
        │                      │
   ┌────▼──────────────────────▼────┐
   │ QueueFS, KVFS, MemFS, SQLFS,  │
   │ ProxyFS, S3FS, LocalFS, etc.  │
   └───────────────────────────────┘
```

The MountableFS layer routes requests to the appropriate plugin based on the requested path, enabling seamless integration of multiple services.

## Built-in Plugins

- **QueueFS**: Message queue operations via files (publish/subscribe)
- **KVFS**: Key-value data storage (simple get/set operations)
- **MemFS**: In-memory temporary storage (fast, volatile)
- **SQLFS**: Database-backed operations (persistent, queryable)
- **ProxyFS**: Remote server federation (mount remote PFS servers)
- **S3FS**: S3-compatible object storage integration
- **LocalFS**: Local filesystem access
- **HTTPFS**: HTTP-based file access

## Common Use Cases

1. **Unified Data Access**: Access databases, object storage, and local files through a single interface
2. **Service Composition**: Combine multiple data sources at different mount points
3. **Remote Federation**: Mount remote PFS servers as local directories
4. **Plugin Development**: Extend functionality with custom plugins (WebAssembly, shared libraries)
5. **Streaming Operations**: Stream large files or continuous data (logs, metrics)
6. **Pattern Matching**: Use grep for searching across different backends

## Working with PFS via MCP

When using PFS through this MCP server, you have access to all these capabilities through simple tool calls. Each tool operation maps to the PFS REST API, allowing you to:
- Navigate mounted plugins as directory hierarchies
- Read/write data across different backend services
- Search for patterns using grep
- Manage plugin lifecycle (mount/unmount)
- Monitor system health

The key insight is that whether you're reading from a SQL database at /db/users/data, an S3 bucket at /s3/logs/2024.txt, or a local file at /local/config.json, you use the same consistent file operations."""
                    )
                )
            raise ValueError(f"Unknown prompt: {name}")

        @self.server.list_tools()
        async def list_tools() -> list[Tool]:
            """List available PFS tools"""
            return [
                Tool(
                    name="pfs_ls",
                    description="List directory contents in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Directory path to list (default: /)",
                                "default": "/"
                            }
                        }
                    }
                ),
                Tool(
                    name="pfs_cat",
                    description="Read file content from PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "File path to read"
                            },
                            "offset": {
                                "type": "integer",
                                "description": "Starting offset (default: 0)",
                                "default": 0
                            },
                            "size": {
                                "type": "integer",
                                "description": "Number of bytes to read (default: -1 for all)",
                                "default": -1
                            }
                        },
                        "required": ["path"]
                    }
                ),
                Tool(
                    name="pfs_write",
                    description="Write content to a file in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "File path to write to"
                            },
                            "content": {
                                "type": "string",
                                "description": "Content to write to the file"
                            }
                        },
                        "required": ["path", "content"]
                    }
                ),
                Tool(
                    name="pfs_mkdir",
                    description="Create a directory in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Directory path to create"
                            },
                            "mode": {
                                "type": "string",
                                "description": "Permissions mode (default: 755)",
                                "default": "755"
                            }
                        },
                        "required": ["path"]
                    }
                ),
                Tool(
                    name="pfs_rm",
                    description="Remove a file or directory from PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Path to remove"
                            },
                            "recursive": {
                                "type": "boolean",
                                "description": "Remove directories recursively (default: false)",
                                "default": False
                            }
                        },
                        "required": ["path"]
                    }
                ),
                Tool(
                    name="pfs_stat",
                    description="Get file or directory information from PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Path to get information about"
                            }
                        },
                        "required": ["path"]
                    }
                ),
                Tool(
                    name="pfs_mv",
                    description="Move or rename a file/directory in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "old_path": {
                                "type": "string",
                                "description": "Source path"
                            },
                            "new_path": {
                                "type": "string",
                                "description": "Destination path"
                            }
                        },
                        "required": ["old_path", "new_path"]
                    }
                ),
                Tool(
                    name="pfs_grep",
                    description="Search for pattern in files using regular expressions",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Path to search in (file or directory)"
                            },
                            "pattern": {
                                "type": "string",
                                "description": "Regular expression pattern to search for"
                            },
                            "recursive": {
                                "type": "boolean",
                                "description": "Search recursively in directories (default: false)",
                                "default": False
                            },
                            "case_insensitive": {
                                "type": "boolean",
                                "description": "Case-insensitive search (default: false)",
                                "default": False
                            }
                        },
                        "required": ["path", "pattern"]
                    }
                ),
                Tool(
                    name="pfs_mounts",
                    description="List all mounted plugins in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                Tool(
                    name="pfs_mount",
                    description="Mount a plugin in PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "fstype": {
                                "type": "string",
                                "description": "Filesystem type (e.g., 'sqlfs', 'memfs', 's3fs')"
                            },
                            "path": {
                                "type": "string",
                                "description": "Mount path"
                            },
                            "config": {
                                "type": "object",
                                "description": "Plugin configuration (varies by fstype)",
                                "default": {}
                            }
                        },
                        "required": ["fstype", "path"]
                    }
                ),
                Tool(
                    name="pfs_unmount",
                    description="Unmount a plugin from PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Mount path to unmount"
                            }
                        },
                        "required": ["path"]
                    }
                ),
                Tool(
                    name="pfs_health",
                    description="Check PFS server health status",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                Tool(
                    name="pfs_cp",
                    description="Copy a file or directory within PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "src": {
                                "type": "string",
                                "description": "Source path in PFS"
                            },
                            "dst": {
                                "type": "string",
                                "description": "Destination path in PFS"
                            },
                            "recursive": {
                                "type": "boolean",
                                "description": "Copy directories recursively (default: false)",
                                "default": False
                            },
                            "stream": {
                                "type": "boolean",
                                "description": "Use streaming for large files (default: false)",
                                "default": False
                            }
                        },
                        "required": ["src", "dst"]
                    }
                ),
                Tool(
                    name="pfs_upload",
                    description="Upload a file or directory from local filesystem to PFS",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "local_path": {
                                "type": "string",
                                "description": "Path to local file or directory"
                            },
                            "remote_path": {
                                "type": "string",
                                "description": "Destination path in PFS"
                            },
                            "recursive": {
                                "type": "boolean",
                                "description": "Upload directories recursively (default: false)",
                                "default": False
                            },
                            "stream": {
                                "type": "boolean",
                                "description": "Use streaming for large files (default: false)",
                                "default": False
                            }
                        },
                        "required": ["local_path", "remote_path"]
                    }
                ),
                Tool(
                    name="pfs_download",
                    description="Download a file or directory from PFS to local filesystem",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "remote_path": {
                                "type": "string",
                                "description": "Path in PFS"
                            },
                            "local_path": {
                                "type": "string",
                                "description": "Destination path on local filesystem"
                            },
                            "recursive": {
                                "type": "boolean",
                                "description": "Download directories recursively (default: false)",
                                "default": False
                            },
                            "stream": {
                                "type": "boolean",
                                "description": "Use streaming for large files (default: false)",
                                "default": False
                            }
                        },
                        "required": ["remote_path", "local_path"]
                    }
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Any) -> list[TextContent]:
            """Handle tool calls"""
            try:
                client = self._get_client()

                if name == "pfs_ls":
                    path = arguments.get("path", "/")
                    result = client.ls(path)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2, ensure_ascii=False)
                    )]

                elif name == "pfs_cat":
                    path = arguments["path"]
                    offset = arguments.get("offset", 0)
                    size = arguments.get("size", -1)
                    content = client.cat(path, offset=offset, size=size)
                    # Try to decode as UTF-8, fallback to base64 for binary
                    try:
                        text = content.decode('utf-8')
                    except UnicodeDecodeError:
                        import base64
                        text = f"[Binary content, base64 encoded]\n{base64.b64encode(content).decode('ascii')}"
                    return [TextContent(type="text", text=text)]

                elif name == "pfs_write":
                    path = arguments["path"]
                    content = arguments["content"]
                    result = client.write(path, content.encode('utf-8'))
                    return [TextContent(type="text", text=result)]

                elif name == "pfs_mkdir":
                    path = arguments["path"]
                    mode = arguments.get("mode", "755")
                    result = client.mkdir(path, mode=mode)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_rm":
                    path = arguments["path"]
                    recursive = arguments.get("recursive", False)
                    result = client.rm(path, recursive=recursive)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_stat":
                    path = arguments["path"]
                    result = client.stat(path)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_mv":
                    old_path = arguments["old_path"]
                    new_path = arguments["new_path"]
                    result = client.mv(old_path, new_path)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_grep":
                    path = arguments["path"]
                    pattern = arguments["pattern"]
                    recursive = arguments.get("recursive", False)
                    case_insensitive = arguments.get("case_insensitive", False)
                    result = client.grep(
                        path,
                        pattern,
                        recursive=recursive,
                        case_insensitive=case_insensitive
                    )
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2, ensure_ascii=False)
                    )]

                elif name == "pfs_mounts":
                    result = client.mounts()
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_mount":
                    fstype = arguments["fstype"]
                    path = arguments["path"]
                    config = arguments.get("config", {})
                    result = client.mount(fstype, path, config)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_unmount":
                    path = arguments["path"]
                    result = client.unmount(path)
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_health":
                    result = client.health()
                    return [TextContent(
                        type="text",
                        text=json.dumps(result, indent=2)
                    )]

                elif name == "pfs_cp":
                    src = arguments["src"]
                    dst = arguments["dst"]
                    recursive = arguments.get("recursive", False)
                    stream = arguments.get("stream", False)
                    cp(client, src, dst, recursive=recursive, stream=stream)
                    return [TextContent(
                        type="text",
                        text=f"Successfully copied {src} to {dst}"
                    )]

                elif name == "pfs_upload":
                    local_path = arguments["local_path"]
                    remote_path = arguments["remote_path"]
                    recursive = arguments.get("recursive", False)
                    stream = arguments.get("stream", False)
                    upload(client, local_path, remote_path, recursive=recursive, stream=stream)
                    return [TextContent(
                        type="text",
                        text=f"Successfully uploaded {local_path} to {remote_path}"
                    )]

                elif name == "pfs_download":
                    remote_path = arguments["remote_path"]
                    local_path = arguments["local_path"]
                    recursive = arguments.get("recursive", False)
                    stream = arguments.get("stream", False)
                    download(client, remote_path, local_path, recursive=recursive, stream=stream)
                    return [TextContent(
                        type="text",
                        text=f"Successfully downloaded {remote_path} to {local_path}"
                    )]

                else:
                    return [TextContent(
                        type="text",
                        text=f"Unknown tool: {name}"
                    )]

            except PFSClientError as e:
                logger.error(f"PFS error in {name}: {e}")
                return [TextContent(
                    type="text",
                    text=f"Error: {str(e)}"
                )]
            except Exception as e:
                logger.error(f"Unexpected error in {name}: {e}", exc_info=True)
                return [TextContent(
                    type="text",
                    text=f"Unexpected error: {str(e)}"
                )]

    async def run(self):
        """Run the MCP server"""
        from mcp.server.stdio import stdio_server

        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options()
            )


async def main():
    """Main entry point"""
    import os
    import sys

    # Get PFS server URL from environment or use default
    pfs_url = os.getenv("PFS_SERVER_URL", "http://localhost:8080")

    logger.info(f"Starting PFS MCP Server (connecting to {pfs_url})")

    server = PFSMCPServer(pfs_url)
    await server.run()


def cli():
    """CLI entry point for package script"""
    import asyncio
    asyncio.run(main())


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
