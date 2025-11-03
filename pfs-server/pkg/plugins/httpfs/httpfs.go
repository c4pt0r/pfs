package httpfs

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
	log "github.com/sirupsen/logrus"
)

const (
	PluginName = "httpfs"
)

// getContentType determines the Content-Type based on file extension
func getContentType(filename string) string {
	// Get the base filename (without directory)
	baseName := filepath.Base(filename)
	baseNameUpper := strings.ToUpper(baseName)

	// Special handling for README files (with or without extension)
	// These should display as text/plain in the browser
	if baseNameUpper == "README" ||
	   strings.HasPrefix(baseNameUpper, "README.") {
		return "text/plain; charset=utf-8"
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// Common text formats that should display inline
	textTypes := map[string]string{
		".txt":  "text/plain; charset=utf-8",
		".md":   "text/markdown; charset=utf-8",
		".markdown": "text/markdown; charset=utf-8",
		".json": "application/json; charset=utf-8",
		".xml":  "application/xml; charset=utf-8",
		".html": "text/html; charset=utf-8",
		".htm":  "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "application/javascript; charset=utf-8",
		".yaml": "text/yaml; charset=utf-8",
		".yml":  "text/yaml; charset=utf-8",
		".log":  "text/plain; charset=utf-8",
		".csv":  "text/csv; charset=utf-8",
		".sh":   "text/x-shellscript; charset=utf-8",
		".py":   "text/x-python; charset=utf-8",
		".go":   "text/x-go; charset=utf-8",
		".c":    "text/x-c; charset=utf-8",
		".cpp":  "text/x-c++; charset=utf-8",
		".h":    "text/x-c; charset=utf-8",
		".java": "text/x-java; charset=utf-8",
		".rs":   "text/x-rust; charset=utf-8",
		".sql":  "text/x-sql; charset=utf-8",
	}

	// Image formats
	imageTypes := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".ico":  "image/x-icon",
		".bmp":  "image/bmp",
	}

	// Video formats
	videoTypes := map[string]string{
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".ogg":  "video/ogg",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
	}

	// Audio formats
	audioTypes := map[string]string{
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".m4a":  "audio/mp4",
		".flac": "audio/flac",
	}

	// PDF
	if ext == ".pdf" {
		return "application/pdf"
	}

	// Check our custom maps first
	if ct, ok := textTypes[ext]; ok {
		return ct
	}
	if ct, ok := imageTypes[ext]; ok {
		return ct
	}
	if ct, ok := videoTypes[ext]; ok {
		return ct
	}
	if ct, ok := audioTypes[ext]; ok {
		return ct
	}

	// Fallback to mime package
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}

	// Default to octet-stream for unknown types (will trigger download)
	return "application/octet-stream"
}

// HTTPFS implements FileSystem interface with an embedded HTTP server
// It serves files from a PFS mount path over HTTP like 'python3 -m http.server'
type HTTPFS struct {
	pfsPath      string                // The PFS path to serve (e.g., "/memfs")
	httpPort     string                // HTTP server port
	statusPath   string                // Virtual status file path (e.g., "/httpfs-demo")
	rootFS       filesystem.FileSystem // Reference to the root PFS filesystem
	mu           sync.RWMutex
	server       *http.Server
	pluginName   string
	startTime    time.Time             // Server start time
}

// NewHTTPFS creates a new HTTP file server that serves PFS paths
func NewHTTPFS(pfsPath string, port string, statusPath string, rootFS filesystem.FileSystem) (*HTTPFS, error) {
	if pfsPath == "" {
		return nil, fmt.Errorf("pfs_path is required")
	}

	if rootFS == nil {
		return nil, fmt.Errorf("rootFS is required")
	}

	// Normalize paths
	pfsPath = normalizePath(pfsPath)
	statusPath = normalizePath(statusPath)

	if port == "" {
		port = "8000" // Default port like python http.server
	}

	fs := &HTTPFS{
		pfsPath:    pfsPath,
		httpPort:   port,
		statusPath: statusPath,
		rootFS:     rootFS,
		pluginName: PluginName,
		startTime:  time.Now(),
	}

	// Start HTTP server
	if err := fs.startHTTPServer(); err != nil {
		return nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return fs, nil
}

// normalizePath normalizes a path
func normalizePath(p string) string {
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// Remove trailing slash
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = p[:len(p)-1]
	}
	return p
}

