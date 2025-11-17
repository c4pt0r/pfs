package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/mountablefs"
	log "github.com/sirupsen/logrus"
)

// PluginHandler handles plugin management operations
type PluginHandler struct {
	mfs *mountablefs.MountableFS
}

// NewPluginHandler creates a new plugin handler
func NewPluginHandler(mfs *mountablefs.MountableFS) *PluginHandler {
	return &PluginHandler{mfs: mfs}
}

// MountInfo represents information about a mounted plugin
type MountInfo struct {
	Path       string                 `json:"path"`
	PluginName string                 `json:"pluginName"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// ListMountsResponse represents the response for listing mounts
type ListMountsResponse struct {
	Mounts []MountInfo `json:"mounts"`
}

// ListMounts handles GET /mounts
func (ph *PluginHandler) ListMounts(w http.ResponseWriter, r *http.Request) {
	mounts := ph.mfs.GetMounts()

	var mountInfos []MountInfo
	for _, mount := range mounts {
		mountInfos = append(mountInfos, MountInfo{
			Path:       mount.Path,
			PluginName: mount.Plugin.Name(),
			Config:     mount.Config,
		})
	}

	writeJSON(w, http.StatusOK, ListMountsResponse{Mounts: mountInfos})
}

// UnmountRequest represents an unmount request
type UnmountRequest struct {
	Path string `json:"path"`
}

// Unmount handles POST /unmount
func (ph *PluginHandler) Unmount(w http.ResponseWriter, r *http.Request) {
	var req UnmountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := ph.mfs.Unmount(req.Path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "plugin unmounted"})
}

// MountRequest represents a mount request
type MountRequest struct {
	FSType string                 `json:"fstype"`
	Path   string                 `json:"path"`
	Config map[string]interface{} `json:"config"`
}

// Mount handles POST /mount
func (ph *PluginHandler) Mount(w http.ResponseWriter, r *http.Request) {
	var req MountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.FSType == "" {
		writeError(w, http.StatusBadRequest, "fstype is required")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	if err := ph.mfs.MountPlugin(req.FSType, req.Path, req.Config); err != nil {
		// First check for typed errors
		if errors.Is(err, filesystem.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		// For backward compatibility, check string-based errors that aren't typed yet
		errMsg := err.Error()
		if strings.Contains(errMsg, "unknown filesystem type") || strings.Contains(errMsg, "unknown plugin") ||
			strings.Contains(errMsg, "failed to validate") || strings.Contains(errMsg, "is required") ||
			strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "unknown configuration parameter") {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "plugin mounted"})
}


// LoadPluginRequest represents a request to load an external plugin
type LoadPluginRequest struct {
	LibraryPath string `json:"library_path"`
}

// LoadPluginResponse represents the response for loading a plugin
type LoadPluginResponse struct {
	Message      string `json:"message"`
	PluginName   string `json:"plugin_name"`
	OriginalName string `json:"original_name,omitempty"`
	Renamed      bool   `json:"renamed"`
}

// isHTTPURL checks if a string is an HTTP or HTTPS URL
func isHTTPURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// isAGFSPath checks if a string is a AGFS path (agfs://)
func isAGFSPath(path string) bool {
	return strings.HasPrefix(path, "agfs://")
}

// downloadPluginFromURL downloads a plugin from an HTTP(S) URL to a temporary file
func downloadPluginFromURL(url string) (string, error) {
	log.Infof("Downloading plugin from URL: %s", url)

	// Create HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download from URL: HTTP %d", resp.StatusCode)
	}

	// Determine file extension from URL
	ext := filepath.Ext(url)
	if ext == "" {
		// Default to .so if no extension
		ext = ".so"
	}

	// Create a hash of the URL to use as the filename
	hash := sha256.Sum256([]byte(url))
	hashStr := hex.EncodeToString(hash[:])[:16]

	// Create temporary file with appropriate extension
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("agfs-plugin-%s%s", hashStr, ext))

	// Create the file
	outFile, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer outFile.Close()

	// Copy the downloaded content to the file
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("failed to write downloaded content: %w", err)
	}

	log.Infof("Downloaded plugin to temporary file: %s (%d bytes)", tmpFile, written)
	return tmpFile, nil
}

// readPluginFromAGFS reads a plugin from a AGFS path (agfs://...) to a temporary file
func (ph *PluginHandler) readPluginFromAGFS(agfsPath string) (string, error) {
	// Remove agfs:// prefix to get the actual path
	path := strings.TrimPrefix(agfsPath, "agfs://")
	if path == "" || path == "/" {
		return "", fmt.Errorf("invalid agfs path: %s", agfsPath)
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	log.Infof("Reading plugin from AGFS path: %s", path)

	// Read file from the mountable filesystem
	data, err := ph.mfs.Read(path, 0, -1)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read from AGFS path %s: %w", path, err)
	}

	// Determine file extension from path
	ext := filepath.Ext(path)
	if ext == "" {
		// Default to .so if no extension
		ext = ".so"
	}

	// Create a hash of the path to use as the filename
	hash := sha256.Sum256([]byte(agfsPath))
	hashStr := hex.EncodeToString(hash[:])[:16]

	// Create temporary file with appropriate extension
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("agfs-plugin-%s%s", hashStr, ext))

	// Write the data to the temporary file
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write temporary file: %w", err)
	}

	log.Infof("Read plugin from AGFS to temporary file: %s (%d bytes)", tmpFile, len(data))
	return tmpFile, nil
}

// LoadPlugin handles POST /plugins/load
func (ph *PluginHandler) LoadPlugin(w http.ResponseWriter, r *http.Request) {
	var req LoadPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.LibraryPath == "" {
		writeError(w, http.StatusBadRequest, "library_path is required")
		return
	}

	// Check if the library path is an HTTP(S) URL or AGFS path
	libraryPath := req.LibraryPath
	var tmpFile string
	if isHTTPURL(libraryPath) {
		// Download the plugin from the URL
		downloadedFile, err := downloadPluginFromURL(libraryPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to download plugin: %v", err))
			return
		}
		tmpFile = downloadedFile
		libraryPath = downloadedFile
		log.Infof("Using downloaded plugin from temporary file: %s", libraryPath)
	} else if isAGFSPath(libraryPath) {
		// Read the plugin from AGFS
		agfsFile, err := ph.readPluginFromAGFS(libraryPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read plugin from AGFS: %v", err))
			return
		}
		tmpFile = agfsFile
		libraryPath = agfsFile
		log.Infof("Using plugin from AGFS temporary file: %s", libraryPath)
	}

	plugin, err := ph.mfs.LoadExternalPlugin(libraryPath)
	if err != nil {
		// Clean up temporary file if it was downloaded
		if tmpFile != "" {
			os.Remove(tmpFile)
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Check if plugin was renamed
	response := LoadPluginResponse{
		Message:    "plugin loaded successfully",
		PluginName: plugin.Name(),
		Renamed:    false,
	}

	if renamedPlugin, ok := plugin.(*mountablefs.RenamedPlugin); ok {
		response.OriginalName = renamedPlugin.OriginalName()
		response.Renamed = true
	}

	writeJSON(w, http.StatusOK, response)
}

// UnloadPluginRequest represents a request to unload an external plugin
type UnloadPluginRequest struct {
	LibraryPath string `json:"library_path"`
}

// UnloadPlugin handles POST /plugins/unload
func (ph *PluginHandler) UnloadPlugin(w http.ResponseWriter, r *http.Request) {
	var req UnloadPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.LibraryPath == "" {
		writeError(w, http.StatusBadRequest, "library_path is required")
		return
	}

	if err := ph.mfs.UnloadExternalPlugin(req.LibraryPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "plugin unloaded successfully"})
}

// ListPluginsResponse represents the response for listing plugins
type ListPluginsResponse struct {
	LoadedPlugins []string `json:"loaded_plugins"`
}

// ListPlugins handles GET /plugins
func (ph *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := ph.mfs.GetLoadedExternalPlugins()
	writeJSON(w, http.StatusOK, ListPluginsResponse{LoadedPlugins: plugins})
}

// SetupRoutes sets up plugin management routes with /api/v1 prefix
func (ph *PluginHandler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/mounts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.ListMounts(w, r)
	})

	mux.HandleFunc("/api/v1/mount", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.Mount(w, r)
	})

	mux.HandleFunc("/api/v1/unmount", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.Unmount(w, r)
	})

	// External plugin management endpoints
	mux.HandleFunc("/api/v1/plugins", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.ListPlugins(w, r)
	})

	mux.HandleFunc("/api/v1/plugins/load", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.LoadPlugin(w, r)
	})

	mux.HandleFunc("/api/v1/plugins/unload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		ph.UnloadPlugin(w, r)
	})
}
