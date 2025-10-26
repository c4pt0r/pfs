KVFS Plugin - Key-Value Store Service

This plugin provides a key-value store service through a file system interface.

USAGE:
  Set a key-value pair:
    echo "value" > /keys/<key>

  Get a value:
    cat /keys/<key>

  List all keys:
    ls /keys

  Delete a key:
    rm /keys/<key>

  Rename a key:
    mv /keys/<oldkey> /keys/<newkey>

STRUCTURE:
  /keys/     - Directory containing all key-value pairs
  /README    - This file

EXAMPLES:
  # Set a value
  pfs:/> echo "hello world" > /kvfs/keys/mykey

  # Get a value
  pfs:/> cat /kvfs/keys/mykey
  hello world

  # List all keys
  pfs:/> ls /kvfs/keys

  # Delete a key
  pfs:/> rm /kvfs/keys/mykey

  # Rename a key
  pfs:/> mv /kvfs/keys/oldname /kvfs/keys/newname

## License

Apache License 2.0
