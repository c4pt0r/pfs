#ifndef PFS_HOSTFS_H
#define PFS_HOSTFS_H

#include "pfs_types.h"
#include "pfs_ffi.h"
#include <cstring>

namespace pfs {

// Import host functions from the "env" module
extern "C" {
    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_read")))
    uint64_t host_fs_read(const char* path, int64_t offset, int64_t size);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_write")))
    uint64_t host_fs_write(const char* path, const uint8_t* data, uint32_t len);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_stat")))
    uint64_t host_fs_stat(const char* path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_readdir")))
    uint64_t host_fs_readdir(const char* path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_create")))
    uint32_t host_fs_create(const char* path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_mkdir")))
    uint32_t host_fs_mkdir(const char* path, uint32_t perm);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_remove")))
    uint32_t host_fs_remove(const char* path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_remove_all")))
    uint32_t host_fs_remove_all(const char* path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_rename")))
    uint32_t host_fs_rename(const char* old_path, const char* new_path);

    __attribute__((import_module("env"))) __attribute__((import_name("host_fs_chmod")))
    uint32_t host_fs_chmod(const char* path, uint32_t mode);
}

// Helper to read string from pointer
inline std::string read_string_from_ptr(uint32_t ptr) {
    if (ptr == 0) {
        return "";
    }

    // Find null terminator
    const char* start_ptr = reinterpret_cast<const char*>(ptr);
    size_t len = 0;
    while (start_ptr[len] != '\0') {
        len++;
    }

    return std::string(start_ptr, len);
}

// HostFS provides access to the host filesystem from WASM
class HostFS {
public:
    // Read data from a file on the host filesystem
    static Result<std::vector<uint8_t>> read(const std::string& path, int64_t offset, int64_t size) {
        uint64_t result = host_fs_read(path.c_str(), offset, size);

        // Unpack: lower 32 bits = pointer, upper 32 bits = size
        uint32_t data_ptr = (uint32_t)(result & 0xFFFFFFFF);
        uint32_t data_size = (uint32_t)((result >> 32) & 0xFFFFFFFF);

        if (data_ptr == 0) {
            return Error::io("read failed");
        }

        // Read data from memory
        const uint8_t* ptr = reinterpret_cast<const uint8_t*>(data_ptr);
        std::vector<uint8_t> data(ptr, ptr + data_size);
        return data;
    }

    // Write data to a file on the host filesystem
    static Result<std::vector<uint8_t>> write(const std::string& path, const std::vector<uint8_t>& data) {
        uint64_t result = host_fs_write(path.c_str(), data.data(), data.size());

        // Unpack: lower 32 bits = pointer, upper 32 bits = size
        uint32_t response_ptr = (uint32_t)(result & 0xFFFFFFFF);
        uint32_t response_size = (uint32_t)((result >> 32) & 0xFFFFFFFF);

        if (response_ptr == 0) {
            return Error::io("write failed");
        }

        // Read response from memory
        const uint8_t* ptr = reinterpret_cast<const uint8_t*>(response_ptr);
        std::vector<uint8_t> response(ptr, ptr + response_size);
        return response;
    }

    // Get file information
    static Result<FileInfo> stat(const std::string& path) {
        uint64_t result = host_fs_stat(path.c_str());

        // Unpack: lower 32 bits = json pointer, upper 32 bits = error pointer
        uint32_t json_ptr = (uint32_t)(result & 0xFFFFFFFF);
        uint32_t err_ptr = (uint32_t)((result >> 32) & 0xFFFFFFFF);

        // Check for error
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }

        if (json_ptr == 0) {
            return Error::not_found();
        }

        std::string json_str = read_string_from_ptr(json_ptr);
        return ffi::JsonParser::parse_fileinfo(json_str);
    }

    // Read directory contents
    static Result<std::vector<FileInfo>> readdir(const std::string& path) {
        uint64_t result = host_fs_readdir(path.c_str());

        // Unpack: lower 32 bits = json pointer, upper 32 bits = error pointer
        uint32_t json_ptr = (uint32_t)(result & 0xFFFFFFFF);
        uint32_t err_ptr = (uint32_t)((result >> 32) & 0xFFFFFFFF);

        // Check for error
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }

        if (json_ptr == 0) {
            return std::vector<FileInfo>();
        }

        std::string json_str = read_string_from_ptr(json_ptr);
        return ffi::JsonParser::parse_fileinfo_array(json_str);
    }

    // Create a new file
    static Result<void> create(const std::string& path) {
        uint32_t err_ptr = host_fs_create(path.c_str());
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }

    // Create a directory
    static Result<void> mkdir(const std::string& path, uint32_t perm) {
        uint32_t err_ptr = host_fs_mkdir(path.c_str(), perm);
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }

    // Remove a file or empty directory
    static Result<void> remove(const std::string& path) {
        uint32_t err_ptr = host_fs_remove(path.c_str());
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }

    // Remove a file or directory recursively
    static Result<void> remove_all(const std::string& path) {
        uint32_t err_ptr = host_fs_remove_all(path.c_str());
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }

    // Rename a file or directory
    static Result<void> rename(const std::string& old_path, const std::string& new_path) {
        uint32_t err_ptr = host_fs_rename(old_path.c_str(), new_path.c_str());
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }

    // Change file permissions
    static Result<void> chmod(const std::string& path, uint32_t mode) {
        uint32_t err_ptr = host_fs_chmod(path.c_str(), mode);
        if (err_ptr != 0) {
            std::string err_str = read_string_from_ptr(err_ptr);
            return Error::other(err_str);
        }
        return Result<void>();
    }
};

} // namespace pfs

#endif // PFS_HOSTFS_H
