//! Host filesystem access from WASM
//!
//! This module provides access to the host filesystem exposed by agfs-server.
//! WASM plugins can use this to access files on the host system.

use crate::types::{Error, FileInfo, Result};
use std::ffi::CString;

// Import host functions from the "env" module
#[link(wasm_import_module = "env")]
extern "C" {
    fn host_fs_read(path: *const u8, offset: i64, size: i64) -> u64;
    fn host_fs_write(path: *const u8, data: *const u8, len: u32) -> u64;
    fn host_fs_stat(path: *const u8) -> u64;
    fn host_fs_readdir(path: *const u8) -> u64;
    fn host_fs_create(path: *const u8) -> u32;
    fn host_fs_mkdir(path: *const u8, perm: u32) -> u32;
    fn host_fs_remove(path: *const u8) -> u32;
    fn host_fs_remove_all(path: *const u8) -> u32;
    fn host_fs_rename(old_path: *const u8, new_path: *const u8) -> u32;
    fn host_fs_chmod(path: *const u8, mode: u32) -> u32;
}

/// HostFS provides access to the host filesystem from WASM
pub struct HostFS;

impl HostFS {
    /// Read data from a file on the host filesystem
    pub fn read(path: &str, offset: i64, size: i64) -> Result<Vec<u8>> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let result = host_fs_read(path_c.as_ptr() as *const u8, offset, size);

            // Unpack: lower 32 bits = pointer, upper 32 bits = size
            let data_ptr = (result & 0xFFFFFFFF) as u32;
            let data_size = ((result >> 32) & 0xFFFFFFFF) as u32;

            if data_ptr == 0 {
                return Err(Error::Io("read failed".to_string()));
            }

            // Read data from memory
            let slice = std::slice::from_raw_parts(data_ptr as *const u8, data_size as usize);
            Ok(slice.to_vec())
        }
    }

    /// Write data to a file on the host filesystem
    pub fn write(path: &str, data: &[u8]) -> Result<Vec<u8>> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let result = host_fs_write(
                path_c.as_ptr() as *const u8,
                data.as_ptr(),
                data.len() as u32,
            );

            // Unpack: lower 32 bits = pointer, upper 32 bits = size
            let response_ptr = (result & 0xFFFFFFFF) as u32;
            let response_size = ((result >> 32) & 0xFFFFFFFF) as u32;

            if response_ptr == 0 {
                return Err(Error::Io("write failed".to_string()));
            }

            // Read response from memory
            let slice = std::slice::from_raw_parts(response_ptr as *const u8, response_size as usize);
            Ok(slice.to_vec())
        }
    }

    /// Get file information
    pub fn stat(path: &str) -> Result<FileInfo> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let result = host_fs_stat(path_c.as_ptr() as *const u8);

            // Unpack: lower 32 bits = json pointer, upper 32 bits = error pointer
            let json_ptr = (result & 0xFFFFFFFF) as u32;
            let err_ptr = ((result >> 32) & 0xFFFFFFFF) as u32;

            // Check for error
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }

            if json_ptr == 0 {
                return Err(Error::NotFound);
            }

            let json_str = read_string_from_ptr(json_ptr);
            serde_json::from_str(&json_str)
                .map_err(|e| Error::Other(format!("failed to parse stat result: {}", e)))
        }
    }

    /// Read directory contents
    pub fn readdir(path: &str) -> Result<Vec<FileInfo>> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let result = host_fs_readdir(path_c.as_ptr() as *const u8);

            // Unpack: lower 32 bits = json pointer, upper 32 bits = error pointer
            let json_ptr = (result & 0xFFFFFFFF) as u32;
            let err_ptr = ((result >> 32) & 0xFFFFFFFF) as u32;

            // Check for error
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }

            if json_ptr == 0 {
                return Ok(Vec::new());
            }

            let json_str = read_string_from_ptr(json_ptr);
            serde_json::from_str(&json_str)
                .map_err(|e| Error::Other(format!("failed to parse readdir result: {}", e)))
        }
    }

    /// Create a new file
    pub fn create(path: &str) -> Result<()> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_create(path_c.as_ptr() as *const u8);
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }

    /// Create a directory
    pub fn mkdir(path: &str, perm: u32) -> Result<()> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_mkdir(path_c.as_ptr() as *const u8, perm);
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }

    /// Remove a file or empty directory
    pub fn remove(path: &str) -> Result<()> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_remove(path_c.as_ptr() as *const u8);
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }

    /// Remove a file or directory recursively
    pub fn remove_all(path: &str) -> Result<()> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_remove_all(path_c.as_ptr() as *const u8);
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }

    /// Rename a file or directory
    pub fn rename(old_path: &str, new_path: &str) -> Result<()> {
        let old_path_c = CString::new(old_path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;
        let new_path_c = CString::new(new_path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_rename(
                old_path_c.as_ptr() as *const u8,
                new_path_c.as_ptr() as *const u8,
            );
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }

    /// Change file permissions
    pub fn chmod(path: &str, mode: u32) -> Result<()> {
        let path_c = CString::new(path).map_err(|_| Error::InvalidInput("invalid path".to_string()))?;

        unsafe {
            let err_ptr = host_fs_chmod(path_c.as_ptr() as *const u8, mode);
            if err_ptr != 0 {
                let err_str = read_string_from_ptr(err_ptr);
                return Err(Error::Other(err_str));
            }
            Ok(())
        }
    }
}

/// Read a null-terminated string from a pointer
unsafe fn read_string_from_ptr(ptr: u32) -> String {
    if ptr == 0 {
        return String::new();
    }

    // Find the null terminator
    let mut len = 0;
    let start_ptr = ptr as *const u8;
    while *start_ptr.add(len) != 0 {
        len += 1;
    }

    // Read the string
    let slice = std::slice::from_raw_parts(start_ptr, len);
    String::from_utf8_lossy(slice).to_string()
}
