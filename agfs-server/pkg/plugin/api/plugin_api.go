package api

import (
	"fmt"
	"unsafe"
)

// ExternalPlugin represents a dynamically loaded plugin from a shared library
// This bridges the C-compatible API with Go's ServicePlugin interface
type ExternalPlugin struct {
	libHandle   uintptr
	pluginPtr   unsafe.Pointer
	name        string
	vtable      *PluginVTable
	fileSystem  *ExternalFileSystem
}

// PluginVTable contains function pointers to the plugin's C-compatible API
type PluginVTable struct {
	// Plugin lifecycle functions
	PluginNew        func() unsafe.Pointer
	PluginFree       func(unsafe.Pointer)
	PluginName       func(unsafe.Pointer) *byte
	PluginValidate   func(unsafe.Pointer, *byte) *byte // Returns error string or nil
	PluginInitialize func(unsafe.Pointer, *byte) *byte // Returns error string or nil
	PluginShutdown   func(unsafe.Pointer) *byte        // Returns error string or nil
	PluginGetReadme  func(unsafe.Pointer) *byte

	// FileSystem operation functions
	FSCreate     func(unsafe.Pointer, *byte) *byte
	FSMkdir      func(unsafe.Pointer, *byte, uint32) *byte
	FSRemove     func(unsafe.Pointer, *byte) *byte
	FSRemoveAll  func(unsafe.Pointer, *byte) *byte
	FSRead       func(unsafe.Pointer, *byte, int64, int64, *int) *byte // Returns data, sets size
	FSWrite      func(unsafe.Pointer, *byte, *byte, int) *byte         // Returns response
	FSReadDir    func(unsafe.Pointer, *byte, *int) *FileInfoArray      // Returns array, sets count
	FSStat       func(unsafe.Pointer, *byte) *FileInfoC
	FSRename     func(unsafe.Pointer, *byte, *byte) *byte
	FSChmod      func(unsafe.Pointer, *byte, uint32) *byte
}

// FileInfoC is the C-compatible representation of filesystem.FileInfo
type FileInfoC struct {
	Name    *byte  // C string
	Size    int64
	Mode    uint32
	ModTime int64  // Unix timestamp
	IsDir   int32  // Boolean as int
	// Metadata fields
	MetaName    *byte
	MetaType    *byte
	MetaContent *byte // JSON-encoded map[string]string
}

// FileInfoArray is used for returning multiple FileInfo from C
type FileInfoArray struct {
	Items *FileInfoC
	Count int
}

// ExternalFileSystem implements filesystem.FileSystem by delegating to C functions
type ExternalFileSystem struct {
	pluginPtr unsafe.Pointer
	vtable    *PluginVTable
}

// Helper functions to convert between Go and C types

// CString converts a Go string to a C string (caller must free)
func CString(s string) *byte {
	if s == "" {
		return nil
	}
	b := append([]byte(s), 0)
	return &b[0]
}

// GoString converts a C string to a Go string
func GoString(cstr *byte) string {
	if cstr == nil {
		return ""
	}
	var length int
	for {
		ptr := unsafe.Pointer(uintptr(unsafe.Pointer(cstr)) + uintptr(length))
		if *(*byte)(ptr) == 0 {
			break
		}
		length++
	}
	if length == 0 {
		return ""
	}
	return string(unsafe.Slice(cstr, length))
}

// GoError converts a C error string to a Go error, or nil if no error
func GoError(errStr *byte) error {
	if errStr == nil {
		return nil
	}
	msg := GoString(errStr)
	if msg == "" {
		return nil
	}
	// Return a simple error with the message
	return fmt.Errorf("%s", msg)
}
