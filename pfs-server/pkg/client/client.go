package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
)

// Client is a Go client for PFS HTTP API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new PFS client
// baseURL should include the API version path, e.g., "http://localhost:8080/api/v1"
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NewClientWithHTTPClient creates a new PFS client with custom HTTP client
func NewClientWithHTTPClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a success response from the API
type SuccessResponse struct {
	Message string `json:"message"`
}

// FileInfoResponse represents file info response from the API
type FileInfoResponse struct {
	Name    string              `json:"name"`
	Size    int64               `json:"size"`
	Mode    uint32              `json:"mode"`
	ModTime string              `json:"modTime"`
	IsDir   bool                `json:"isDir"`
	Meta    filesystem.MetaData `json:"meta,omitempty"`
}

// ListResponse represents directory listing response from the API
type ListResponse struct {
	Files []FileInfoResponse `json:"files"`
}

// RenameRequest represents a rename request
type RenameRequest struct {
	NewPath string `json:"newPath"`
}

// ChmodRequest represents a chmod request
type ChmodRequest struct {
	Mode uint32 `json:"mode"`
}

func (c *Client) doRequest(method, endpoint string, query url.Values, body io.Reader) (*http.Response, error) {
	u := c.baseURL + endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		return fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
	}

	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
}

// Create creates a new file
func (c *Client) Create(path string) error {
	query := url.Values{}
	query.Set("path", path)

	resp, err := c.doRequest(http.MethodPost, "/files", query, nil)
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// Mkdir creates a new directory
func (c *Client) Mkdir(path string, perm uint32) error {
	query := url.Values{}
	query.Set("path", path)
	query.Set("mode", fmt.Sprintf("%o", perm))

	resp, err := c.doRequest(http.MethodPost, "/directories", query, nil)
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// Remove removes a file or empty directory
func (c *Client) Remove(path string) error {
	query := url.Values{}
	query.Set("path", path)
	query.Set("recursive", "false")

	resp, err := c.doRequest(http.MethodDelete, "/files", query, nil)
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// RemoveAll removes a path and any children it contains
func (c *Client) RemoveAll(path string) error {
	query := url.Values{}
	query.Set("path", path)
	query.Set("recursive", "true")

	resp, err := c.doRequest(http.MethodDelete, "/files", query, nil)
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// Read reads file content with optional offset and size
// offset: starting position (0 means from beginning)
// size: number of bytes to read (-1 means read all)
// Returns io.EOF if offset+size >= file size (reached end of file)
func (c *Client) Read(path string, offset int64, size int64) ([]byte, error) {
	query := url.Values{}
	query.Set("path", path)
	if offset > 0 {
		query.Set("offset", fmt.Sprintf("%d", offset))
	}
	if size >= 0 {
		query.Set("size", fmt.Sprintf("%d", size))
	}

	resp, err := c.doRequest(http.MethodGet, "/files", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// Write writes data to a file, creating it if necessary
func (c *Client) Write(path string, data []byte) ([]byte, error) {
	query := url.Values{}
	query.Set("path", path)

	resp, err := c.doRequest(http.MethodPut, "/files", query, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	var successResp SuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&successResp); err != nil {
		return nil, fmt.Errorf("failed to decode success response: %w", err)
	}

	return []byte(successResp.Message), nil
}

// ReadDir lists the contents of a directory
func (c *Client) ReadDir(path string) ([]filesystem.FileInfo, error) {
	query := url.Values{}
	query.Set("path", path)

	resp, err := c.doRequest(http.MethodGet, "/directories", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	files := make([]filesystem.FileInfo, 0, len(listResp.Files))
	for _, f := range listResp.Files {
		modTime, _ := time.Parse(time.RFC3339Nano, f.ModTime)
		files = append(files, filesystem.FileInfo{
			Name:    f.Name,
			Size:    f.Size,
			Mode:    f.Mode,
			ModTime: modTime,
			IsDir:   f.IsDir,
			Meta:    f.Meta,
		})
	}

	return files, nil
}

// Stat returns file information
func (c *Client) Stat(path string) (*filesystem.FileInfo, error) {
	query := url.Values{}
	query.Set("path", path)

	resp, err := c.doRequest(http.MethodGet, "/stat", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	var fileInfo FileInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileInfo); err != nil {
		return nil, fmt.Errorf("failed to decode file info response: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339Nano, fileInfo.ModTime)

	return &filesystem.FileInfo{
		Name:    fileInfo.Name,
		Size:    fileInfo.Size,
		Mode:    fileInfo.Mode,
		ModTime: modTime,
		IsDir:   fileInfo.IsDir,
		Meta:    fileInfo.Meta,
	}, nil
}

// Rename renames/moves a file or directory
func (c *Client) Rename(oldPath, newPath string) error {
	query := url.Values{}
	query.Set("path", oldPath)

	reqBody := RenameRequest{NewPath: newPath}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal rename request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/rename", query, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// Chmod changes file permissions
func (c *Client) Chmod(path string, mode uint32) error {
	query := url.Values{}
	query.Set("path", path)

	reqBody := ChmodRequest{Mode: mode}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal chmod request: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/chmod", query, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}

	return c.handleErrorResponse(resp)
}

// Health checks the health of the PFS server
func (c *Client) Health() error {
	resp, err := c.doRequest(http.MethodGet, "/health", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ReadStream opens a streaming connection to read from a file
// Returns an io.ReadCloser that streams data from the server
// The caller is responsible for closing the reader
func (c *Client) ReadStream(path string) (io.ReadCloser, error) {
	query := url.Values{}
	query.Set("path", path)
	query.Set("stream", "true") // Enable streaming mode

	// Create request with no timeout for streaming
	streamClient := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	reqURL := fmt.Sprintf("%s/files?%s", c.baseURL, query.Encode())
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to decode error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, errResp.Error)
	}

	// Return the response body as a ReadCloser
	// Caller must close it when done
	return resp.Body, nil
}
