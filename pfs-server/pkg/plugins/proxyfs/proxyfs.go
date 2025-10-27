package proxyfs

import (
	"fmt"
	"io"
	"time"

	"github.com/c4pt0r/pfs-server/pkg/client"
	"github.com/c4pt0r/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs-server/pkg/plugin"
)

const (
	PluginName = "proxyfs" // Name of this plugin
)

// ProxyFS implements filesystem.FileSystem by proxying to a remote PFS HTTP API
// All file system operations are transparently forwarded to the remote server
type ProxyFS struct {
	client     *client.Client
	pluginName string
	baseURL    string // Store base URL for reload
}

// NewProxyFS creates a new ProxyFS that redirects to a remote PFS server
// baseURL should include the API version, e.g., "http://localhost:8080/api/v1"
func NewProxyFS(baseURL string, pluginName string) *ProxyFS {
	return &ProxyFS{
		client:     client.NewClient(baseURL),
		pluginName: pluginName,
		baseURL:    baseURL,
	}
}

// Reload recreates the HTTP client, useful for refreshing connections
func (pfs *ProxyFS) Reload() error {
	// Create a new client to refresh the connection
	pfs.client = client.NewClient(pfs.baseURL)

	// Test the new connection
	if err := pfs.client.Health(); err != nil {
		return fmt.Errorf("failed to connect after reload: %w", err)
	}

	return nil
}

func (pfs *ProxyFS) Create(path string) error {
	return pfs.client.Create(path)
}

func (pfs *ProxyFS) Mkdir(path string, perm uint32) error {
	return pfs.client.Mkdir(path, perm)
}

func (pfs *ProxyFS) Remove(path string) error {
	return pfs.client.Remove(path)
}

func (pfs *ProxyFS) RemoveAll(path string) error {
	return pfs.client.RemoveAll(path)
}

func (pfs *ProxyFS) Read(path string, offset int64, size int64) ([]byte, error) {
	// Special handling for /reload
	if path == "/reload" {
		data := []byte("Write to this file to reload the proxy connection\n")
		return plugin.ApplyRangeRead(data, offset, size)
	}
	return pfs.client.Read(path, offset, size)
}

func (pfs *ProxyFS) Write(path string, data []byte) ([]byte, error) {
	// Special handling for /reload - trigger hot reload
	if path == "/reload" {
		if err := pfs.Reload(); err != nil {
			return nil, fmt.Errorf("reload failed: %w", err)
		}
		return []byte("ProxyFS reloaded successfully"), nil
	}
	return pfs.client.Write(path, data)
}

func (pfs *ProxyFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	files, err := pfs.client.ReadDir(path)
	if err != nil {
		return nil, err
	}

	// Add /reload virtual file to root directory listing
	if path == "/" {
		reloadFile := filesystem.FileInfo{
			Name:    "reload",
			Size:    0,
			Mode:    0o200,            // write-only
			ModTime: files[0].ModTime, // Use same time as first file
			IsDir:   false,
			Meta: filesystem.MetaData{
				Type: "control",
				Content: map[string]string{
					"description": "Write to this file to reload proxy connection",
				},
			},
		}
		files = append(files, reloadFile)
	}

	return files, nil
}

func (pfs *ProxyFS) Stat(path string) (*filesystem.FileInfo, error) {
	// Special handling for /reload
	if path == "/reload" {
		return &filesystem.FileInfo{
			Name:    "reload",
			Size:    0,
			Mode:    0o200, // write-only
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Type: "control",
				Content: map[string]string{
					"description": "Write to this file to reload proxy connection",
					"remote-url":  pfs.baseURL,
				},
			},
		}, nil
	}

	// Get stat from remote
	stat, err := pfs.client.Stat(path)
	if err != nil {
		return nil, err
	}

	// Add remote URL to metadata
	if stat.Meta.Content == nil {
		stat.Meta.Content = make(map[string]string)
	}
	stat.Meta.Content["remote-url"] = pfs.baseURL

	return stat, nil
}

func (pfs *ProxyFS) Rename(oldPath, newPath string) error {
	return pfs.client.Rename(oldPath, newPath)
}

func (pfs *ProxyFS) Chmod(path string, mode uint32) error {
	return pfs.client.Chmod(path, mode)
}

func (pfs *ProxyFS) Open(path string) (io.ReadCloser, error) {
	data, err := pfs.client.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(io.Reader(newBytesReader(data))), nil
}

func (pfs *ProxyFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &proxyWriter{
		pfs:  pfs,
		path: path,
		buf:  make([]byte, 0),
	}, nil
}

// OpenStream implements filesystem.Streamer interface
func (pfs *ProxyFS) OpenStream(path string) (filesystem.StreamReader, error) {
	// Use the client's ReadStream to get a streaming connection
	streamReader, err := pfs.client.ReadStream(path)
	if err != nil {
		return nil, err
	}

	// Return a ProxyStreamReader that implements filesystem.StreamReader
	return &ProxyStreamReader{
		reader: streamReader,
		path:   path,
		buf:    make([]byte, 64*1024), // 64KB buffer for chunked reads
	}, nil
}

// GetStream returns a streaming reader for remote streamfs files
// Deprecated: Use OpenStream instead
func (pfs *ProxyFS) GetStream(path string) (interface{}, error) {
	// Use the client's ReadStream to get a streaming connection
	streamReader, err := pfs.client.ReadStream(path)
	if err != nil {
		return nil, err
	}

	// Wrap the io.ReadCloser in a ProxyStream for backward compatibility
	return &ProxyStream{
		reader: streamReader,
		path:   path,
	}, nil
}

