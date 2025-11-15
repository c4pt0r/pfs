// HelloFS WASM - C++ implementation
//
// A simple filesystem plugin demonstrating PFS C++ SDK usage
// Returns a single file with "Hello World" content
// Also demonstrates accessing the host filesystem

#include "../pfs-cpp-sdk/pfs.h"

class HelloFS : public pfs::FileSystem {
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
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::read(host_path, offset, size);
        }
        return pfs::Error::not_found();
    }

    pfs::Result<pfs::FileInfo> stat(const std::string& path) override {
        if (path == "/") {
            return pfs::FileInfo::dir("", 0755);
        }
        if (path == "/hello.txt") {
            return pfs::FileInfo::file("hello.txt", 21, 0644);
        }
        if (path == "/host" && !host_prefix.empty()) {
            return pfs::FileInfo::dir("host", 0755);
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::stat(host_path);
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
        }
        if (path == "/host" && !host_prefix.empty()) {
            return pfs::HostFS::readdir(host_prefix);
        }
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::readdir(host_path);
        }
        return pfs::Error::not_found();
    }

    pfs::Result<std::vector<uint8_t>> write(const std::string& path,
                                            const std::vector<uint8_t>& data) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::write(host_path, data);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> create(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::create(host_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> mkdir(const std::string& path, uint32_t perm) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::mkdir(host_path, perm);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> remove(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::remove(host_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> remove_all(const std::string& path) override {
        auto host_path = get_host_path(path);
        if (!host_path.empty()) {
            return pfs::HostFS::remove_all(host_path);
        }
        return pfs::Error::permission_denied();
    }

    pfs::Result<void> rename(const std::string& old_path, const std::string& new_path) override {
        auto host_old = get_host_path(old_path);
        auto host_new = get_host_path(new_path);
        if (!host_old.empty() && !host_new.empty()) {
            return pfs::HostFS::rename(host_old, host_new);
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
