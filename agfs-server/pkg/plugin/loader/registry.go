package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

// PluginInfo contains metadata about a discovered plugin
type PluginInfo struct {
	Path     string
	Name     string
	Type     PluginType
	IsLoaded bool
}

// DiscoverPlugins searches for plugin files in a directory (both native and WASM)
func DiscoverPlugins(dir string) ([]PluginInfo, error) {
	if dir == "" {
		return []PluginInfo{}, nil
	}

	// Check if directory exists
	stat, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []PluginInfo{}, nil
		}
		return nil, fmt.Errorf("failed to stat plugin directory: %w", err)
	}

	if !stat.IsDir() {
		return nil, fmt.Errorf("plugin path is not a directory: %s", dir)
	}

	// Get plugin extension for current platform
	nativeExt := getPluginExtension()
	wasmExt := ".wasm"

	// Find all plugin files
	var plugins []PluginInfo

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warnf("Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Check if file has plugin extension (native or WASM)
		var pluginType PluginType
		var name string

		if strings.HasSuffix(info.Name(), nativeExt) {
			pluginType = PluginTypeNative
			name = strings.TrimSuffix(info.Name(), nativeExt)
		} else if strings.HasSuffix(info.Name(), wasmExt) {
			pluginType = PluginTypeWASM
			name = strings.TrimSuffix(info.Name(), wasmExt)
		} else {
			// Not a plugin file, skip
			return nil
		}

		plugins = append(plugins, PluginInfo{
			Path:     path,
			Name:     name,
			Type:     pluginType,
			IsLoaded: false,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk plugin directory: %w", err)
	}

	log.Infof("Discovered %d plugin(s) in %s (%d native, %d WASM)",
		len(plugins), dir, countPluginsByType(plugins, PluginTypeNative), countPluginsByType(plugins, PluginTypeWASM))
	return plugins, nil
}

// countPluginsByType counts plugins of a specific type
func countPluginsByType(plugins []PluginInfo, pluginType PluginType) int {
	count := 0
	for _, p := range plugins {
		if p.Type == pluginType {
			count++
		}
	}
	return count
}

// getPluginExtension returns the shared library extension for the current platform
func getPluginExtension() string {
	switch runtime.GOOS {
	case "darwin":
		return ".dylib"
	case "linux":
		return ".so"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

// LoadPluginsFromDirectory loads all plugins from a directory
func (pl *PluginLoader) LoadPluginsFromDirectory(dir string) ([]string, []error) {
	plugins, err := DiscoverPlugins(dir)
	if err != nil {
		return nil, []error{err}
	}

	var loaded []string
	var errors []error

	for _, pluginInfo := range plugins {
		_, err := pl.LoadPlugin(pluginInfo.Path)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to load %s: %w", pluginInfo.Name, err))
			log.Errorf("Failed to load plugin %s: %v", pluginInfo.Path, err)
		} else {
			loaded = append(loaded, pluginInfo.Path)
			log.Infof("Loaded plugin: %s", pluginInfo.Name)
		}
	}

	return loaded, errors
}

// ValidatePluginPath validates that a plugin path is safe to load
func ValidatePluginPath(path string) error {
	// Check if path exists
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("plugin file not found: %w", err)
	}

	if stat.IsDir() {
		return fmt.Errorf("plugin path is a directory, not a file")
	}

	// Check extension (either native or WASM)
	nativeExt := getPluginExtension()
	wasmExt := ".wasm"
	if !strings.HasSuffix(path, nativeExt) && !strings.HasSuffix(path, wasmExt) {
		return fmt.Errorf("invalid plugin file extension (expected %s or %s)", nativeExt, wasmExt)
	}

	// Check file is readable
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open plugin file: %w", err)
	}
	file.Close()

	return nil
}
