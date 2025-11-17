package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/api"
	log "github.com/sirupsen/logrus"
	"github.com/tetratelabs/wazero"
	wazeroapi "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// LoadedWASMPlugin tracks a loaded WASM plugin
type LoadedWASMPlugin struct {
	Path     string
	Plugin   plugin.ServicePlugin
	Runtime  wazero.Runtime
	Module   wazeroapi.Module
	RefCount int
	mu       sync.Mutex
}

// WASMPluginLoader manages loading and unloading of WASM plugins
type WASMPluginLoader struct {
	loadedPlugins map[string]*LoadedWASMPlugin
	mu            sync.RWMutex
}

// NewWASMPluginLoader creates a new WASM plugin loader
func NewWASMPluginLoader() *WASMPluginLoader {
	return &WASMPluginLoader{
		loadedPlugins: make(map[string]*LoadedWASMPlugin),
	}
}

// LoadWASMPlugin loads a plugin from a WASM file
// If hostFS is provided, it will be exposed to the WASM plugin as host functions
func (wl *WASMPluginLoader) LoadWASMPlugin(wasmPath string, hostFS ...interface{}) (plugin.ServicePlugin, error) {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	// Check if already loaded
	absPath, err := filepath.Abs(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// For WASM plugins, if already loaded, create a new instance with unique key
	// This allows hot reloading of the same WASM file
	if _, exists := wl.loadedPlugins[absPath]; exists {
		log.Infof("WASM plugin %s already loaded, creating new instance", absPath)

		// Find a unique key for this instance
		counter := 1
		var uniqueKey string
		for {
			uniqueKey = fmt.Sprintf("%s#%d", absPath, counter)
			if _, exists := wl.loadedPlugins[uniqueKey]; !exists {
				break
			}
			counter++
		}
		absPath = uniqueKey
		log.Infof("Using unique key for new WASM instance: %s", absPath)
	}

	// Read WASM binary
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM file %s: %w", wasmPath, err)
	}

	// Create a new WASM runtime
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)

	// Instantiate WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Always instantiate host filesystem module (required by WASM modules that import these functions)
	// If no hostFS is provided, use stub functions that return errors
	var fs filesystem.FileSystem
	if len(hostFS) > 0 && hostFS[0] != nil {
		// Type assert to filesystem.FileSystem
		var ok bool
		fs, ok = hostFS[0].(filesystem.FileSystem)
		if !ok {
			r.Close(ctx)
			return nil, fmt.Errorf("hostFS is not a filesystem.FileSystem")
		}
		log.Infof("Registering host filesystem for WASM plugin")
	} else {
		log.Infof("No host filesystem provided, using stub functions")
		fs = nil // Will be handled by api functions
	}

	_, err = r.NewHostModuleBuilder("env").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32, offset, size int64) uint64 {
				return api.HostFSRead(ctx, mod, []uint64{uint64(pathPtr), uint64(offset), uint64(size)}, fs)[0]
			}).
			Export("host_fs_read").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr, dataPtr, dataLen uint32) uint64 {
				return api.HostFSWrite(ctx, mod, []uint64{uint64(pathPtr), uint64(dataPtr), uint64(dataLen)}, fs)[0]
			}).
			Export("host_fs_write").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32) uint64 {
				return api.HostFSStat(ctx, mod, []uint64{uint64(pathPtr)}, fs)[0]
			}).
			Export("host_fs_stat").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32) uint64 {
				return api.HostFSReadDir(ctx, mod, []uint64{uint64(pathPtr)}, fs)[0]
			}).
			Export("host_fs_readdir").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32) uint32 {
				return uint32(api.HostFSCreate(ctx, mod, []uint64{uint64(pathPtr)}, fs)[0])
			}).
			Export("host_fs_create").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr, perm uint32) uint32 {
				return uint32(api.HostFSMkdir(ctx, mod, []uint64{uint64(pathPtr), uint64(perm)}, fs)[0])
			}).
			Export("host_fs_mkdir").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32) uint32 {
				return uint32(api.HostFSRemove(ctx, mod, []uint64{uint64(pathPtr)}, fs)[0])
			}).
			Export("host_fs_remove").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr uint32) uint32 {
				return uint32(api.HostFSRemoveAll(ctx, mod, []uint64{uint64(pathPtr)}, fs)[0])
			}).
			Export("host_fs_remove_all").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, oldPathPtr, newPathPtr uint32) uint32 {
				return uint32(api.HostFSRename(ctx, mod, []uint64{uint64(oldPathPtr), uint64(newPathPtr)}, fs)[0])
			}).
			Export("host_fs_rename").
			NewFunctionBuilder().
			WithFunc(func(ctx context.Context, mod wazeroapi.Module, pathPtr, mode uint32) uint32 {
				return uint32(api.HostFSChmod(ctx, mod, []uint64{uint64(pathPtr), uint64(mode)}, fs)[0])
			}).
			Export("host_fs_chmod").
			Instantiate(ctx)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate host filesystem module: %w", err)
	}

	// Compile and instantiate the WASM module
	compiledModule, err := r.CompileModule(ctx, wasmBytes)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Instantiate the module without filesystem access
	// WASM plugins are not allowed to access the local filesystem
	config := wazero.NewModuleConfig().
		WithName("plugin").
		WithStdout(os.Stdout). // Enable stdout
		WithStderr(os.Stderr)  // Enable stderr

	module, err := r.InstantiateModule(ctx, compiledModule, config)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	log.Infof("Loaded WASM module: %s", wasmPath)

	// Create WASM plugin wrapper
	wasmPlugin, err := api.NewWASMPlugin(ctx, module)
	if err != nil {
		module.Close(ctx)
		r.Close(ctx)
		return nil, fmt.Errorf("failed to create WASM plugin wrapper: %w", err)
	}

	// Track loaded plugin
	loaded := &LoadedWASMPlugin{
		Path:     absPath,
		Plugin:   wasmPlugin,
		Runtime:  r,
		Module:   module,
		RefCount: 1,
	}
	wl.loadedPlugins[absPath] = loaded

	log.Infof("Successfully loaded WASM plugin: %s (name: %s)", absPath, wasmPlugin.Name())
	return wasmPlugin, nil
}

