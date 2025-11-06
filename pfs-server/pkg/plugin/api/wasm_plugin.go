package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	log "github.com/sirupsen/logrus"
	wazeroapi "github.com/tetratelabs/wazero/api"
)

// WASMPlugin represents a plugin loaded from a WASM module
type WASMPlugin struct {
	ctx        context.Context
	module     wazeroapi.Module
	name       string
	fileSystem *WASMFileSystem
}

// WASMFileSystem implements filesystem.FileSystem by delegating to WASM functions
type WASMFileSystem struct {
	ctx    context.Context
	module wazeroapi.Module
}

// NewWASMPlugin creates a new WASM plugin wrapper
func NewWASMPlugin(ctx context.Context, module wazeroapi.Module) (*WASMPlugin, error) {
	// Verify required functions exist
	if module.ExportedFunction("plugin_new") == nil {
		return nil, fmt.Errorf("WASM module missing required function: plugin_new")
	}

	// Call plugin_new to initialize the plugin
	results, err := module.ExportedFunction("plugin_new").Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call plugin_new: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("plugin_new returned no results")
	}

	// Get plugin name
	name := "wasm-plugin"
	if nameFunc := module.ExportedFunction("plugin_name"); nameFunc != nil {
		if nameResults, err := nameFunc.Call(ctx); err == nil && len(nameResults) > 0 {
			// Read string from memory
			if nameStr, ok := readStringFromMemory(module, uint32(nameResults[0])); ok {
				name = nameStr
			}
		}
	}

	wp := &WASMPlugin{
		ctx:    ctx,
		module: module,
		name:   name,
		fileSystem: &WASMFileSystem{
			ctx:    ctx,
			module: module,
		},
	}

	return wp, nil
}

// Name returns the plugin name
func (wp *WASMPlugin) Name() string {
	return wp.name
}

// Validate validates the plugin configuration
func (wp *WASMPlugin) Validate(config map[string]interface{}) error {
	validateFunc := wp.module.ExportedFunction("plugin_validate")
	if validateFunc == nil {
		// If validate function is not exported, assume validation passes
		return nil
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config to WASM memory
	configPtr, err := writeStringToMemory(wp.module, string(configJSON))
	if err != nil {
		return fmt.Errorf("failed to write config to memory: %w", err)
	}

	// Call validate function
	results, err := validateFunc.Call(wp.ctx, uint64(configPtr))
	if err != nil {
		return fmt.Errorf("validate call failed: %w", err)
	}

	// Check for error return (non-zero means error)
	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wp.module, uint32(results[0])); ok {
			return fmt.Errorf("validation failed: %s", errMsg)
		}
		return fmt.Errorf("validation failed")
	}

	return nil
}

// Initialize initializes the plugin with configuration
func (wp *WASMPlugin) Initialize(config map[string]interface{}) error {
	initFunc := wp.module.ExportedFunction("plugin_initialize")
	if initFunc == nil {
		// If initialize function is not exported, assume initialization succeeds
		return nil
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config to WASM memory
	configPtr, err := writeStringToMemory(wp.module, string(configJSON))
	if err != nil {
		return fmt.Errorf("failed to write config to memory: %w", err)
	}

	// Call initialize function
	results, err := initFunc.Call(wp.ctx, uint64(configPtr))
	if err != nil {
		return fmt.Errorf("initialize call failed: %w", err)
	}

	// Check for error return
	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wp.module, uint32(results[0])); ok {
			return fmt.Errorf("initialization failed: %s", errMsg)
		}
		return fmt.Errorf("initialization failed")
	}

	return nil
}

// GetFileSystem returns the file system implementation
func (wp *WASMPlugin) GetFileSystem() filesystem.FileSystem {
	return wp.fileSystem
}

// GetReadme returns the plugin README
func (wp *WASMPlugin) GetReadme() string {
	readmeFunc := wp.module.ExportedFunction("plugin_get_readme")
	if readmeFunc == nil {
		return ""
	}

	results, err := readmeFunc.Call(wp.ctx)
	if err != nil {
		log.Warnf("Failed to get readme: %v", err)
		return ""
	}

	if len(results) > 0 {
		if readme, ok := readStringFromMemory(wp.module, uint32(results[0])); ok {
			return readme
		}
	}

	return ""
}

