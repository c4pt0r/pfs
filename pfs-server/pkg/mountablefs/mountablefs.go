package mountablefs

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs-server/pkg/plugin"
	log "github.com/sirupsen/logrus"
)

// Meta values for MountableFS
const (
	MetaValueRoot       = "root"
	MetaValueMountPoint = "mount-point"
)

// MountPoint represents a mounted service plugin
type MountPoint struct {
	Path   string
	Plugin plugin.ServicePlugin
	Config map[string]interface{} // Plugin configuration
}

// PluginFactory is a function that creates a new plugin instance
type PluginFactory func() plugin.ServicePlugin

// MountableFS is a FileSystem that supports mounting service plugins at specific paths
type MountableFS struct {
	mounts          map[string]*MountPoint
	mountPaths      []string // sorted by length (longest first) for prefix matching
	pluginFactories map[string]PluginFactory
	mu              sync.RWMutex
}

// NewMountableFS creates a new mountable file system
func NewMountableFS() *MountableFS {
	return &MountableFS{
		mounts:          make(map[string]*MountPoint),
		mountPaths:      []string{},
		pluginFactories: make(map[string]PluginFactory),
	}
}

// RegisterPluginFactory registers a plugin factory for dynamic mounting
func (mfs *MountableFS) RegisterPluginFactory(name string, factory PluginFactory) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()
	mfs.pluginFactories[name] = factory
}

// Mount mounts a service plugin at the specified path
func (mfs *MountableFS) Mount(path string, plugin plugin.ServicePlugin) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	// Normalize path
	path = normalizePath(path)

	// Check if path is already mounted
	if _, exists := mfs.mounts[path]; exists {
		return fmt.Errorf("path already has a mount: %s", path)
	}

	// Add mount (no config for static mounts)
	mfs.mounts[path] = &MountPoint{
		Path:   path,
		Plugin: plugin,
		Config: make(map[string]interface{}),
	}

	// Update mount paths list and sort by length (longest first)
	mfs.mountPaths = append(mfs.mountPaths, path)
	mfs.sortMountPaths()

	return nil
}

// MountPlugin dynamically mounts a plugin at the specified path
func (mfs *MountableFS) MountPlugin(fstype string, path string, config map[string]interface{}) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	// Normalize path
	path = normalizePath(path)

	// Check if path is already mounted
	if _, exists := mfs.mounts[path]; exists {
		return fmt.Errorf("path already has a mount: %s", path)
	}

	// Get plugin factory
	factory, ok := mfs.pluginFactories[fstype]
	if !ok {
		return fmt.Errorf("unknown filesystem type: %s", fstype)
	}

	// Create plugin instance
	pluginInstance := factory()

	// Initialize plugin with config
	if err := pluginInstance.Initialize(config); err != nil {
		return fmt.Errorf("failed to initialize plugin: %v", err)
	}

	// Add mount
	mfs.mounts[path] = &MountPoint{
		Path:   path,
		Plugin: pluginInstance,
		Config: config,
	}

	// Update mount paths list and sort by length (longest first)
	mfs.mountPaths = append(mfs.mountPaths, path)
	mfs.sortMountPaths()

	log.Infof("mounted %s at %s", fstype, path)
	return nil
}

// Unmount unmounts a plugin from the specified path
func (mfs *MountableFS) Unmount(path string) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	path = normalizePath(path)

	mount, exists := mfs.mounts[path]
	if !exists {
		return fmt.Errorf("no mount at path: %s", path)
	}

	// Shutdown the plugin
	if err := mount.Plugin.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown plugin: %v", err)
	}

	delete(mfs.mounts, path)

	// Remove from mount paths
	for i, p := range mfs.mountPaths {
		if p == path {
			mfs.mountPaths = append(mfs.mountPaths[:i], mfs.mountPaths[i+1:]...)
			break
		}
	}

	log.Infof("Unmounted plugin at %s", path)
	return nil
}

// GetMounts returns all mount points
func (mfs *MountableFS) GetMounts() []*MountPoint {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	mounts := make([]*MountPoint, 0, len(mfs.mounts))
	for _, mount := range mfs.mounts {
		mounts = append(mounts, mount)
	}
	return mounts
}

