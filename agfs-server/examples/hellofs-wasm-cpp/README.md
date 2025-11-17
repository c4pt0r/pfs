# HelloFS WASM C++ - AGFS C++ SDK Example

An AGFS WASM plugin example written in C++, demonstrating how to develop custom filesystem plugins using `agfs-cpp-sdk`.

## Features

- ✅ Pure C++ implementation with type safety
- ✅ Simple and easy-to-use API
- ✅ Host filesystem access support (HostFS)
- ✅ Automatic FFI handling
- ✅ No manual memory management required

## Project Structure

```
hellofs-wasm-cpp/
├── agfs-cpp-sdk/          # C++ SDK
│   ├── agfs.h             # Main header (only include this)
│   ├── agfs_types.h       # Type definitions
│   ├── agfs_ffi.h         # FFI helpers
│   ├── agfs_hostfs.h      # HostFS access
│   ├── agfs_filesystem.h  # FileSystem base class
│   ├── agfs_export.h      # Export macros
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
# Load using agfs command
agfs plugins load agfs://path/to/hellofs-wasm-cpp.wasm

# Read file
agfs cat /hellofs-cpp/hello.txt

# If host_prefix is configured, access host filesystem
# agfs cat /hellofs-cpp/host/some-file.txt
```

## Basic Usage Examples

### Minimal Plugin

```cpp
#include "agfs-cpp-sdk/agfs.h"

class MyFS : public agfs::FileSystem {
public:
    const char* name() const override {
        return "myfs";
    }

    agfs::Result<agfs::FileInfo> stat(const std::string& path) override {
        if (path == "/") {
            return agfs::FileInfo::dir("", 0755);
        }
        if (path == "/hello.txt") {
            return agfs::FileInfo::file("hello.txt", 12, 0644);
        }
        return agfs::Error::not_found();
    }

    agfs::Result<std::vector<agfs::FileInfo>> readdir(const std::string& path) override {
        if (path == "/") {
            std::vector<agfs::FileInfo> entries;
            entries.push_back(agfs::FileInfo::file("hello.txt", 12, 0644));
            return entries;
        }
        return agfs::Error::not_found();
    }

    agfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                           int64_t offset, int64_t size) override {
        if (path == "/hello.txt") {
            std::string content = "Hello World\n";
            return std::vector<uint8_t>(content.begin(), content.end());
        }
        return agfs::Error::not_found();
    }
};

AGFS_EXPORT_PLUGIN(MyFS)
```

### Using Configuration

```cpp
class ConfigurableFS : public agfs::FileSystem {
private:
    std::string prefix;

public:
    agfs::Result<void> initialize(const agfs::Config& config) override {
        const char* p = config.get_str("prefix");
        if (p) prefix = p;
        return agfs::Result<void>();
    }
    // ... other methods
};
```

### Accessing Host Filesystem

```cpp
agfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                       int64_t offset, int64_t size) override {
    if (path.rfind("/host/", 0) == 0) {
        std::string host_path = "/tmp" + path.substr(5);
        auto result = agfs::HostFS::read(host_path, offset, size);
        if (result.is_err()) {
            return agfs::Error::other("Failed to read from host");
        }
        return result.unwrap();
    }
    // ... handle other paths
}
```

## API Reference

### agfs::FileSystem

All plugins must inherit from `agfs::FileSystem`.

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

### agfs::Result<T>

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

### agfs::Error

Error types:

```cpp
agfs::Error::not_found()
agfs::Error::permission_denied()
agfs::Error::already_exists()
agfs::Error::is_directory()
agfs::Error::not_directory()
agfs::Error::read_only()
agfs::Error::invalid_input("message")
agfs::Error::io("message")
agfs::Error::other("message")
```

### agfs::FileInfo

File information:

```cpp
// Create file info
auto file = agfs::FileInfo::file("name", size, mode);
auto dir = agfs::FileInfo::dir("name", mode);

// Add metadata
file.with_meta(metadata).with_mod_time(timestamp);
```

### agfs::HostFS

Access host filesystem:

```cpp
// Read file
auto data = agfs::HostFS::read("/path/to/file", 0, -1);

// Get file info
auto info = agfs::HostFS::stat("/path/to/file");

// List directory
auto entries = agfs::HostFS::readdir("/path/to/dir");

// Write file
auto response = agfs::HostFS::write("/path/to/file", data);

// Create/delete/rename etc.
agfs::HostFS::create("/path/to/file");
agfs::HostFS::mkdir("/path/to/dir", 0755);
agfs::HostFS::remove("/path/to/file");
agfs::HostFS::rename("/old", "/new");
```

## Comparison with Rust Version

| Feature | Rust | C++ |
|---------|------|-----|
| Type System | Strong | Strong |
| Memory Safety | Compile-time guarantee | Manual (SDK encapsulated) |
| Error Handling | `Result<T, Error>` | `Result<T>` |
| Macro | `export_plugin!()` | `AGFS_EXPORT_PLUGIN()` |
| Learning Curve | Medium | Low (if familiar with C++) |
| File Size | ~10KB | ~121KB |

## Dependencies

- **nlohmann/json** - JSON library for parsing configuration and serializing FileInfo
  - Header-only library (included in `agfs-cpp-sdk/json.hpp`)
  - Version: 3.11.3
  - License: MIT

## More Information

- See `src/main.cpp` for a complete implementation example
- Refer to agfs-cpp-sdk header files for complete API documentation

## License

Same as agfs-server
