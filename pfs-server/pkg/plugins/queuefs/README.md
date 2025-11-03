QueueFS Plugin - Message Queue Service

This plugin provides a message queue service through a file system interface.

DYNAMIC MOUNTING WITH PFS SHELL:

  Interactive shell:
  pfs:/> mount queuefs /queue
  pfs:/> mount queuefs /tasks
  pfs:/> mount queuefs /messages

  Direct command:
  uv run pfs mount queuefs /queue
  uv run pfs mount queuefs /jobs

CONFIGURATION PARAMETERS:

  None required - QueueFS works with default settings

USAGE:
  Enqueue a message:
    echo "your message" > /enqueue

  Dequeue a message:
    cat /dequeue

  Peek at next message (without removing):
    cat /peek

  Get queue size:
    cat /size

  Clear the queue:
    echo "" > /clear

FILES:
  /enqueue  - Write-only file to enqueue messages
  /dequeue  - Read-only file to dequeue messages
  /peek     - Read-only file to peek at next message
  /size     - Read-only file showing queue size
  /clear    - Write-only file to clear all messages
  /README   - This file

EXAMPLES:
  # Enqueue a message
  pfs:/> echo "task-123" > /queuefs/enqueue

  # Check queue size
  pfs:/> cat /queuefs/size
  1

  # Dequeue a message
  pfs:/> cat /queuefs/dequeue
  {"id":"...","data":"task-123","timestamp":"..."}

## License

Apache License 2.0
