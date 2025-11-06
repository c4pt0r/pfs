//! FFI boundary layer
//!
//! This module handles all C interop safely. All unsafe code is contained here.

use crate::filesystem::FileSystem;
use crate::types::FileInfo;
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_int, c_void};
use std::ptr;
use std::sync::Mutex;

/// C-compatible FileInfo structure
#[repr(C)]
pub struct FileInfoC {
    name: *const c_char,
    size: i64,
    mode: u32,
    mod_time: i64,
    is_dir: c_int,
    meta_name: *const c_char,
    meta_type: *const c_char,
    meta_content: *const c_char,
}

/// C-compatible array of FileInfo structures
#[repr(C)]
pub struct FileInfoArray {
    items: *mut FileInfoC,
    count: c_int,
}

/// Convert FileInfo to C representation
impl From<&FileInfo> for FileInfoC {
    fn from(info: &FileInfo) -> Self {
        FileInfoC {
            name: CString::new(info.name.as_str())
                .expect("name contains null byte")
                .into_raw(),
            size: info.size,
            mode: info.mode,
            mod_time: info.mod_time,
            is_dir: if info.is_dir { 1 } else { 0 },
            meta_name: CString::new(info.metadata.name.as_str())
                .expect("meta_name contains null byte")
                .into_raw(),
            meta_type: CString::new(info.metadata.file_type.as_str())
                .expect("meta_type contains null byte")
                .into_raw(),
            meta_content: CString::new(info.metadata.content.as_str())
                .expect("meta_content contains null byte")
                .into_raw(),
        }
    }
}

impl Drop for FileInfoC {
    fn drop(&mut self) {
        // Clean up all allocated strings
        unsafe {
            if !self.name.is_null() {
                let _ = CString::from_raw(self.name as *mut c_char);
            }
            if !self.meta_name.is_null() {
                let _ = CString::from_raw(self.meta_name as *mut c_char);
            }
            if !self.meta_type.is_null() {
                let _ = CString::from_raw(self.meta_type as *mut c_char);
            }
            if !self.meta_content.is_null() {
                let _ = CString::from_raw(self.meta_content as *mut c_char);
            }
        }
    }
}

/// Wrapper to make FileSystem thread-safe
pub struct PluginWrapper<T: FileSystem> {
    pub fs: Mutex<T>,
    pub name: CString,
    pub readme: CString,
}

impl<T: FileSystem> PluginWrapper<T> {
    pub fn new() -> Self {
        let fs = T::default();
        let name = CString::new(fs.name()).expect("plugin name contains null byte");
        let readme = CString::new(fs.readme()).expect("readme contains null byte");

        Self {
            fs: Mutex::new(fs),
            name,
            readme,
        }
    }
}

/// Helper to safely convert C string to Rust str
unsafe fn c_str_to_str<'a>(ptr: *const c_char) -> Result<&'a str, &'static str> {
    if ptr.is_null() {
        return Err("null pointer");
    }
    CStr::from_ptr(ptr).to_str().map_err(|_| "invalid UTF-8")
}

/// Helper to create error C string
fn error_to_c_string(msg: &str) -> *const c_char {
    CString::new(msg)
        .expect("error message contains null byte")
        .into_raw()
}

/// Success indicator (NULL in C API)
fn success() -> *const c_char {
    ptr::null()
}

// Helper functions used by the export_plugin! macro

