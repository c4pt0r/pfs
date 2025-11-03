MemFS Plugin - In-Memory File System

This plugin provides a full-featured in-memory file system.

DYNAMIC MOUNTING WITH PFS SHELL:

  Interactive shell:
  pfs:/> mount memfs /mem
  pfs:/> mount memfs /tmp
  pfs:/> mount memfs /scratch init_dirs='["/home","/tmp","/data"]'

  Direct command:
  uv run pfs mount memfs /mem
  uv run pfs mount memfs /tmp init_dirs='["/work","/cache"]'

CONFIGURATION PARAMETERS:

  Optional:
  - init_dirs: Array of directories to create automatically on mount

  Examples:
  pfs:/> mount memfs /workspace init_dirs='["/projects","/builds","/logs"]'

FEATURES:
  - Standard file system operations (create, read, write, delete)
  - Directory support with hierarchical structure
  - File permissions (chmod)
  - File/directory renaming and moving
  - Metadata tracking

USAGE:
  Create a file:
    touch /path/to/file

  Write to a file:
    echo "content" > /path/to/file

  Read a file:
    cat /path/to/file

  Create a directory:
    mkdir /path/to/dir

  List directory:
    ls /path/to/dir

  Remove file/directory:
    rm /path/to/file
    rm -r /path/to/dir

  Move/rename:
    mv /old/path /new/path

  Change permissions:
    chmod 755 /path/to/file

EXAMPLES:
  pfs:/> mkdir /memfs/data
  pfs:/> echo "hello" > /memfs/data/file.txt
  pfs:/> cat /memfs/data/file.txt
  hello
  pfs:/> ls /memfs/data
  pfs:/> mv /memfs/data/file.txt /memfs/data/renamed.txt

VERSION: 1.0.0
AUTHOR: VFS Server

## License

Apache License 2.0
