package heartbeatfs

import (
	"bytes"
	"container/heap"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
)

const (
	PluginName = "heartbeatfs"
)

// expiryHeapItem represents an item in the expiry priority queue
type expiryHeapItem struct {
	name       string
	expireTime time.Time
	index      int // index in the heap
}

// expiryHeap implements heap.Interface for managing expiry times
type expiryHeap []*expiryHeapItem

func (h expiryHeap) Len() int           { return len(h) }
func (h expiryHeap) Less(i, j int) bool { return h[i].expireTime.Before(h[j].expireTime) }
func (h expiryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *expiryHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*expiryHeapItem)
	item.index = n
	*h = append(*h, item)
}

func (h *expiryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// HeartbeatItem represents a heartbeat instance
type HeartbeatItem struct {
	name           string
	lastHeartbeat  time.Time
	expireTime     time.Time
	timeout        time.Duration // timeout duration for this item
	heapItem       *expiryHeapItem // reference to heap item for fast update
	mu             sync.RWMutex
}

// HeartbeatFSPlugin provides a heartbeat monitoring service through a file system interface
// Each heartbeat item is a directory containing control files
// Operations:
//   mkdir /heartbeatfs/<dir>     - Create new heartbeat item
//   touch /<dir>/keepalive       - Update heartbeat timestamp
//   echo "data" > /<dir>/keepalive - Update heartbeat timestamp
//   cat /<dir>/ctl               - Read heartbeat status
type HeartbeatFSPlugin struct {
	items          map[string]*HeartbeatItem
	expiryHeap     expiryHeap
	mu             sync.RWMutex
	heapMu         sync.Mutex // separate lock for heap operations
	metadata       plugin.PluginMetadata
	stopChan       chan struct{}
	wg             sync.WaitGroup
	defaultTimeout time.Duration // default timeout from config
}

// NewHeartbeatFSPlugin creates a new heartbeat monitoring plugin
func NewHeartbeatFSPlugin() *HeartbeatFSPlugin {
	hb := &HeartbeatFSPlugin{
		items:      make(map[string]*HeartbeatItem),
		expiryHeap: make(expiryHeap, 0),
		metadata: plugin.PluginMetadata{
			Name:        PluginName,
			Version:     "1.0.0",
			Description: "Heartbeat monitoring service plugin",
			Author:      "AGFS Server",
		},
		stopChan:       make(chan struct{}),
		defaultTimeout: 5 * time.Minute, // default 5 minutes if not configured
	}
	heap.Init(&hb.expiryHeap)
	return hb
}

// cleanupExpiredItems runs in background and removes expired heartbeat items
// Uses a min-heap to efficiently track and remove only expired items
func (hb *HeartbeatFSPlugin) cleanupExpiredItems() {
	defer hb.wg.Done()

	for {
		// Calculate next wake up time based on the earliest expiry
		var sleepDuration time.Duration

		hb.heapMu.Lock()
		if len(hb.expiryHeap) == 0 {
			hb.heapMu.Unlock()
			sleepDuration = 1 * time.Second // default check interval when no items
		} else {
			nextExpiry := hb.expiryHeap[0].expireTime
			hb.heapMu.Unlock()

			now := time.Now()
			if nextExpiry.After(now) {
				sleepDuration = nextExpiry.Sub(now)
				// Cap maximum sleep to avoid sleeping too long
				if sleepDuration > 1*time.Second {
					sleepDuration = 1 * time.Second
				}
			} else {
				sleepDuration = 0 // process immediately
			}
		}

		// Sleep or wait for stop signal
		if sleepDuration > 0 {
			select {
			case <-hb.stopChan:
				return
			case <-time.After(sleepDuration):
			}
		}

		// Process expired items
		now := time.Now()

		for {
			hb.heapMu.Lock()
			if len(hb.expiryHeap) == 0 {
				hb.heapMu.Unlock()
				break
			}

			// Check if the earliest item has expired
			earliest := hb.expiryHeap[0]
			if earliest.expireTime.After(now) {
				hb.heapMu.Unlock()
				break
			}

			// Remove from heap
			heap.Pop(&hb.expiryHeap)
			hb.heapMu.Unlock()

			// Remove from items map
			hb.mu.Lock()
			delete(hb.items, earliest.name)
			hb.mu.Unlock()
		}
	}
}

func (hb *HeartbeatFSPlugin) Name() string {
	return hb.metadata.Name
}

func (hb *HeartbeatFSPlugin) Validate(cfg map[string]interface{}) error {
	allowedKeys := []string{"mount_path", "default_timeout"}
	for key := range cfg {
		found := false
		for _, allowed := range allowedKeys {
			if key == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown configuration parameter: %s (allowed: %v)", key, allowedKeys)
		}
	}
	return nil
}

func (hb *HeartbeatFSPlugin) Initialize(config map[string]interface{}) error {
	// Load default_timeout from config
	if timeoutVal, ok := config["default_timeout"]; ok {
		switch v := timeoutVal.(type) {
		case int:
			hb.defaultTimeout = time.Duration(v) * time.Second
		case float64:
			hb.defaultTimeout = time.Duration(v) * time.Second
		case string:
			// Try to parse as duration string (e.g., "5m", "300s")
			if d, err := time.ParseDuration(v); err == nil {
				hb.defaultTimeout = d
			}
		}
	}

	// Start background cleanup goroutine
	hb.wg.Add(1)
	go hb.cleanupExpiredItems()
	return nil
}

func (hb *HeartbeatFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &heartbeatFS{plugin: hb}
}

func (hb *HeartbeatFSPlugin) GetReadme() string {
	return `HeartbeatFS Plugin - Heartbeat Monitoring Service

This plugin provides a heartbeat monitoring service through a file system interface.

USAGE:
  Create a new heartbeat item:
    mkdir /heartbeatfs/<name>

  Update heartbeat (keepalive):
    touch /heartbeatfs/<name>/keepalive
    echo "ping" > /heartbeatfs/<name>/keepalive

  Update timeout:
    echo "timeout=60" > /heartbeatfs/<name>/ctl

  Check heartbeat status:
    cat /heartbeatfs/<name>/ctl

  Check if heartbeat is alive (stat will fail if expired):
    stat /heartbeatfs/<name>

  List all heartbeat items:
    ls /heartbeatfs

  Remove heartbeat item:
    rm -r /heartbeatfs/<name>

STRUCTURE:
  /<name>/           - Directory for each heartbeat item (auto-deleted when expired)
  /<name>/keepalive  - Touch or write to update heartbeat
  /<name>/ctl        - Read to get status, write to update timeout (timeout=N in seconds)
  /README            - This file

BEHAVIOR:
  - Default timeout: 5 minutes (300 seconds) from last heartbeat
  - Timeout can be customized per item by writing to ctl file
  - Expired items are automatically removed by the system
  - Use stat to check if an item still exists (alive)

EXAMPLES:
  # Create a heartbeat item
  agfs:/> mkdir /heartbeatfs/myservice

  # Send heartbeat
  agfs:/> touch /heartbeatfs/myservice/keepalive

  # Set custom timeout (60 seconds)
  agfs:/> echo "timeout=60" > /heartbeatfs/myservice/ctl

  # Check status
  agfs:/> cat /heartbeatfs/myservice/ctl
  last_heartbeat_ts: 2024-11-21T10:30:00Z
  expire_ts: 2024-11-21T10:31:00Z
  timeout: 60
  status: alive

  # Check if still alive (will fail if expired)
  agfs:/> stat /heartbeatfs/myservice
`
}

func (hb *HeartbeatFSPlugin) Shutdown() error {
	// Stop cleanup goroutine
	close(hb.stopChan)
	hb.wg.Wait()

	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.items = nil
	return nil
}

// heartbeatFS implements the FileSystem interface for heartbeat operations
type heartbeatFS struct {
	plugin *HeartbeatFSPlugin
}

func (hfs *heartbeatFS) Create(path string) error {
	return fmt.Errorf("use mkdir to create heartbeat items")
}

func (hfs *heartbeatFS) Mkdir(path string, perm uint32) error {
	if path == "/" {
		return nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 1 {
		return fmt.Errorf("can only create heartbeat items at root level")
	}

	name := parts[0]
	if name == "" || name == "README" {
		return fmt.Errorf("invalid heartbeat item name: %s", name)
	}

	hfs.plugin.mu.Lock()
	defer hfs.plugin.mu.Unlock()

	if _, exists := hfs.plugin.items[name]; exists {
		return fmt.Errorf("heartbeat item already exists: %s", name)
	}

	now := time.Now()
	defaultTimeout := hfs.plugin.defaultTimeout
	expireTime := now.Add(defaultTimeout)

	// Create heap item
	heapItem := &expiryHeapItem{
		name:       name,
		expireTime: expireTime,
	}

	// Create heartbeat item
	item := &HeartbeatItem{
		name:          name,
		lastHeartbeat: now,
		timeout:       defaultTimeout,
		expireTime:    expireTime,
		heapItem:      heapItem,
	}

	hfs.plugin.items[name] = item

	// Add to heap
	hfs.plugin.heapMu.Lock()
	heap.Push(&hfs.plugin.expiryHeap, heapItem)
	hfs.plugin.heapMu.Unlock()

	return nil
}

func (hfs *heartbeatFS) Remove(path string) error {
	return hfs.RemoveAll(path)
}

func (hfs *heartbeatFS) RemoveAll(path string) error {
	if path == "/" {
		return fmt.Errorf("cannot remove root")
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	name := parts[0]

	hfs.plugin.mu.Lock()
	item, exists := hfs.plugin.items[name]
	if !exists {
		hfs.plugin.mu.Unlock()
		return fmt.Errorf("heartbeat item not found: %s", name)
	}
	delete(hfs.plugin.items, name)
	hfs.plugin.mu.Unlock()

	// Remove from heap
	hfs.plugin.heapMu.Lock()
	if item.heapItem != nil && item.heapItem.index >= 0 {
		heap.Remove(&hfs.plugin.expiryHeap, item.heapItem.index)
	}
	hfs.plugin.heapMu.Unlock()

	return nil
}

func (hfs *heartbeatFS) Read(path string, offset int64, size int64) ([]byte, error) {
	if path == "/" {
		return nil, fmt.Errorf("is a directory")
	}

	if path == "/README" {
		data := []byte(hfs.plugin.GetReadme())
		return plugin.ApplyRangeRead(data, offset, size)
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	name := parts[0]
	file := parts[1]

	hfs.plugin.mu.RLock()
	item, exists := hfs.plugin.items[name]
	hfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("heartbeat item not found: %s", name)
	}

	var data []byte
	switch file {
	case "keepalive":
		data = []byte("")
	case "ctl":
		item.mu.RLock()
		now := time.Now()
		status := "alive"
		if now.After(item.expireTime) {
			status = "expired"
		}
		data = []byte(fmt.Sprintf("last_heartbeat_ts: %s\nexpire_ts: %s\ntimeout: %d\nstatus: %s\n",
			item.lastHeartbeat.Format(time.RFC3339),
			item.expireTime.Format(time.RFC3339),
			int(item.timeout.Seconds()),
			status))
		item.mu.RUnlock()
	default:
		return nil, fmt.Errorf("invalid file: %s", file)
	}

	return plugin.ApplyRangeRead(data, offset, size)
}

func (hfs *heartbeatFS) Write(path string, data []byte) ([]byte, error) {
	if path == "/" {
		return nil, fmt.Errorf("cannot write to directory")
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	name := parts[0]
	file := parts[1]

	hfs.plugin.mu.RLock()
	item, exists := hfs.plugin.items[name]
	hfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("heartbeat item not found: %s", name)
	}

	now := time.Now()

	switch file {
	case "keepalive":
		// Update heartbeat timestamp
		item.mu.Lock()
		item.lastHeartbeat = now
		newExpireTime := now.Add(item.timeout)
		item.expireTime = newExpireTime
		heapItem := item.heapItem
		item.mu.Unlock()

		// Update heap
		hfs.plugin.heapMu.Lock()
		if heapItem != nil && heapItem.index >= 0 {
			heapItem.expireTime = newExpireTime
			heap.Fix(&hfs.plugin.expiryHeap, heapItem.index)
		}
		hfs.plugin.heapMu.Unlock()

	case "ctl":
		// Parse timeout=N from data
		content := strings.TrimSpace(string(data))
		var newTimeout int
		_, err := fmt.Sscanf(content, "timeout=%d", &newTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid ctl command, use 'timeout=N' (seconds)")
		}
		if newTimeout <= 0 {
			return nil, fmt.Errorf("timeout must be positive")
		}

		// Update timeout and recalculate expire time
		item.mu.Lock()
		item.timeout = time.Duration(newTimeout) * time.Second
		// Recalculate expire time based on last heartbeat and new timeout
		newExpireTime := item.lastHeartbeat.Add(item.timeout)
		item.expireTime = newExpireTime
		heapItem := item.heapItem
		item.mu.Unlock()

		// Update heap
		hfs.plugin.heapMu.Lock()
		if heapItem != nil && heapItem.index >= 0 {
			heapItem.expireTime = newExpireTime
			heap.Fix(&hfs.plugin.expiryHeap, heapItem.index)
		}
		hfs.plugin.heapMu.Unlock()

	default:
		return nil, fmt.Errorf("can only write to keepalive or ctl files")
	}

	return nil, nil
}

func (hfs *heartbeatFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path == "/" {
		hfs.plugin.mu.RLock()
		defer hfs.plugin.mu.RUnlock()

		files := make([]filesystem.FileInfo, 0, len(hfs.plugin.items)+1)

		// Add README
		readme := hfs.plugin.GetReadme()
		files = append(files, filesystem.FileInfo{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444,
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "doc",
			},
		})

		// Add each heartbeat item
		for name := range hfs.plugin.items {
			files = append(files, filesystem.FileInfo{
				Name:    name,
				Size:    0,
				Mode:    0755,
				ModTime: time.Now(),
				IsDir:   true,
				Meta: filesystem.MetaData{
					Name: PluginName,
					Type: "heartbeat_dir",
				},
			})
		}

		return files, nil
	}

	// List files in heartbeat item directory
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 1 {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	name := parts[0]
	hfs.plugin.mu.RLock()
	item, exists := hfs.plugin.items[name]
	hfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("heartbeat item not found: %s", name)
	}

	item.mu.RLock()
	defer item.mu.RUnlock()

	return []filesystem.FileInfo{
		{
			Name:    "keepalive",
			Size:    0,
			Mode:    0644,
			ModTime: item.lastHeartbeat,
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "keepalive",
			},
		},
		{
			Name:    "ctl",
			Size:    0,
			Mode:    0644,
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "control",
			},
		},
	}, nil
}

