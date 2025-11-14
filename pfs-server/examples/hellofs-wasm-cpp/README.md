# HelloFS WASM C++ - PFS C++ SDK Example

A PFS WASM plugin example written in C++, demonstrating how to develop custom filesystem plugins using `pfs-cpp-sdk`.

## Features

- ✅ Pure C++ implementation with type safety
- ✅ Simple and easy-to-use API
- ✅ Host filesystem access support (HostFS)
- ✅ Automatic FFI handling
- ✅ No manual memory management required

## Project Structure

```
hellofs-wasm-cpp/
├── pfs-cpp-sdk/          # C++ SDK
│   ├── pfs.h             # Main header (only include this)
│   ├── pfs_types.h       # Type definitions
│   ├── pfs_ffi.h         # FFI helpers
│   ├── pfs_hostfs.h      # HostFS access
│   ├── pfs_filesystem.h  # FileSystem base class
│   ├── pfs_export.h      # Export macros
│   └── json.hpp          # nlohmann/json (third-party library)
├── src/
│   └── main.cpp          # HelloFS implementation
├── Makefile              # Build script
└── README.md             # This file
```

## Build Requirements

**Emscripten** compiler toolchain is recommended.

### Install Emscripten

macOS:
```bash
brew install emscripten
```

Linux:
```bash
# Ubuntu/Debian
sudo apt install emscripten

# Arch Linux
sudo pacman -S emscripten
```

Optional: Install wasm-opt for optimization:
```bash
brew install binaryen  # macOS
sudo apt install binaryen  # Ubuntu/Debian
```

## Quick Start

### 1. Build

```bash
make build
```

This generates `hellofs-wasm-cpp.wasm` file (~121KB).

### 2. Use the Plugin

Create a configuration file `config.yaml`:

```yaml
filesystems:
  - name: hellofs-cpp
    type: wasm
    mount: /hellofs-cpp
    config:
      wasm_path: ./hellofs-wasm-cpp.wasm
      # Optional: configure host filesystem access
      # host_prefix: /tmp
```

### 3. Load and Test

```bash
# Load using pfs command
pfs plugins load pfs://path/to/hellofs-wasm-cpp.wasm

# Read file
pfs cat /hellofs-cpp/hello.txt

# If host_prefix is configured, access host filesystem
# pfs cat /hellofs-cpp/host/some-file.txt
```

## Basic Usage Examples

### Minimal Plugin

```cpp
#include "pfs-cpp-sdk/pfs.h"

class MyFS : public pfs::FileSystem {
public:
    const char* name() const override {
        return "myfs";
    }

    pfs::Result<pfs::FileInfo> stat(const std::string& path) override {
        if (path == "/") {
            return pfs::FileInfo::dir("", 0755);
        }
        if (path == "/hello.txt") {
            return pfs::FileInfo::file("hello.txt", 12, 0644);
        }
        return pfs::Error::not_found();
    }

    pfs::Result<std::vector<pfs::FileInfo>> readdir(const std::string& path) override {
        if (path == "/") {
            std::vector<pfs::FileInfo> entries;
            entries.push_back(pfs::FileInfo::file("hello.txt", 12, 0644));
            return entries;
        }
        return pfs::Error::not_found();
    }

    pfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                           int64_t offset, int64_t size) override {
        if (path == "/hello.txt") {
            std::string content = "Hello World\n";
            return std::vector<uint8_t>(content.begin(), content.end());
        }
        return pfs::Error::not_found();
    }
};

PFS_EXPORT_PLUGIN(MyFS)
```

### Using Configuration

```cpp
class ConfigurableFS : public pfs::FileSystem {
private:
    std::string prefix;

public:
    pfs::Result<void> initialize(const pfs::Config& config) override {
        const char* p = config.get_str("prefix");
        if (p) prefix = p;
        return pfs::Result<void>();
    }
    // ... other methods
};
```

### Accessing Host Filesystem

```cpp
pfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                       int64_t offset, int64_t size) override {
    if (path.rfind("/host/", 0) == 0) {
        std::string host_path = "/tmp" + path.substr(5);
        auto result = pfs::HostFS::read(host_path, offset, size);
        if (result.is_err()) {
            return pfs::Error::other("Failed to read from host");
        }
        return result.unwrap();
    }
    // ... handle other paths
}
```

## API Reference

### pfs::FileSystem

All plugins must inherit from `pfs::FileSystem`.

**Required methods:**
- `const char* name()` - Return plugin name
- `Result<FileInfo> stat(path)` - Get file information
- `Result<vector<FileInfo>> readdir(path)` - List directory contents

**Optional methods:**
- `const char* readme()` - Return documentation
- `Result<void> validate(config)` - Validate configuration
- `Result<void> initialize(config)` - Initialize plugin
- `Result<void> shutdown()` - Shutdown plugin
- `Result<vector<uint8_t>> read(path, offset, size)` - Read file
- `Result<vector<uint8_t>> write(path, data)` - Write file
- `Result<void> create(path)` - Create file
- `Result<void> mkdir(path, perm)` - Create directory
- `Result<void> remove(path)` - Remove file/directory
- `Result<void> remove_all(path)` - Recursively remove
- `Result<void> rename(old_path, new_path)` - Rename
- `Result<void> chmod(path, mode)` - Change permissions

### pfs::Result<T>

Similar to Rust's Result type:

```cpp
auto result = read_file();
if (result.is_ok()) {
    auto data = result.unwrap();
    // use data
} else {
    auto error = result.unwrap_err();
    // handle error
}
```

### pfs::Error

Error types:

```cpp
pfs::Error::not_found()
pfs::Error::permission_denied()
pfs::Error::already_exists()
pfs::Error::is_directory()
pfs::Error::not_directory()
pfs::Error::read_only()
pfs::Error::invalid_input("message")
pfs::Error::io("message")
pfs::Error::other("message")
```

### pfs::FileInfo

File information:

```cpp
// Create file info
auto file = pfs::FileInfo::file("name", size, mode);
auto dir = pfs::FileInfo::dir("name", mode);

// Add metadata
file.with_meta(metadata).with_mod_time(timestamp);
```

### pfs::HostFS

Access host filesystem:

```cpp
// Read file
auto data = pfs::HostFS::read("/path/to/file", 0, -1);

// Get file info
auto info = pfs::HostFS::stat("/path/to/file");

// List directory
auto entries = pfs::HostFS::readdir("/path/to/dir");

// Write file
auto response = pfs::HostFS::write("/path/to/file", data);

// Create/delete/rename etc.
pfs::HostFS::create("/path/to/file");
pfs::HostFS::mkdir("/path/to/dir", 0755);
pfs::HostFS::remove("/path/to/file");
pfs::HostFS::rename("/old", "/new");
```

## Comparison with Rust Version

| Feature | Rust | C++ |
|---------|------|-----|
| Type System | Strong | Strong |
| Memory Safety | Compile-time guarantee | Manual (SDK encapsulated) |
| Error Handling | `Result<T, Error>` | `Result<T>` |
| Macro | `export_plugin!()` | `PFS_EXPORT_PLUGIN()` |
| Learning Curve | Medium | Low (if familiar with C++) |
| File Size | ~10KB | ~121KB |

## Dependencies

- **nlohmann/json** - JSON library for parsing configuration and serializing FileInfo
  - Header-only library (included in `pfs-cpp-sdk/json.hpp`)
  - Version: 3.11.3
  - License: MIT

## More Information

- See `src/main.cpp` for a complete implementation example
- Refer to pfs-cpp-sdk header files for complete API documentation

## License

Same as pfs-server
