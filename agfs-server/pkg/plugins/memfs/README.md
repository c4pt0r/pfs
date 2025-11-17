MemFS Plugin - In-Memory File System

This plugin provides a full-featured in-memory file system.

DYNAMIC MOUNTING WITH AGFS SHELL:

  Interactive shell:
  agfs:/> mount memfs /mem
  agfs:/> mount memfs /tmp
  agfs:/> mount memfs /scratch init_dirs='["/home","/tmp","/data"]'

  Direct command:
  uv run agfs mount memfs /mem
  uv run agfs mount memfs /tmp init_dirs='["/work","/cache"]'

CONFIGURATION PARAMETERS:

  Optional:
  - init_dirs: Array of directories to create automatically on mount

  Examples:
  agfs:/> mount memfs /workspace init_dirs='["/projects","/builds","/logs"]'

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
  agfs:/> mkdir /memfs/data
  agfs:/> echo "hello" > /memfs/data/file.txt
  agfs:/> cat /memfs/data/file.txt
  hello
  agfs:/> ls /memfs/data
  agfs:/> mv /memfs/data/file.txt /memfs/data/renamed.txt

VERSION: 1.0.0
AUTHOR: VFS Server

## License

Apache License 2.0