func (hfs *heartbeatFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "root",
			},
		}, nil
	}

	if path == "/README" {
		readme := hfs.plugin.GetReadme()
		return &filesystem.FileInfo{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444,
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "doc",
			},
		}, nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	name := parts[0]

	hfs.plugin.mu.RLock()
	item, exists := hfs.plugin.items[name]
	hfs.plugin.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("heartbeat item not found: %s", name)
	}

	// If requesting the directory itself
	if len(parts) == 1 {
		return &filesystem.FileInfo{
			Name:    name,
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "heartbeat_dir",
			},
		}, nil
	}

	// If requesting a file in the directory
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	file := parts[1]
	item.mu.RLock()
	defer item.mu.RUnlock()

	switch file {
	case "keepalive":
		return &filesystem.FileInfo{
			Name:    "keepalive",
			Size:    0,
			Mode:    0644,
			ModTime: item.lastHeartbeat,
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "keepalive",
			},
		}, nil
	case "ctl":
		return &filesystem.FileInfo{
			Name:    "ctl",
			Size:    0,
			Mode:    0644,
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "control",
			},
		}, nil
	default:
		return nil, fmt.Errorf("file not found: %s", file)
	}
}

func (hfs *heartbeatFS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("rename not supported in heartbeatfs")
}

func (hfs *heartbeatFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("chmod not supported in heartbeatfs")
}

