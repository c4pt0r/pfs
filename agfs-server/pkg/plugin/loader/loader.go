package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/api"
	"github.com/ebitengine/purego"
	log "github.com/sirupsen/logrus"
)

// PluginType represents the type of plugin
type PluginType int

const (
	// PluginTypeUnknown represents an unknown plugin type
	PluginTypeUnknown PluginType = iota
	// PluginTypeNative represents a native shared library plugin (.so, .dylib, .dll)
	PluginTypeNative
	// PluginTypeWASM represents a WebAssembly plugin (.wasm)
	PluginTypeWASM
)

// String returns the string representation of the plugin type
func (pt PluginType) String() string {
	switch pt {
	case PluginTypeNative:
		return "native"
	case PluginTypeWASM:
		return "wasm"
	default:
		return "unknown"
	}
}

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
	wasmLoader    *WASMPluginLoader
	mu            sync.RWMutex
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader() *PluginLoader {
	return &PluginLoader{
		loadedPlugins: make(map[string]*LoadedPlugin),
		wasmLoader:    NewWASMPluginLoader(),
	}
}


// DetectPluginType detects the type of plugin based on file content and extension
func DetectPluginType(libraryPath string) (PluginType, error) {
	// Check if file exists
	if _, err := os.Stat(libraryPath); err != nil {
		return PluginTypeUnknown, fmt.Errorf("plugin file not found: %w", err)
	}

	// Try to read file magic number
	file, err := os.Open(libraryPath)
	if err != nil {
		return PluginTypeUnknown, fmt.Errorf("failed to open plugin file: %w", err)
	}
	defer file.Close()

	// Read first 4 bytes for magic number detection
	magic := make([]byte, 4)
	n, err := file.Read(magic)
	if err != nil || n < 4 {
		// If we can't read magic, fall back to extension
		return detectPluginTypeByExtension(libraryPath), nil
	}

	// Check WASM magic number: 0x00 0x61 0x73 0x6D ("\0asm")
	if magic[0] == 0x00 && magic[1] == 0x61 && magic[2] == 0x73 && magic[3] == 0x6D {
		return PluginTypeWASM, nil
	}

	// Check ELF magic number: 0x7F 'E' 'L' 'F' (Linux .so)
	if magic[0] == 0x7F && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
		return PluginTypeNative, nil
	}

	// Check Mach-O magic numbers (macOS .dylib)
	// 32-bit: 0xFE 0xED 0xFA 0xCE or 0xCE 0xFA 0xED 0xFE
	// 64-bit: 0xFE 0xED 0xFA 0xCF or 0xCF 0xFA 0xED 0xFE
	// Fat binary: 0xCA 0xFE 0xBA 0xBE or 0xBE 0xBA 0xFE 0xCA
	if (magic[0] == 0xFE && magic[1] == 0xED && magic[2] == 0xFA && (magic[3] == 0xCE || magic[3] == 0xCF)) ||
		(magic[0] == 0xCE && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) ||
		(magic[0] == 0xCF && magic[1] == 0xFA && magic[2] == 0xED && magic[3] == 0xFE) ||
		(magic[0] == 0xCA && magic[1] == 0xFE && magic[2] == 0xBA && magic[3] == 0xBE) ||
		(magic[0] == 0xBE && magic[1] == 0xBA && magic[2] == 0xFE && magic[3] == 0xCA) {
		return PluginTypeNative, nil
	}

	// Check PE magic number: 'M' 'Z' (Windows .dll) - first 2 bytes
	if magic[0] == 'M' && magic[1] == 'Z' {
		return PluginTypeNative, nil
	}

	// Fall back to extension-based detection
	return detectPluginTypeByExtension(libraryPath), nil
}

// detectPluginTypeByExtension detects plugin type based on file extension (fallback)
func detectPluginTypeByExtension(libraryPath string) PluginType {
	ext := strings.ToLower(filepath.Ext(libraryPath))
	switch ext {
	case ".wasm":
		return PluginTypeWASM
	case ".so", ".dylib", ".dll":
		return PluginTypeNative
	default:
		return PluginTypeUnknown
	}
}

// LoadPluginWithType loads a plugin with an explicitly specified type
// For WASM plugins, optional hostFS can be provided to allow access to host filesystem
func (pl *PluginLoader) LoadPluginWithType(libraryPath string, pluginType PluginType, hostFS ...interface{}) (plugin.ServicePlugin, error) {
	log.Debugf("Loading plugin with type %s: %s", pluginType, libraryPath)

	// Load based on specified type
	switch pluginType {
	case PluginTypeWASM:
		return pl.wasmLoader.LoadWASMPlugin(libraryPath, hostFS...)
	case PluginTypeNative:
		return pl.loadNativePlugin(libraryPath)
	default:
		return nil, fmt.Errorf("unsupported plugin type: %s", pluginType)
	}
}