// resolvePFSPath converts a URL path to a PFS path
func (fs *HTTPFS) resolvePFSPath(urlPath string) string {
	urlPath = normalizePath(urlPath)
	if urlPath == "/" {
		return fs.pfsPath
	}
	return path.Join(fs.pfsPath, urlPath)
}

// startHTTPServer starts the HTTP server
func (fs *HTTPFS) startHTTPServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", fs.handleHTTPRequest)

	fs.server = &http.Server{
		Addr:    ":" + fs.httpPort,
		Handler: mux,
	}

	go func() {
		log.Infof("[httpfs] Starting HTTP server on port %s, serving PFS path: %s", fs.httpPort, fs.pfsPath)
		log.Infof("[httpfs] HTTP server listening at http://localhost:%s", fs.httpPort)
		if err := fs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorf("[httpfs] HTTP server error on port %s: %v", fs.httpPort, err)
		} else if err == http.ErrServerClosed {
			log.Infof("[httpfs] HTTP server on port %s closed gracefully", fs.httpPort)
		}
	}()

	return nil
}

// handleHTTPRequest handles HTTP requests
func (fs *HTTPFS) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	pfsPath := fs.resolvePFSPath(urlPath)

	log.Infof("[httpfs:%s] %s %s (PFS path: %s) from %s", fs.httpPort, r.Method, urlPath, pfsPath, r.RemoteAddr)

	// Get file info
	info, err := fs.rootFS.Stat(pfsPath)
	if err != nil {
		log.Warnf("[httpfs:%s] Not found: %s (PFS: %s)", fs.httpPort, urlPath, pfsPath)
		http.NotFound(w, r)
		return
	}

	// If it's a directory, list contents
	if info.IsDir {
		fs.serveDirectory(w, r, pfsPath, urlPath)
		return
	}

	// Serve file
	fs.serveFile(w, r, pfsPath)
}