// UnloadWASMPlugin unloads a WASM plugin (decrements ref count, unloads when reaches 0)
func (wl *WASMPluginLoader) UnloadWASMPlugin(wasmPath string) error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	absPath, err := filepath.Abs(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	loaded, exists := wl.loadedPlugins[absPath]
	if !exists {
		return fmt.Errorf("WASM plugin not loaded: %s", absPath)
	}

	loaded.mu.Lock()
	loaded.RefCount--
	refCount := loaded.RefCount
	loaded.mu.Unlock()

	if refCount <= 0 {
		// Shutdown plugin
		if err := loaded.Plugin.Shutdown(); err != nil {
			log.Warnf("Error shutting down WASM plugin %s: %v", absPath, err)
		}

		// Close module and runtime
		ctx := context.Background()
		if err := loaded.Module.Close(ctx); err != nil {
			log.Warnf("Error closing WASM module %s: %v", absPath, err)
		}
		if err := loaded.Runtime.Close(ctx); err != nil {
			log.Warnf("Error closing WASM runtime %s: %v", absPath, err)
		}

		// Remove from tracking
		delete(wl.loadedPlugins, absPath)
		log.Infof("Unloaded WASM plugin: %s", absPath)
	} else {
		log.Infof("Decremented WASM plugin ref count: %s (refCount: %d)", absPath, refCount)
	}

	return nil
}

// GetLoadedPlugins returns a list of all loaded WASM plugins
func (wl *WASMPluginLoader) GetLoadedPlugins() []string {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	paths := make([]string, 0, len(wl.loadedPlugins))
	for path := range wl.loadedPlugins {
		paths = append(paths, path)
	}
	return paths
}

// IsLoaded checks if a WASM plugin is currently loaded
func (wl *WASMPluginLoader) IsLoaded(wasmPath string) bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()

	absPath, err := filepath.Abs(wasmPath)
	if err != nil {
		return false
	}

	_, exists := wl.loadedPlugins[absPath]
	return exists
}
