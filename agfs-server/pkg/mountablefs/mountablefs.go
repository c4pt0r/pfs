package mountablefs

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/loader"
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
	mounts             map[string]*MountPoint
	mountPaths         []string // sorted by length (longest first) for prefix matching
	pluginFactories    map[string]PluginFactory
	pluginLoader       *loader.PluginLoader // For loading external plugins
	pluginNameCounters map[string]int       // Track counters for plugin names
	mu                 sync.RWMutex
}

// NewMountableFS creates a new mountable file system
func NewMountableFS() *MountableFS {
	return &MountableFS{
		mounts:             make(map[string]*MountPoint),
		mountPaths:         []string{},
		pluginFactories:    make(map[string]PluginFactory),
		pluginLoader:       loader.NewPluginLoader(),
		pluginNameCounters: make(map[string]int),
	}
}

// GetPluginLoader returns the plugin loader instance
func (mfs *MountableFS) GetPluginLoader() *loader.PluginLoader {
	return mfs.pluginLoader
}

// RenamedPlugin wraps a plugin with a different name
type RenamedPlugin struct {
	plugin.ServicePlugin
	originalName string
	renamedName  string
}

// Name returns the renamed plugin name
func (rp *RenamedPlugin) Name() string {
	return rp.renamedName
}

// OriginalName returns the original plugin name
func (rp *RenamedPlugin) OriginalName() string {
	return rp.originalName
}

// generateUniquePluginName generates a unique plugin name with incremental suffix
// Must be called with mfs.mu held (write lock)
func (mfs *MountableFS) generateUniquePluginName(baseName string) string {
	// Check if base name is available
	if _, exists := mfs.pluginFactories[baseName]; !exists {
		// Base name is available, initialize counter
		mfs.pluginNameCounters[baseName] = 0
		return baseName
	}

	// Base name exists, increment counter and generate new name
	mfs.pluginNameCounters[baseName]++
	counter := mfs.pluginNameCounters[baseName]
	newName := fmt.Sprintf("%s-%d", baseName, counter)

	// Ensure the generated name doesn't conflict (defensive programming)
	for {
		if _, exists := mfs.pluginFactories[newName]; !exists {
			return newName
		}
		mfs.pluginNameCounters[baseName]++
		counter = mfs.pluginNameCounters[baseName]
		newName = fmt.Sprintf("%s-%d", baseName, counter)
	}
}

// RegisterPluginFactory registers a plugin factory for dynamic mounting
func (mfs *MountableFS) RegisterPluginFactory(name string, factory PluginFactory) {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()
	mfs.pluginFactories[name] = factory
}

// CreatePlugin creates a plugin instance from a registered factory
func (mfs *MountableFS) CreatePlugin(name string) plugin.ServicePlugin {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	factory, ok := mfs.pluginFactories[name]
	if !ok {
		return nil
	}
	return factory()
}