// findMount finds the mount point for a given path
// Returns the mount and the relative path within the mount
func (mfs *MountableFS) findMount(path string) (*MountPoint, string, bool) {
	path = normalizePath(path)

	// Check each mount path (longest first)
	for _, mountPath := range mfs.mountPaths {
		if path == mountPath {
			// Exact match - path is the mount point itself
			return mfs.mounts[mountPath], "/", true
		}
		if strings.HasPrefix(path, mountPath+"/") {
			// Path is under this mount
			relPath := strings.TrimPrefix(path, mountPath)
			return mfs.mounts[mountPath], relPath, true
		}
	}

	return nil, "", false
}

// sortMountPaths sorts mount paths by length (longest first) for correct prefix matching
func (mfs *MountableFS) sortMountPaths() {
	// Simple bubble sort since we don't expect many mounts
	n := len(mfs.mountPaths)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if len(mfs.mountPaths[j]) < len(mfs.mountPaths[j+1]) {
				mfs.mountPaths[j], mfs.mountPaths[j+1] = mfs.mountPaths[j+1], mfs.mountPaths[j]
			}
		}
	}
}

// normalizePath normalizes a path
func normalizePath(path string) string {
	if path == "" || path == "/" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Remove trailing slash
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}

// Delegate all FileSystem methods to either base FS or mounted plugin

func (mfs *MountableFS) Create(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Create(relPath)
	}
	return fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Mkdir(path string, perm uint32) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Mkdir(relPath, perm)
	}
	return fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Remove(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Remove(relPath)
	}
	return fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) RemoveAll(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().RemoveAll(relPath)
	}
	return fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Read(path string, offset int64, size int64) ([]byte, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Read(relPath, offset, size)
	}
	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Write(path string, data []byte) ([]byte, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Write(relPath, data)
	}
	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	// Normalize path
	path = normalizePath(path)

	// If listing root, show all top-level mount point directories
	if path == "/" {
		var infos []filesystem.FileInfo
		seenDirs := make(map[string]bool)

		for mountPath := range mfs.mounts {
			// Extract the first level directory name
			name := mountPath[1:] // Remove leading slash
			if name == "" {
				continue
			}
			// Get first component
			firstSlash := strings.Index(name, "/")
			if firstSlash > 0 {
				name = name[:firstSlash]
			}

			// Add if not already seen
			if !seenDirs[name] {
				seenDirs[name] = true
				infos = append(infos, filesystem.FileInfo{
					Name:    name,
					Size:    0,
					Mode:    0755,
					ModTime: time.Now(),
					IsDir:   true,
					Meta: map[string]string{
						filesystem.MetaKeyPluginName: "rootfs",
						filesystem.MetaKeyType:       MetaValueMountPoint,
					},
				})
			}
		}
		return infos, nil
	}

	// Check if path is a mount point or within a mount
	mount, relPath, found := mfs.findMount(path)
	if found {
		return mount.Plugin.GetFileSystem().ReadDir(relPath)
	}

	// Check if path is a parent directory of mount points
	// List all subdirectories/mounts under this path
	pathPrefix := path + "/"
	var infos []filesystem.FileInfo
	seenDirs := make(map[string]bool)

	for mountPath := range mfs.mounts {
		if strings.HasPrefix(mountPath, pathPrefix) {
			// Extract the next level directory/mount name
			remainder := strings.TrimPrefix(mountPath, pathPrefix)

			// Get the first component of the remainder
			var name string
			slashIdx := strings.Index(remainder, "/")
			if slashIdx > 0 {
				name = remainder[:slashIdx]
			} else {
				name = remainder
			}

			// Add if not already seen
			if !seenDirs[name] {
				seenDirs[name] = true
				infos = append(infos, filesystem.FileInfo{
					Name:    name,
					Size:    0,
					Mode:    0755,
					ModTime: time.Now(),
					IsDir:   true,
					Meta:    map[string]string{filesystem.MetaKeyType: MetaValueMountPoint},
				})
			}
		}
	}

	if len(infos) > 0 {
		return infos, nil
	}

	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Stat(path string) (*filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	path = normalizePath(path)

	// Check if path is root
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta:    map[string]string{filesystem.MetaKeyType: MetaValueRoot},
		}, nil
	}

	// Check if path is a mount point or within a mount
	mount, relPath, found := mfs.findMount(path)
	if found {
		stat, err := mount.Plugin.GetFileSystem().Stat(relPath)
		if err != nil {
			return nil, err
		}

		// If querying the mount point itself (not a file within it),
		// fix the name to show the mount point name instead of "/"
		if path == mount.Path && stat.Name == "/" {
			// Extract the last component of the mount path
			name := path[1:] // Remove leading slash
			if lastSlash := strings.LastIndex(name, "/"); lastSlash >= 0 {
				name = name[lastSlash+1:]
			}
			if name == "" {
				name = "/"
			}
			stat.Name = name
		}

		return stat, nil
	}

	// Check if path is a parent directory of any mount points
	// For example, /mnt when mounts exist at /mnt/queue and /mnt/kv
	pathPrefix := path + "/"
	for mountPath := range mfs.mounts {
		if strings.HasPrefix(mountPath, pathPrefix) {
			// This path is a parent directory of a mount point
			name := path[1:] // Remove leading slash
			if name == "" {
				name = "/"
			} else {
				// Get the last component of the path
				lastSlash := strings.LastIndex(name, "/")
				if lastSlash >= 0 {
					name = name[lastSlash+1:]
				}
			}
			return &filesystem.FileInfo{
				Name:    name,
				Size:    0,
				Mode:    0755,
				ModTime: time.Now(),
				IsDir:   true,
				Meta:    map[string]string{filesystem.MetaKeyType: MetaValueMountPoint},
			}, nil
		}
	}

	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Rename(oldPath, newPath string) error {
	mfs.mu.RLock()
	oldMount, oldRelPath, oldFound := mfs.findMount(oldPath)
	newMount, newRelPath, newFound := mfs.findMount(newPath)
	mfs.mu.RUnlock()

	// Both paths must be in the same filesystem
	if oldFound && newFound {
		if oldMount != newMount {
			return fmt.Errorf("cannot rename across different mounts")
		}
		return oldMount.Plugin.GetFileSystem().Rename(oldRelPath, newRelPath)
	}

	return fmt.Errorf("cannot rename: paths not in same mounted filesystem")
}

