// HelloFS WASM - C++ implementation
//
// A simple filesystem plugin demonstrating AGFS C++ SDK usage
// Returns a single file with "Hello World" content
// Also demonstrates accessing the host filesystem

#include "../agfs-cpp-sdk/agfs.h"

class HelloFS : public agfs::FileSystem {
private:
    std::string host_prefix;

    // Convert /host/xxx to actual host path, or return empty if not host path
    std::string get_host_path(const std::string& path) const {
        if (path.rfind("/host/", 0) == 0 && !host_prefix.empty()) {
            return host_prefix + path.substr(5);  // Remove "/host", add prefix
        }
        return "";
    }

public:
    const char* name() const override {
        return "hellofs-wasm-cpp";
    }

    const char* readme() const override {
        return "HelloFS WASM (C++) - Demonstrates host filesystem access\n"
               " - /hello.txt - Returns 'Hello World from C++'\n"
               " - /host/* - Proxies to host filesystem (if configured host_prefix)";
    }

    agfs::Result<void> initialize(const agfs::Config& config) override {
        // Get optional host_prefix from config
        const char* prefix = config.get_str("host_prefix");
        if (prefix != nullptr) {
            host_prefix = prefix;
        }
        return agfs::Result<void>();
    }

    agfs::Result<std::vector<uint8_t>> read(const std::string& path,
                                           int64_t offset, int64_t size) override {
        if (path == "/hello.txt") {
            std::string content = "Hello World from C++\n";
            std::vector<uint8_t> data(content.begin(), content.end());
            return data;
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::read(host_path, offset, size);
        }
        return agfs::Error::not_found();
    }

    agfs::Result<agfs::FileInfo> stat(const std::string& path) override {
        if (path == "/") {
            return agfs::FileInfo::dir("", 0755);
        }
        if (path == "/hello.txt") {
            return agfs::FileInfo::file("hello.txt", 21, 0644);
        }
        if (path == "/host" && !host_prefix.empty()) {
            return agfs::FileInfo::dir("host", 0755);
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::stat(host_path);
        }
        return agfs::Error::not_found();
    }

    agfs::Result<std::vector<agfs::FileInfo>> readdir(const std::string& path) override {
        if (path == "/") {
            std::vector<agfs::FileInfo> entries;
            entries.push_back(agfs::FileInfo::file("hello.txt", 21, 0644));
            if (!host_prefix.empty()) {
                entries.push_back(agfs::FileInfo::dir("host", 0755));
            }
            return entries;
        }
        if (path == "/host" && !host_prefix.empty()) {
            return agfs::HostFS::readdir(host_prefix);
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::readdir(host_path);
        }
        return agfs::Error::not_found();
    }

    agfs::Result<std::vector<uint8_t>> write(const std::string& path,
                                            const std::vector<uint8_t>& data) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::write(host_path, data);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> create(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::create(host_path);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> mkdir(const std::string& path, uint32_t perm) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::mkdir(host_path, perm);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> remove(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::remove(host_path);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> remove_all(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return agfs::HostFS::remove_all(host_path);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> rename(const std::string& old_path, const std::string& new_path) override {
        auto host_old = get_host_path(old_path);
        auto host_new = get_host_path(new_path);
        if (!host_old.empty() && !host_new.empty()) {
            return agfs::HostFS::rename(host_old, host_new);
        }
        return agfs::Error::permission_denied();
    }

    agfs::Result<void> chmod(const std::string& path, uint32_t mode) override {
        (void)path; (void)mode;
        return agfs::Result<void>(); // no-op
    }
};

// Export the plugin
AGFS_EXPORT_PLUGIN(HelloFS)
