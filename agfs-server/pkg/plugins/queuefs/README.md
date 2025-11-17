QueueFS Plugin - Message Queue Service

This plugin provides a message queue service through a file system interface.

DYNAMIC MOUNTING WITH AGFS SHELL:

  Interactive shell:
  agfs:/> mount queuefs /queue
  agfs:/> mount queuefs /tasks
  agfs:/> mount queuefs /messages

  Direct command:
  uv run agfs mount queuefs /queue
  uv run agfs mount queuefs /jobs

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
  agfs:/> echo "task-123" > /queuefs/enqueue

  # Check queue size
  agfs:/> cat /queuefs/size
  1

  # Dequeue a message
  agfs:/> cat /queuefs/dequeue
  {"id":"...","data":"task-123","timestamp":"..."}

## License

Apache License 2.0