// ProxyStreamReader adapts an io.ReadCloser to filesystem.StreamReader
// It reads chunks from the remote stream with timeout support
type ProxyStreamReader struct {
	reader io.ReadCloser
	path   string
	buf    []byte // Buffer for reading chunks
}

// ReadChunk implements filesystem.StreamReader
func (psr *ProxyStreamReader) ReadChunk(timeout time.Duration) ([]byte, bool, error) {
	// Set read deadline if possible
	// Note: HTTP response bodies don't support deadlines, so timeout is best-effort

	// Read a chunk from the stream
	n, err := psr.reader.Read(psr.buf)

	if n > 0 {
		// Make a copy of the data to return
		chunk := make([]byte, n)
		copy(chunk, psr.buf[:n])
		return chunk, false, nil
	}

	if err == io.EOF {
		return nil, true, io.EOF
	}

	if err != nil {
		return nil, false, err
	}

	// No data and no error - unlikely but handle it
	return nil, false, fmt.Errorf("read timeout")
}

// Close implements filesystem.StreamReader
func (psr *ProxyStreamReader) Close() error {
	return psr.reader.Close()
}

// ProxyStream wraps an io.ReadCloser to provide streaming functionality
// Deprecated: Used for backward compatibility with old GetStream interface
type ProxyStream struct {
	reader io.ReadCloser
	path   string
}

// Read implements io.Reader
func (ps *ProxyStream) Read(p []byte) (n int, err error) {
	return ps.reader.Read(p)
}

// Close implements io.Closer
func (ps *ProxyStream) Close() error {
	return ps.reader.Close()
}

// proxyWriter implements io.WriteCloser for ProxyFS
type proxyWriter struct {
	pfs  *ProxyFS
	path string
	buf  []byte
}

func (w *proxyWriter) Write(p []byte) (n int, error error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *proxyWriter) Close() error {
	_, err := w.pfs.Write(w.path, w.buf)
	return err
}

// bytesReader wraps a byte slice to implement io.Reader
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data, pos: 0}
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// ProxyFSPlugin wraps ProxyFS as a plugin that can be mounted in PFS
// It enables remote file system access through the PFS plugin system
type ProxyFSPlugin struct {
	fs      *ProxyFS
	baseURL string
}

// NewProxyFSPlugin creates a new ProxyFS plugin
// baseURL should be the full API endpoint, e.g., "http://remote-server:8080/api/v1"
func NewProxyFSPlugin(baseURL string) *ProxyFSPlugin {
	return &ProxyFSPlugin{
		baseURL: baseURL,
		fs:      NewProxyFS(baseURL, PluginName),
	}
}

func (p *ProxyFSPlugin) Name() string {
	return PluginName
}

func (p *ProxyFSPlugin) Initialize(config map[string]interface{}) error {
	// Override base URL if provided in config
	// Expected config: {"base_url": "http://remote-server:8080/api/v1"}
	if config != nil {
		if url, ok := config["base_url"].(string); ok && url != "" {
			p.baseURL = url
			p.fs = NewProxyFS(url, PluginName)
		}
	}

	// Test connection to remote server with health check
	if err := p.fs.client.Health(); err != nil {
		return fmt.Errorf("failed to connect to remote PFS server at %s: %w", p.baseURL, err)
	}

	return nil
}

func (p *ProxyFSPlugin) GetFileSystem() filesystem.FileSystem {
	return p.fs
}

func (p *ProxyFSPlugin) GetReadme() string {
	return `ProxyFS Plugin - Remote PFS Proxy

This plugin proxies all file system operations to a remote PFS HTTP API server.

FEATURES:
  - Transparent proxying of all file system operations
  - Full compatibility with PFS HTTP API
  - Connects to remote PFS servers
  - Supports all standard file operations
  - Supports streaming operations (cat --stream)
  - Transparent proxying of remote streamfs
  - Implements filesystem.Streamer interface

CONFIGURATION:
  base_url: URL of the remote PFS server (e.g., "http://remote:8080/api/v1")

HOT RELOAD:
  ProxyFS provides a special /reload file for hot-reloading the connection:

  Echo to /reload to refresh the proxy connection:
    echo '' > /proxyfs/reload

  This is useful when:
  - Remote server was restarted
  - Network connection was interrupted
  - Need to refresh connection pool

USAGE:
  All standard file operations are proxied to the remote server:

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

STREAMING SUPPORT:
  ProxyFS transparently proxies streaming operations to remote PFS servers.

  Access remote streamfs:
    pfs cat --stream /proxyfs/remote/streamfs/video | ffplay -

  Write to remote streamfs:
    cat file.mp4 | pfs write --stream /proxyfs/remote/streamfs/video

  All streaming features from remote streamfs are fully supported:
  - Real-time data streaming
  - Ring buffer with historical data
  - Multiple concurrent readers (fanout)
  - Persistent connections (no timeout disconnect)

EXAMPLES:
  # Standard file operations
  pfs:/> mkdir /proxyfs/remote/data
  pfs:/> echo "hello" > /proxyfs/remote/data/file.txt
  pfs:/> cat /proxyfs/remote/data/file.txt
  hello
  pfs:/> ls /proxyfs/remote/data

  # Streaming operations (outside REPL)
  $ pfs cat --stream /proxyfs/remote/streamfs/logs
  $ cat video.mp4 | pfs write --stream /proxyfs/remote/streamfs/video

USE CASES:
  - Connect to remote PFS instances
  - Federation of multiple PFS servers
  - Access remote services through local mount points
  - Distributed file system scenarios
  - Stream video/audio from remote streamfs
  - Remote real-time data streaming

`
}

func (p *ProxyFSPlugin) Shutdown() error {
	return nil
}

// Ensure ProxyFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*ProxyFSPlugin)(nil)
