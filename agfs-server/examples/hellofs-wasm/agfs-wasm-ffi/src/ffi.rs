//! FFI bridge between Rust and Go for AGFS
//!
//! This module handles the low-level FFI details, converting between
//! C-compatible types and safe Rust types.

use crate::memory::{pack_u64, Buffer, CString};
use crate::types::{Config, Error, FileInfo, Result};
use crate::FileSystem;

/// Convert a Result to an error pointer (null = success)
pub fn result_to_error_ptr<T>(result: Result<T>) -> *mut u8 {
    match result {
        Ok(_) => CString::null(),
        Err(e) => CString::new(&e.to_string()).into_raw(),
    }
}

/// Read config from JSON pointer
pub fn read_config(config_ptr: *const u8) -> Result<Config> {
    if config_ptr.is_null() {
        return Ok(Config {
            inner: serde_json::Map::new(),
        });
    }

    let json_str = unsafe { CString::from_ptr(config_ptr) };

    serde_json::from_str::<serde_json::Value>(&json_str)
        .map(Config::from)
        .map_err(|e| Error::InvalidInput(format!("Invalid config JSON: {}", e)))
}

/// Serialize FileInfo to JSON and return as C string
pub fn fileinfo_to_json_ptr(info: &FileInfo) -> Result<*mut u8> {
    let json = serde_json::to_string(info)
        .map_err(|e| Error::Other(format!("JSON serialization failed: {}", e)))?;

    Ok(CString::new(&json).into_raw())
}

/// Serialize Vec<FileInfo> to JSON array and return as C string
pub fn fileinfo_vec_to_json_ptr(infos: &[FileInfo]) -> Result<*mut u8> {
    let json = serde_json::to_string(infos)
        .map_err(|e| Error::Other(format!("JSON serialization failed: {}", e)))?;

    Ok(CString::new(&json).into_raw())
}

/// Handle fs_read FFI call
pub fn handle_read<FS: FileSystem>(fs: &FS, path_ptr: *const u8, offset: i64, size: i64) -> u64 {
    let path = unsafe { CString::from_ptr(path_ptr) };

    match fs.read(&path, offset, size) {
        Ok(data) => {
            let len = data.len() as u32;
            let buffer = Buffer::from_bytes(&data);
            let ptr = buffer.into_raw() as u32;
            pack_u64(ptr, len)
        }
        Err(_) => 0, // Return 0 to indicate error
    }
}

/// Handle fs_stat FFI call
pub fn handle_stat<FS: FileSystem>(fs: &FS, path_ptr: *const u8) -> u64 {
    let path = unsafe { CString::from_ptr(path_ptr) };

    match fs.stat(&path) {
        Ok(info) => match fileinfo_to_json_ptr(&info) {
            Ok(json_ptr) => pack_u64(json_ptr as u32, 0),
            Err(e) => {
                let err_ptr = CString::new(&e.to_string()).into_raw();
                pack_u64(0, err_ptr as u32)
            }
        },
        Err(e) => {
            let err_ptr = CString::new(&e.to_string()).into_raw();
            pack_u64(0, err_ptr as u32)
        }
    }
}

/// Handle fs_readdir FFI call
pub fn handle_readdir<FS: FileSystem>(fs: &FS, path_ptr: *const u8) -> u64 {
    let path = unsafe { CString::from_ptr(path_ptr) };

    match fs.readdir(&path) {
        Ok(infos) => match fileinfo_vec_to_json_ptr(&infos) {
            Ok(json_ptr) => pack_u64(json_ptr as u32, 0),
            Err(e) => {
                let err_ptr = CString::new(&e.to_string()).into_raw();
                pack_u64(0, err_ptr as u32)
            }
        },
        Err(e) => {
            let err_ptr = CString::new(&e.to_string()).into_raw();
            pack_u64(0, err_ptr as u32)
        }
    }
}

/// Handle fs_write FFI call
pub fn handle_write<FS: FileSystem>(
    fs: &mut FS,
    path_ptr: *const u8,
    data_ptr: *const u8,
    size: usize,
) -> u64 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    let data = unsafe { std::slice::from_raw_parts(data_ptr, size) };

    match fs.write(&path, data) {
        Ok(response) => {
            let len = response.len() as u32;
            let buffer = Buffer::from_bytes(&response);
            let ptr = buffer.into_raw() as u32;
            pack_u64(ptr, len)
        }
        Err(_) => 0, // Return 0 to indicate error
    }
}

/// Handle fs_create FFI call
pub fn handle_create<FS: FileSystem>(fs: &mut FS, path_ptr: *const u8) -> *mut u8 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    result_to_error_ptr(fs.create(&path))
}

/// Handle fs_mkdir FFI call
pub fn handle_mkdir<FS: FileSystem>(fs: &mut FS, path_ptr: *const u8, perm: u32) -> *mut u8 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    result_to_error_ptr(fs.mkdir(&path, perm))
}

/// Handle fs_remove FFI call
pub fn handle_remove<FS: FileSystem>(fs: &mut FS, path_ptr: *const u8) -> *mut u8 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    result_to_error_ptr(fs.remove(&path))
}

/// Handle fs_remove_all FFI call
pub fn handle_remove_all<FS: FileSystem>(fs: &mut FS, path_ptr: *const u8) -> *mut u8 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    result_to_error_ptr(fs.remove_all(&path))
}

/// Handle fs_rename FFI call
pub fn handle_rename<FS: FileSystem>(
    fs: &mut FS,
    old_path_ptr: *const u8,
    new_path_ptr: *const u8,
) -> *mut u8 {
    let old_path = unsafe { CString::from_ptr(old_path_ptr) };
    let new_path = unsafe { CString::from_ptr(new_path_ptr) };
    result_to_error_ptr(fs.rename(&old_path, &new_path))
}

/// Handle fs_chmod FFI call
pub fn handle_chmod<FS: FileSystem>(fs: &mut FS, path_ptr: *const u8, mode: u32) -> *mut u8 {
    let path = unsafe { CString::from_ptr(path_ptr) };
    result_to_error_ptr(fs.chmod(&path, mode))
}
