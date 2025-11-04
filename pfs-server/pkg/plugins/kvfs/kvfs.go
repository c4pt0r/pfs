package kvfs

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
)

const (
	PluginName = "kvfs" // Name of this plugin
)

// Meta values for KVFS plugin
const (
	MetaValueDir  = "dir"  // KV store directory
	MetaValueFile = "file" // KV store data file
)

// KVFSPlugin provides a key-value store service through a file system interface
// Each key is represented as a file, and the file content is the value
// Operations:
//
//	GET /keys/<key>    - Read value
//	PUT /keys/<key>    - Write value
//	DELETE /keys/<key> - Delete key
//	GET /keys          - List all keys
type KVFSPlugin struct {
	store    map[string][]byte
	mu       sync.RWMutex
	metadata plugin.PluginMetadata
}

// NewKVFSPlugin creates a new key-value store plugin
func NewKVFSPlugin() *KVFSPlugin {
	return &KVFSPlugin{
		store: make(map[string][]byte),
		metadata: plugin.PluginMetadata{
			Name:        PluginName,
			Version:     "1.0.0",
			Description: "Key-Value store service plugin",
			Author:      "VFS Server",
		},
	}
}

func (kv *KVFSPlugin) Name() string {
	return kv.metadata.Name
}

func (kv *KVFSPlugin) Validate(cfg map[string]interface{}) error {
	// Check for unknown parameters
	allowedKeys := []string{"initial_data", "mount_path"}
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

	// Validate initial_data if provided
	if val, exists := cfg["initial_data"]; exists {
		// Check if it's a map[string]interface{} or map[string]string
		switch val.(type) {
		case map[string]interface{}, map[string]string:
			// Valid types
		default:
			return fmt.Errorf("initial_data must be a map/object")
		}
	}
	return nil
}

func (kv *KVFSPlugin) Initialize(config map[string]interface{}) error {
	// Load initial data if provided
	if data, ok := config["initial_data"].(map[string]string); ok {
		for k, v := range data {
			kv.store[k] = []byte(v)
		}
	}
	return nil
}

func (kv *KVFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &kvFS{plugin: kv}
}

func (kv *KVFSPlugin) GetReadme() string {
	return `KVFS Plugin - Key-Value Store Service

This plugin provides a key-value store service through a file system interface.

USAGE:
  Set a key-value pair:
    echo "value" > /keys/<key>

  Get a value:
    cat /keys/<key>

  List all keys:
    ls /keys

  Delete a key:
    rm /keys/<key>

  Rename a key:
    mv /keys/<oldkey> /keys/<newkey>

STRUCTURE:
  /keys/     - Directory containing all key-value pairs
  /README    - This file

EXAMPLES:
  # Set a value
  pfs:/> echo "hello world" > /kvfs/keys/mykey

  # Get a value
  pfs:/> cat /kvfs/keys/mykey
  hello world

  # List all keys
  pfs:/> ls /kvfs/keys

  # Delete a key
  pfs:/> rm /kvfs/keys/mykey

  # Rename a key
  pfs:/> mv /kvfs/keys/oldname /kvfs/keys/newname
`
}

func (kv *KVFSPlugin) Shutdown() error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store = nil
	return nil
}

// kvFS implements the FileSystem interface for key-value operations
type kvFS struct {
	plugin *KVFSPlugin
}

func (kvfs *kvFS) Create(path string) error {
	if path == "/" || path == "/keys" {
		return fmt.Errorf("cannot create: %s", path)
	}

	// Only allow creating files under /keys/
	if !strings.HasPrefix(path, "/keys/") {
		return fmt.Errorf("keys must be under /keys/ directory")
	}

	key := strings.TrimPrefix(path, "/keys/")
	if key == "" {
		return fmt.Errorf("key name cannot be empty")
	}

	kvfs.plugin.mu.Lock()
	defer kvfs.plugin.mu.Unlock()

	if _, exists := kvfs.plugin.store[key]; exists {
		return fmt.Errorf("key already exists: %s", key)
	}

	kvfs.plugin.store[key] = []byte{}
	return nil
}

func (kvfs *kvFS) Mkdir(path string, perm uint32) error {
	if path == "/keys" {
		return nil // /keys directory always exists
	}
	return fmt.Errorf("cannot create directories in kvfs service")
}

func (kvfs *kvFS) Remove(path string) error {
	if !strings.HasPrefix(path, "/keys/") {
		return fmt.Errorf("can only remove keys under /keys/")
	}

	key := strings.TrimPrefix(path, "/keys/")
	if key == "" {
		return fmt.Errorf("key name cannot be empty")
	}

	kvfs.plugin.mu.Lock()
	defer kvfs.plugin.mu.Unlock()

	if _, exists := kvfs.plugin.store[key]; !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	delete(kvfs.plugin.store, key)
	return nil
}

func (kvfs *kvFS) RemoveAll(path string) error {
	if path == "/keys" {
		// Clear all keys
		kvfs.plugin.mu.Lock()
		defer kvfs.plugin.mu.Unlock()
		kvfs.plugin.store = make(map[string][]byte)
		return nil
	}
	return kvfs.Remove(path)
}

