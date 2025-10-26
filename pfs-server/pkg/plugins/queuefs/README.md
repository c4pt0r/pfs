QueueFS Plugin - Message Queue Service

This plugin provides a message queue service through a file system interface.

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
