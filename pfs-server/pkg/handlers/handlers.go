package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	log "github.com/sirupsen/logrus"
)

// Handler wraps the FileSystem and provides HTTP handlers
type Handler struct {
	fs         filesystem.FileSystem
	version    string
	gitCommit  string
	buildTime  string
}

// NewHandler creates a new Handler
func NewHandler(fs filesystem.FileSystem) *Handler {
	return &Handler{
		fs:        fs,
		version:   "dev",
		gitCommit: "unknown",
		buildTime: "unknown",
	}
}

// SetVersionInfo sets the version information for the handler
func (h *Handler) SetVersionInfo(version, gitCommit, buildTime string) {
	h.version = version
	h.gitCommit = gitCommit
	h.buildTime = buildTime
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string `json:"message"`
}

// FileInfoResponse represents file info response
type FileInfoResponse struct {
	Name    string                `json:"name"`
	Size    int64                 `json:"size"`
	Mode    uint32                `json:"mode"`
	ModTime string                `json:"modTime"`
	IsDir   bool                  `json:"isDir"`
	Meta    filesystem.MetaData   `json:"meta,omitempty"` // Structured metadata
}

// ListResponse represents directory listing response
type ListResponse struct {
	Files []FileInfoResponse `json:"files"`
}

// WriteRequest represents a write request
type WriteRequest struct {
	Data string `json:"data"`
}

// RenameRequest represents a rename request
type RenameRequest struct {
	NewPath string `json:"newPath"`
}

