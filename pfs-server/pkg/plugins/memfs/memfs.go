package memfs

import (
	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
)

const (
	PluginName = "memfs" // Name of this plugin
)

// MemFSPlugin wraps MemoryFS as a plugin
type MemFSPlugin struct {
	fs *MemoryFS
}

// NewMemFSPlugin creates a new MemFS plugin
func NewMemFSPlugin() *MemFSPlugin {
	return &MemFSPlugin{
		fs: NewMemoryFSWithPlugin(PluginName),
	}
}

func (p *MemFSPlugin) Name() string {
	return PluginName
}

func (p *MemFSPlugin) Initialize(config map[string]interface{}) error {
	// Create README file
	readme := []byte(p.GetReadme())
	_ = p.fs.Create("/README")
	_, _ = p.fs.Write("/README", readme)
	_ = p.fs.Chmod("/README", 0444) // Make it read-only

	// Initialize with some default directories if needed
	if config != nil {
		if initDirs, ok := config["init_dirs"].([]string); ok {
			for _, dir := range initDirs {
				_ = p.fs.Mkdir(dir, 0755)
			}
		}
	}
	return nil
}

func (p *MemFSPlugin) GetFileSystem() filesystem.FileSystem {
	return p.fs
}

func (p *MemFSPlugin) GetReadme() string {
	return `MemFS Plugin - In-Memory File System

This plugin provides a full-featured in-memory file system.

FEATURES:
  - Standard file system operations (create, read, write, delete)
  - Directory support with hierarchical structure
  - File permissions (chmod)
  - File/directory renaming and moving
  - Metadata tracking

USAGE:
  Create a file:
    touch /path/to/file

  Write to a file:
    echo "content" > /path/to/file

  Read a file:
    cat /path/to/file

  Create a directory:
    mkdir /path/to/dir

  List directory:
    ls /path/to/dir

  Remove file/directory:
    rm /path/to/file
    rm -r /path/to/dir

  Move/rename:
    mv /old/path /new/path

  Change permissions:
    chmod 755 /path/to/file

EXAMPLES:
  pfs:/> mkdir /memfs/data
  pfs:/> echo "hello" > /memfs/data/file.txt
  pfs:/> cat /memfs/data/file.txt
  hello
  pfs:/> ls /memfs/data
  pfs:/> mv /memfs/data/file.txt /memfs/data/renamed.txt

VERSION: 1.0.0
AUTHOR: VFS Server
`
}

func (p *MemFSPlugin) Shutdown() error {
	return nil
}

// Ensure MemFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*MemFSPlugin)(nil)
