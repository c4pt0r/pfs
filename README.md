AGFS
---

Aggregated File System (Agent FS), originally known as pfs (Plugin File System)

"Everything is a file."

A tribute to Plan9, but in RESTful APIs

## Installation

### Quick Install agfs-{server, shell} (Daily Build)

```bash
curl -fsSL https://raw.githubusercontent.com/c4pt0r/agfs/master/install.sh | sh
```


### Docker (agfs-server)

```bash
$ docker pull c4pt0r/agfs-server:latest
```

```plain
$ agfs shell
    ___   _____________ _____
   /   | / ____/ ____// ___/
  / /| |/ / __/ /_   \__ \
 / ___ / /_/ / __/  ___/ /
/_/  |_\____/_/    /____/

Client: agfs-cli 1.0.0 (git: f50d08d, built: 2025-11-16 22:40:49)
Server: version=nightly-8-g6f26aea-dirty, commit=6f26aea, build=2025-11-17_18:23:32
Connected to agfs server at http://localhost:8080/api/v1
press 'help' or '?' for help


// list dir

agfs:/> ls
drwxr-xr-x        0 2025-11-17 15:20:24 sqlfs2/
drwxr-xr-x        0 2025-11-17 15:20:24 sqlfs/
drwxr-xr-x        0 2025-11-17 15:20:24 s3fs/
drwxr-xr-x        0 2025-11-17 15:20:24 queuefs/
drwxr-xr-x        0 2025-11-17 15:20:24 local/
drwxr-xr-x        0 2025-11-17 15:20:24 streamfs/
drwxr-xr-x        0 2025-11-17 15:20:24 queuefs_mem/


// using queuefs

agfs:/> cd queuefs_mem/
agfs:/queuefs_mem> ls
-r--r--r--     3044 2025-11-17 15:20:31 README
agfs:/queuefs_mem> mkdir test_queue
agfs:/queuefs_mem> cd test_queue/
agfs:/queuefs_mem/test_queue> ls
--w--w--w-        0 2025-11-17 15:20:38 enqueue
-r--r--r--        0 2025-11-17 15:20:38 dequeue
-r--r--r--        0 2025-11-17 15:20:38 peek
-r--r--r--        1 2025-11-17 15:20:38 size
--w--w--w-        0 2025-11-17 15:20:38 clear
agfs:/queuefs_mem/test_queue> echo hello > enqueue
019a941e-ea7a-7ee0-93af-f96cea484833
agfs:/queuefs_mem/test_queue> cat dequeue
{"id":"019a941e-ea7a-7ee0-93af-f96cea484833","data":"hello\n","timestamp":"2025-11-17T15:20:45.434785-08:00"}
agfs:/queuefs_mem/test_queue>



// turn a cloud database into a filesystem

agfs:/sqlfs2> cd tidb
agfs:/sqlfs2/tidb> ls
cdrwxr-xr-x        0 2025-11-17 15:22:47 INFORMATION_SCHEMA/
drwxr-xr-x        0 2025-11-17 15:22:47 PERFORMANCE_SCHEMA/
drwxr-xr-x        0 2025-11-17 15:22:47 test/
agfs:/sqlfs2/tidb> cd test
agfs:/sqlfs2/tidb/test> ls
drwxr-xr-x        0 2025-11-17 15:21:51 jobs/
agfs:/sqlfs2/tidb/test> cd jobs
agfs:/sqlfs2/tidb/test/jobs> ls
-r--r--r--        0 2025-11-17 15:22:00 schema
-r--r--r--        0 2025-11-17 15:22:00 count
--w--w--w-        0 2025-11-17 15:22:00 query
--w--w--w-        0 2025-11-17 15:22:00 execute
agfs:/sqlfs2/tidb/test/jobs> cat schema
CREATE TABLE `jobs` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `schedule` varchar(255) DEFAULT NULL,
  `body` longtext DEFAULT NULL,
  `executor` varchar(255) DEFAULT NULL,
  `meta` text DEFAULT NULL,
  `last_run_utc` timestamp NULL DEFAULT NULL,
  `create_time_utc` timestamp DEFAULT CURRENT_TIMESTAMP,
  `enabled` tinyint(1) DEFAULT '1',
  `last_run_exit_code` int(11) DEFAULT NULL,
  `last_run_stdout` longtext DEFAULT NULL,
  `last_run_stderr` longtext DEFAULT NULL,
  PRIMARY KEY (`id`) /*T![clustered_index] CLUSTERED */,
  UNIQUE KEY `name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin AUTO_INCREMENT=420001
agfs:/sqlfs2/tidb/test/jobs> echo 'select 1' > query
[
  {
    "1": 1
  }
]


// Write your own Filesystem (in Wasm, see more details at agfs-server/examples) and mount it dynamicly

agfs:/> plugins load pfs://s3fs/aws/hellofs-wasm.wasm
Reading plugin from AGFS path: /s3fs/aws/hellofs-wasm.wasm
Downloaded to temporary file: /var/folders/lf/nj7v40x934j5s8f8qmtcsx_m0000gn/T/agfs_plugin_vgcajk57.wasm (96192 bytes)
Loaded external plugin: hellofs-wasm
  Source: pfs://s3fs/aws/hellofs-wasm.wasm
  Temporary file: /var/folders/lf/nj7v40x934j5s8f8qmtcsx_m0000gn/T/agfs_plugin_vgcajk57.wasm
Cleaned up temporary file: /var/folders/lf/nj7v40x934j5s8f8qmtcsx_m0000gn/T/agfs_plugin_vgcajk57.wasm
agfs:/> mount hellofs-wasm /hello-wasm
  plugin mounted
agfs:/> cd /hello-wasm
agfs:/hello-wasm> ls
-rw-r--r--       12 0001-01-01 00:00:00 hello.txt
agfs:/hello-wasm> cat hello.txt
Hello World
agfs:/hello-wasm>


// Basic pipeline / IO redirect support 

agfs:/s3fs/aws> echo 'hello world' > hello.txt
Written 12 bytes to hello.txt
agfs:/s3fs/aws> cat hello.txt | tee -a /sqlfs/tidb/hello_in_database | tee -a /local/hello_in_local
hello world
agfs:/s3fs/aws> cat /sqlfs/tidb/hello_in_database
hello world
agfs:/s3fs/aws> cat /local/hello_in_local
hello world
agfs:/s3fs/aws>
```


See more details in:
- [agfs-shell/README.md](https://github.com/c4pt0r/agfs/blob/master/agfs-shell/README.md)
- [agfs-server/README.md](https://github.com/c4pt0r/agfs/blob/master/agfs-server/README.md)





