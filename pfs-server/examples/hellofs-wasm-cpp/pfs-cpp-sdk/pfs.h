#ifndef PFS_H
#define PFS_H

// PFS C++ SDK for WebAssembly Plugin Development
//
// This SDK provides a C++ interface for building PFS filesystem plugins
// that can be compiled to WebAssembly and loaded by pfs-server.
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
//   #include "pfs.h"
//
//   class MyFS : public pfs::FileSystem {
//   public:
//       const char* name() const override { return "myfs"; }
//
//       pfs::Result<pfs::FileInfo> stat(const std::string& path) override {
//           if (path == "/") {
//               return pfs::FileInfo::dir("", 0755);
//           }
//           return pfs::Error::not_found();
//       }
//
//       pfs::Result<std::vector<pfs::FileInfo>> readdir(const std::string& path) override {
//           if (path == "/") {
//               return std::vector<pfs::FileInfo>{
//                   pfs::FileInfo::file("hello.txt", 12, 0644)
//               };
//           }
//           return pfs::Error::not_found();
//       }
//
//       pfs::Result<std::vector<uint8_t>> read(const std::string& path,
//                                               int64_t offset, int64_t size) override {
//           if (path == "/hello.txt") {
//               std::string content = "Hello World\n";
//               return std::vector<uint8_t>(content.begin(), content.end());
//           }
//           return pfs::Error::not_found();
//       }
//   };
//
//   PFS_EXPORT_PLUGIN(MyFS);
//

#include "pfs_types.h"
#include "pfs_ffi.h"
#include "pfs_hostfs.h"
#include "pfs_filesystem.h"
#include "pfs_export.h"

#endif // PFS_H
