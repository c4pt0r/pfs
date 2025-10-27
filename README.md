PFS (Plugable File System)
---

"Everything is a file."

A tribute to Plan9

## Installation

### Quick Install pfs-server (Daily Build)

```bash
curl -fsSL https://raw.githubusercontent.com/c4pt0r/pfs/master/install.sh | sh
```

### pfs-shell

```
$ uv run pfs sh
    ____  ___________
   / __ \/ ____/ ___/
  / /_/ / /_   \__ \
 / ____/ __/  ___/ /
/_/   /_/    /____/

Connected to pfs server at http://localhost:8080/api/v1
press 'help' or '?' for help

pfs:/> tree  .
/
├── hellofs/
│   └── hello [14B]
├── kvfs/
│   ├── keys/
│   └── README [724B]
├── memfs/
│   └── README [947B]
├── queuefs/
│   ├── README [871B]
│   ├── clear [0B]
│   ├── dequeue [0B]
│   ├── enqueue [0B]
│   ├── peek [0B]
│   └── size [1B]
├── s3fs/
│   └── aws/
├── serverinfofs/
│   ├── README [709B]
│   ├── server_info [266B]
│   ├── stats [126B]
│   ├── uptime [14B]
│   └── version [6B]
├── sqlfs/
│   ├── sqlite/
│   └── tidb/
│       ├── README [694B]
│       ├── hello [5B]
│       ├── memfs.readme [922B]
│       ├── test [0B]
│       ├── uptime.txt [19B]
│       └── uptime_copy [14B]
└── streamfs/
    └── README [3.9K]

12 directories, 21 files
```
