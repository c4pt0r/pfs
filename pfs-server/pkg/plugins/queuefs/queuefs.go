package queuefs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
)

const (
	PluginName = "queuefs" // Name of this plugin
)

// Meta values for QueueFS plugin
const (
	MetaValueQueueControl = "control" // Queue control files (enqueue, dequeue, peek, clear)
	MetaValueQueueStatus  = "status"  // Queue status files (size)
)

// QueueFSPlugin provides a message queue service through a file system interface
// Each queue is a directory containing control files:
//
//	/queue_name/enqueue - write to this file to enqueue a message
//	/queue_name/dequeue - read from this file to dequeue a message
//	/queue_name/peek    - read to peek at the next message without removing it
//	                      The peek file's modTime reflects the latest enqueued message timestamp
//	                      This can be used for implementing poll offset logic
//	/queue_name/size    - read to get queue size
//	/queue_name/clear   - write to this file to clear the queue
type QueueFSPlugin struct {
	queues   map[string]*Queue // Map of queue name to Queue instance
	mu       sync.RWMutex      // Protects the queues map
	metadata plugin.PluginMetadata
}

// Queue represents a single message queue
type Queue struct {
	messages        []QueueMessage
	mu              sync.Mutex
	lastEnqueueTime time.Time // Tracks the timestamp of the most recently enqueued message
}

