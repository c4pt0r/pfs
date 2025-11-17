package sqlfs

import (
	"container/list"
	"sync"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
)

// CacheEntry represents a cached directory listing
type CacheEntry struct {
	Files   []filesystem.FileInfo
	ModTime time.Time
}

// ListDirCache implements an LRU cache for directory listings
type ListDirCache struct {
	mu        sync.RWMutex
	cache     map[string]*list.Element // path -> list element
	lruList   *list.List               // LRU list of cache entries
	maxSize   int                      // maximum number of entries
	ttl       time.Duration            // time-to-live for cache entries
	enabled   bool                     // whether cache is enabled
	hitCount  uint64                   // cache hit counter
	missCount uint64                   // cache miss counter
}

// cacheItem is the value stored in the LRU list
type cacheItem struct {
	path  string
	entry *CacheEntry
}

// NewListDirCache creates a new directory listing cache
func NewListDirCache(maxSize int, ttl time.Duration, enabled bool) *ListDirCache {
	if maxSize <= 0 {
		maxSize = 1000 // default max size
	}
	if ttl <= 0 {
		ttl = 5 * time.Second // default TTL
	}

	return &ListDirCache{
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
		maxSize: maxSize,
		ttl:     ttl,
		enabled: enabled,
	}
}

// Get retrieves a cached directory listing
func (c *ListDirCache) Get(path string) ([]filesystem.FileInfo, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.cache[path]
	if !ok {
		c.missCount++
		return nil, false
	}

	item := elem.Value.(*cacheItem)

	// Check if entry is expired
	if time.Since(item.entry.ModTime) > c.ttl {
		c.lruList.Remove(elem)
		delete(c.cache, path)
		c.missCount++
		return nil, false
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(elem)
	c.hitCount++

	// Return a copy to prevent external modification
	files := make([]filesystem.FileInfo, len(item.entry.Files))
	copy(files, item.entry.Files)
	return files, true
}

// Put adds a directory listing to the cache
func (c *ListDirCache) Put(path string, files []filesystem.FileInfo) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if elem, ok := c.cache[path]; ok {
		// Update existing entry
		item := elem.Value.(*cacheItem)
		item.entry.Files = files
		item.entry.ModTime = time.Now()
		c.lruList.MoveToFront(elem)
		return
	}

	// Create new entry
	entry := &CacheEntry{
		Files:   files,
		ModTime: time.Now(),
	}

	item := &cacheItem{
		path:  path,
		entry: entry,
	}

	elem := c.lruList.PushFront(item)
	c.cache[path] = elem

	// Evict oldest entry if cache is full
	if c.lruList.Len() > c.maxSize {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			oldestItem := oldest.Value.(*cacheItem)
			delete(c.cache, oldestItem.path)
		}
	}
}

// Invalidate removes a specific path from the cache
func (c *ListDirCache) Invalidate(path string) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[path]; ok {
		c.lruList.Remove(elem)
		delete(c.cache, path)
	}
}

// InvalidatePrefix removes all paths with the given prefix from cache
// This is useful when a directory or its parent is modified
func (c *ListDirCache) InvalidatePrefix(prefix string) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Collect paths to invalidate
	toDelete := make([]string, 0)
	for path := range c.cache {
		if path == prefix || isDescendant(path, prefix) {
			toDelete = append(toDelete, path)
		}
	}

	// Remove from cache
	for _, path := range toDelete {
		if elem, ok := c.cache[path]; ok {
			c.lruList.Remove(elem)
			delete(c.cache, path)
		}
	}
}

// InvalidateParent invalidates the parent directory of a given path
func (c *ListDirCache) InvalidateParent(path string) {
	parent := getParentPath(path)
	c.Invalidate(parent)
}

// Clear removes all entries from the cache
func (c *ListDirCache) Clear() {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*list.Element)
	c.lruList = list.New()
}

// isDescendant checks if path is a descendant of parent
func isDescendant(path, parent string) bool {
	// A path is not a descendant of itself
	if path == parent {
		return false
	}

	// Special case for root: everything is a descendant except root itself
	if parent == "/" {
		return path != "/"
	}

	// Check if path starts with parent + "/"
	if len(path) <= len(parent) {
		return false
	}

	return path[:len(parent)] == parent && path[len(parent)] == '/'
}
