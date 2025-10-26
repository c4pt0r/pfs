package memfs

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs-server/pkg/plugin"
)

// Meta values for MemFS plugin
const (
	MetaValueDir  = "dir"
	MetaValueFile = "file"
)

// Node represents a file or directory in memory
type Node struct {
	Name     string
	IsDir    bool
	Data     []byte
	Mode     uint32
	ModTime  time.Time
	Children map[string]*Node
}

// MemoryFS implements FileSystem interface with in-memory storage
type MemoryFS struct {
	root       *Node
	mu         sync.RWMutex
	pluginName string
}

// NewMemoryFS creates a new in-memory file system
func NewMemoryFS() *MemoryFS {
	return NewMemoryFSWithPlugin("")
}

// NewMemoryFSWithPlugin creates a new in-memory file system with a plugin name
func NewMemoryFSWithPlugin(pluginName string) *MemoryFS {
	return &MemoryFS{
		root: &Node{
			Name:     "/",
			IsDir:    true,
			Mode:     0755,
			ModTime:  time.Now(),
			Children: make(map[string]*Node),
		},
		pluginName: pluginName,
	}
}

// normalizePath normalizes the path
func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return filepath.Clean(path)
}

// getNode retrieves a node from the tree
func (mfs *MemoryFS) getNode(path string) (*Node, error) {
	path = normalizePath(path)

	if path == "/" {
		return mfs.root, nil
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	current := mfs.root

	for _, part := range parts {
		if !current.IsDir {
			return nil, fmt.Errorf("not a directory: %s", path)
		}
		next, exists := current.Children[part]
		if !exists {
			return nil, fmt.Errorf("no such file or directory: %s", path)
		}
		current = next
	}

	return current, nil
}

// getParentNode retrieves the parent node and the basename
func (mfs *MemoryFS) getParentNode(path string) (*Node, string, error) {
	path = normalizePath(path)

	if path == "/" {
		return nil, "", fmt.Errorf("cannot get parent of root")
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)

	parent, err := mfs.getNode(dir)
	if err != nil {
		return nil, "", err
	}

	if !parent.IsDir {
		return nil, "", fmt.Errorf("parent is not a directory")
	}

	return parent, base, nil
}

// Create creates a new file
func (mfs *MemoryFS) Create(path string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	parent, name, err := mfs.getParentNode(path)
	if err != nil {
		return err
	}

	if _, exists := parent.Children[name]; exists {
		return fmt.Errorf("file already exists: %s", path)
	}

	parent.Children[name] = &Node{
		Name:     name,
		IsDir:    false,
		Data:     []byte{},
		Mode:     0644,
		ModTime:  time.Now(),
		Children: nil,
	}

	return nil
}

// Mkdir creates a new directory
func (mfs *MemoryFS) Mkdir(path string, perm uint32) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	parent, name, err := mfs.getParentNode(path)
	if err != nil {
		return err
	}

	if _, exists := parent.Children[name]; exists {
		return fmt.Errorf("directory already exists: %s", path)
	}

	parent.Children[name] = &Node{
		Name:     name,
		IsDir:    true,
		Mode:     perm,
		ModTime:  time.Now(),
		Children: make(map[string]*Node),
	}

	return nil
}

// Remove removes a file or empty directory
func (mfs *MemoryFS) Remove(path string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	if normalizePath(path) == "/" {
		return fmt.Errorf("cannot remove root directory")
	}

	parent, name, err := mfs.getParentNode(path)
	if err != nil {
		return err
	}

	node, exists := parent.Children[name]
	if !exists {
		return fmt.Errorf("no such file or directory: %s", path)
	}

	if node.IsDir && len(node.Children) > 0 {
		return fmt.Errorf("directory not empty: %s", path)
	}

	delete(parent.Children, name)
	return nil
}

// RemoveAll removes a path and any children it contains
func (mfs *MemoryFS) RemoveAll(path string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	if normalizePath(path) == "/" {
		return fmt.Errorf("cannot remove root directory")
	}

	parent, name, err := mfs.getParentNode(path)
	if err != nil {
		return err
	}

	if _, exists := parent.Children[name]; !exists {
		return fmt.Errorf("no such file or directory: %s", path)
	}

	delete(parent.Children, name)
	return nil
}