func (hfs *heartbeatFS) Open(path string) (io.ReadCloser, error) {
	data, err := hfs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (hfs *heartbeatFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &heartbeatWriter{hfs: hfs, path: path, buf: &bytes.Buffer{}}, nil
}

type heartbeatWriter struct {
	hfs  *heartbeatFS
	path string
	buf  *bytes.Buffer
}

func (hw *heartbeatWriter) Write(p []byte) (n int, err error) {
	return hw.buf.Write(p)
}

func (hw *heartbeatWriter) Close() error {
	_, err := hw.hfs.Write(hw.path, hw.buf.Bytes())
	return err
}

// Touch implements filesystem.Toucher interface
// Efficiently updates timestamp by directly updating heartbeat item
func (hfs *heartbeatFS) Touch(path string) error {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid path for touch: %s", path)
	}

	name := parts[0]
	file := parts[1]

	// Only support touching keepalive file
	if file != "keepalive" {
		return fmt.Errorf("can only touch keepalive file")
	}

	hfs.plugin.mu.RLock()
	item, exists := hfs.plugin.items[name]
	hfs.plugin.mu.RUnlock()

	if !exists {
		return fmt.Errorf("heartbeat item not found: %s", name)
	}

	// Update heartbeat timestamp efficiently (no content read/write)
	now := time.Now()
	item.mu.Lock()
	item.lastHeartbeat = now
	newExpireTime := now.Add(item.timeout)
	item.expireTime = newExpireTime
	heapItem := item.heapItem
	item.mu.Unlock()

	// Update heap
	hfs.plugin.heapMu.Lock()
	if heapItem != nil && heapItem.index >= 0 {
		heapItem.expireTime = newExpireTime
		heap.Fix(&hfs.plugin.expiryHeap, heapItem.index)
	}
	hfs.plugin.heapMu.Unlock()

	return nil
}
