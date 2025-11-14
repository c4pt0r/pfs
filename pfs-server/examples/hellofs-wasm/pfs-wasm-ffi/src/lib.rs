//! PFS WASM SDK
//!
//! A safe, high-level SDK for building PFS filesystem plugins in Rust/WASM.
//!
//! # Features
//!
//! - **Safe**: Minimal unsafe code, all hidden behind safe abstractions
//! - **Easy**: Implement a simple trait, export with a macro
//! - **Flexible**: Support for both read-only and read-write filesystems
//! - **Type-safe**: Strong typing for all filesystem operations
//!
//! # Example
//!
//! ```ignore
//! use pfs_wasm_ffi::prelude::*;
//!
//! #[derive(Default)]
//! struct HelloFS;
//!
//! impl ReadOnlyFileSystem for HelloFS {
//!     fn name(&self) -> &str {
//!         "hellofs"
//!     }
//!
//!     fn read(&self, path: &str, offset: i64, size: i64) -> Result<Vec<u8>> {
//!         if path == "/hello" {
//!             let content = b"Hello from WASM!";
//!             let start = offset.min(content.len() as i64) as usize;
//!             let end = if size < 0 {
//!                 content.len()
//!             } else {
//!                 (offset + size).min(content.len() as i64) as usize
//!             };
//!             Ok(content[start..end].to_vec())
//!         } else {
//!             Err(Error::NotFound)
//!         }
//!     }
//!
//!     fn stat(&self, path: &str) -> Result<FileInfo> {
//!         match path {
//!             "/" => Ok(FileInfo::dir("", 0o755)),
//!             "/hello" => Ok(FileInfo::file("hello", 17, 0o644)),
//!             _ => Err(Error::NotFound),
//!         }
//!     }
//!
//!     fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
//!         if path == "/" {
//!             Ok(vec![FileInfo::file("hello", 17, 0o644)])
//!         } else {
//!             Err(Error::NotFound)
//!         }
//!     }
//! }
//!
//! export_plugin!(HelloFS);
//! ```

pub mod ffi;
pub mod filesystem;
pub mod macros;
pub mod memory;
pub mod types;
pub mod host_fs;

// Re-exports for convenience
pub use filesystem::{FileSystem, ReadOnlyFileSystem};
pub use types::{Config, Error, FileInfo, MetaData, Result};
pub use host_fs::HostFS;

/// Prelude module with common imports
pub mod prelude {
    pub use crate::export_plugin;
    pub use crate::filesystem::{FileSystem, ReadOnlyFileSystem};
    pub use crate::types::{Config, Error, FileInfo, MetaData, Result};
    pub use crate::host_fs::HostFS;
}
