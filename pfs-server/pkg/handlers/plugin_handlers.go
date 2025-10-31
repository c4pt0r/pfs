package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/c4pt0r/pfs/pfs-server/pkg/mountablefs"
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
		// Return appropriate status codes based on error type
		errMsg := err.Error()
		if strings.Contains(errMsg, "already has a mount") || strings.Contains(errMsg, "already mounted") {
			writeError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(errMsg, "unknown filesystem type") || strings.Contains(errMsg, "unknown plugin") {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "plugin mounted"})
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
}