// Mount mounts a service plugin at the specified path
func (mfs *MountableFS) Mount(path string, plugin plugin.ServicePlugin) error {
	mfs.mu.Lock()
	defer mfs.mu.Unlock()

	// Normalize path
	path = filesystem.NormalizePath(path)

	// Check if path is already mounted
	if _, exists := mfs.mounts[path]; exists {
		return filesystem.NewAlreadyExistsError("mount", path)
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
	path = filesystem.NormalizePath(path)

	// Check if path is already mounted
	if _, exists := mfs.mounts[path]; exists {
		return filesystem.NewAlreadyExistsError("mount", path)
	}

	// Get plugin factory
	factory, ok := mfs.pluginFactories[fstype]
	if !ok {
		return fmt.Errorf("unknown filesystem type: %s", fstype)
	}

	// Create plugin instance
	pluginInstance := factory()

	// Special handling for plugins that need rootFS reference
	// Check if plugin has SetRootFS method (e.g., httpfs, proxyfs)
	type rootFSSetter interface {
		SetRootFS(filesystem.FileSystem)
	}
	if setter, ok := pluginInstance.(rootFSSetter); ok {
		setter.SetRootFS(mfs)
		log.Debugf("Set rootFS for plugin %s at %s", fstype, path)
	}

	// Inject mount_path into config for plugins that need to know their virtual path
	configWithPath := make(map[string]interface{})
	for k, v := range config {
		configWithPath[k] = v
	}
	configWithPath["mount_path"] = path

	// Validate plugin configuration
	if err := pluginInstance.Validate(configWithPath); err != nil {
		return fmt.Errorf("failed to validate plugin: %v", err)
	}

	// Initialize plugin with config
	if err := pluginInstance.Initialize(configWithPath); err != nil {
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

	path = filesystem.NormalizePath(path)

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

// LoadExternalPluginWithType loads a plugin with an explicitly specified type
func (mfs *MountableFS) LoadExternalPluginWithType(libraryPath string, pluginType loader.PluginType) (plugin.ServicePlugin, error) {
	// For WASM plugins, pass MountableFS as host filesystem to allow access to all agfs paths
	var p plugin.ServicePlugin
	var err error
	if pluginType == loader.PluginTypeWASM {
		log.Infof("Loading WASM plugin with host filesystem access to all agfs paths")
		p, err = mfs.pluginLoader.LoadPluginWithType(libraryPath, pluginType, mfs)
	} else {
		p, err = mfs.pluginLoader.LoadPluginWithType(libraryPath, pluginType)
	}
	if err != nil {
		return nil, err
	}

	// Register the plugin as a factory so it can be mounted
	pluginName := p.Name()
	mfs.RegisterPluginFactory(pluginName, func() plugin.ServicePlugin {
		// For external plugins, we need to return the already loaded instance
		// since we can't create new instances from the loaded library
		return p
	})

	log.Infof("Registered external plugin factory: %s (type: %s)", pluginName, pluginType)
	return p, nil
}

// LoadExternalPlugin loads a plugin from a shared library file
// The plugin type is automatically detected based on file content
// If a plugin with the same name already exists, automatically appends a numeric suffix
func (mfs *MountableFS) LoadExternalPlugin(libraryPath string) (plugin.ServicePlugin, error) {
	// Detect plugin type first
	pluginType, err := loader.DetectPluginType(libraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect plugin type: %w", err)
	}

	// For WASM plugins, use LoadExternalPluginWithType to pass host filesystem
	if pluginType == loader.PluginTypeWASM {
		return mfs.LoadExternalPluginWithType(libraryPath, pluginType)
	}

	// For other plugin types, use regular loading
	p, err := mfs.pluginLoader.LoadPlugin(libraryPath)
	if err != nil {
		return nil, err
	}

	// Get original plugin name
	originalName := p.Name()

	mfs.mu.Lock()

	// Generate unique name
	finalName := mfs.generateUniquePluginName(originalName)
	renamed := (finalName != originalName)

	if renamed {
		log.Infof("Plugin name '%s' already exists, using '%s' instead", originalName, finalName)
	}

	// Create wrapped plugin if renamed
	var pluginToRegister plugin.ServicePlugin = p
	if renamed {
		pluginToRegister = &RenamedPlugin{
			ServicePlugin: p,
			originalName:  originalName,
			renamedName:   finalName,
		}
	}

	// Register the plugin with final name
	mfs.pluginFactories[finalName] = func() plugin.ServicePlugin {
		// For external plugins, we need to return the already loaded instance
		// since we can't create new instances from the loaded library
		return pluginToRegister
	}

	mfs.mu.Unlock()

	log.Infof("Registered external plugin factory: %s", finalName)

	// Return wrapped plugin if renamed
	if renamed {
		return &RenamedPlugin{
			ServicePlugin: p,
			originalName:  originalName,
			renamedName:   finalName,
		}, nil
	}

	return p, nil
}

// UnloadExternalPluginWithType unloads an external plugin with an explicitly specified type
func (mfs *MountableFS) UnloadExternalPluginWithType(libraryPath string, pluginType loader.PluginType) error {
	return mfs.pluginLoader.UnloadPluginWithType(libraryPath, pluginType)
}

// UnloadExternalPlugin unloads an external plugin
// The plugin type is automatically detected based on file content
func (mfs *MountableFS) UnloadExternalPlugin(libraryPath string) error {
	return mfs.pluginLoader.UnloadPlugin(libraryPath)
}

// GetLoadedExternalPlugins returns a list of loaded external plugin paths
func (mfs *MountableFS) GetLoadedExternalPlugins() []string {
	return mfs.pluginLoader.GetLoadedPlugins()
}

// LoadExternalPluginsFromDirectory loads all plugins from a directory
func (mfs *MountableFS) LoadExternalPluginsFromDirectory(dir string) ([]string, []error) {
	return mfs.pluginLoader.LoadPluginsFromDirectory(dir)
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
	path = filesystem.NormalizePath(path)

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
	sort.Slice(mfs.mountPaths, func(i, j int) bool {
		return len(mfs.mountPaths[i]) > len(mfs.mountPaths[j])
	})
}

// Delegate all FileSystem methods to either base FS or mounted plugin

func (mfs *MountableFS) Create(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Create(relPath)
	}
	return filesystem.NewPermissionDeniedError("create", path, "not allowed to create file in rootfs, use mount instead")
}

func (mfs *MountableFS) Mkdir(path string, perm uint32) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Mkdir(relPath, perm)
	}
	return filesystem.NewPermissionDeniedError("mkdir", path, "not allowed to create directory in rootfs, use mount instead")
}