func (kvfs *kvFS) Read(path string, offset int64, size int64) ([]byte, error) {
	if path == "/" || path == "/keys" {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	var data []byte
	if path == "/README" {
		data = []byte(kvfs.plugin.GetReadme())
	} else if strings.HasPrefix(path, "/keys/") {
		key := strings.TrimPrefix(path, "/keys/")
		if key == "" {
			return nil, fmt.Errorf("key name cannot be empty")
		}

		kvfs.plugin.mu.RLock()
		value, exists := kvfs.plugin.store[key]
		kvfs.plugin.mu.RUnlock()

		if !exists {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		data = value
	} else {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	return plugin.ApplyRangeRead(data, offset, size)
}

func (kvfs *kvFS) Write(path string, data []byte) ([]byte, error) {
	if path == "/" || path == "/keys" {
		return nil, fmt.Errorf("cannot write to directory: %s", path)
	}

	if !strings.HasPrefix(path, "/keys/") {
		return nil, fmt.Errorf("keys must be under /keys/ directory")
	}

	key := strings.TrimPrefix(path, "/keys/")
	if key == "" {
		return nil, fmt.Errorf("key name cannot be empty")
	}

	kvfs.plugin.mu.Lock()
	defer kvfs.plugin.mu.Unlock()

	kvfs.plugin.store[key] = data
	return nil, nil
}

func (kvfs *kvFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path == "/" {
		// Root directory contains /keys and README
		readme := kvfs.plugin.GetReadme()
		return []filesystem.FileInfo{
			{
				Name:    "README",
				Size:    int64(len(readme)),
				Mode:    0444,
				ModTime: time.Now(),
				IsDir:   false,
				Meta: filesystem.MetaData{
					Name: PluginName,
					Type: "doc",
				},
			},
			{
				Name:    "keys",
				Size:    0,
				Mode:    0755,
				ModTime: time.Now(),
				IsDir:   true,
				Meta: filesystem.MetaData{
					Name: PluginName,
					Type: MetaValueDir,
				},
			},
		}, nil
	}

	if path == "/keys" {
		// List all keys
		kvfs.plugin.mu.RLock()
		defer kvfs.plugin.mu.RUnlock()

		files := make([]filesystem.FileInfo, 0, len(kvfs.plugin.store))
		for key, value := range kvfs.plugin.store {
			files = append(files, filesystem.FileInfo{
				Name:    filepath.Base(key),
				Size:    int64(len(value)),
				Mode:    0644,
				ModTime: time.Now(),
				IsDir:   false,
				Meta: filesystem.MetaData{
					Name: PluginName,
					Type: MetaValueFile,
				},
			})
		}

		return files, nil
	}

	return nil, fmt.Errorf("not a directory: %s", path)
}

func (kvfs *kvFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/" || path == "/keys" {
		return &filesystem.FileInfo{
			Name:    filepath.Base(path),
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: MetaValueDir,
			},
		}, nil
	}

	if path == "/README" {
		readme := kvfs.plugin.GetReadme()
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

	if !strings.HasPrefix(path, "/keys/") {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	key := strings.TrimPrefix(path, "/keys/")
	if key == "" {
		return nil, fmt.Errorf("key name cannot be empty")
	}

	kvfs.plugin.mu.RLock()
	defer kvfs.plugin.mu.RUnlock()

	value, exists := kvfs.plugin.store[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return &filesystem.FileInfo{
		Name:    filepath.Base(key),
		Size:    int64(len(value)),
		Mode:    0644,
		ModTime: time.Now(),
		IsDir:   false,
		Meta: filesystem.MetaData{
			Name: PluginName,
			Type: MetaValueFile,
		},
	}, nil
}

func (kvfs *kvFS) Rename(oldPath, newPath string) error {
	if !strings.HasPrefix(oldPath, "/keys/") || !strings.HasPrefix(newPath, "/keys/") {
		return fmt.Errorf("can only rename keys under /keys/")
	}

	oldKey := strings.TrimPrefix(oldPath, "/keys/")
	newKey := strings.TrimPrefix(newPath, "/keys/")

	if oldKey == "" || newKey == "" {
		return fmt.Errorf("key name cannot be empty")
	}

	kvfs.plugin.mu.Lock()
	defer kvfs.plugin.mu.Unlock()

	value, exists := kvfs.plugin.store[oldKey]
	if !exists {
		return fmt.Errorf("key not found: %s", oldKey)
	}

	if _, exists := kvfs.plugin.store[newKey]; exists {
		return fmt.Errorf("key already exists: %s", newKey)
	}

	kvfs.plugin.store[newKey] = value
	delete(kvfs.plugin.store, oldKey)

	return nil
}

func (kvfs *kvFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("cannot change permissions in kvfs service")
}

func (kvfs *kvFS) Open(path string) (io.ReadCloser, error) {
	data, err := kvfs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (kvfs *kvFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &kvWriter{kvfs: kvfs, path: path, buf: &bytes.Buffer{}}, nil
}

type kvWriter struct {
	kvfs *kvFS
	path string
	buf  *bytes.Buffer
}

func (kw *kvWriter) Write(p []byte) (n int, err error) {
	return kw.buf.Write(p)
}

func (kw *kvWriter) Close() error {
	_, err := kw.kvfs.Write(kw.path, kw.buf.Bytes())
	return err
}

