#ifndef AGFS_FILESYSTEM_H
#define AGFS_FILESYSTEM_H

#include "agfs_types.h"

namespace agfs {

// FileSystem base class that plugin developers should implement
class FileSystem {
public:
    virtual ~FileSystem() = default;

    // Returns the name of this filesystem plugin
    virtual const char* name() const = 0;

    // Returns the README/documentation for this plugin
    virtual const char* readme() const {
        return "No documentation available";
    }

    // Validate the configuration before initialization
    virtual Result<void> validate(const Config& config) {
        (void)config; // unused
        return Result<void>();
    }

    // Initialize the filesystem with the given configuration
    virtual Result<void> initialize(const Config& config) {
        (void)config; // unused
        return Result<void>();
    }

    // Shutdown the filesystem
    virtual Result<void> shutdown() {
        return Result<void>();
    }

    // Read data from a file
    virtual Result<std::vector<uint8_t>> read(const std::string& path, int64_t offset, int64_t size) {
        (void)path; (void)offset; (void)size; // unused
        return Error::read_only();
    }

    // Write data to a file (returns response data)
    virtual Result<std::vector<uint8_t>> write(const std::string& path, const std::vector<uint8_t>& data) {
        (void)path; (void)data; // unused
        return Error::read_only();
    }

    // Create a new empty file
    virtual Result<void> create(const std::string& path) {
        (void)path; // unused
        return Error::read_only();
    }

    // Create a new directory
    virtual Result<void> mkdir(const std::string& path, uint32_t perm) {
        (void)path; (void)perm; // unused
        return Error::read_only();
    }

    // Remove a file or empty directory
    virtual Result<void> remove(const std::string& path) {
        (void)path; // unused
        return Error::read_only();
    }

    // Remove a file or directory and all its contents
    virtual Result<void> remove_all(const std::string& path) {
        (void)path; // unused
        return Error::read_only();
    }

    // Get file information
    virtual Result<FileInfo> stat(const std::string& path) = 0;

    // List directory contents
    virtual Result<std::vector<FileInfo>> readdir(const std::string& path) = 0;

    // Rename/move a file or directory
    virtual Result<void> rename(const std::string& old_path, const std::string& new_path) {
        (void)old_path; (void)new_path; // unused
        return Error::read_only();
    }

    // Change file permissions
    virtual Result<void> chmod(const std::string& path, uint32_t mode) {
        (void)path; (void)mode; // unused
        return Result<void>(); // Default: no-op
    }
};

} // namespace agfs

#endif // AGFS_FILESYSTEM_H
