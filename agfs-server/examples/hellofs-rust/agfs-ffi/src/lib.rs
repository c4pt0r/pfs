//! # AGFS Plugin SDK
//!
//! A Rust SDK for building filesystem plugins for AGFS Server with clean separation
//! between business logic and FFI concerns.
//!
//! ## Features
//!
//! - Type-safe filesystem interface via the `FileSystem` trait
//! - Automatic FFI bindings generation
//! - Memory-safe FFI boundary layer
//! - Comprehensive error handling
//! - Built-in testing support
//!
//! ## Example
//!
//! ```rust
//! use agfs_ffi::prelude::*;
//!
//! #[derive(Default)]
//! struct MyFS;
//!
//! impl FileSystem for MyFS {
//!     fn name(&self) -> &str {
//!         "my-fs"
//!     }
//!
//!     fn read(&self, _path: &str, _offset: i64, _size: i64) -> Result<String> {
//!         Ok("Hello, World!".to_string())
//!     }
//!
//!     fn stat(&self, _path: &str) -> Result<FileInfo> {
//!         Ok(FileInfo::file("test", 13, 0o644))
//!     }
//!
//!     fn readdir(&self, _path: &str) -> Result<Vec<FileInfo>> {
//!         Ok(vec![])
//!     }
//! }
//!
//! // In a real plugin, you would export FFI functions:
//! // export_plugin!(MyFS);
//! ```

pub mod error;
pub mod ffi;
pub mod filesystem;
pub mod types;

/// Prelude module for convenient imports
pub mod prelude {
    pub use crate::error::{FileSystemError, Result};
    pub use crate::filesystem::FileSystem;
    pub use crate::types::{FileInfo, FileMetadata};
    pub use crate::export_plugin;
}

// Re-export main types
pub use error::{FileSystemError, Result};
pub use filesystem::FileSystem;
pub use types::{FileInfo, FileMetadata};

/// Macro to export a FileSystem implementation as a C-compatible plugin
///
/// This macro generates all the necessary FFI functions to interface with AGFS Server.
///
/// # Example
///
/// ```rust,ignore
/// use agfs_ffi::prelude::*;
///
/// #[derive(Default)]
/// struct MyFS;
/// impl FileSystem for MyFS { /* ... */ }
///
/// export_plugin!(MyFS);
/// ```
#[macro_export]
macro_rules! export_plugin {
    ($fs_type:ty) => {
        use $crate::ffi::PluginWrapper;
        use std::ffi::CString;
        use std::os::raw::{c_char, c_int, c_void};
        use std::ptr;

        #[no_mangle]
        pub extern "C" fn PluginNew() -> *mut c_void {
            let wrapper = Box::new(PluginWrapper::<$fs_type>::new());
            Box::into_raw(wrapper) as *mut c_void
        }

        #[no_mangle]
        pub extern "C" fn PluginFree(plugin: *mut c_void) {
            if !plugin.is_null() {
                unsafe {
                    let _ = Box::from_raw(plugin as *mut PluginWrapper<$fs_type>);
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn PluginName(plugin: *mut c_void) -> *const c_char {
            if plugin.is_null() {
                return ptr::null();
            }
            unsafe {
                let wrapper = &*(plugin as *const PluginWrapper<$fs_type>);
                wrapper.name.as_ptr()
            }
        }

        #[no_mangle]
        pub extern "C" fn PluginValidate(
            plugin: *mut c_void,
            config_json: *const c_char,
        ) -> *const c_char {
            $crate::ffi::plugin_validate::<$fs_type>(plugin, config_json)
        }

        #[no_mangle]
        pub extern "C" fn PluginInitialize(
            plugin: *mut c_void,
            config_json: *const c_char,
        ) -> *const c_char {
            $crate::ffi::plugin_initialize::<$fs_type>(plugin, config_json)
        }

        #[no_mangle]
        pub extern "C" fn PluginShutdown(plugin: *mut c_void) -> *const c_char {
            $crate::ffi::plugin_shutdown::<$fs_type>(plugin)
        }

        #[no_mangle]
        pub extern "C" fn PluginGetReadme(plugin: *mut c_void) -> *const c_char {
            if plugin.is_null() {
                return ptr::null();
            }
            unsafe {
                let wrapper = &*(plugin as *const PluginWrapper<$fs_type>);
                wrapper.readme.as_ptr()
            }
        }

        #[no_mangle]
        pub extern "C" fn FSRead(
            plugin: *mut c_void,
            path: *const c_char,
            offset: i64,
            size: i64,
            out_len: *mut c_int,
        ) -> *const c_char {
            $crate::ffi::fs_read::<$fs_type>(plugin, path, offset, size, out_len)
        }

        #[no_mangle]
        pub extern "C" fn FSStat(
            plugin: *mut c_void,
            path: *const c_char,
        ) -> *mut $crate::ffi::FileInfoC {
            $crate::ffi::fs_stat::<$fs_type>(plugin, path)
        }

        #[no_mangle]
        pub extern "C" fn FSReadDir(
            plugin: *mut c_void,
            path: *const c_char,
            out_count: *mut c_int,
        ) -> *mut $crate::ffi::FileInfoArray {
            $crate::ffi::fs_readdir::<$fs_type>(plugin, path, out_count)
        }

        #[no_mangle]
        pub extern "C" fn FSCreate(plugin: *mut c_void, path: *const c_char) -> *const c_char {
            $crate::ffi::fs_create::<$fs_type>(plugin, path)
        }

        #[no_mangle]
        pub extern "C" fn FSMkdir(
            plugin: *mut c_void,
            path: *const c_char,
            mode: u32,
        ) -> *const c_char {
            $crate::ffi::fs_mkdir::<$fs_type>(plugin, path, mode)
        }

        #[no_mangle]
        pub extern "C" fn FSRemove(plugin: *mut c_void, path: *const c_char) -> *const c_char {
            $crate::ffi::fs_remove::<$fs_type>(plugin, path)
        }

        #[no_mangle]
        pub extern "C" fn FSRemoveAll(
            plugin: *mut c_void,
            path: *const c_char,
        ) -> *const c_char {
            $crate::ffi::fs_remove_all::<$fs_type>(plugin, path)
        }

        #[no_mangle]
        pub extern "C" fn FSWrite(
            plugin: *mut c_void,
            path: *const c_char,
            data: *const c_char,
            data_len: c_int,
        ) -> *const c_char {
            $crate::ffi::fs_write::<$fs_type>(plugin, path, data, data_len)
        }

        #[no_mangle]
        pub extern "C" fn FSRename(
            plugin: *mut c_void,
            old_path: *const c_char,
            new_path: *const c_char,
        ) -> *const c_char {
            $crate::ffi::fs_rename::<$fs_type>(plugin, old_path, new_path)
        }

        #[no_mangle]
        pub extern "C" fn FSChmod(
            plugin: *mut c_void,
            path: *const c_char,
            mode: u32,
        ) -> *const c_char {
            $crate::ffi::fs_chmod::<$fs_type>(plugin, path, mode)
        }
    };
}
