package api

import (
	"context"
	"encoding/json"

	"github.com/c4pt0r/agfs/agfs-server/pkg/filesystem"
	log "github.com/sirupsen/logrus"
	wazeroapi "github.com/tetratelabs/wazero/api"
)

// Host function implementations for filesystem operations
// These functions are exported to WASM modules and allow them to access the host filesystem

func HostFSRead(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])
	offset := int64(params[1])
	size := int64(params[2])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		log.Errorf("host_fs_read: failed to read path from memory")
		return []uint64{0} // Return 0 to indicate error
	}

	log.Debugf("host_fs_read: path=%s, offset=%d, size=%d", path, offset, size)

	// Check if filesystem is provided
	if fs == nil {
		log.Errorf("host_fs_read: no host filesystem provided")
		return []uint64{0}
	}

	data, err := fs.Read(path, offset, size)
	if err != nil {
		log.Errorf("host_fs_read: error reading file: %v", err)
		return []uint64{0}
	}

	// Write data to WASM memory
	dataPtr, err := writeBytesToMemory(mod, data)
	if err != nil {
		log.Errorf("host_fs_read: failed to write data to memory: %v", err)
		return []uint64{0}
	}

	// Pack pointer and size into single u64
	// Lower 32 bits = pointer, upper 32 bits = size
	packed := uint64(dataPtr) | (uint64(len(data)) << 32)
	return []uint64{packed}
}

func HostFSWrite(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])
	dataPtr := uint32(params[1])
	dataLen := uint32(params[2])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		log.Errorf("host_fs_write: failed to read path from memory")
		return []uint64{0}
	}

	data, ok := mod.Memory().Read(dataPtr, dataLen)
	if !ok {
		log.Errorf("host_fs_write: failed to read data from memory")
		return []uint64{0}
	}

	log.Debugf("host_fs_write: path=%s, dataLen=%d", path, dataLen)

	if fs == nil {
		log.Errorf("host_fs_write: no host filesystem provided")
		return []uint64{0}
	}

	response, err := fs.Write(path, data)
	if err != nil {
		log.Errorf("host_fs_write: error writing file: %v", err)
		return []uint64{0}
	}

	// Write response to WASM memory
	responsePtr, err := writeBytesToMemory(mod, response)
	if err != nil {
		log.Errorf("host_fs_write: failed to write response to memory: %v", err)
		return []uint64{0}
	}

	// Pack pointer and size
	packed := uint64(responsePtr) | (uint64(len(response)) << 32)
	return []uint64{packed}
}

func HostFSStat(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		log.Errorf("host_fs_stat: failed to read path from memory")
		return []uint64{0}
	}

	log.Debugf("host_fs_stat: path=%s", path)

	if fs == nil {
		log.Errorf("host_fs_stat: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr) << 32}
	}

	fileInfo, err := fs.Stat(path)
	if err != nil {
		log.Errorf("host_fs_stat: error stating file: %v", err)
		// Pack error: upper 32 bits = error pointer
		errStr := err.Error()
		errPtr, err := writeStringToMemory(mod, errStr)
		if err != nil {
			return []uint64{0}
		}
		return []uint64{uint64(errPtr) << 32}
	}

	// Serialize fileInfo to JSON
	jsonData, err := json.Marshal(fileInfo)
	if err != nil {
		log.Errorf("host_fs_stat: failed to marshal fileInfo: %v", err)
		return []uint64{0}
	}

	jsonPtr, err := writeStringToMemory(mod, string(jsonData))
	if err != nil {
		log.Errorf("host_fs_stat: failed to write JSON to memory: %v", err)
		return []uint64{0}
	}

	// Pack: lower 32 bits = json pointer, upper 32 bits = 0 (no error)
	return []uint64{uint64(jsonPtr)}
}

