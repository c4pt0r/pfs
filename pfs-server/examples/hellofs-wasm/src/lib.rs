//! HelloFS WASM - Simplest possible filesystem plugin
//!
//! Returns a single file with "Hello World" content

use pfs_wasm_ffi::prelude::*;

#[derive(Default)]
pub struct HelloFS;

impl FileSystem for HelloFS {
    fn name(&self) -> &str {
        "hellofs-wasm"
    }

    fn readme(&self) -> &str {
        "HelloFS WASM - Returns 'Hello World'"
    }

    fn initialize(&mut self, _config: &Config) -> Result<()> {
        Ok(())
    }

    fn read(&self, path: &str, _offset: i64, _size: i64) -> Result<Vec<u8>> {
        match path {
            "/hello.txt" => Ok(b"Hello World\n".to_vec()),
            _ => Err(Error::NotFound),
        }
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        match path {
            "/" => Ok(FileInfo::dir("", 0o755)),
            "/hello.txt" => Ok(FileInfo::file("hello.txt", 12, 0o644)),
            _ => Err(Error::NotFound),
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        match path {
            "/" => Ok(vec![FileInfo::file("hello.txt", 12, 0o644)]),
            _ => Err(Error::NotFound),
        }
    }

    fn write(&mut self, _path: &str, _data: &[u8]) -> Result<Vec<u8>> {
        Err(Error::PermissionDenied)
    }

    fn create(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn mkdir(&mut self, _path: &str, _perm: u32) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn remove(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn remove_all(&mut self, _path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn rename(&mut self, _old_path: &str, _new_path: &str) -> Result<()> {
        Err(Error::PermissionDenied)
    }

    fn chmod(&mut self, _path: &str, _mode: u32) -> Result<()> {
        Ok(())
    }
}

export_plugin!(HelloFS);
