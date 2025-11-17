#ifndef AGFS_EXPORT_H
#define AGFS_EXPORT_H

#include "agfs_types.h"
#include "agfs_ffi.h"
#include "agfs_filesystem.h"

namespace agfs {
namespace internal {

// Global plugin instance
template<typename T>
struct PluginInstance {
    static T* instance;
};

template<typename T>
T* PluginInstance<T>::instance = nullptr;

} // namespace internal
} // namespace agfs

// Export a FileSystem implementation as a WASM plugin
#define AGFS_EXPORT_PLUGIN(PluginType) \
    static PluginType* g_plugin_instance = nullptr; \
    \
    extern "C" { \
    \
    __attribute__((export_name("plugin_new"))) \
    int plugin_new() { \
        g_plugin_instance = new PluginType(); \
        return 1; \
    } \
    \
    __attribute__((export_name("plugin_name"))) \
    char* plugin_name() { \
        if (!g_plugin_instance) return nullptr; \
        return agfs::ffi::copy_string(g_plugin_instance->name()); \
    } \
    \
    __attribute__((export_name("plugin_get_readme"))) \
    char* plugin_get_readme() { \
        if (!g_plugin_instance) return nullptr; \
        return agfs::ffi::copy_string(g_plugin_instance->readme()); \
    } \
    \
    __attribute__((export_name("plugin_validate"))) \
    char* plugin_validate(const char* config_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        agfs::Config config = agfs::ffi::JsonParser::parse_config(config_ptr); \
        auto result = g_plugin_instance->validate(config); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("plugin_initialize"))) \
    char* plugin_initialize(const char* config_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        agfs::Config config = agfs::ffi::JsonParser::parse_config(config_ptr); \
        auto result = g_plugin_instance->initialize(config); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("plugin_shutdown"))) \
    char* plugin_shutdown() { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        auto result = g_plugin_instance->shutdown(); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_read"))) \
    uint64_t fs_read(const char* path_ptr, int64_t offset, int64_t size) { \
        if (!g_plugin_instance) return 0; \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->read(path, offset, size); \
        if (result.is_err()) { \
            return 0; \
        } \
        auto& data = result.unwrap(); \
        uint32_t len = data.size(); \
        uint8_t* buf = (uint8_t*)agfs::ffi::wasm_malloc(len); \
        std::memcpy(buf, data.data(), len); \
        return agfs::ffi::pack_u64((uint32_t)buf, len); \
    } \
    \
    __attribute__((export_name("fs_stat"))) \
    uint64_t fs_stat(const char* path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::pack_u64(0, (uint32_t)agfs::ffi::copy_string("not initialized")); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->stat(path); \
        if (result.is_err()) { \
            char* err_ptr = agfs::ffi::copy_string(result.unwrap_err().to_string()); \
            return agfs::ffi::pack_u64(0, (uint32_t)err_ptr); \
        } \
        std::string json = agfs::ffi::JsonParser::serialize_fileinfo(result.unwrap()); \
        char* json_ptr = agfs::ffi::copy_string(json); \
        return agfs::ffi::pack_u64((uint32_t)json_ptr, 0); \
    } \
    \
    __attribute__((export_name("fs_readdir"))) \
    uint64_t fs_readdir(const char* path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::pack_u64(0, (uint32_t)agfs::ffi::copy_string("not initialized")); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->readdir(path); \
        if (result.is_err()) { \
            char* err_ptr = agfs::ffi::copy_string(result.unwrap_err().to_string()); \
            return agfs::ffi::pack_u64(0, (uint32_t)err_ptr); \
        } \
        std::string json = agfs::ffi::JsonParser::serialize_fileinfo_array(result.unwrap()); \
        char* json_ptr = agfs::ffi::copy_string(json); \
        return agfs::ffi::pack_u64((uint32_t)json_ptr, 0); \
    } \
    \
    __attribute__((export_name("fs_write"))) \
    uint64_t fs_write(const char* path_ptr, const uint8_t* data_ptr, size_t size) { \
        if (!g_plugin_instance) return 0; \
        std::string path = agfs::ffi::read_string(path_ptr); \
        std::vector<uint8_t> data(data_ptr, data_ptr + size); \
        auto result = g_plugin_instance->write(path, data); \
        if (result.is_err()) { \
            return 0; \
        } \
        auto& response = result.unwrap(); \
        uint32_t len = response.size(); \
        uint8_t* buf = (uint8_t*)agfs::ffi::wasm_malloc(len); \
        std::memcpy(buf, response.data(), len); \
        return agfs::ffi::pack_u64((uint32_t)buf, len); \
    } \
    \
    __attribute__((export_name("fs_create"))) \
    char* fs_create(const char* path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->create(path); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_mkdir"))) \
    char* fs_mkdir(const char* path_ptr, uint32_t perm) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->mkdir(path, perm); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_remove"))) \
    char* fs_remove(const char* path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->remove(path); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_remove_all"))) \
    char* fs_remove_all(const char* path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->remove_all(path); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_rename"))) \
    char* fs_rename(const char* old_path_ptr, const char* new_path_ptr) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string old_path = agfs::ffi::read_string(old_path_ptr); \
        std::string new_path = agfs::ffi::read_string(new_path_ptr); \
        auto result = g_plugin_instance->rename(old_path, new_path); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    __attribute__((export_name("fs_chmod"))) \
    char* fs_chmod(const char* path_ptr, uint32_t mode) { \
        if (!g_plugin_instance) return agfs::ffi::copy_string("not initialized"); \
        std::string path = agfs::ffi::read_string(path_ptr); \
        auto result = g_plugin_instance->chmod(path, mode); \
        if (result.is_err()) { \
            return agfs::ffi::copy_string(result.unwrap_err().to_string()); \
        } \
        return nullptr; \
    } \
    \
    } /* extern "C" */

#endif // AGFS_EXPORT_H
