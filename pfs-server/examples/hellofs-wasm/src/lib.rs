//! HelloFS WASM with WASI filesystem support
//!
//! A WebAssembly plugin that uses WASI to create and manage files
//! in the local /tmp directory.

use pfs_wasm_ffi::prelude::*;
use std::fs;
use std::io::{Read as IoRead, Write as IoWrite, Seek, SeekFrom};
use std::path::{Path, PathBuf};

const README: &str = r#"# HelloFS WASM Plugin with WASI

A filesystem plugin that uses WASI to manage files in /tmp directory.

## Features
- Create files using WASI filesystem APIs
- Read/write files in /tmp directory
- Full filesystem operations (mkdir, remove, rename, etc.)
- Demonstrates WASM + WASI capabilities

## Files
All files are stored in /tmp/hellofs-wasm/
"#;

/// HelloFS - A filesystem that uses WASI to manage files in /tmp
pub struct HelloFS {
    base_path: PathBuf,
}

impl Default for HelloFS {
    fn default() -> Self {
        Self {
            base_path: PathBuf::from("/tmp/hellofs-wasm"),
        }
    }
}

impl HelloFS {
    /// Convert virtual path to real filesystem path
    fn real_path(&self, path: &str) -> PathBuf {
        let path = path.trim_start_matches('/');
        if path.is_empty() {
            self.base_path.clone()
        } else {
            self.base_path.join(path)
        }
    }

    /// Get relative name from full path
    fn get_name(path: &Path) -> String {
        path.file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("")
            .to_string()
    }
}

impl FileSystem for HelloFS {
    fn name(&self) -> &str {
        "hellofs-wasm"
    }

    fn readme(&self) -> &str {
        README
    }

    fn initialize(&mut self, _config: &Config) -> Result<()> {
        // Create base directory if it doesn't exist
        fs::create_dir_all(&self.base_path)
            .map_err(|e| Error::Io(format!("Failed to create base directory: {}", e)))?;

        // Create a welcome file
        let welcome_path = self.base_path.join("welcome.txt");
        if !welcome_path.exists() {
            fs::write(&welcome_path, "Welcome to HelloFS WASM!\n")
                .map_err(|e| Error::Io(format!("Failed to create welcome file: {}", e)))?;
        }

        Ok(())
    }

    fn read(&self, path: &str, offset: i64, size: i64) -> Result<Vec<u8>> {
        let real_path = self.real_path(path);

        let mut file = fs::File::open(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to open file: {}", e)),
            })?;

        // Seek to offset
        if offset > 0 {
            file.seek(SeekFrom::Start(offset as u64))
                .map_err(|e| Error::Io(format!("Failed to seek: {}", e)))?;
        }

        // Read data
        let mut buffer = if size > 0 {
            vec![0u8; size as usize]
        } else {
            Vec::new()
        };

        if size > 0 {
            let bytes_read = file.read(&mut buffer)
                .map_err(|e| Error::Io(format!("Failed to read: {}", e)))?;
            buffer.truncate(bytes_read);
        } else {
            file.read_to_end(&mut buffer)
                .map_err(|e| Error::Io(format!("Failed to read: {}", e)))?;
        }

        Ok(buffer)
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        let real_path = self.real_path(path);

        let metadata = fs::metadata(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to stat: {}", e)),
            })?;

        let name = if path == "/" {
            String::new()
        } else {
            Self::get_name(&real_path)
        };

        if metadata.is_dir() {
            Ok(FileInfo::dir(&name, 0o755))
        } else {
            Ok(FileInfo::file(&name, metadata.len() as i64, 0o644))
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        let real_path = self.real_path(path);

        let entries = fs::read_dir(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to read directory: {}", e)),
            })?;

        let mut files = Vec::new();
        for entry in entries {
            let entry = entry
                .map_err(|e| Error::Io(format!("Failed to read entry: {}", e)))?;

            let metadata = entry.metadata()
                .map_err(|e| Error::Io(format!("Failed to get metadata: {}", e)))?;

            let name = entry.file_name()
                .to_string_lossy()
                .to_string();

            let info = if metadata.is_dir() {
                FileInfo::dir(&name, 0o755)
            } else {
                FileInfo::file(&name, metadata.len() as i64, 0o644)
            };

            files.push(info);
        }

        Ok(files)
    }

    fn write(&mut self, path: &str, data: &[u8]) -> Result<()> {
        let real_path = self.real_path(path);

        let mut file = fs::OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .open(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to open file for writing: {}", e)),
            })?;

        file.write_all(data)
            .map_err(|e| Error::Io(format!("Failed to write: {}", e)))?;

        Ok(())
    }

    fn create(&mut self, path: &str) -> Result<()> {
        let real_path = self.real_path(path);

        fs::File::create(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::AlreadyExists => Error::AlreadyExists,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to create file: {}", e)),
            })?;

        Ok(())
    }

    fn mkdir(&mut self, path: &str, _perm: u32) -> Result<()> {
        let real_path = self.real_path(path);

        fs::create_dir(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::AlreadyExists => Error::AlreadyExists,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to create directory: {}", e)),
            })?;

        Ok(())
    }

    fn remove(&mut self, path: &str) -> Result<()> {
        let real_path = self.real_path(path);

        let metadata = fs::metadata(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                _ => Error::Io(format!("Failed to stat: {}", e)),
            })?;

        if metadata.is_dir() {
            fs::remove_dir(&real_path)
                .map_err(|e| Error::Io(format!("Failed to remove directory: {}", e)))?;
        } else {
            fs::remove_file(&real_path)
                .map_err(|e| Error::Io(format!("Failed to remove file: {}", e)))?;
        }

        Ok(())
    }

    fn remove_all(&mut self, path: &str) -> Result<()> {
        let real_path = self.real_path(path);

        let metadata = fs::metadata(&real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                _ => Error::Io(format!("Failed to stat: {}", e)),
            })?;

        if metadata.is_dir() {
            fs::remove_dir_all(&real_path)
                .map_err(|e| Error::Io(format!("Failed to remove directory recursively: {}", e)))?;
        } else {
            fs::remove_file(&real_path)
                .map_err(|e| Error::Io(format!("Failed to remove file: {}", e)))?;
        }

        Ok(())
    }

    fn rename(&mut self, old_path: &str, new_path: &str) -> Result<()> {
        let old_real_path = self.real_path(old_path);
        let new_real_path = self.real_path(new_path);

        fs::rename(&old_real_path, &new_real_path)
            .map_err(|e| match e.kind() {
                std::io::ErrorKind::NotFound => Error::NotFound,
                std::io::ErrorKind::PermissionDenied => Error::PermissionDenied,
                _ => Error::Io(format!("Failed to rename: {}", e)),
            })?;

        Ok(())
    }

    fn chmod(&mut self, _path: &str, _mode: u32) -> Result<()> {
        // WASI doesn't support chmod on all platforms
        // Just return success for now
        Ok(())
    }
}

// Export the plugin with a single macro call
export_plugin!(HelloFS);