// Shutdown shuts down the plugin
func (wp *WASMPlugin) Shutdown() error {
	shutdownFunc := wp.module.ExportedFunction("plugin_shutdown")
	if shutdownFunc == nil {
		return nil
	}

	results, err := shutdownFunc.Call(wp.ctx)
	if err != nil {
		return fmt.Errorf("shutdown call failed: %w", err)
	}

	// Check for error return
	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wp.module, uint32(results[0])); ok {
			return fmt.Errorf("shutdown failed: %s", errMsg)
		}
		return fmt.Errorf("shutdown failed")
	}

	return nil
}

// WASMFileSystem implementations

func (wfs *WASMFileSystem) Create(path string) error {
	createFunc := wfs.module.ExportedFunction("fs_create")
	if createFunc == nil {
		return fmt.Errorf("fs_create not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return err
	}

	results, err := createFunc.Call(wfs.ctx, uint64(pathPtr))
	if err != nil {
		return fmt.Errorf("fs_create failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("create failed")
	}

	return nil
}

func (wfs *WASMFileSystem) Mkdir(path string, perm uint32) error {
	mkdirFunc := wfs.module.ExportedFunction("fs_mkdir")
	if mkdirFunc == nil {
		return fmt.Errorf("fs_mkdir not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return err
	}

	results, err := mkdirFunc.Call(wfs.ctx, uint64(pathPtr), uint64(perm))
	if err != nil {
		return fmt.Errorf("fs_mkdir failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("mkdir failed")
	}

	return nil
}

func (wfs *WASMFileSystem) Remove(path string) error {
	removeFunc := wfs.module.ExportedFunction("fs_remove")
	if removeFunc == nil {
		return fmt.Errorf("fs_remove not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return err
	}

	results, err := removeFunc.Call(wfs.ctx, uint64(pathPtr))
	if err != nil {
		return fmt.Errorf("fs_remove failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("remove failed")
	}

	return nil
}

func (wfs *WASMFileSystem) RemoveAll(path string) error {
	removeAllFunc := wfs.module.ExportedFunction("fs_remove_all")
	if removeAllFunc == nil {
		// Fall back to Remove if RemoveAll not implemented
		return wfs.Remove(path)
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return err
	}

	results, err := removeAllFunc.Call(wfs.ctx, uint64(pathPtr))
	if err != nil {
		return fmt.Errorf("fs_remove_all failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("remove_all failed")
	}

	return nil
}

func (wfs *WASMFileSystem) Read(path string, offset int64, size int64) ([]byte, error) {
	readFunc := wfs.module.ExportedFunction("fs_read")
	if readFunc == nil {
		return nil, fmt.Errorf("fs_read not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return nil, err
	}

	results, err := readFunc.Call(wfs.ctx, uint64(pathPtr), uint64(offset), uint64(size))
	if err != nil {
		return nil, fmt.Errorf("fs_read failed: %w", err)
	}

	if len(results) < 1 {
		return nil, fmt.Errorf("fs_read returned invalid results")
	}

	// Unpack u64: lower 32 bits = pointer, upper 32 bits = size
	packed := results[0]
	dataPtr := uint32(packed & 0xFFFFFFFF)
	dataSize := uint32((packed >> 32) & 0xFFFFFFFF)

	if dataPtr == 0 {
		return nil, fmt.Errorf("read failed")
	}

	data, ok := wfs.module.Memory().Read(dataPtr, dataSize)
	if !ok {
		return nil, fmt.Errorf("failed to read data from memory")
	}

	return data, nil
}

func (wfs *WASMFileSystem) Write(path string, data []byte) ([]byte, error) {
	writeFunc := wfs.module.ExportedFunction("fs_write")
	if writeFunc == nil {
		return nil, fmt.Errorf("fs_write not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return nil, err
	}

	dataPtr, err := writeBytesToMemory(wfs.module, data)
	if err != nil {
		return nil, err
	}

	results, err := writeFunc.Call(wfs.ctx, uint64(pathPtr), uint64(dataPtr), uint64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("fs_write failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("write failed")
	}

	return []byte(""), nil
}

func (wfs *WASMFileSystem) ReadDir(path string) ([]filesystem.FileInfo, error) {
	readDirFunc := wfs.module.ExportedFunction("fs_readdir")
	if readDirFunc == nil {
		return nil, fmt.Errorf("fs_readdir not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return nil, err
	}

	results, err := readDirFunc.Call(wfs.ctx, uint64(pathPtr))
	if err != nil {
		return nil, fmt.Errorf("fs_readdir failed: %w", err)
	}

	if len(results) < 1 {
		return nil, fmt.Errorf("fs_readdir returned invalid results")
	}

	// Unpack u64: lower 32 bits = json pointer, upper 32 bits = error pointer
	packed := results[0]
	jsonPtr := uint32(packed & 0xFFFFFFFF)
	errPtr := uint32((packed >> 32) & 0xFFFFFFFF)

	// Check for error
	if errPtr != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, errPtr); ok {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("readdir failed")
	}

	if jsonPtr == 0 {
		return []filesystem.FileInfo{}, nil
	}

	jsonStr, ok := readStringFromMemory(wfs.module, jsonPtr)
	if !ok {
		return nil, fmt.Errorf("failed to read readdir result")
	}

	var fileInfos []filesystem.FileInfo
	if err := json.Unmarshal([]byte(jsonStr), &fileInfos); err != nil {
		return nil, fmt.Errorf("failed to unmarshal readdir result: %w", err)
	}

	return fileInfos, nil
}

func (wfs *WASMFileSystem) Stat(path string) (*filesystem.FileInfo, error) {
	log.Debugf("WASM Stat called with path: %s", path)
	statFunc := wfs.module.ExportedFunction("fs_stat")
	if statFunc == nil {
		return nil, fmt.Errorf("fs_stat not implemented")
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		log.Errorf("Failed to write path to memory: %v", err)
		return nil, err
	}

	log.Debugf("Calling fs_stat WASM function with pathPtr=%d", pathPtr)
	results, err := statFunc.Call(wfs.ctx, uint64(pathPtr))
	if err != nil {
		log.Errorf("fs_stat WASM call failed: %v", err)
		return nil, fmt.Errorf("fs_stat failed: %w", err)
	}
	log.Debugf("fs_stat returned %d results", len(results))

	if len(results) < 1 {
		return nil, fmt.Errorf("fs_stat returned invalid results")
	}

	// Unpack u64: lower 32 bits = json pointer, upper 32 bits = error pointer
	packed := results[0]
	jsonPtr := uint32(packed & 0xFFFFFFFF)
	errPtr := uint32((packed >> 32) & 0xFFFFFFFF)

	// Check for error
	if errPtr != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, errPtr); ok {
			return nil, fmt.Errorf("%s", errMsg)
		}
		return nil, fmt.Errorf("stat failed")
	}

	if jsonPtr == 0 {
		return nil, fmt.Errorf("stat returned null")
	}

	jsonStr, ok := readStringFromMemory(wfs.module, jsonPtr)
	if !ok {
		return nil, fmt.Errorf("failed to read stat result")
	}

	var fileInfo filesystem.FileInfo
	if err := json.Unmarshal([]byte(jsonStr), &fileInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stat result: %w", err)
	}

	return &fileInfo, nil
}

func (wfs *WASMFileSystem) Rename(oldPath, newPath string) error {
	renameFunc := wfs.module.ExportedFunction("fs_rename")
	if renameFunc == nil {
		return fmt.Errorf("fs_rename not implemented")
	}

	oldPathPtr, err := writeStringToMemory(wfs.module, oldPath)
	if err != nil {
		return err
	}

	newPathPtr, err := writeStringToMemory(wfs.module, newPath)
	if err != nil {
		return err
	}

	results, err := renameFunc.Call(wfs.ctx, uint64(oldPathPtr), uint64(newPathPtr))
	if err != nil {
		return fmt.Errorf("fs_rename failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("rename failed")
	}

	return nil
}

func (wfs *WASMFileSystem) Chmod(path string, mode uint32) error {
	chmodFunc := wfs.module.ExportedFunction("fs_chmod")
	if chmodFunc == nil {
		// Chmod is optional, silently ignore if not implemented
		return nil
	}

	pathPtr, err := writeStringToMemory(wfs.module, path)
	if err != nil {
		return err
	}

	results, err := chmodFunc.Call(wfs.ctx, uint64(pathPtr), uint64(mode))
	if err != nil {
		return fmt.Errorf("fs_chmod failed: %w", err)
	}

	if len(results) > 0 && results[0] != 0 {
		if errMsg, ok := readStringFromMemory(wfs.module, uint32(results[0])); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("chmod failed")
	}

	return nil
}

func (wfs *WASMFileSystem) Open(path string) (io.ReadCloser, error) {
	// For WASM plugins, we can implement Open by reading the entire file
	// This is a simple implementation; more sophisticated implementations
	// could use streaming or chunked reads
	data, err := wfs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(io.NewSectionReader(&bytesReaderAt{data}, 0, int64(len(data)))), nil
}

func (wfs *WASMFileSystem) OpenWrite(path string) (io.WriteCloser, error) {
	// For WASM plugins, we return a WriteCloser that buffers writes
	// and flushes on close
	return &wasmWriteCloser{
		fs:   wfs,
		path: path,
		buf:  make([]byte, 0),
	}, nil
}

// Helper types for Open/OpenWrite implementation

type wasmWriteCloser struct {
	fs   *WASMFileSystem
	path string
	buf  []byte
}

func (w *wasmWriteCloser) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *wasmWriteCloser) Close() error {
	_, err := w.fs.Write(w.path, w.buf)
	return err
}

// Helper functions for memory management

func readStringFromMemory(module wazeroapi.Module, ptr uint32) (string, bool) {
	if ptr == 0 {
		return "", false
	}

	mem := module.Memory()
	if mem == nil {
		return "", false
	}

	// Read until null terminator
	var length uint32
	for {
		b, ok := mem.ReadByte(ptr + length)
		if !ok {
			return "", false
		}
		if b == 0 {
			break
		}
		length++
	}

	if length == 0 {
		return "", true
	}

	data, ok := mem.Read(ptr, length)
	if !ok {
		return "", false
	}

	return string(data), true
}

func writeStringToMemory(module wazeroapi.Module, s string) (uint32, error) {
	// Allocate memory in WASM module
	allocFunc := module.ExportedFunction("malloc")
	if allocFunc == nil {
		return 0, fmt.Errorf("malloc function not found in WASM module")
	}

	size := uint32(len(s) + 1) // +1 for null terminator
	results, err := allocFunc.Call(context.Background(), uint64(size))
	if err != nil {
		return 0, fmt.Errorf("malloc failed: %w", err)
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("malloc returned no results")
	}

	ptr := uint32(results[0])
	if ptr == 0 {
		return 0, fmt.Errorf("malloc returned null pointer")
	}

	// Write string to memory
	mem := module.Memory()
	data := append([]byte(s), 0) // Add null terminator
	if !mem.Write(ptr, data) {
		return 0, fmt.Errorf("failed to write string to memory")
	}

	return ptr, nil
}

func writeBytesToMemory(module wazeroapi.Module, data []byte) (uint32, error) {
	// Allocate memory in WASM module
	allocFunc := module.ExportedFunction("malloc")
	if allocFunc == nil {
		return 0, fmt.Errorf("malloc function not found in WASM module")
	}

	size := uint32(len(data))
	results, err := allocFunc.Call(context.Background(), uint64(size))
	if err != nil {
		return 0, fmt.Errorf("malloc failed: %w", err)
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("malloc returned no results")
	}

	ptr := uint32(results[0])
	if ptr == 0 {
		return 0, fmt.Errorf("malloc returned null pointer")
	}

	// Write data to memory
	mem := module.Memory()
	if !mem.Write(ptr, data) {
		return 0, fmt.Errorf("failed to write bytes to memory")
	}

	return ptr, nil
}