// Read reads file content with optional offset and size
func (mfs *MemoryFS) Read(path string, offset int64, size int64) ([]byte, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	node, err := mfs.getNode(path)
	if err != nil {
		return nil, err
	}

	if node.IsDir {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	return plugin.ApplyRangeRead(node.Data, offset, size)
}

// Write writes data to a file, creating it if necessary
func (mfs *MemoryFS) Write(path string, data []byte) ([]byte, error) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	parent, name, err := mfs.getParentNode(path)
	if err != nil {
		return nil, err
	}

	node, exists := parent.Children[name]
	if !exists {
		// Create the file
		node = &Node{
			Name:     name,
			IsDir:    false,
			Data:     data,
			Mode:     0644,
			ModTime:  time.Now(),
			Children: nil,
		}
		parent.Children[name] = node
	} else {
		if node.IsDir {
			return nil, fmt.Errorf("is a directory: %s", path)
		}
		node.Data = data
		node.ModTime = time.Now()
	}

	return nil, nil
}

// ReadDir lists the contents of a directory
func (mfs *MemoryFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	node, err := mfs.getNode(path)
	if err != nil {
		return nil, err
	}

	if !node.IsDir {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	var infos []filesystem.FileInfo
	for _, child := range node.Children {
		meta := make(map[string]string)
		if mfs.pluginName != "" {
			meta[filesystem.MetaKeyPluginName] = mfs.pluginName
		}
		if child.IsDir {
			meta[filesystem.MetaKeyType] = MetaValueDir
		} else {
			meta[filesystem.MetaKeyType] = MetaValueFile
		}

		infos = append(infos, filesystem.FileInfo{
			Name:    child.Name,
			Size:    int64(len(child.Data)),
			Mode:    child.Mode,
			ModTime: child.ModTime,
			IsDir:   child.IsDir,
			Meta:    meta,
		})
	}

	return infos, nil
}

// Stat returns file information
func (mfs *MemoryFS) Stat(path string) (*filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	node, err := mfs.getNode(path)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]string)
	if mfs.pluginName != "" {
		meta[filesystem.MetaKeyPluginName] = mfs.pluginName
	}
	if node.IsDir {
		meta[filesystem.MetaKeyType] = MetaValueDir
	} else {
		meta[filesystem.MetaKeyType] = MetaValueFile
	}

	return &filesystem.FileInfo{
		Name:    node.Name,
		Size:    int64(len(node.Data)),
		Mode:    node.Mode,
		ModTime: node.ModTime,
		IsDir:   node.IsDir,
		Meta:    meta,
	}, nil
}

// Rename renames/moves a file or directory
func (mfs *MemoryFS) Rename(oldPath, newPath string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	oldParent, oldName, err := mfs.getParentNode(oldPath)
	if err != nil {
		return err
	}

	node, exists := oldParent.Children[oldName]
	if !exists {
		return fmt.Errorf("no such file or directory: %s", oldPath)
	}

	newParent, newName, err := mfs.getParentNode(newPath)
	if err != nil {
		return err
	}

	if _, exists := newParent.Children[newName]; exists {
		return fmt.Errorf("file already exists: %s", newPath)
	}

	// Move the node
	delete(oldParent.Children, oldName)
	node.Name = newName
	newParent.Children[newName] = node

	return nil
}

// Chmod changes file permissions
func (mfs *MemoryFS) Chmod(path string, mode uint32) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	node, err := mfs.getNode(path)
	if err != nil {
		return err
	}

	node.Mode = mode
	return nil
}

// memoryReadCloser wraps a bytes.Reader to implement io.ReadCloser
type memoryReadCloser struct {
	*bytes.Reader
}

func (m *memoryReadCloser) Close() error {
	return nil
}

// Open opens a file for reading
func (mfs *MemoryFS) Open(path string) (io.ReadCloser, error) {
	data, err := mfs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return &memoryReadCloser{bytes.NewReader(data)}, nil
}

// memoryWriteCloser implements io.WriteCloser for in-memory files
type memoryWriteCloser struct {
	buffer *bytes.Buffer
	mfs    *MemoryFS
	path   string
}

func (m *memoryWriteCloser) Write(p []byte) (n int, err error) {
	return m.buffer.Write(p)
}

func (m *memoryWriteCloser) Close() error {
	_, err := m.mfs.Write(m.path, m.buffer.Bytes())
	return err
}

// OpenWrite opens a file for writing
func (mfs *MemoryFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &memoryWriteCloser{
		buffer: &bytes.Buffer{},
		mfs:    mfs,
		path:   path,
	}, nil
}