type QueueMessage struct {
	ID        string    `json:"id"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// NewQueueFSPlugin creates a new queue plugin
func NewQueueFSPlugin() *QueueFSPlugin {
	return &QueueFSPlugin{
		queues: make(map[string]*Queue),
		metadata: plugin.PluginMetadata{
			Name:        PluginName,
			Version:     "1.0.0",
			Description: "Message queue service plugin with multiple queue support",
			Author:      "VFS Server",
		},
	}
}

func (q *QueueFSPlugin) Name() string {
	return q.metadata.Name
}

func (q *QueueFSPlugin) Validate(cfg map[string]interface{}) error {
	// Only mount_path is allowed (injected by framework)
	for key := range cfg {
		if key != "mount_path" {
			return fmt.Errorf("unknown configuration parameter: %s (queuefs accepts no configuration)", key)
		}
	}
	return nil
}

func (q *QueueFSPlugin) Initialize(config map[string]interface{}) error {
	return nil
}

func (q *QueueFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &queueFS{plugin: q}
}

func (q *QueueFSPlugin) GetReadme() string {
	return `QueueFS Plugin - Multiple Message Queue Service

This plugin provides multiple message queue services through a file system interface.
Each queue is a directory containing control files for queue operations.

STRUCTURE:
  /queuefs/
    README          - This documentation
    <queue_name>/   - A queue directory
      enqueue       - Write-only file to enqueue messages
      dequeue       - Read-only file to dequeue messages
      peek          - Read-only file to peek at next message
      size          - Read-only file showing queue size
      clear         - Write-only file to clear all messages

WORKFLOW:
  1. Create a queue:
     mkdir /queuefs/my_queue

  2. Enqueue messages:
     echo "your message" > /queuefs/my_queue/enqueue

  3. Dequeue messages:
     cat /queuefs/my_queue/dequeue

  4. Check queue size:
     cat /queuefs/my_queue/size

  5. Peek without removing:
     cat /queuefs/my_queue/peek

  6. Clear the queue:
     echo "" > /queuefs/my_queue/clear

  7. Delete the queue:
     rm -rf /queuefs/my_queue

NESTED QUEUES:
  You can create queues in nested directories:
    mkdir -p /queuefs/logs/errors
    echo "error: timeout" > /queuefs/logs/errors/enqueue
    cat /queuefs/logs/errors/dequeue

EXAMPLES:
  # Create multiple queues for different purposes
  pfs:/> mkdir /queuefs/orders
  pfs:/> mkdir /queuefs/notifications
  pfs:/> mkdir /queuefs/logs/errors

  # Enqueue messages to different queues
  pfs:/> echo "order-123" > /queuefs/orders/enqueue
  pfs:/> echo "user login" > /queuefs/notifications/enqueue
  pfs:/> echo "connection timeout" > /queuefs/logs/errors/enqueue

  # Check queue sizes
  pfs:/> cat /queuefs/orders/size
  1

  # Dequeue messages
  pfs:/> cat /queuefs/orders/dequeue
  {"id":"...","data":"order-123","timestamp":"..."}

  # List all queues
  pfs:/> ls /queuefs/
  README  orders  notifications  logs

  # Delete a queue when done
  pfs:/> rm -rf /queuefs/orders
`
}

func (q *QueueFSPlugin) Shutdown() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queues = nil
	return nil
}

// queueFS implements the FileSystem interface for queue operations
type queueFS struct {
	plugin *QueueFSPlugin
}

// Control file operations supported within each queue directory
var queueOperations = map[string]bool{
	"enqueue": true,
	"dequeue": true,
	"peek":    true,
	"size":    true,
	"clear":   true,
}

// parseQueuePath parses a path like "/queue_name/operation" or "/dir/queue_name/operation"
// Returns (queueName, operation, isDir, error)
// Examples:
//   - "/myqueue" -> ("myqueue", "", true, nil) - queue directory
//   - "/myqueue/enqueue" -> ("myqueue", "enqueue", false, nil) - queue operation
//   - "/dir/myqueue" -> ("dir/myqueue", "", true, nil) - nested queue directory
//   - "/dir/myqueue/dequeue" -> ("dir/myqueue", "dequeue", false, nil) - nested queue operation
func parseQueuePath(path string) (queueName string, operation string, isDir bool, err error) {
	// Clean the path
	path = filepath.Clean(path)

	if path == "/" || path == "." {
		return "", "", true, nil
	}

	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Split path into components
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return "", "", true, nil
	}

	// Check if the last component is a queue operation
	lastPart := parts[len(parts)-1]
	if queueOperations[lastPart] {
		// This is a queue operation file
		if len(parts) == 1 {
			return "", "", false, fmt.Errorf("invalid path: operation without queue name")
		}
		queueName = strings.Join(parts[:len(parts)-1], "/")
		operation = lastPart
		return queueName, operation, false, nil
	}

	// This is a queue directory (or parent directory)
	queueName = strings.Join(parts, "/")
	return queueName, "", true, nil
}

// isValidQueueOperation checks if an operation name is valid
func isValidQueueOperation(op string) bool {
	return queueOperations[op]
}

func (qfs *queueFS) Create(path string) error {
	_, operation, isDir, err := parseQueuePath(path)
	if err != nil {
		return err
	}

	if isDir {
		return fmt.Errorf("cannot create files: %s is a directory", path)
	}

	if operation != "" && isValidQueueOperation(operation) {
		// Control files are virtual, no need to create
		return nil
	}

	return fmt.Errorf("cannot create files in queuefs: %s", path)
}

func (qfs *queueFS) Mkdir(path string, perm uint32) error {
	queueName, _, isDir, err := parseQueuePath(path)
	if err != nil {
		return err
	}

	if !isDir {
		return fmt.Errorf("cannot create directory: %s is not a valid directory path", path)
	}

	if queueName == "" {
		return fmt.Errorf("invalid queue name")
	}

	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	// Check if queue already exists
	if _, exists := qfs.plugin.queues[queueName]; exists {
		return fmt.Errorf("queue already exists: %s", queueName)
	}

	// Create new queue
	qfs.plugin.queues[queueName] = &Queue{
		messages:        []QueueMessage{},
		lastEnqueueTime: time.Time{},
	}

	return nil
}

func (qfs *queueFS) Remove(path string) error {
	_, operation, isDir, err := parseQueuePath(path)
	if err != nil {
		return err
	}

	if isDir {
		return fmt.Errorf("cannot remove directory with Remove: use RemoveAll instead")
	}

	if operation != "" {
		return fmt.Errorf("cannot remove control files: %s", path)
	}

	return fmt.Errorf("cannot remove: %s", path)
}

func (qfs *queueFS) RemoveAll(path string) error {
	queueName, _, isDir, err := parseQueuePath(path)
	if err != nil {
		return err
	}

	if !isDir {
		return fmt.Errorf("cannot remove: %s is not a directory", path)
	}

	if queueName == "" {
		return fmt.Errorf("cannot remove root directory")
	}

	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	// Check if queue exists
	if _, exists := qfs.plugin.queues[queueName]; !exists {
		return fmt.Errorf("queue does not exist: %s", queueName)
	}

	// Remove the queue
	delete(qfs.plugin.queues, queueName)

	return nil
}

func (qfs *queueFS) Read(path string, offset int64, size int64) ([]byte, error) {
	// Special case: README at root
	if path == "/README" {
		data := []byte(qfs.plugin.GetReadme())
		return plugin.ApplyRangeRead(data, offset, size)
	}

	queueName, operation, isDir, err := parseQueuePath(path)
	if err != nil {
		return nil, err
	}

	if isDir {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	if operation == "" {
		return nil, fmt.Errorf("no such file: %s", path)
	}

	// Get the queue
	qfs.plugin.mu.RLock()
	queue, exists := qfs.plugin.queues[queueName]
	qfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("queue does not exist: %s", queueName)
	}

	var data []byte

	switch operation {
	case "dequeue":
		data, err = qfs.dequeue(queue)
	case "peek":
		data, err = qfs.peek(queue)
	case "size":
		data, err = qfs.size(queue)
	case "enqueue", "clear":
		// Write-only files - return descriptive message
		return []byte(""), fmt.Errorf("permission denied: %s is write-only", path)
	default:
		return nil, fmt.Errorf("no such file: %s", path)
	}

	if err != nil {
		return nil, err
	}

	return plugin.ApplyRangeRead(data, offset, size)
}

func (qfs *queueFS) Write(path string, data []byte) ([]byte, error) {
	queueName, operation, isDir, err := parseQueuePath(path)
	if err != nil {
		return nil, err
	}

	if isDir {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	if operation == "" {
		return nil, fmt.Errorf("cannot write to: %s", path)
	}

	// Get the queue
	qfs.plugin.mu.RLock()
	queue, exists := qfs.plugin.queues[queueName]
	qfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("queue does not exist: %s", queueName)
	}

	switch operation {
	case "enqueue":
		msgID, err := qfs.enqueue(queue, data)
		if err != nil {
			return nil, err
		}
		return msgID, nil
	case "clear":
		if err := qfs.clear(queue); err != nil {
			return nil, err
		}
		return []byte("OK"), nil
	default:
		return nil, fmt.Errorf("cannot write to: %s", path)
	}
}

func (qfs *queueFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	queueName, _, isDir, err := parseQueuePath(path)
	if err != nil {
		return nil, err
	}

	if !isDir {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	now := time.Now()

	// Root directory: list all queues + README
	if path == "/" || queueName == "" {
		qfs.plugin.mu.RLock()
		defer qfs.plugin.mu.RUnlock()

		readme := qfs.plugin.GetReadme()
		files := []filesystem.FileInfo{
			{
				Name:    "README",
				Size:    int64(len(readme)),
				Mode:    0444, // read-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "doc"},
			},
		}

		// Add all queue directories
		for qName := range qfs.plugin.queues {
			// For nested queues, only show top-level directories at root
			parts := strings.Split(qName, "/")
			topLevel := parts[0]

			// Check if we already added this top-level directory
			found := false
			for _, f := range files {
				if f.Name == topLevel && f.IsDir {
					found = true
					break
				}
			}

			if !found {
				files = append(files, filesystem.FileInfo{
					Name:    topLevel,
					Size:    0,
					Mode:    0755,
					ModTime: now,
					IsDir:   true,
					Meta:    filesystem.MetaData{Name: PluginName, Type: "queue"},
				})
			}
		}

		return files, nil
	}

	// Queue directory or intermediate directory: list control files or subdirectories
	qfs.plugin.mu.RLock()
	queue, exists := qfs.plugin.queues[queueName]
	qfs.plugin.mu.RUnlock()

	if exists {
		// This is an actual queue directory - list control files
		queue.mu.Lock()
		queueSize := len(queue.messages)
		lastEnqueueTime := queue.lastEnqueueTime
		queue.mu.Unlock()

		// Use lastEnqueueTime for peek's ModTime, or current time if no messages yet
		peekModTime := lastEnqueueTime
		if peekModTime.IsZero() {
			peekModTime = now
		}

		files := []filesystem.FileInfo{
			{
				Name:    "enqueue",
				Size:    0,
				Mode:    0222, // write-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueControl},
			},
			{
				Name:    "dequeue",
				Size:    0,
				Mode:    0444, // read-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueControl},
			},
			{
				Name:    "peek",
				Size:    0,
				Mode:    0444,        // read-only
				ModTime: peekModTime, // Use last enqueue time for poll offset tracking
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueControl},
			},
			{
				Name:    "size",
				Size:    int64(len(strconv.Itoa(queueSize))),
				Mode:    0444, // read-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueStatus},
			},
			{
				Name:    "clear",
				Size:    0,
				Mode:    0222, // write-only
				ModTime: now,
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueControl},
			},
		}

		return files, nil
	}

	// This might be an intermediate directory (e.g., /dir when we have /dir/queue)
	qfs.plugin.mu.RLock()
	defer qfs.plugin.mu.RUnlock()

	prefix := queueName + "/"
	subdirs := make(map[string]bool)

	for qName := range qfs.plugin.queues {
		if strings.HasPrefix(qName, prefix) {
			// Extract the next level subdirectory
			remainder := strings.TrimPrefix(qName, prefix)
			parts := strings.Split(remainder, "/")
			if len(parts) > 0 {
				subdirs[parts[0]] = true
			}
		}
	}

	if len(subdirs) == 0 {
		return nil, fmt.Errorf("no such directory: %s", path)
	}

	files := []filesystem.FileInfo{}
	for subdir := range subdirs {
		files = append(files, filesystem.FileInfo{
			Name:    subdir,
			Size:    0,
			Mode:    0755,
			ModTime: now,
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "queue"},
		})
	}

	return files, nil
}

func (qfs *queueFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName},
		}, nil
	}

	// Special case: README at root
	if path == "/README" {
		readme := qfs.plugin.GetReadme()
		return &filesystem.FileInfo{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444,
			ModTime: time.Now(),
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "doc"},
		}, nil
	}

	queueName, operation, isDir, err := parseQueuePath(path)
	if err != nil {
		return nil, err
	}

	// Directory stat
	if isDir {
		qfs.plugin.mu.RLock()
		_, exists := qfs.plugin.queues[queueName]
		qfs.plugin.mu.RUnlock()

		if !exists && queueName != "" {
			// Check if it's an intermediate directory
			qfs.plugin.mu.RLock()
			prefix := queueName + "/"
			hasChildren := false
			for qName := range qfs.plugin.queues {
				if strings.HasPrefix(qName, prefix) {
					hasChildren = true
					break
				}
			}
			qfs.plugin.mu.RUnlock()

			if !hasChildren {
				return nil, fmt.Errorf("no such directory: %s", path)
			}
		}

		name := filepath.Base(path)
		if name == "." || name == "/" {
			name = "/"
		}

		return &filesystem.FileInfo{
			Name:    name,
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "queue"},
		}, nil
	}

	// Control file stat
	if operation == "" {
		return nil, fmt.Errorf("no such file: %s", path)
	}

	// Check if queue exists
	qfs.plugin.mu.RLock()
	queue, exists := qfs.plugin.queues[queueName]
	qfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("queue does not exist: %s", queueName)
	}

	mode := uint32(0644)
	if operation == "enqueue" || operation == "clear" {
		mode = 0222
	} else {
		mode = 0444
	}

	fileType := MetaValueQueueControl
	size := int64(0)
	modTime := time.Now()

	if operation == "size" {
		fileType = MetaValueQueueStatus
		queue.mu.Lock()
		queueSize := len(queue.messages)
		queue.mu.Unlock()
		size = int64(len(strconv.Itoa(queueSize)))
	} else if operation == "peek" {
		// Use lastEnqueueTime for peek's ModTime to support poll offset logic
		queue.mu.Lock()
		if !queue.lastEnqueueTime.IsZero() {
			modTime = queue.lastEnqueueTime
		}
		queue.mu.Unlock()
	}

	return &filesystem.FileInfo{
		Name:    operation,
		Size:    size,
		Mode:    mode,
		ModTime: modTime,
		IsDir:   false,
		Meta:    filesystem.MetaData{Name: PluginName, Type: fileType},
	}, nil
}

func (qfs *queueFS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("cannot rename files in queuefs service")
}

func (qfs *queueFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("cannot change permissions in queuefs service")
}

func (qfs *queueFS) Open(path string) (io.ReadCloser, error) {
	data, err := qfs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (qfs *queueFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &queueWriter{qfs: qfs, path: path, buf: &bytes.Buffer{}}, nil
}

type queueWriter struct {
	qfs  *queueFS
	path string
	buf  *bytes.Buffer
}

func (qw *queueWriter) Write(p []byte) (n int, err error) {
	return qw.buf.Write(p)
}

func (qw *queueWriter) Close() error {
	_, err := qw.qfs.Write(qw.path, qw.buf.Bytes())
	return err
}

// Queue operations

func (qfs *queueFS) enqueue(queue *Queue, data []byte) ([]byte, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	now := time.Now()
	msg := QueueMessage{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		Data:      string(data),
		Timestamp: now,
	}

	queue.messages = append(queue.messages, msg)

	// Update lastEnqueueTime to ensure monotonic increase for poll offset logic
	// Even if messages arrive at the same nanosecond, we ensure the time is always increasing
	if now.After(queue.lastEnqueueTime) {
		queue.lastEnqueueTime = now
	} else {
		// If time hasn't advanced, add 1 nanosecond to ensure monotonic increase
		queue.lastEnqueueTime = queue.lastEnqueueTime.Add(1 * time.Nanosecond)
	}

	return []byte(msg.ID), nil
}

func (qfs *queueFS) dequeue(queue *Queue) ([]byte, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	if len(queue.messages) == 0 {
		// Return empty JSON object instead of error for empty queue
		return []byte("{}"), nil
	}

	msg := queue.messages[0]
	queue.messages = queue.messages[1:]

	return json.Marshal(msg)
}

func (qfs *queueFS) peek(queue *Queue) ([]byte, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	if len(queue.messages) == 0 {
		// Return empty JSON object instead of error for empty queue
		return []byte("{}"), nil
	}

	msg := queue.messages[0]
	return json.Marshal(msg)
}

func (qfs *queueFS) size(queue *Queue) ([]byte, error) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	return []byte(strconv.Itoa(len(queue.messages))), nil
}

func (qfs *queueFS) clear(queue *Queue) error {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	queue.messages = []QueueMessage{}
	// Reset lastEnqueueTime when queue is cleared
	queue.lastEnqueueTime = time.Time{}
	return nil
}