// LoadPlugin loads a plugin from a shared library file (.so, .dylib, .dll) or WASM file (.wasm)
// The plugin type is automatically detected based on file magic number and extension
func (pl *PluginLoader) LoadPlugin(libraryPath string) (plugin.ServicePlugin, error) {
	// Detect plugin type
	pluginType, err := DetectPluginType(libraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect plugin type: %w", err)
	}

	log.Debugf("Auto-detected plugin type: %s for %s", pluginType, libraryPath)

	// Use LoadPluginWithType for actual loading
	return pl.LoadPluginWithType(libraryPath, pluginType)
}

// loadNativePlugin loads a native shared library plugin
func (pl *PluginLoader) loadNativePlugin(libraryPath string) (plugin.ServicePlugin, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Check if already loaded
	absPath, err := filepath.Abs(libraryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// For native plugins, if already loaded, create a temp copy
	// This allows loading multiple versions of the same file
	if _, exists := pl.loadedPlugins[absPath]; exists {
		log.Infof("Native plugin %s already loaded, creating new instance from copy", absPath)

		// Create a unique temp copy
		tempDir := os.TempDir()
		baseName := filepath.Base(libraryPath)
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)

		// Find an available filename
		counter := 1
		var tempLibPath string
		for {
			tempLibPath = filepath.Join(tempDir, fmt.Sprintf("%s.%d%s", nameWithoutExt, counter, ext))
			if _, err := os.Stat(tempLibPath); os.IsNotExist(err) {
				break
			}
			counter++
		}

		// Copy file
		if err := copyFile(libraryPath, tempLibPath); err != nil {
			return nil, fmt.Errorf("failed to create temp copy: %w", err)
		}

		// Use the temp path as key
		absPath = tempLibPath
		log.Infof("Created temp copy at: %s", absPath)
	}

	// Determine dlopen flags based on platform
	flags := getDlopenFlags()

	// Open the shared library
	libHandle, err := purego.Dlopen(absPath, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to open library %s: %w", absPath, err)
	}

	log.Infof("Loaded library: %s (handle: %v)", absPath, libHandle)

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

// UnloadPluginWithType unloads a plugin with an explicitly specified type
func (pl *PluginLoader) UnloadPluginWithType(libraryPath string, pluginType PluginType) error {
	log.Debugf("Unloading plugin with type %s: %s", pluginType, libraryPath)

	// Unload based on specified type
	switch pluginType {
	case PluginTypeWASM:
		return pl.wasmLoader.UnloadWASMPlugin(libraryPath)
	case PluginTypeNative:
		return pl.unloadNativePlugin(libraryPath)
	default:
		return fmt.Errorf("unsupported plugin type: %s", pluginType)
	}
}

// UnloadPlugin unloads a plugin (decrements ref count, unloads when reaches 0)
// The plugin type is automatically detected based on file magic number and extension
func (pl *PluginLoader) UnloadPlugin(libraryPath string) error {
	// Detect plugin type
	pluginType, err := DetectPluginType(libraryPath)
	if err != nil {
		return fmt.Errorf("failed to detect plugin type: %w", err)
	}

	// Use UnloadPluginWithType for actual unloading
	return pl.UnloadPluginWithType(libraryPath, pluginType)
}

// unloadNativePlugin unloads a native shared library plugin
func (pl *PluginLoader) unloadNativePlugin(libraryPath string) error {
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

// GetLoadedPlugins returns a list of all loaded plugins (both native and WASM)
func (pl *PluginLoader) GetLoadedPlugins() []string {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	paths := make([]string, 0, len(pl.loadedPlugins))
	for path := range pl.loadedPlugins {
		paths = append(paths, path)
	}

	// Add WASM plugins
	wasmPaths := pl.wasmLoader.GetLoadedPlugins()
	paths = append(paths, wasmPaths...)

	return paths
}

// IsLoadedWithType checks if a plugin of a specific type is currently loaded
func (pl *PluginLoader) IsLoadedWithType(libraryPath string, pluginType PluginType) bool {
	// Check based on specified type
	switch pluginType {
	case PluginTypeWASM:
		return pl.wasmLoader.IsLoaded(libraryPath)
	case PluginTypeNative:
		return pl.isNativePluginLoaded(libraryPath)
	default:
		return false
	}
}

// IsLoaded checks if a plugin is currently loaded (both native and WASM)
// The plugin type is automatically detected based on file magic number and extension
func (pl *PluginLoader) IsLoaded(libraryPath string) bool {
	// Detect plugin type
	pluginType, err := DetectPluginType(libraryPath)
	if err != nil {
		log.Debugf("Failed to detect plugin type for %s: %v", libraryPath, err)
		return false
	}

	// Use IsLoadedWithType for actual check
	return pl.IsLoadedWithType(libraryPath, pluginType)
}

// isNativePluginLoaded checks if a native plugin is currently loaded
func (pl *PluginLoader) isNativePluginLoaded(libraryPath string) bool {
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

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, data, 0755)
	if err != nil {
		return err
	}

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
