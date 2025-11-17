//! High-level agfs filesystem trait for WASM plugins

use crate::types::{Config, FileInfo, Result};

/// Filesystem trait that plugin developers should implement
///
/// All methods have default implementations that return appropriate errors,
/// so you only need to implement the operations your filesystem supports.
pub trait FileSystem {
    /// Returns the name of this filesystem plugin
    fn name(&self) -> &str;

    /// Returns the README/documentation for this plugin
    fn readme(&self) -> &str {
        "No documentation available"
    }

    /// Validate the configuration before initialization
    ///
    /// This is called before `initialize` and should check that all
    /// required configuration values are present and valid.
    fn validate(&self, _config: &Config) -> Result<()> {
        Ok(())
    }

    /// Initialize the filesystem with the given configuration
    ///
    /// This is called after successful validation and before any
    /// filesystem operations.
    fn initialize(&mut self, _config: &Config) -> Result<()> {
        Ok(())
    }

    /// Shutdown the filesystem
    ///
    /// This is called when the filesystem is being unmounted.
    /// Use this to cleanup resources.
    fn shutdown(&mut self) -> Result<()> {
        Ok(())
    }

    /// Read data from a file
    ///
    /// # Arguments
    /// * `path` - The file path
    /// * `offset` - Starting position (0 for beginning)
    /// * `size` - Number of bytes to read (-1 for all)
    fn read(&self, _path: &str, _offset: i64, _size: i64) -> Result<Vec<u8>> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Write data to a file
    /// Returns response data (can be used to return results back to caller)
    fn write(&mut self, _path: &str, _data: &[u8]) -> Result<Vec<u8>> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Create a new empty file
    fn create(&mut self, _path: &str) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Create a new directory
    fn mkdir(&mut self, _path: &str, _perm: u32) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Remove a file or empty directory
    fn remove(&mut self, _path: &str) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Remove a file or directory and all its contents
    fn remove_all(&mut self, _path: &str) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Get file information
    fn stat(&self, path: &str) -> Result<FileInfo>;

    /// List directory contents
    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>>;

    /// Rename/move a file or directory
    fn rename(&mut self, _old_path: &str, _new_path: &str) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }

    /// Change file permissions
    fn chmod(&mut self, _path: &str, _mode: u32) -> Result<()> {
        Err(crate::types::Error::ReadOnly)
    }
}

/// Read-only filesystem helper
///
/// This trait provides common functionality for read-only filesystems.
/// Implement this instead of `FileSystem` if your filesystem is read-only.
pub trait ReadOnlyFileSystem {
    /// Returns the name of this filesystem plugin
    fn name(&self) -> &str;

    /// Returns the README/documentation for this plugin
    fn readme(&self) -> &str {
        "No documentation available"
    }

    /// Read data from a file
    fn read(&self, path: &str, offset: i64, size: i64) -> Result<Vec<u8>>;

    /// Get file information
    fn stat(&self, path: &str) -> Result<FileInfo>;

    /// List directory contents
    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>>;
}

// Automatically implement FileSystem for any ReadOnlyFileSystem
impl<T: ReadOnlyFileSystem> FileSystem for T {
    fn name(&self) -> &str {
        ReadOnlyFileSystem::name(self)
    }

    fn readme(&self) -> &str {
        ReadOnlyFileSystem::readme(self)
    }

    fn read(&self, path: &str, offset: i64, size: i64) -> Result<Vec<u8>> {
        ReadOnlyFileSystem::read(self, path, offset, size)
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        ReadOnlyFileSystem::stat(self, path)
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        ReadOnlyFileSystem::readdir(self, path)
    }
}