// serveFile serves a file
func (fs *HTTPFS) serveFile(w http.ResponseWriter, r *http.Request, pfsPath string) {
	// Get file info for headers
	info, err := fs.rootFS.Stat(pfsPath)
	if err != nil {
		http.Error(w, "Failed to stat file", http.StatusInternalServerError)
		log.Errorf("[httpfs:%s] Failed to stat file %s: %v", fs.httpPort, pfsPath, err)
		return
	}

	// Determine content type based on file extension
	contentType := getContentType(pfsPath)
	log.Infof("[httpfs:%s] Serving file: %s (size: %d bytes, type: %s)", fs.httpPort, pfsPath, info.Size, contentType)

	// Try to open file using Open method
	reader, err := fs.rootFS.Open(pfsPath)
	if err != nil {
		// Fallback: use Read method if Open is not supported
		log.Debugf("[httpfs:%s] Open failed for %s, falling back to Read: %v", fs.httpPort, pfsPath, err)
		data, err := fs.rootFS.Read(pfsPath, 0, -1)
		// EOF is expected when reading the entire file
		if err != nil && err != io.EOF {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			log.Errorf("[httpfs:%s] Failed to read file %s: %v", fs.httpPort, pfsPath, err)
			return
		}

		// Set headers
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Header().Set("Last-Modified", info.ModTime.Format(http.TimeFormat))

		// Write content
		w.Write(data)
		log.Infof("[httpfs:%s] Sent file: %s (%d bytes via Read)", fs.httpPort, pfsPath, len(data))
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	w.Header().Set("Last-Modified", info.ModTime.Format(http.TimeFormat))

	// Copy content
	written, _ := io.Copy(w, reader)
	log.Infof("[httpfs:%s] Sent file: %s (%d bytes via stream)", fs.httpPort, pfsPath, written)
}

// serveDirectory serves a directory listing
func (fs *HTTPFS) serveDirectory(w http.ResponseWriter, r *http.Request, pfsPath string, urlPath string) {
	entries, err := fs.rootFS.ReadDir(pfsPath)
	if err != nil {
		log.Errorf("[httpfs:%s] Failed to read directory %s: %v", fs.httpPort, pfsPath, err)
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}

	log.Infof("[httpfs:%s] Serving directory: %s (%d entries)", fs.httpPort, pfsPath, len(entries))

	// Sort entries: directories first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	// Build directory listing
	type FileEntry struct {
		Name    string
		IsDir   bool
		Size    int64
		ModTime string
		URL     string
	}

	var files []FileEntry
	for _, entry := range entries {
		name := entry.Name
		url := path.Join(urlPath, name)
		if entry.IsDir {
			name += "/"
			url += "/"
		}

		files = append(files, FileEntry{
			Name:    name,
			IsDir:   entry.IsDir,
			Size:    entry.Size,
			ModTime: entry.ModTime.Format("2006-01-02 15:04:05"),
			URL:     url,
		})
	}

	// Render HTML
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Directory listing for {{.Path}}</title>
    <style>
        body { font-family: monospace; margin: 20px; }
        h1 { border-bottom: 1px solid #ccc; padding-bottom: 10px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { text-align: left; padding: 8px; }
        tr:hover { background-color: #f5f5f5; }
        th { background-color: #e0e0e0; }
        a { text-decoration: none; color: #0066cc; }
        a:hover { text-decoration: underline; }
        .size { text-align: right; }
        .info { color: #666; font-style: italic; margin-top: 20px; }
    </style>
</head>
<body>
    <h1>Directory listing for {{.Path}}</h1>
    <hr>
    {{if .Parent}}
    <p><a href="{{.Parent}}">&#8593; Parent Directory</a></p>
    {{end}}
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th class="size">Size</th>
                <th>Modified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td><a href="{{.URL}}">{{.Name}}</a></td>
                <td class="size">{{if .IsDir}}-{{else}}{{.Size}}{{end}}</td>
                <td>{{.ModTime}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    <hr>
    <p class="info">pfs httpfs server - serving: {{.PFSPath}}</p>
</body>
</html>`

	t, err := template.New("directory").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	parent := ""
	if urlPath != "/" {
		// Clean the path to remove trailing slash before getting parent
		// This is important because path.Dir("/level1/") returns "/level1"
		// but path.Dir("/level1") returns "/"
		cleanPath := strings.TrimSuffix(urlPath, "/")
		parent = path.Dir(cleanPath)
		// Ensure parent path ends with / for proper directory navigation
		// But don't add extra / if already at root
		if parent != "/" && !strings.HasSuffix(parent, "/") {
			parent = parent + "/"
		}
	}

	data := struct {
		Path    string
		PFSPath string
		Parent  string
		Files   []FileEntry
	}{
		Path:    urlPath,
		PFSPath: pfsPath,
		Parent:  parent,
		Files:   files,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, data)
}

// FileSystem interface implementation - these are placeholder implementations
// since httpfs doesn't provide its own filesystem, it just serves another PFS path via HTTP

func (fs *HTTPFS) Create(path string) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Mkdir(path string, perm uint32) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Remove(path string) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) RemoveAll(path string) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Read(path string, offset int64, size int64) ([]byte, error) {
	// Check if this is the virtual status file
	if path == "/" || path == "" {
		// Return status information
		statusData := []byte(fs.getStatusInfo())

		// Handle offset and size
		if offset >= int64(len(statusData)) {
			return []byte{}, io.EOF
		}

		data := statusData[offset:]
		if size > 0 && int64(len(data)) > size {
			data = data[:size]
		}

		return data, nil
	}

	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Write(path string, data []byte) ([]byte, error) {
	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Stat(path string) (*filesystem.FileInfo, error) {
	// Check if this is the virtual status file
	if path == "/" || path == "" {
		statusData := fs.getStatusInfo()
		return &filesystem.FileInfo{
			Name:    "status",
			Size:    int64(len(statusData)),
			Mode:    0444, // Read-only
			ModTime: fs.startTime,
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: "httpfs-status",
				Type: "virtual",
			},
		}, nil
	}

	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) Open(path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

func (fs *HTTPFS) OpenWrite(path string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("httpfs is read-only via filesystem interface, use HTTP to access files")
}

// getStatusInfo returns the status information for this httpfs instance
func (fs *HTTPFS) getStatusInfo() string {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	uptime := time.Since(fs.startTime)

	status := fmt.Sprintf(`HTTPFS Instance Status
======================

Virtual Path:    %s
PFS Source Path: %s
HTTP Port:       %s
HTTP Endpoint:   http://localhost:%s

Server Status:   Running
Start Time:      %s
Uptime:          %s

Access this HTTP server:
  Browser:       http://localhost:%s/
  CLI:           curl http://localhost:%s/

Serving content from PFS path: %s
All files under %s are accessible via HTTP on port %s
`,
		fs.statusPath,
		fs.pfsPath,
		fs.httpPort,
		fs.httpPort,
		fs.startTime.Format("2006-01-02 15:04:05"),
		uptime.Round(time.Second).String(),
		fs.httpPort,
		fs.httpPort,
		fs.pfsPath,
		fs.pfsPath,
		fs.httpPort,
	)

	return status
}

// Shutdown stops the HTTP server
func (fs *HTTPFS) Shutdown() error {
	if fs.server != nil {
		log.Infof("[httpfs:%s] Shutting down HTTP server...", fs.httpPort)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := fs.server.Shutdown(ctx)
		if err != nil {
			log.Errorf("[httpfs:%s] Error during shutdown: %v", fs.httpPort, err)
		} else {
			log.Infof("[httpfs:%s] HTTP server shutdown complete", fs.httpPort)
		}
		return err
	}
	return nil
}

// HTTPFSPlugin wraps HTTPFS as a plugin
type HTTPFSPlugin struct {
	fs         *HTTPFS
	pfsPath    string
	httpPort   string
	statusPath string
	rootFS     filesystem.FileSystem
}

// NewHTTPFSPlugin creates a new HTTPFS plugin
func NewHTTPFSPlugin() *HTTPFSPlugin {
	return &HTTPFSPlugin{}
}

func (p *HTTPFSPlugin) Name() string {
	return PluginName
}

// SetRootFS sets the root filesystem reference
func (p *HTTPFSPlugin) SetRootFS(rootFS filesystem.FileSystem) {
	p.rootFS = rootFS
}

func (p *HTTPFSPlugin) Initialize(config map[string]interface{}) error {
	// Parse configuration
	pfsPath, ok := config["pfs_path"].(string)
	if !ok || pfsPath == "" {
		return fmt.Errorf("pfs_path is required in configuration")
	}

	p.pfsPath = pfsPath

	// Get HTTP port (optional, defaults to 8000)
	httpPort := "8000"
	if port, ok := config["http_port"].(string); ok && port != "" {
		httpPort = port
	}
	p.httpPort = httpPort

	// Get mount path (virtual status path)
	statusPath := "/"
	if mountPath, ok := config["mount_path"].(string); ok && mountPath != "" {
		statusPath = mountPath
	}
	p.statusPath = statusPath

	// Create HTTPFS instance if rootFS is available
	if p.rootFS != nil {
		fs, err := NewHTTPFS(p.pfsPath, p.httpPort, p.statusPath, p.rootFS)
		if err != nil {
			return fmt.Errorf("failed to initialize httpfs: %w", err)
		}
		p.fs = fs
		log.Infof("[httpfs] Initialized with PFS path: %s, HTTP server: http://localhost:%s, Status path: %s", pfsPath, httpPort, statusPath)
	} else {
		log.Infof("[httpfs] Configured to serve PFS path: %s on HTTP port: %s (will start after rootFS is available)", pfsPath, httpPort)
	}

	return nil
}

func (p *HTTPFSPlugin) GetFileSystem() filesystem.FileSystem {
	// Lazy initialization: create HTTPFS instance if not already created
	if p.fs == nil && p.rootFS != nil {
		fs, err := NewHTTPFS(p.pfsPath, p.httpPort, p.statusPath, p.rootFS)
		if err != nil {
			log.Errorf("[httpfs] Failed to initialize: %v", err)
			return nil
		}
		p.fs = fs
	}
	return p.fs
}

func (p *HTTPFSPlugin) GetReadme() string {
	readmeContent := fmt.Sprintf(`HTTPFS Plugin - HTTP File Server for PFS Paths

This plugin serves a PFS mount path over HTTP, similar to 'python3 -m http.server'.
Unlike serving local files, this exposes any PFS filesystem (memfs, queuefs, s3fs, etc.) via HTTP.

FEATURES:
  - Serve any PFS path via HTTP (e.g., /memfs, /queuefs, /s3fs)
  - Browse files and directories in web browser
  - Download files via HTTP
  - Pretty HTML directory listings
  - Access PFS virtual filesystems through HTTP
  - Read-only HTTP access (modifications should be done through PFS API)
  - Support for dynamic mounting via PFS Shell mount command

CONFIGURATION:

  Basic configuration:
  [plugins.httpfs]
  enabled = true
  path = "/httpfs"              # This is just a placeholder, not used for serving

    [plugins.httpfs.config]
    pfs_path = "/memfs"         # The PFS path to serve (e.g., /memfs, /queuefs)
    http_port = "8000"          # Optional, defaults to 8000

  Example - Serve memfs:
  [plugins.httpfs_mem]
  enabled = true
  path = "/httpfs_mem"

    [plugins.httpfs_mem.config]
    pfs_path = "/memfs"
    http_port = "9000"

  Example - Serve queuefs:
  [plugins.httpfs_queue]
  enabled = true
  path = "/httpfs_queue"

    [plugins.httpfs_queue.config]
    pfs_path = "/queuefs"
    http_port = "9001"

CURRENT CONFIGURATION:
  PFS Path: %s
  HTTP Server: http://localhost:%s

DYNAMIC MOUNTING:

  You can dynamically mount httpfs at runtime using PFS Shell:

  # In PFS Shell REPL:
  > mount httpfs /httpfs-demo pfs_path=/memfs http_port=10000
    plugin mounted

  > mounts
  httpfs on /httpfs-demo (plugin: httpfs, pfs_path=/memfs, http_port=10000)

  > unmount /httpfs-demo
  Unmounted plugin at /httpfs-demo

  # Via command line:
  pfs mount httpfs /httpfs-demo pfs_path=/memfs http_port=10000
  pfs unmount /httpfs-demo

  # Via REST API:
  curl -X POST http://localhost:8080/api/v1/mount \
       -H "Content-Type: application/json" \
       -d '{
         "fstype": "httpfs",
         "path": "/httpfs-demo",
         "config": {
           "pfs_path": "/memfs",
           "http_port": "10000"
         }
       }'

  Dynamic mounting advantages:
  - No server restart required
  - Mount/unmount on demand
  - Multiple instances with different configurations
  - Flexible port and path selection

USAGE:

  Via Web Browser:
    Open: http://localhost:%s
    Browse directories and download files from PFS

  Via curl:
    # List directory
    curl http://localhost:%s/

    # Download file
    curl http://localhost:%s/file.txt

    # Access subdirectory
    curl http://localhost:%s/subdir/

EXAMPLES:

  # Serve memfs on port 9000
  http://localhost:9000 -> shows contents of /memfs

  # Serve queuefs on port 9001
  http://localhost:9001 -> shows contents of /queuefs

  # Access files in browser
  Open http://localhost:%s in your browser
  Click on files to download
  Click on directories to browse

NOTES:
  - The HTTP server starts automatically when the plugin is initialized
  - Files are served with proper MIME types
  - Directory listings are formatted as pretty HTML
  - httpfs provides HTTP read-only access to PFS paths
  - To modify files, use the PFS API directly
  - Multiple httpfs instances can serve different PFS paths on different ports

USE CASES:
  - Expose in-memory files (memfs) via HTTP for easy access
  - Browse queue contents (queuefs) in a web browser
  - Share S3 files (s3fs) through a simple HTTP interface
  - Provide web access to any PFS filesystem
  - Quick file sharing without setting up separate web servers
  - Debug and inspect PFS filesystems visually

ADVANTAGES:
  - Works with any PFS filesystem (not just local files)
  - Simple HTTP interface for complex backends
  - Multiple instances can serve different paths
  - No data duplication - serves directly from PFS
  - Lightweight and fast

VERSION: 1.0.0
AUTHOR: PFS Server
`, p.pfsPath, p.httpPort, p.httpPort, p.httpPort, p.httpPort, p.httpPort, p.httpPort)

	return readmeContent
}

func (p *HTTPFSPlugin) Shutdown() error {
	log.Infof("[httpfs] Plugin shutting down (port: %s, path: %s)", p.httpPort, p.pfsPath)
	if p.fs != nil {
		return p.fs.Shutdown()
	}
	return nil
}

// Ensure HTTPFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*HTTPFSPlugin)(nil)
var _ filesystem.FileSystem = (*HTTPFS)(nil)
