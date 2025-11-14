#ifndef PFS_FFI_H
#define PFS_FFI_H

#include "pfs_types.h"
#include "json.hpp"
#include <cstring>
#include <cstdlib>

using json = nlohmann::json;

namespace pfs {
namespace ffi {

// Memory management functions
inline void* wasm_malloc(size_t size) {
    return malloc(size);
}

inline void wasm_free(void* ptr) {
    free(ptr);
}

// String helpers
inline char* copy_string(const std::string& str) {
    if (str.empty()) {
        return nullptr;
    }
    char* buf = (char*)wasm_malloc(str.length() + 1);
    std::memcpy(buf, str.c_str(), str.length());
    buf[str.length()] = '\0';
    return buf;
}

inline std::string read_string(const char* ptr) {
    if (ptr == nullptr) {
        return "";
    }
    return std::string(ptr);
}

// Pack two u32 into u64
inline uint64_t pack_u64(uint32_t low, uint32_t high) {
    return ((uint64_t)high << 32) | (uint64_t)low;
}

// Unpack u64 to two u32
inline void unpack_u64(uint64_t packed, uint32_t& low, uint32_t& high) {
    low = (uint32_t)(packed & 0xFFFFFFFF);
    high = (uint32_t)((packed >> 32) & 0xFFFFFFFF);
}

// JSON parsing helpers using nlohmann/json
class JsonParser {
public:
    static Config parse_config(const char* json_str) {
        Config config;
        if (json_str == nullptr) {
            return config;
        }

        auto j = json::parse(json_str, nullptr, false);
        if (j.is_discarded() || !j.is_object()) {
            return config;
        }

        for (auto& [key, value] : j.items()) {
            if (value.is_string()) {
                config.values[key] = value.get<std::string>();
            } else if (value.is_number()) {
                config.values[key] = std::to_string(value.get<double>());
            } else if (value.is_boolean()) {
                config.values[key] = value.get<bool>() ? "true" : "false";
            }
        }

        return config;
    }

    static std::string serialize_fileinfo(const FileInfo& info) {
        json j = {
            {"Name", info.name},
            {"Size", info.size},
            {"Mode", info.mode},
            {"ModTime", "0001-01-01T00:00:00Z"},
            {"IsDir", info.is_dir}
        };

        if (info.meta.has_value()) {
            auto meta_content = json::parse(info.meta->content, nullptr, false);
            j["Meta"] = {
                {"Name", info.meta->name},
                {"Type", info.meta->type},
                {"Content", meta_content.is_discarded() ? json{} : meta_content}
            };
        }

        return j.dump();
    }

    static std::string serialize_fileinfo_array(const std::vector<FileInfo>& infos) {
        json j = json::array();
        for (const auto& info : infos) {
            j.push_back({
                {"Name", info.name},
                {"Size", info.size},
                {"Mode", info.mode},
                {"ModTime", "0001-01-01T00:00:00Z"},
                {"IsDir", info.is_dir}
            });
        }
        return j.dump();
    }

    static FileInfo parse_fileinfo(const std::string& json_str) {
        FileInfo info;

        auto j = json::parse(json_str, nullptr, false);
        if (j.is_discarded() || !j.is_object()) {
            return info;
        }

        info.name = j.value("Name", "");
        info.size = j.value("Size", 0);
        info.mode = j.value("Mode", 0);
        info.is_dir = j.value("IsDir", false);

        return info;
    }

    static std::vector<FileInfo> parse_fileinfo_array(const std::string& json_str) {
        std::vector<FileInfo> infos;

        auto j = json::parse(json_str, nullptr, false);
        if (j.is_discarded() || !j.is_array()) {
            return infos;
        }

        for (const auto& item : j) {
            if (!item.is_object()) continue;

            FileInfo info;
            info.name = item.value("Name", "");
            info.size = item.value("Size", 0);
            info.mode = item.value("Mode", 0);
            info.is_dir = item.value("IsDir", false);
            infos.push_back(info);
        }

        return infos;
    }
};

} // namespace ffi
} // namespace pfs

#endif // PFS_FFI_H
