package plugin

import (
	"github.com/c4pt0r/pfs-server/pkg/filesystem"
)

// ServicePlugin defines the interface for a service that can be mounted to a path
// Each plugin acts as a virtual file system providing service-specific operations
type ServicePlugin interface {
	// Name returns the plugin name
	Name() string

	// Initialize initializes the plugin with optional configuration
	Initialize(config map[string]interface{}) error

	// GetFileSystem returns the FileSystem implementation for this plugin
	// This allows the plugin to handle file operations in a service-specific way
	GetFileSystem() filesystem.FileSystem

	// GetReadme returns the README content for this plugin
	// This provides documentation about the plugin's functionality and usage
	GetReadme() string

	// Shutdown gracefully shuts down the plugin
	Shutdown() error
}

// MountPoint represents a mounted service plugin
type MountPoint struct {
	Path   string
	Plugin ServicePlugin
}

// PluginMetadata contains information about a plugin
type PluginMetadata struct {
	Name        string
	Version     string
	Description string
	Author      string
}

