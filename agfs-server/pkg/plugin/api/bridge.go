package api

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	"github.com/c4pt0r/agfs/agfs-server/pkg/plugin"
)

// NewExternalPlugin creates a new external plugin wrapper
func NewExternalPlugin(libHandle uintptr, vtable *PluginVTable) (*ExternalPlugin, error) {
	if vtable.PluginNew == nil {
		return nil, fmt.Errorf("plugin missing required PluginNew function")
	}

	// Create plugin instance
	pluginPtr := vtable.PluginNew()
	if pluginPtr == nil {
		return nil, fmt.Errorf("PluginNew returned null pointer")
	}

	// Get plugin name
	var name string
	if vtable.PluginName != nil {
		namePtr := vtable.PluginName(pluginPtr)
		name = GoString(namePtr)
	}

	ep := &ExternalPlugin{
		libHandle: libHandle,
		pluginPtr: pluginPtr,
		name:      name,
		vtable:    vtable,
	}

	// Create filesystem wrapper
	ep.fileSystem = &ExternalFileSystem{
		pluginPtr: pluginPtr,
		vtable:    vtable,
	}

	return ep, nil
}

// Implement plugin.ServicePlugin interface

func (ep *ExternalPlugin) Name() string {
	return ep.name
}

func (ep *ExternalPlugin) Validate(config map[string]interface{}) error {
	if ep.vtable.PluginValidate == nil {
		return nil // Validation not implemented
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configCStr := CString(string(configJSON))
	errPtr := ep.vtable.PluginValidate(ep.pluginPtr, configCStr)
	return GoError(errPtr)
}

func (ep *ExternalPlugin) Initialize(config map[string]interface{}) error {
	if ep.vtable.PluginInitialize == nil {
		return nil // Initialization not required
	}

	// Convert config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configCStr := CString(string(configJSON))
	errPtr := ep.vtable.PluginInitialize(ep.pluginPtr, configCStr)
	return GoError(errPtr)
}

func (ep *ExternalPlugin) GetFileSystem() filesystem.FileSystem {
	return ep.fileSystem
}

func (ep *ExternalPlugin) GetReadme() string {
	if ep.vtable.PluginGetReadme == nil {
		return ""
	}

	readmePtr := ep.vtable.PluginGetReadme(ep.pluginPtr)
	return GoString(readmePtr)
}

func (ep *ExternalPlugin) Shutdown() error {
	if ep.vtable.PluginShutdown == nil {
		return nil
	}

	errPtr := ep.vtable.PluginShutdown(ep.pluginPtr)
	err := GoError(errPtr)

	// Free the plugin instance
	if ep.vtable.PluginFree != nil {
		ep.vtable.PluginFree(ep.pluginPtr)
	}

	return err
}

// Implement filesystem.FileSystem interface

func (efs *ExternalFileSystem) Create(path string) error {
	if efs.vtable.FSCreate == nil {
		return fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	errPtr := efs.vtable.FSCreate(efs.pluginPtr, pathCStr)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) Mkdir(path string, perm uint32) error {
	if efs.vtable.FSMkdir == nil {
		return fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	errPtr := efs.vtable.FSMkdir(efs.pluginPtr, pathCStr, perm)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) Remove(path string) error {
	if efs.vtable.FSRemove == nil {
		return fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	errPtr := efs.vtable.FSRemove(efs.pluginPtr, pathCStr)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) RemoveAll(path string) error {
	if efs.vtable.FSRemoveAll == nil {
		return fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	errPtr := efs.vtable.FSRemoveAll(efs.pluginPtr, pathCStr)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) Read(path string, offset int64, size int64) ([]byte, error) {
	if efs.vtable.FSRead == nil {
		return nil, fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	var dataLen int
	dataPtr := efs.vtable.FSRead(efs.pluginPtr, pathCStr, offset, size, &dataLen)

	if dataPtr == nil {
		if dataLen < 0 {
			return nil, fmt.Errorf("read failed")
		}
		return []byte{}, nil
	}

	// Copy data from C to Go
	data := make([]byte, dataLen)
	for i := 0; i < dataLen; i++ {
		ptr := unsafe.Pointer(uintptr(unsafe.Pointer(dataPtr)) + uintptr(i))
		data[i] = *(*byte)(ptr)
	}

	return data, nil
}

func (efs *ExternalFileSystem) Write(path string, data []byte) ([]byte, error) {
	if efs.vtable.FSWrite == nil {
		return nil, fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	var dataCStr *byte
	if len(data) > 0 {
		dataCStr = &data[0]
	}

	responsePtr := efs.vtable.FSWrite(efs.pluginPtr, pathCStr, dataCStr, len(data))
	if responsePtr == nil {
		return []byte{}, nil
	}

	response := GoString(responsePtr)
	return []byte(response), nil
}

func (efs *ExternalFileSystem) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if efs.vtable.FSReadDir == nil {
		return nil, fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	var count int
	arrPtr := efs.vtable.FSReadDir(efs.pluginPtr, pathCStr, &count)

	if arrPtr == nil || count == 0 {
		return []filesystem.FileInfo{}, nil
	}

	// Convert C array to Go slice
	infos := make([]filesystem.FileInfo, count)
	for i := 0; i < count; i++ {
		cInfoPtr := unsafe.Pointer(uintptr(unsafe.Pointer(arrPtr.Items)) + uintptr(i)*unsafe.Sizeof(FileInfoC{}))
		cInfo := (*FileInfoC)(cInfoPtr)
		goInfo := FileInfoCToGo(cInfo)
		if goInfo != nil {
			infos[i] = *goInfo
		}
	}

	return infos, nil
}

func (efs *ExternalFileSystem) Stat(path string) (*filesystem.FileInfo, error) {
	if efs.vtable.FSStat == nil {
		return nil, fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	cInfo := efs.vtable.FSStat(efs.pluginPtr, pathCStr)

	if cInfo == nil {
		return nil, fmt.Errorf("stat failed")
	}

	return FileInfoCToGo(cInfo), nil
}

func (efs *ExternalFileSystem) Rename(oldPath, newPath string) error {
	if efs.vtable.FSRename == nil {
		return fmt.Errorf("not implemented")
	}

	oldPathCStr := CString(oldPath)
	newPathCStr := CString(newPath)
	errPtr := efs.vtable.FSRename(efs.pluginPtr, oldPathCStr, newPathCStr)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) Chmod(path string, mode uint32) error {
	if efs.vtable.FSChmod == nil {
		return fmt.Errorf("not implemented")
	}

	pathCStr := CString(path)
	errPtr := efs.vtable.FSChmod(efs.pluginPtr, pathCStr, mode)
	return GoError(errPtr)
}

func (efs *ExternalFileSystem) Open(path string) (io.ReadCloser, error) {
	// Default implementation using Read
	data, err := efs.Read(path, 0, -1)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(io.NewSectionReader(&bytesReaderAt{data}, 0, int64(len(data)))), nil
}

func (efs *ExternalFileSystem) OpenWrite(path string) (io.WriteCloser, error) {
	return &writeCloser{fs: efs, path: path}, nil
}

// Helper types

type bytesReaderAt struct {
	data []byte
}

func (b *bytesReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	if n < len(p) {
		err = io.EOF
	}
	return
}

type writeCloser struct {
	fs   *ExternalFileSystem
	path string
	buf  []byte
}

func (wc *writeCloser) Write(p []byte) (n int, err error) {
	wc.buf = append(wc.buf, p...)
	return len(p), nil
}

func (wc *writeCloser) Close() error {
	_, err := wc.fs.Write(wc.path, wc.buf)
	return err
}

// FileInfoCToGo with proper time handling
func FileInfoCToGo(c *FileInfoC) *filesystem.FileInfo {
	if c == nil {
		return nil
	}

	info := &filesystem.FileInfo{
		Name:    GoString(c.Name),
		Size:    c.Size,
		Mode:    c.Mode,
		ModTime: time.Unix(c.ModTime, 0),
		IsDir:   c.IsDir != 0,
		Meta: filesystem.MetaData{
			Name:    GoString(c.MetaName),
			Type:    GoString(c.MetaType),
			Content: make(map[string]string),
		},
	}

	// Parse MetaContent JSON if present
	if c.MetaContent != nil {
		contentStr := GoString(c.MetaContent)
		if contentStr != "" {
			json.Unmarshal([]byte(contentStr), &info.Meta.Content)
		}
	}

	return info
}

// Ensure ExternalPlugin implements plugin.ServicePlugin
var _ plugin.ServicePlugin = (*ExternalPlugin)(nil)

// Ensure ExternalFileSystem implements filesystem.FileSystem
var _ filesystem.FileSystem = (*ExternalFileSystem)(nil)