func (mfs *MountableFS) Chmod(path string, mode uint32) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Chmod(relPath, mode)
	}
	return fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) Open(path string) (io.ReadCloser, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Open(relPath)
	}
	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

func (mfs *MountableFS) OpenWrite(path string) (io.WriteCloser, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().OpenWrite(relPath)
	}
	return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
}

// OpenStream implements filesystem.Streamer interface
func (mfs *MountableFS) OpenStream(path string) (filesystem.StreamReader, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
	}

	// Check if the filesystem supports Streamer interface
	fs := mount.Plugin.GetFileSystem()
	if streamer, ok := fs.(filesystem.Streamer); ok {
		log.Debugf("[mountablefs] OpenStream: found streamer for path %s (relPath: %s, fs type: %T)", path, relPath, fs)
		return streamer.OpenStream(relPath)
	}

	log.Warnf("[mountablefs] OpenStream: filesystem does not support streaming: %s (fs type: %T)", path, fs)
	return nil, fmt.Errorf("filesystem does not support streaming: %s", path)
}

// GetStream tries to get a stream from the underlying filesystem if it supports streaming
// Deprecated: Use OpenStream instead
func (mfs *MountableFS) GetStream(path string) (interface{}, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("no filesystem mounted at path: %s", path)
	}

	// Check if the filesystem supports GetStream method (for backward compatibility)
	type streamGetter interface {
		GetStream(path string) (interface{}, error)
	}

	fs := mount.Plugin.GetFileSystem()
	if sg, ok := fs.(streamGetter); ok {
		log.Debugf("[mountablefs] GetStream: found stream getter for path %s (relPath: %s, fs type: %T)", path, relPath, fs)
		return sg.GetStream(relPath)
	}

	log.Warnf("[mountablefs] GetStream: filesystem does not support streaming: %s (fs type: %T)", path, fs)
	return nil, fmt.Errorf("filesystem does not support streaming: %s", path)
}
