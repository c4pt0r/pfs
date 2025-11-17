package hellofs

import (
	"errors"
	"io"
	"time"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin/config"
)

const (
	PluginName = "hellofs"
)

// HelloFSPlugin is a minimal plugin that only provides a single "hello" file
type HelloFSPlugin struct{}

// NewHelloFSPlugin creates a new HelloFS plugin
func NewHelloFSPlugin() *HelloFSPlugin {
	return &HelloFSPlugin{}
}

func (p *HelloFSPlugin) Name() string {
	return PluginName
}

func (p *HelloFSPlugin) Validate(cfg map[string]interface{}) error {
	// Only mount_path is allowed (injected by framework)
	allowedKeys := []string{"mount_path"}
	return config.ValidateOnlyKnownKeys(cfg, allowedKeys)
}

func (p *HelloFSPlugin) Initialize(config map[string]interface{}) error {
	return nil
}

func (p *HelloFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &HelloFS{}
}

func (p *HelloFSPlugin) GetReadme() string {
	return `HelloFS Plugin - Minimal Demo

This plugin provides a single file: /hello

USAGE:
  cat /hellofs/hello
`
}

func (p *HelloFSPlugin) Shutdown() error {
	return nil
}

// HelloFS is a minimal filesystem that only supports reading /hello
type HelloFS struct{}

func (fs *HelloFS) Read(path string, offset int64, size int64) ([]byte, error) {
	if path == "/hello" {
		data := []byte("Hello, World!\n")
		return plugin.ApplyRangeRead(data, offset, size)
	}
	return nil, errors.New("file not found")
}

func (fs *HelloFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/hello" {
		return &filesystem.FileInfo{
			Name:    "hello",
			Size:    14,
			Mode:    0444,
			ModTime: time.Now(),
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "file"},
		}, nil
	}
	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0555,
			ModTime: time.Now(),
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: PluginName, Type: "directory"},
		}, nil
	}
	return nil, errors.New("file not found")
}

func (fs *HelloFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path == "/" {
		return []filesystem.FileInfo{
			{
				Name:    "hello",
				Size:    14,
				Mode:    0444,
				ModTime: time.Now(),
				IsDir:   false,
				Meta:    filesystem.MetaData{Name: PluginName, Type: "file"},
			},
		}, nil
	}
	return nil, errors.New("not a directory")
}

// Unsupported operations
func (fs *HelloFS) Create(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) Mkdir(path string, perm uint32) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) Remove(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) RemoveAll(path string) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) Write(path string, data []byte) ([]byte, error) {
	return nil, errors.New("read-only filesystem")
}

func (fs *HelloFS) Rename(oldPath, newPath string) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) Chmod(path string, mode uint32) error {
	return errors.New("read-only filesystem")
}

func (fs *HelloFS) Open(path string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (fs *HelloFS) OpenWrite(path string) (io.WriteCloser, error) {
	return nil, errors.New("read-only filesystem")
}

// Ensure HelloFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*HelloFSPlugin)(nil)
var _ filesystem.FileSystem = (*HelloFS)(nil)
