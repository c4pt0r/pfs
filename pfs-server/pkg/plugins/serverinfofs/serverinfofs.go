package serverinfofs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
)

// ServerInfoFSPlugin provides server metadata and information
type ServerInfoFSPlugin struct {
	startTime time.Time
	version   string
}

// NewServerInfoFSPlugin creates a new ServerInfoFS plugin
func NewServerInfoFSPlugin() *ServerInfoFSPlugin {
	return &ServerInfoFSPlugin{
		startTime: time.Now(),
		version:   "1.0.0",
	}
}

func (p *ServerInfoFSPlugin) Name() string {
	return "serverinfofs"
}

func (p *ServerInfoFSPlugin) Initialize(config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["version"].(string); ok {
			p.version = v
		}
	}
	return nil
}

func (p *ServerInfoFSPlugin) GetFileSystem() filesystem.FileSystem {
	return &serverInfoFS{plugin: p}
}

func (p *ServerInfoFSPlugin) GetReadme() string {
	return `ServerInfoFS Plugin - Server Metadata and Information

This plugin provides runtime information about the PFS server.

USAGE:
  View server version:
    cat /version

  View server uptime:
    cat /uptime

  View server info:
    cat /info

FILES:
  /version  - Server version information
  /uptime   - Server uptime since start
  /info     - Complete server information (JSON)
  /README   - This file

EXAMPLES:
  # Check server version
  pfs:/> cat /serverinfofs/version
  1.0.0

  # Check uptime
  pfs:/> cat /serverinfofs/uptime
  Server uptime: 5m30s

  # Get complete info
  pfs:/> cat /serverinfofs/server_info
  {
    "version": "1.0.0",
    "uptime": "5m30s",
    "go_version": "go1.21",
    ...
  }
`
}

func (p *ServerInfoFSPlugin) Shutdown() error {
	return nil
}

// serverInfoFS implements the FileSystem interface for server metadata
type serverInfoFS struct {
	plugin *ServerInfoFSPlugin
}

// Virtual files in serverinfofs
const (
	fileServerInfo = "/server_info"
	fileUptime     = "/uptime"
	fileVersion    = "/version"
	fileStats      = "/stats"
	fileReadme     = "/README"
)

func (fs *serverInfoFS) isValidPath(path string) bool {
	switch path {
	case "/", fileServerInfo, fileUptime, fileVersion, fileStats, fileReadme:
		return true
	default:
		return false
	}
}

func (fs *serverInfoFS) getServerInfo() map[string]interface{} {
	uptime := time.Since(fs.plugin.startTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"version":      fs.plugin.version,
		"uptime":       uptime.String(),
		"startTime":    fs.plugin.startTime.Format(time.RFC3339),
		"goVersion":    runtime.Version(),
		"numCPU":       runtime.NumCPU(),
		"numGoroutine": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc":      m.Alloc,
			"totalAlloc": m.TotalAlloc,
			"sys":        m.Sys,
			"numGC":      m.NumGC,
		},
	}
}

func (fs *serverInfoFS) Read(path string, offset int64, size int64) ([]byte, error) {
	if !fs.isValidPath(path) {
		return nil, fmt.Errorf("no such file or directory: %s", path)
	}

	if path == "/" {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	var data []byte
	var err error

	switch path {
	case fileServerInfo:
		info := fs.getServerInfo()
		data, err = json.MarshalIndent(info, "", "  ")
		if err != nil {
			return nil, err
		}

	case fileUptime:
		uptime := time.Since(fs.plugin.startTime)
		data = []byte(uptime.String())

	case fileVersion:
		data = []byte(fs.plugin.version)

	case fileStats:
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		stats := map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"memory": map[string]interface{}{
				"alloc":      m.Alloc,
				"totalAlloc": m.TotalAlloc,
				"sys":        m.Sys,
				"numGC":      m.NumGC,
			},
		}
		data, err = json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return nil, err
		}

	case fileReadme:
		data = []byte(fs.plugin.GetReadme())

	default:
		return nil, fmt.Errorf("no such file: %s", path)
	}

	// if data is not ended by '\n' then add it
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	return plugin.ApplyRangeRead(data, offset, size)
}

func (fs *serverInfoFS) Write(path string, data []byte) ([]byte, error) {
	return nil, fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) Create(path string) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) Mkdir(path string, perm uint32) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) Remove(path string) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) RemoveAll(path string) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path != "/" {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	now := time.Now()
	readme := fs.plugin.GetReadme()

	// Generate content for each file to get accurate sizes
	serverInfoData, _ := fs.Read(fileServerInfo, 0, -1)
	uptimeData, _ := fs.Read(fileUptime, 0, -1)
	versionData, _ := fs.Read(fileVersion, 0, -1)
	statsData, _ := fs.Read(fileStats, 0, -1)

	return []filesystem.FileInfo{
		{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444,
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: "serverinfofs", Type: "doc"},
		},
		{
			Name:    "server_info",
			Size:    int64(len(serverInfoData)),
			Mode:    0444,
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: "serverinfofs", Type: "info"},
		},
		{
			Name:    "uptime",
			Size:    int64(len(uptimeData)),
			Mode:    0444,
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: "serverinfofs", Type: "info"},
		},
		{
			Name:    "version",
			Size:    int64(len(versionData)),
			Mode:    0444,
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: "serverinfofs", Type: "info"},
		},
		{
			Name:    "stats",
			Size:    int64(len(statsData)),
			Mode:    0444,
			ModTime: now,
			IsDir:   false,
			Meta:    filesystem.MetaData{Name: "serverinfofs", Type: "info"},
		},
	}, nil
}

func (fs *serverInfoFS) Stat(path string) (*filesystem.FileInfo, error) {
	if !fs.isValidPath(path) {
		return nil, fmt.Errorf("no such file or directory: %s", path)
	}

	now := time.Now()

	if path == "/" {
		return &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0555,
			ModTime: now,
			IsDir:   true,
			Meta:    filesystem.MetaData{Name: "serverinfofs"},
		}, nil
	}

	// For files, read content to get size
	data, err := fs.Read(path, 0, -1)
	if err != nil && err != io.EOF {
		return nil, err
	}

	fileType := "info"
	if path == fileReadme {
		fileType = "doc"
	}

	return &filesystem.FileInfo{
		Name:    path[1:], // Remove leading slash
		Size:    int64(len(data)),
		Mode:    0444,
		ModTime: now,
		IsDir:   false,
		Meta:    filesystem.MetaData{Name: "serverinfofs", Type: fileType},
	}, nil
}

func (fs *serverInfoFS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

func (fs *serverInfoFS) Open(path string) (io.ReadCloser, error) {
	data, err := fs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (fs *serverInfoFS) OpenWrite(path string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("operation not permitted: serverinfofs is read-only")
}

