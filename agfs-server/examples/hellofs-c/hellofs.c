#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <stdint.h>

// Plugin structure
typedef struct {
    int initialized;
} HelloFSPlugin;

// FileInfo structure matching Go's FileInfoC
typedef struct {
    const char* Name;
    int64_t Size;
    uint32_t Mode;
    int64_t ModTime;
    int32_t IsDir;
    const char* MetaName;
    const char* MetaType;
    const char* MetaContent;
} FileInfoC;

// FileInfo array structure
typedef struct {
    FileInfoC* Items;
    int Count;
} FileInfoArray;

// Plugin lifecycle functions
void* PluginNew() {
    HelloFSPlugin* plugin = (HelloFSPlugin*)malloc(sizeof(HelloFSPlugin));
    if (plugin != NULL) {
        plugin->initialized = 0;
    }
    return plugin;
}

void PluginFree(void* plugin) {
    if (plugin != NULL) {
        free(plugin);
    }
}

const char* PluginName(void* plugin) {
    return "hellofs-c";
}

const char* PluginValidate(void* plugin, const char* config_json) {
    // No validation needed for this simple plugin
    return NULL; // NULL means no error
}

const char* PluginInitialize(void* plugin, const char* config_json) {
    if (plugin == NULL) {
        return "plugin is null";
    }
    HelloFSPlugin* p = (HelloFSPlugin*)plugin;
    p->initialized = 1;
    return NULL; // NULL means success
}

const char* PluginShutdown(void* plugin) {
    if (plugin != NULL) {
        HelloFSPlugin* p = (HelloFSPlugin*)plugin;
        p->initialized = 0;
    }
    return NULL;
}

const char* PluginGetReadme(void* plugin) {
    return "# HelloFS C Plugin\n\n"
           "A simple read-only filesystem plugin written in C.\n\n"
           "## Features\n"
           "- Single file: /hello containing 'Hello from C dynamic library!'\n"
           "- Demonstrates C plugin interface for agfs-server\n";
}

// File system functions
const char* FSRead(void* plugin, const char* path, int64_t offset, int64_t size, int* out_len) {
    if (strcmp(path, "/hello") == 0) {
        const char* content = "Hello from C dynamic library!\n";
        int content_len = strlen(content);

        // Handle offset
        if (offset >= content_len) {
            *out_len = 0;
            return "";
        }

        // Calculate actual read length
        int64_t remaining = content_len - offset;
        int64_t read_len = (size > 0 && size < remaining) ? size : remaining;

        // Allocate and copy data
        char* result = (char*)malloc(read_len + 1);
        memcpy(result, content + offset, read_len);
        result[read_len] = '\0';

        *out_len = read_len;
        return result;
    }

    *out_len = -1;
    return "file not found";
}

FileInfoC* FSStat(void* plugin, const char* path) {
    FileInfoC* info = (FileInfoC*)malloc(sizeof(FileInfoC));
    time_t now = time(NULL);

    if (strcmp(path, "/") == 0) {
        info->Name = strdup("");
        info->Size = 0;
        info->Mode = 0755;
        info->ModTime = now;
        info->IsDir = 1;
        info->MetaName = strdup("hellofs-c");
        info->MetaType = strdup("directory");
        info->MetaContent = strdup("{}");
        return info;
    } else if (strcmp(path, "/hello") == 0) {
        const char* content = "Hello from C dynamic library!\n";
        info->Name = strdup("hello");
        info->Size = strlen(content);
        info->Mode = 0644;
        info->ModTime = now;
        info->IsDir = 0;
        info->MetaName = strdup("hellofs-c");
        info->MetaType = strdup("text");
        info->MetaContent = strdup("{\"language\":\"c\"}");
        return info;
    }

    free(info);
    return NULL;
}

FileInfoArray* FSReadDir(void* plugin, const char* path, int* out_count) {
    if (strcmp(path, "/") == 0) {
        FileInfoArray* result = (FileInfoArray*)malloc(sizeof(FileInfoArray));
        result->Count = 1;
        result->Items = (FileInfoC*)malloc(sizeof(FileInfoC));

        time_t now = time(NULL);
        const char* content = "Hello from C dynamic library!\n";

        result->Items[0].Name = strdup("hello");
        result->Items[0].Size = strlen(content);
        result->Items[0].Mode = 0644;
        result->Items[0].ModTime = now;
        result->Items[0].IsDir = 0;
        result->Items[0].MetaName = strdup("hellofs-c");
        result->Items[0].MetaType = strdup("text");
        result->Items[0].MetaContent = strdup("{\"language\":\"c\"}");

        *out_count = 1;
        return result;
    }

    *out_count = -1;
    return NULL;
}

// Stub implementations for other required functions
const char* FSCreate(void* plugin, const char* path) {
    return "operation not supported: read-only filesystem";
}

const char* FSMkdir(void* plugin, const char* path, uint32_t mode) {
    return "operation not supported: read-only filesystem";
}

const char* FSRemove(void* plugin, const char* path) {
    return "operation not supported: read-only filesystem";
}

const char* FSRemoveAll(void* plugin, const char* path) {
    return "operation not supported: read-only filesystem";
}

const char* FSWrite(void* plugin, const char* path, const char* data, int data_len) {
    return "operation not supported: read-only filesystem";
}

const char* FSRename(void* plugin, const char* old_path, const char* new_path) {
    return "operation not supported: read-only filesystem";
}

const char* FSChmod(void* plugin, const char* path, uint32_t mode) {
    return "operation not supported: read-only filesystem";
}