func HostFSReadDir(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		log.Errorf("host_fs_readdir: failed to read path from memory")
		return []uint64{0}
	}

	log.Debugf("host_fs_readdir: path=%s", path)

	if fs == nil {
		log.Errorf("host_fs_readdir: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr) << 32}
	}

	fileInfos, err := fs.ReadDir(path)
	if err != nil {
		log.Errorf("host_fs_readdir: error reading directory: %v", err)
		errStr := err.Error()
		errPtr, err := writeStringToMemory(mod, errStr)
		if err != nil {
			return []uint64{0}
		}
		return []uint64{uint64(errPtr) << 32}
	}

	// Serialize fileInfos to JSON
	jsonData, err := json.Marshal(fileInfos)
	if err != nil {
		log.Errorf("host_fs_readdir: failed to marshal fileInfos: %v", err)
		return []uint64{0}
	}

	jsonPtr, err := writeStringToMemory(mod, string(jsonData))
	if err != nil {
		log.Errorf("host_fs_readdir: failed to write JSON to memory: %v", err)
		return []uint64{0}
	}

	return []uint64{uint64(jsonPtr)}
}

func HostFSCreate(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		return []uint64{1} // Error
	}

	log.Debugf("host_fs_create: path=%s", path)

	if fs == nil {
		log.Errorf("host_fs_create: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.Create(path)
	if err != nil {
		log.Errorf("host_fs_create: error creating file: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0} // Success
}

func HostFSMkdir(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])
	perm := uint32(params[1])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		return []uint64{1}
	}

	log.Debugf("host_fs_mkdir: path=%s, perm=%o", path, perm)

	if fs == nil {
		log.Errorf("host_fs_mkdir: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.Mkdir(path, perm)
	if err != nil {
		log.Errorf("host_fs_mkdir: error creating directory: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0}
}

func HostFSRemove(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		return []uint64{1}
	}

	log.Debugf("host_fs_remove: path=%s", path)

	if fs == nil {
		log.Errorf("host_fs_remove: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.Remove(path)
	if err != nil {
		log.Errorf("host_fs_remove: error removing: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0}
}

func HostFSRemoveAll(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		return []uint64{1}
	}

	log.Debugf("host_fs_remove_all: path=%s", path)

	if fs == nil {
		log.Errorf("host_fs_remove_all: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.RemoveAll(path)
	if err != nil {
		log.Errorf("host_fs_remove_all: error removing: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0}
}

func HostFSRename(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	oldPathPtr := uint32(params[0])
	newPathPtr := uint32(params[1])

	oldPath, ok := readStringFromMemory(mod, oldPathPtr)
	if !ok {
		return []uint64{1}
	}

	newPath, ok := readStringFromMemory(mod, newPathPtr)
	if !ok {
		return []uint64{1}
	}

	log.Debugf("host_fs_rename: oldPath=%s, newPath=%s", oldPath, newPath)

	if fs == nil {
		log.Errorf("host_fs_rename: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.Rename(oldPath, newPath)
	if err != nil {
		log.Errorf("host_fs_rename: error renaming: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0}
}

func HostFSChmod(ctx context.Context, mod wazeroapi.Module, params []uint64, fs filesystem.FileSystem) []uint64 {
	pathPtr := uint32(params[0])
	mode := uint32(params[1])

	path, ok := readStringFromMemory(mod, pathPtr)
	if !ok {
		return []uint64{1}
	}

	log.Debugf("host_fs_chmod: path=%s, mode=%o", path, mode)

	if fs == nil {
		log.Errorf("host_fs_chmod: no host filesystem provided")
		errPtr, _ := writeStringToMemory(mod, "no host filesystem provided")
		return []uint64{uint64(errPtr)}
	}

	err := fs.Chmod(path, mode)
	if err != nil {
		log.Errorf("host_fs_chmod: error changing mode: %v", err)
		errPtr, _ := writeStringToMemory(mod, err.Error())
		return []uint64{uint64(errPtr)}
	}

	return []uint64{0}
}