func (mfs *MountableFS) Remove(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Remove(relPath)
	}
	return filesystem.NewNotFoundError("remove", path)
}

func (mfs *MountableFS) RemoveAll(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().RemoveAll(relPath)
	}
	return filesystem.NewNotFoundError("removeall", path)
}

func (mfs *MountableFS) Read(path string, offset int64, size int64) ([]byte, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Read(relPath, offset, size)
	}
	return nil, filesystem.NewNotFoundError("read", path)
}

func (mfs *MountableFS) Write(path string, data []byte) ([]byte, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Write(relPath, data)
	}
	return nil, filesystem.NewNotFoundError("write", path)
}

func (mfs *MountableFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	// Normalize path
	path = filesystem.NormalizePath(path)

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
					Meta: filesystem.MetaData{
						Name: "rootfs",
						Type: MetaValueMountPoint,
					},
				})
			}
		}
		return infos, nil
	}

	// Check if path is a mount point or within a mount
	mount, relPath, found := mfs.findMount(path)
	if found {
		// Get contents from the mounted filesystem
		infos, err := mount.Plugin.GetFileSystem().ReadDir(relPath)
		if err != nil {
			return nil, err
		}

		// Check if there are any child mounts under this path that should be shown
		// Build the full path we're listing
		fullPath := path
		if relPath != "/" {
			// We're listing a subdirectory within a mount
			fullPath = mount.Path
			if relPath != "/" {
				fullPath = fullPath + relPath
			}
		}
		pathPrefix := fullPath + "/"

		// Find child mount points
		seenDirs := make(map[string]bool)
		for _, info := range infos {
			seenDirs[info.Name] = true
		}

		// Look for mounts that are children of the current path
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
						Meta: filesystem.MetaData{
							Type: MetaValueMountPoint,
						},
					})
				}
			}
		}

		return infos, nil
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
					Meta: filesystem.MetaData{
						Type: MetaValueMountPoint,
					},
				})
			}
		}
	}

	if len(infos) > 0 {
		return infos, nil
	}

	return nil, filesystem.NewNotFoundError("readdir", path)
}

func (mfs *MountableFS) Stat(path string) (*filesystem.FileInfo, error) {
	mfs.mu.RLock()
	defer mfs.mu.RUnlock()

	path = filesystem.NormalizePath(path)

	// Check if path is root
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Type: MetaValueRoot,
			},
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
				Meta: filesystem.MetaData{
					Type: MetaValueMountPoint,
				},
			}, nil
		}
	}

	return nil, filesystem.NewNotFoundError("stat", path)
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
	return filesystem.NewNotFoundError("chmod", path)
}

// Touch implements filesystem.Toucher interface
func (mfs *MountableFS) Touch(path string) error {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		fs := mount.Plugin.GetFileSystem()
		// Check if the underlying filesystem implements Toucher
		if toucher, ok := fs.(filesystem.Toucher); ok {
			return toucher.Touch(relPath)
		}
		// Fallback: inefficient implementation - read and write back
		info, err := fs.Stat(relPath)
		if err == nil {
			// File exists - read current content and write it back
			if !info.IsDir {
				data, readErr := fs.Read(relPath, 0, -1)
				if readErr != nil {
					return readErr
				}
				_, writeErr := fs.Write(relPath, data)
				return writeErr
			}
			return fmt.Errorf("cannot touch directory")
		} else {
			// File doesn't exist - create with empty content
			_, err := fs.Write(relPath, []byte{})
			return err
		}
	}
	return filesystem.NewNotFoundError("touch", path)
}

func (mfs *MountableFS) Open(path string) (io.ReadCloser, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().Open(relPath)
	}
	return nil, filesystem.NewNotFoundError("open", path)
}

func (mfs *MountableFS) OpenWrite(path string) (io.WriteCloser, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if found {
		return mount.Plugin.GetFileSystem().OpenWrite(relPath)
	}
	return nil, filesystem.NewNotFoundError("openwrite", path)
}

// OpenStream implements filesystem.Streamer interface
func (mfs *MountableFS) OpenStream(path string) (filesystem.StreamReader, error) {
	mfs.mu.RLock()
	mount, relPath, found := mfs.findMount(path)
	mfs.mu.RUnlock()

	if !found {
		return nil, filesystem.NewNotFoundError("openstream", path)
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
		return nil, filesystem.NewNotFoundError("getstream", path)
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
