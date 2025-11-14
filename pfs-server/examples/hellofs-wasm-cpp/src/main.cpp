// HelloFS WASM - C++ implementation
//
// A simple filesystem plugin demonstrating PFS C++ SDK usage
// Returns a single file with "Hello World" content
// Also demonstrates accessing the host filesystem

#include "../pfs-cpp-sdk/pfs.h"

class HelloFS : public pfs::FileSystem {
private:
    std::string host_prefix;

public:
    const char* name() const override {
        return "hellofs-wasm-cpp";
    }

    const char* readme() const override {
        return "HelloFS WASM (C++) - Demonstrates host filesystem access\n"
               " - /hello.txt - Returns 'Hello World from C++'\n"
               " - /host/* - Proxies to host filesystem (if configured)";
    }

    pfs::Result<void> initialize(const pfs::Config& config) override {
        // Get optional host_prefix from config
        const char* prefix = config.get_str("host_prefix");
        if (prefix != nullptr) {
            host_prefix = prefix;
        }
        return pfs::Result<void>();
    }

    pfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                           int64_t offset, int64_t size) override {
        if (path == "/hello.txt") {
            std::string content = "Hello World from C++\n";
            std::vector<uint8_t> data(content.begin(), content.end());
            return data;
        } else if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            auto result = pfs::HostFS::read(full_path, offset, size);
            if (result.is_err()) {
                return pfs::Error::other("host fs: " + result.unwrap_err().to_string());
            }
            return result.unwrap();
        }
        return pfs::Error::not_found();
    }

    pfs::Result<pfs::FileInfo> stat(const std::string& path) override {
        if (path == "/") {
            return pfs::FileInfo::dir("", 0755);
        } else if (path == "/hello.txt") {
            return pfs::FileInfo::file("hello.txt", 21, 0644);
        } else if (path == "/host" && !host_prefix.empty()) {
            return pfs::FileInfo::dir("host", 0755);
        } else if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            auto result = pfs::HostFS::stat(full_path);
            if (result.is_err()) {
                return pfs::Error::other("host fs: " + result.unwrap_err().to_string());
            }
            return result.unwrap();
        }
        return pfs::Error::not_found();
    }

    pfs::Result<std::vector<pfs::FileInfo>> readdir(const std::string& path) override {
        if (path == "/") {
            std::vector<pfs::FileInfo> entries;
            entries.push_back(pfs::FileInfo::file("hello.txt", 21, 0644));
            if (!host_prefix.empty()) {
                entries.push_back(pfs::FileInfo::dir("host", 0755));
            }
            return entries;
        } else if (path == "/host" && !host_prefix.empty()) {
            // Read from host filesystem root
            auto result = pfs::HostFS::readdir(host_prefix);
            if (result.is_err()) {
                return pfs::Error::other("host fs: " + result.unwrap_err().to_string());
            }
            return result.unwrap();
        } else if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            auto result = pfs::HostFS::readdir(full_path);
            if (result.is_err()) {
                return pfs::Error::other("host fs: " + result.unwrap_err().to_string());
            }
            return result.unwrap();
        }
        return pfs::Error::not_found();
    }

    pfs::Result<std::vector<uint8_t>> write(const std::string& path,
                                            const std::vector<uint8_t>& data) override {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            auto result = pfs::HostFS::write(full_path, data);
            if (result.is_err()) {
                return pfs::Error::other("host fs: " + result.unwrap_err().to_string());
            }
            return result.unwrap();
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> create(const std::string& path) override {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            return pfs::HostFS::create(full_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> mkdir(const std::string& path, uint32_t perm) override {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            return pfs::HostFS::mkdir(full_path, perm);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> remove(const std::string& path) override {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            return pfs::HostFS::remove(full_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> remove_all(const std::string& path) override {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem
            std::string host_path = path.substr(5); // Remove "/host"
            std::string full_path = host_prefix + host_path;
            return pfs::HostFS::remove_all(full_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> rename(const std::string& old_path, const std::string& new_path) override {
        if (old_path.rfind("/host/", 0) == 0 && new_path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            // Proxy to host filesystem (both paths must be in host)
            std::string host_old_path = old_path.substr(5); // Remove "/host"
            std::string host_new_path = new_path.substr(5); // Remove "/host"
            std::string full_old_path = host_prefix + host_old_path;
            std::string full_new_path = host_prefix + host_new_path;
            return pfs::HostFS::rename(full_old_path, full_new_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> chmod(const std::string& path, uint32_t mode) override {
        (void)path; (void)mode;
        return pfs::Result<void>(); // no-op
    }
};

// Export the plugin
PFS_EXPORT_PLUGIN(HelloFS)
