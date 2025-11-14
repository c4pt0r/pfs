#ifndef PFS_TYPES_H
#define PFS_TYPES_H

#include <string>
#include <vector>
#include <map>
#include <optional>
#include <cstdint>
#include <stdexcept>

namespace pfs {

// Forward declarations
class MetaData;

// Error types matching the Rust implementation
enum class ErrorKind {
    NotFound,
    PermissionDenied,
    AlreadyExists,
    IsDirectory,
    NotDirectory,
    ReadOnly,
    InvalidInput,
    Io,
    Other
};

// Error class
class Error {
public:
    ErrorKind kind;
    std::string message;

    Error(ErrorKind k, const std::string& msg = "") : kind(k), message(msg) {}

    static Error not_found() { return Error(ErrorKind::NotFound, "file not found"); }
    static Error permission_denied() { return Error(ErrorKind::PermissionDenied, "permission denied"); }
    static Error already_exists() { return Error(ErrorKind::AlreadyExists, "file already exists"); }
    static Error is_directory() { return Error(ErrorKind::IsDirectory, "is a directory"); }
    static Error not_directory() { return Error(ErrorKind::NotDirectory, "not a directory"); }
    static Error read_only() { return Error(ErrorKind::ReadOnly, "read-only filesystem"); }
    static Error invalid_input(const std::string& msg) { return Error(ErrorKind::InvalidInput, msg); }
    static Error io(const std::string& msg) { return Error(ErrorKind::Io, msg); }
    static Error other(const std::string& msg) { return Error(ErrorKind::Other, msg); }

    std::string to_string() const {
        if (!message.empty()) {
            return message;
        }
        switch (kind) {
            case ErrorKind::NotFound: return "file not found";
            case ErrorKind::PermissionDenied: return "permission denied";
            case ErrorKind::AlreadyExists: return "file already exists";
            case ErrorKind::IsDirectory: return "is a directory";
            case ErrorKind::NotDirectory: return "not a directory";
            case ErrorKind::ReadOnly: return "read-only filesystem";
            default: return "unknown error";
        }
    }
};

// Result type (similar to Rust's Result)
template<typename T>
class Result {
private:
    bool is_ok_;
    union {
        T value_;
        Error error_;
    };

public:
    // Success constructor
    Result(const T& val) : is_ok_(true), value_(val) {}
    Result(T&& val) : is_ok_(true), value_(std::move(val)) {}

    // Error constructor
    Result(const Error& err) : is_ok_(false), error_(err) {}
    Result(Error&& err) : is_ok_(false), error_(std::move(err)) {}

    // Copy constructor
    Result(const Result& other) : is_ok_(other.is_ok_) {
        if (is_ok_) {
            new (&value_) T(other.value_);
        } else {
            new (&error_) Error(other.error_);
        }
    }

    // Move constructor
    Result(Result&& other) noexcept : is_ok_(other.is_ok_) {
        if (is_ok_) {
            new (&value_) T(std::move(other.value_));
        } else {
            new (&error_) Error(std::move(other.error_));
        }
    }

    ~Result() {
        if (is_ok_) {
            value_.~T();
        } else {
            error_.~Error();
        }
    }

    bool is_ok() const { return is_ok_; }
    bool is_err() const { return !is_ok_; }

    T& unwrap() {
        if (!is_ok_) {
            // In WASM without exceptions, we can't throw
            // This is a programming error if called on Err
            __builtin_trap();
        }
        return value_;
    }

    const T& unwrap() const {
        if (!is_ok_) {
            __builtin_trap();
        }
        return value_;
    }

    Error& unwrap_err() {
        if (is_ok_) {
            __builtin_trap();
        }
        return error_;
    }

    const Error& unwrap_err() const {
        if (is_ok_) {
            __builtin_trap();
        }
        return error_;
    }
};

// Specialization for void
template<>
class Result<void> {
private:
    bool is_ok_;
    Error error_;

public:
    Result() : is_ok_(true), error_(ErrorKind::Other) {}
    Result(const Error& err) : is_ok_(false), error_(err) {}
    Result(Error&& err) : is_ok_(false), error_(std::move(err)) {}

    bool is_ok() const { return is_ok_; }
    bool is_err() const { return !is_ok_; }

    void unwrap() const {
        if (!is_ok_) {
            __builtin_trap();
        }
    }

    const Error& unwrap_err() const {
        if (is_ok_) {
            __builtin_trap();
        }
        return error_;
    }
};

// Metadata structure
class MetaData {
public:
    std::string name;
    std::string type;
    std::string content; // JSON string

    MetaData() = default;
    MetaData(const std::string& n, const std::string& t, const std::string& c = "{}")
        : name(n), type(t), content(c) {}
};

// File information structure
class FileInfo {
public:
    std::string name;
    int64_t size;
    uint32_t mode;
    int64_t mod_time;
    bool is_dir;
    std::optional<MetaData> meta;

    FileInfo() : size(0), mode(0), mod_time(0), is_dir(false) {}

    // Helper constructors
    static FileInfo file(const std::string& name, int64_t size, uint32_t mode) {
        FileInfo info;
        info.name = name;
        info.size = size;
        info.mode = mode;
        info.mod_time = 0;
        info.is_dir = false;
        return info;
    }

    static FileInfo dir(const std::string& name, uint32_t mode) {
        FileInfo info;
        info.name = name;
        info.size = 0;
        info.mode = mode;
        info.mod_time = 0;
        info.is_dir = true;
        return info;
    }

    FileInfo& with_meta(const MetaData& m) {
        meta = m;
        return *this;
    }

    FileInfo& with_mod_time(int64_t timestamp) {
        mod_time = timestamp;
        return *this;
    }
};

// Configuration class
class Config {
public:
    std::map<std::string, std::string> values;

    const char* get_str(const char* key) const {
        auto it = values.find(key);
        if (it != values.end()) {
            return it->second.c_str();
        }
        return nullptr;
    }

    int64_t get_i64(const char* key, int64_t default_value = 0) const {
        auto it = values.find(key);
        if (it != values.end()) {
            return std::stoll(it->second);
        }
        return default_value;
    }

    bool get_bool(const char* key, bool default_value = false) const {
        auto it = values.find(key);
        if (it != values.end()) {
            return it->second == "true" || it->second == "1";
        }
        return default_value;
    }

    bool contains(const char* key) const {
        return values.find(key) != values.end();
    }
};

} // namespace pfs

#endif // PFS_TYPES_H