pub fn plugin_validate<T: FileSystem>(
    plugin: *mut c_void,
    config_json: *const c_char,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let config = unsafe {
        match c_str_to_str(config_json) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.validate(config) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn plugin_initialize<T: FileSystem>(
    plugin: *mut c_void,
    config_json: *const c_char,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let config = unsafe {
        match c_str_to_str(config_json) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let mut fs = wrapper.fs.lock().unwrap();
        match fs.initialize(config) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn plugin_shutdown<T: FileSystem>(plugin: *mut c_void) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let mut fs = wrapper.fs.lock().unwrap();
        match fs.shutdown() {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_read<T: FileSystem>(
    plugin: *mut c_void,
    path: *const c_char,
    offset: i64,
    size: i64,
    out_len: *mut c_int,
) -> *const c_char {
    if plugin.is_null() {
        unsafe {
            *out_len = -1;
        }
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => {
                *out_len = -1;
                return error_to_c_string(e);
            }
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.read(path_str, offset, size) {
            Ok(content) => {
                *out_len = content.len() as c_int;
                CString::new(content)
                    .expect("content contains null byte")
                    .into_raw()
            }
            Err(e) => {
                *out_len = -1;
                error_to_c_string(&e.to_string())
            }
        }
    }
}

pub fn fs_stat<T: FileSystem>(plugin: *mut c_void, path: *const c_char) -> *mut FileInfoC {
    if plugin.is_null() {
        return ptr::null_mut();
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(_) => return ptr::null_mut(),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.stat(path_str) {
            Ok(info) => Box::into_raw(Box::new(FileInfoC::from(&info))),
            Err(_) => ptr::null_mut(),
        }
    }
}

pub fn fs_readdir<T: FileSystem>(
    plugin: *mut c_void,
    path: *const c_char,
    out_count: *mut c_int,
) -> *mut FileInfoArray {
    if plugin.is_null() {
        unsafe {
            *out_count = -1;
        }
        return ptr::null_mut();
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(_) => {
                *out_count = -1;
                return ptr::null_mut();
            }
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.readdir(path_str) {
            Ok(files) => {
                let count = files.len();
                let items: Vec<FileInfoC> = files.iter().map(FileInfoC::from).collect();

                let mut items_vec = items.into_boxed_slice();
                let items_ptr = items_vec.as_mut_ptr();
                std::mem::forget(items_vec);

                let array = Box::new(FileInfoArray {
                    items: items_ptr,
                    count: count as c_int,
                });

                *out_count = count as c_int;
                Box::into_raw(array)
            }
            Err(_) => {
                *out_count = -1;
                ptr::null_mut()
            }
        }
    }
}

pub fn fs_create<T: FileSystem>(plugin: *mut c_void, path: *const c_char) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.create(path_str) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_mkdir<T: FileSystem>(
    plugin: *mut c_void,
    path: *const c_char,
    mode: u32,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.mkdir(path_str, mode) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_remove<T: FileSystem>(plugin: *mut c_void, path: *const c_char) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.remove(path_str) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_remove_all<T: FileSystem>(plugin: *mut c_void, path: *const c_char) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.remove_all(path_str) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_write<T: FileSystem>(
    plugin: *mut c_void,
    path: *const c_char,
    data: *const c_char,
    data_len: c_int,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    let data_slice = unsafe {
        if data.is_null() || data_len < 0 {
            return error_to_c_string("invalid data");
        }
        std::slice::from_raw_parts(data as *const u8, data_len as usize)
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.write(path_str, data_slice) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_rename<T: FileSystem>(
    plugin: *mut c_void,
    old_path: *const c_char,
    new_path: *const c_char,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let old_path_str = unsafe {
        match c_str_to_str(old_path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    let new_path_str = unsafe {
        match c_str_to_str(new_path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.rename(old_path_str, new_path_str) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}

pub fn fs_chmod<T: FileSystem>(
    plugin: *mut c_void,
    path: *const c_char,
    mode: u32,
) -> *const c_char {
    if plugin.is_null() {
        return error_to_c_string("plugin is null");
    }

    let path_str = unsafe {
        match c_str_to_str(path) {
            Ok(s) => s,
            Err(e) => return error_to_c_string(e),
        }
    };

    unsafe {
        let wrapper = &*(plugin as *const PluginWrapper<T>);
        let fs = wrapper.fs.lock().unwrap();
        match fs.chmod(path_str, mode) {
            Ok(_) => success(),
            Err(e) => error_to_c_string(&e.to_string()),
        }
    }
}
