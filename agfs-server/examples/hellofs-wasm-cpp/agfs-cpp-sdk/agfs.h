#ifndef AGFS_H
#define AGFS_H

// AGFS C++ SDK for WebAssembly Plugin Development
//
// This SDK provides a C++ interface for building AGFS filesystem plugins
// that can be compiled to WebAssembly and loaded by agfs-server.
//
// Features:
// - Type-safe C++ API
// - Easy-to-use FileSystem base class
// - Host filesystem access via HostFS
// - Automatic FFI handling
// - Simple export macro
//
// Example usage:
//
//   #include "agfs.h"
//
//   class MyFS : public agfs::FileSystem {
//   public:
//       const char* name() const override { return "myfs"; }
//
//       agfs::Result<agfs::FileInfo> stat(const std::string& path) override {
//           if (path == "/") {
//               return agfs::FileInfo::dir("", 0755);
//           }
//           return agfs::Error::not_found();
//       }
//
//       agfs::Result<std::vector<agfs::FileInfo>> readdir(const std::string& path) override {
//           if (path == "/") {
//               return std::vector<agfs::FileInfo>{
//                   agfs::FileInfo::file("hello.txt", 12, 0644)
//               };
//           }
//           return agfs::Error::not_found();
//       }
//
//       agfs::Result<std::vector<uint8_t>> read(const std::string& path,
//                                               int64_t offset, int64_t size) override {
//           if (path == "/hello.txt") {
//               std::string content = "Hello World\n";
//               return std::vector<uint8_t>(content.begin(), content.end());
//           }
//           return agfs::Error::not_found();
//       }
//   };
//
//   AGFS_EXPORT_PLUGIN(MyFS);
//

#include "agfs_types.h"
#include "agfs_ffi.h"
#include "agfs_hostfs.h"
#include "agfs_filesystem.h"
#include "agfs_export.h"

#endif // AGFS_H
