package queuefs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/c4pt0r/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs-server/pkg/plugin"
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
// Files represent queue operations:
//
//	/enqueue - write to this file to enqueue a message
//	/dequeue - read from this file to dequeue a message
//	/peek    - read to peek at the next message without removing it
//	             The peek file's modTime reflects the latest enqueued message timestamp
//	             This can be used for implementing poll offset logic
//	/size    - read to get queue size
//	/clear   - write to this file to clear the queue
type QueueFSPlugin struct {
	queue           []QueueMessage
	mu              sync.Mutex
	metadata        plugin.PluginMetadata
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
		queue: []QueueMessage{},
		metadata: plugin.PluginMetadata{
			Name:        PluginName,
			Version:     "1.0.0",
			Description: "Message queue service plugin",
			Author:      "VFS Server",
		},
	}
}

func (q *QueueFSPlugin) Name() string {
	return q.metadata.Name
}

func (q *QueueFSPlugin) Initialize(config map[string]interface{}) error {
	return nil
}

func (q *QueueFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &queueFS{plugin: q}
}

func (q *QueueFSPlugin) GetReadme() string {
	return `QueueFS Plugin - Message Queue Service

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
`
}

func (q *QueueFSPlugin) Shutdown() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = nil
	return nil
}

// queueFS implements the FileSystem interface for queue operations
type queueFS struct {
	plugin *QueueFSPlugin
}

var controlFiles = map[string]bool{
	"/enqueue": true,
	"/dequeue": true,
	"/peek":    true,
	"/size":    true,
	"/clear":   true,
	"/README":  true,
}

func (qfs *queueFS) Create(path string) error {
	if _, ok := controlFiles[path]; ok {
		return nil // Control files already exist
	}
	return fmt.Errorf("cannot create files in queuefs service")
}

func (qfs *queueFS) Mkdir(path string, perm uint32) error {
	return fmt.Errorf("cannot create directories in queuefs service")
}

func (qfs *queueFS) Remove(path string) error {
	return fmt.Errorf("cannot remove files in queuefs service")
}

func (qfs *queueFS) RemoveAll(path string) error {
	return fmt.Errorf("cannot remove files in queuefs service")
}

func (qfs *queueFS) Read(path string, offset int64, size int64) ([]byte, error) {
	var data []byte
	var err error

	switch path {
	case "/dequeue":
		data, err = qfs.dequeue()
	case "/peek":
		data, err = qfs.peek()
	case "/size":
		data, err = qfs.size()
	case "/README":
		data = []byte(qfs.plugin.GetReadme())
	case "/enqueue", "/clear":
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
	var msgID []byte
	var err error
	switch path {
	case "/enqueue":
		if msgID, err = qfs.enqueue(data); err != nil {
			return nil, err
		}
		return msgID, nil
	case "/clear":
		if err := qfs.clear(); err != nil {
			return nil, err
		}
		return []byte("OK"), nil
	default:
		return nil, fmt.Errorf("cannot write to: %s", path)
	}
}

func (qfs *queueFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path != "/" {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	qfs.plugin.mu.Lock()
	queueSize := len(qfs.plugin.queue)
	lastEnqueueTime := qfs.plugin.lastEnqueueTime
	qfs.plugin.mu.Unlock()

	now := time.Now()
	readme := qfs.plugin.GetReadme()

	// Use lastEnqueueTime for peek's ModTime, or current time if no messages yet
	peekModTime := lastEnqueueTime
	if peekModTime.IsZero() {
		peekModTime = now
	}

	files := []filesystem.FileInfo{
		{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444, // read-only
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "doc"},
		},
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
			ModTime: time.Now(),
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueStatus},
		},
		{
			Name:    "clear",
			Size:    0,
			Mode:    0222, // write-only
			ModTime: time.Now(),
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: MetaValueQueueControl},
		},
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

	if _, ok := controlFiles[path]; !ok {
		return nil, fmt.Errorf("no such file: %s", path)
	}

	mode := uint32(0644)
	if path == "/enqueue" || path == "/clear" {
		mode = 0222
	} else {
		mode = 0444
	}

	fileType := MetaValueQueueControl
	size := int64(0)
	modTime := time.Now()

	if path == "/size" {
		fileType = MetaValueQueueStatus
	} else if path == "/README" {
		fileType = "doc"
		readme := qfs.plugin.GetReadme()
		size = int64(len(readme))
	} else if path == "/peek" {
		// Use lastEnqueueTime for peek's ModTime to support poll offset logic
		qfs.plugin.mu.Lock()
		if !qfs.plugin.lastEnqueueTime.IsZero() {
			modTime = qfs.plugin.lastEnqueueTime
		}
		qfs.plugin.mu.Unlock()
	}

	return &filesystem.FileInfo{
		Name:    path[1:],
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

func (qfs *queueFS) enqueue(data []byte) ([]byte, error) {
	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	now := time.Now()
	msg := QueueMessage{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		Data:      string(data),
		Timestamp: now,
	}

	qfs.plugin.queue = append(qfs.plugin.queue, msg)

	// Update lastEnqueueTime to ensure monotonic increase for poll offset logic
	// Even if messages arrive at the same nanosecond, we ensure the time is always increasing
	if now.After(qfs.plugin.lastEnqueueTime) {
		qfs.plugin.lastEnqueueTime = now
	} else {
		// If time hasn't advanced, add 1 nanosecond to ensure monotonic increase
		qfs.plugin.lastEnqueueTime = qfs.plugin.lastEnqueueTime.Add(1 * time.Nanosecond)
	}

	return []byte(msg.ID), nil
}

func (qfs *queueFS) dequeue() ([]byte, error) {
	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	if len(qfs.plugin.queue) == 0 {
		// Return empty JSON object instead of error for empty queue
		return []byte("{}"), nil
	}

	msg := qfs.plugin.queue[0]
	qfs.plugin.queue = qfs.plugin.queue[1:]

	return json.Marshal(msg)
}

func (qfs *queueFS) peek() ([]byte, error) {
	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	if len(qfs.plugin.queue) == 0 {
		// Return empty JSON object instead of error for empty queue
		return []byte("{}"), nil
	}

	msg := qfs.plugin.queue[0]
	return json.Marshal(msg)
}

func (qfs *queueFS) size() ([]byte, error) {
	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	return []byte(strconv.Itoa(len(qfs.plugin.queue))), nil
}

func (qfs *queueFS) clear() error {
	qfs.plugin.mu.Lock()
	defer qfs.plugin.mu.Unlock()

	qfs.plugin.queue = []QueueMessage{}
	// Reset lastEnqueueTime when queue is cleared
	qfs.plugin.lastEnqueueTime = time.Time{}
	return nil
}

