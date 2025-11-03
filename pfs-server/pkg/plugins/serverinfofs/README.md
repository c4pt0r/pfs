ServerInfoFS Plugin - Server Metadata and Information

This plugin provides runtime information about the PFS server.

MOUNT:
  pfs:/> mount serverinfofs /info

USAGE:
  View server version:
    cat /version

  View server uptime:
    cat /uptime

  View server info:
    cat /info

FILES:
  /version  - Server version information
  /uptime   - Server uptime since start
  /info     - Complete server information (JSON)
  /README   - This file

EXAMPLES:
  # Check server version
  pfs:/> cat /serverinfofs/version
  1.0.0

  # Check uptime
  pfs:/> cat /serverinfofs/uptime
  Server uptime: 5m30s

  # Get complete info
  pfs:/> cat /serverinfofs/server_info
  {
    "version": "1.0.0",
    "uptime": "5m30s",
    "go_version": "go1.21",
    ...
  }

## License

Apache License 2.0