// ChmodRequest represents a chmod request
type ChmodRequest struct {
	Mode uint32 `json:"mode"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

// mapErrorToStatus maps filesystem errors to HTTP status codes
func mapErrorToStatus(err error) int {
	if errors.Is(err, filesystem.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, filesystem.ErrPermissionDenied) {
		return http.StatusForbidden
	}
	if errors.Is(err, filesystem.ErrInvalidArgument) {
		return http.StatusBadRequest
	}
	if errors.Is(err, filesystem.ErrAlreadyExists) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// CreateFile handles POST /files?path=<path>
func (h *Handler) CreateFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	if err := h.fs.Create(path); err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, SuccessResponse{Message: "file created"})
}

// CreateDirectory handles POST /directories?path=<path>&mode=<mode>
func (h *Handler) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	modeStr := r.URL.Query().Get("mode")
	mode := uint32(0755)
	if modeStr != "" {
		m, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid mode")
			return
		}
		mode = uint32(m)
	}

	if err := h.fs.Mkdir(path, mode); err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, SuccessResponse{Message: "directory created"})
}

// ReadFile handles GET /files?path=<path>&offset=<offset>&size=<size>&stream=<true|false>
func (h *Handler) ReadFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	// Check if streaming mode is requested
	stream := r.URL.Query().Get("stream") == "true"
	if stream {
		h.streamFile(w, r, path)
		return
	}

	// Parse offset and size parameters
	offset := int64(0)
	size := int64(-1) // -1 means read all

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			offset = parsedOffset
		} else {
			writeError(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
	}

	if sizeStr := r.URL.Query().Get("size"); sizeStr != "" {
		if parsedSize, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			size = parsedSize
		} else {
			writeError(w, http.StatusBadRequest, "invalid size parameter")
			return
		}
	}

	data, err := h.fs.Read(path, offset, size)
	if err != nil {
		// Check if it's EOF (reached end of file)
		if err == io.EOF {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			w.Write(data) // Return partial data with 200 OK
			return
		}
		// Map error to appropriate HTTP status code
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// WriteFile handles PUT /files?path=<path>
func (h *Handler) WriteFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	response, err := h.fs.Write(path, data)
	if err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	// Return the custom message from the filesystem
	writeJSON(w, http.StatusOK, SuccessResponse{Message: string(response)})
}

// Delete handles DELETE /files?path=<path>&recursive=<true|false>
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	recursive := r.URL.Query().Get("recursive") == "true"

	var err error
	if recursive {
		err = h.fs.RemoveAll(path)
	} else {
		err = h.fs.Remove(path)
	}

	if err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "deleted"})
}

// ListDirectory handles GET /directories?path=<path>
func (h *Handler) ListDirectory(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	files, err := h.fs.ReadDir(path)
	if err != nil {
		// Map error to appropriate HTTP status code
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	var response ListResponse
	for _, f := range files {
		response.Files = append(response.Files, FileInfoResponse{
			Name:    f.Name,
			Size:    f.Size,
			Mode:    f.Mode,
			ModTime: f.ModTime.Format(time.RFC3339Nano),
			IsDir:   f.IsDir,
			Meta:    f.Meta,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

// Stat handles GET /stat?path=<path>
func (h *Handler) Stat(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	info, err := h.fs.Stat(path)
	if err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	response := FileInfoResponse{
		Name:    info.Name,
		Size:    info.Size,
		Mode:    info.Mode,
		ModTime: info.ModTime.Format(time.RFC3339Nano),
		IsDir:   info.IsDir,
		Meta:    info.Meta,
	}

	writeJSON(w, http.StatusOK, response)
}

// Rename handles POST /rename?path=<path>
func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.NewPath == "" {
		writeError(w, http.StatusBadRequest, "newPath is required")
		return
	}

	if err := h.fs.Rename(path, req.NewPath); err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "renamed"})
}

// Chmod handles POST /chmod?path=<path>
func (h *Handler) Chmod(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path parameter is required")
		return
	}

	var req ChmodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.fs.Chmod(path, req.Mode); err != nil {
		status := mapErrorToStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Message: "permissions changed"})
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildTime string `json:"buildTime"`
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		GitCommit: h.gitCommit,
		BuildTime: h.buildTime,
	}
	writeJSON(w, http.StatusOK, response)
}

// SetupRoutes sets up all HTTP routes with /api/v1 prefix
func (h *Handler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/health", h.Health)
	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateFile(w, r)
		case http.MethodGet:
			h.ReadFile(w, r)
		case http.MethodPut:
			h.WriteFile(w, r)
		case http.MethodDelete:
			h.Delete(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/directories", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateDirectory(w, r)
		case http.MethodGet:
			h.ListDirectory(w, r)
		case http.MethodDelete:
			h.Delete(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	mux.HandleFunc("/api/v1/stat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.Stat(w, r)
	})
	mux.HandleFunc("/api/v1/rename", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.Rename(w, r)
	})
	mux.HandleFunc("/api/v1/chmod", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.Chmod(w, r)
	})
}

// streamFile handles streaming file reads with HTTP chunked transfer encoding
func (h *Handler) streamFile(w http.ResponseWriter, r *http.Request, path string) {
	// Check if filesystem supports streaming
	streamer, ok := h.fs.(filesystem.Streamer)
	if !ok {
		writeError(w, http.StatusBadRequest, "streaming not supported for this filesystem")
		return
	}

	// Open stream for reading
	reader, err := streamer.OpenStream(path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer reader.Close()

	// Stream data to client
	h.streamFromStreamReader(w, r, reader)
}

// streamFromStreamReader streams data from a filesystem.StreamReader using chunked transfer
func (h *Handler) streamFromStreamReader(w http.ResponseWriter, r *http.Request, reader filesystem.StreamReader) {
	// Set headers for chunked transfer
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("ResponseWriter does not support flushing")
		return
	}

	log.Debugf("Starting stream read")

	// Read timeout for each chunk
	timeout := 30 * time.Second

	for {
		// Check if client disconnected
		select {
		case <-r.Context().Done():
			log.Infof("Client disconnected from stream")
			return
		default:
		}

		// Read next chunk from stream (blocking until data available)
		chunk, eof, err := reader.ReadChunk(timeout)

		if err != nil {
			if err == io.EOF {
				log.Infof("Stream closed (EOF)")
				return
			}
			if err.Error() == "read timeout" {
				// Timeout - stream is idle, continue waiting instead of closing
				log.Debugf("Stream read timeout, continuing to wait...")
				continue
			}
			log.Errorf("Error reading from stream: %v", err)
			return
		}

		if len(chunk) > 0 {
			// Write chunk to response in smaller pieces to avoid overwhelming the client
			maxChunkSize := 64 * 1024 // 64KB at a time
			offset := 0

			for offset < len(chunk) {
				// Check if client disconnected
				select {
				case <-r.Context().Done():
					log.Infof("Client disconnected while writing chunk")
					return
				default:
				}
				end := offset + maxChunkSize
				if end > len(chunk) {
					end = len(chunk)
				}
				n, writeErr := w.Write(chunk[offset:end])
				if writeErr != nil {
					log.Debugf("Error writing chunk: %v (this is normal if client disconnected)", writeErr)
					return
				}
				offset += n
				// Flush after each piece
				flusher.Flush()
			}
		}
		if eof {
			log.Infof("Stream completed (EOF)")
			return
		}
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.URL.RawQuery != "" {
			path += "?" + r.URL.RawQuery
		}
		log.Debugf("%s %s", r.Method, path)
		next.ServeHTTP(w, r)
	})
}
