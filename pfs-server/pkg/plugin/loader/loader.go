package loader

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin/api"
	"github.com/ebitengine/purego"
	log "github.com/sirupsen/logrus"
)

// LoadedPlugin tracks a loaded external plugin
type LoadedPlugin struct {
	Path       string
	Plugin     plugin.ServicePlugin
	LibHandle  uintptr
	RefCount   int
	mu         sync.Mutex
}

// PluginLoader manages loading and unloading of external plugins
type PluginLoader struct {
	loadedPlugins map[string]*LoadedPlugin
	mu            sync.RWMutex
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader() *PluginLoader {
	return &PluginLoader{
		loadedPlugins: make(map[string]*LoadedPlugin),
	}
}

// LoadPlugin loads a plugin from a shared library file (.so, .dylib, .dll)
func (pl *PluginLoader) LoadPlugin(libraryPath string) (plugin.ServicePlugin, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Check if already loaded
	absPath, err := filepath.Abs(libraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	if loaded, exists := pl.loadedPlugins[absPath]; exists {
		loaded.mu.Lock()
		loaded.RefCount++
		loaded.mu.Unlock()
		log.Infof("Plugin already loaded, incremented ref count: %s", absPath)
		return loaded.Plugin, nil
	}

	// Determine dlopen flags based on platform
	flags := getDlopenFlags()

	// Open the shared library
	libHandle, err := purego.Dlopen(libraryPath, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to open library %s: %w", libraryPath, err)
	}

	log.Infof("Loaded library: %s (handle: %v)", libraryPath, libHandle)

	// Load the plugin functions
	vtable, err := loadPluginVTable(libHandle)
	if err != nil {
		// TODO: Add Dlclose if purego supports it
		return nil, fmt.Errorf("failed to load plugin vtable: %w", err)
	}

	// Create external plugin wrapper
	externalPlugin, err := api.NewExternalPlugin(libHandle, vtable)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin wrapper: %w", err)
	}

	// Track loaded plugin
	loaded := &LoadedPlugin{
		Path:      absPath,
		Plugin:    externalPlugin,
		LibHandle: libHandle,
		RefCount:  1,
	}
	pl.loadedPlugins[absPath] = loaded

	log.Infof("Successfully loaded plugin: %s (name: %s)", absPath, externalPlugin.Name())
	return externalPlugin, nil
}

// UnloadPlugin unloads a plugin (decrements ref count, unloads when reaches 0)
func (pl *PluginLoader) UnloadPlugin(libraryPath string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	absPath, err := filepath.Abs(libraryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	loaded, exists := pl.loadedPlugins[absPath]
	if !exists {
		return fmt.Errorf("plugin not loaded: %s", absPath)
	}

	loaded.mu.Lock()
	loaded.RefCount--
	refCount := loaded.RefCount
	loaded.mu.Unlock()

	if refCount <= 0 {
		// Shutdown plugin
		if err := loaded.Plugin.Shutdown(); err != nil {
			log.Warnf("Error shutting down plugin %s: %v", absPath, err)
		}

		// Remove from tracking
		delete(pl.loadedPlugins, absPath)

		// Note: purego doesn't currently provide Dlclose, so we can't unload the library
		// The library will remain in memory until process exit
		log.Infof("Unloaded plugin: %s (library remains in memory)", absPath)
	} else {
		log.Infof("Decremented plugin ref count: %s (refCount: %d)", absPath, refCount)
	}

	return nil
}

// GetLoadedPlugins returns a list of all loaded plugins
func (pl *PluginLoader) GetLoadedPlugins() []string {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	paths := make([]string, 0, len(pl.loadedPlugins))
	for path := range pl.loadedPlugins {
		paths = append(paths, path)
	}
	return paths
}

// IsLoaded checks if a plugin is currently loaded
func (pl *PluginLoader) IsLoaded(libraryPath string) bool {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	absPath, err := filepath.Abs(libraryPath)
	if err != nil {
		return false
	}

	_, exists := pl.loadedPlugins[absPath]
	return exists
}

// loadPluginVTable loads all required function pointers from the library
func loadPluginVTable(libHandle uintptr) (*api.PluginVTable, error) {
	vtable := &api.PluginVTable{}

	// Required functions
	if err := loadFunc(libHandle, "PluginNew", &vtable.PluginNew); err != nil {
		return nil, fmt.Errorf("missing required function PluginNew: %w", err)
	}

	// Optional lifecycle functions
	loadFunc(libHandle, "PluginFree", &vtable.PluginFree)
	loadFunc(libHandle, "PluginName", &vtable.PluginName)
	loadFunc(libHandle, "PluginValidate", &vtable.PluginValidate)
	loadFunc(libHandle, "PluginInitialize", &vtable.PluginInitialize)
	loadFunc(libHandle, "PluginShutdown", &vtable.PluginShutdown)
	loadFunc(libHandle, "PluginGetReadme", &vtable.PluginGetReadme)

	// Optional filesystem functions
	loadFunc(libHandle, "FSCreate", &vtable.FSCreate)
	loadFunc(libHandle, "FSMkdir", &vtable.FSMkdir)
	loadFunc(libHandle, "FSRemove", &vtable.FSRemove)
	loadFunc(libHandle, "FSRemoveAll", &vtable.FSRemoveAll)
	loadFunc(libHandle, "FSRead", &vtable.FSRead)
	loadFunc(libHandle, "FSWrite", &vtable.FSWrite)
	loadFunc(libHandle, "FSReadDir", &vtable.FSReadDir)
	loadFunc(libHandle, "FSStat", &vtable.FSStat)
	loadFunc(libHandle, "FSRename", &vtable.FSRename)
	loadFunc(libHandle, "FSChmod", &vtable.FSChmod)

	return vtable, nil
}

// loadFunc loads a single function from the library
func loadFunc(libHandle uintptr, name string, fptr interface{}) error {
	defer func() {
		if r := recover(); r != nil {
			log.Debugf("Function %s not found in library (this may be ok if optional)", name)
		}
	}()

	purego.RegisterLibFunc(fptr, libHandle, name)
	return nil
}

// getDlopenFlags returns platform-specific dlopen flags
func getDlopenFlags() int {
	// RTLD_NOW = resolve all symbols immediately
	// RTLD_LOCAL = symbols not available for subsequently loaded libraries
	const (
		RTLD_NOW   = 0x2
		RTLD_LAZY  = 0x1
		RTLD_LOCAL = 0x0
	)

	switch runtime.GOOS {
	case "darwin", "linux":
		return RTLD_NOW | RTLD_LOCAL
	case "windows":
		// Windows doesn't use the same flags, but purego handles this
		return 0
	default:
		return RTLD_NOW | RTLD_LOCAL
	}
}
