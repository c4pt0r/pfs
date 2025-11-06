//! FileSystem trait definition

use crate::error::{FileSystemError, Result};
use crate::types::FileInfo;

/// Main trait that all filesystem plugins must implement
///
/// This trait defines the interface for interacting with a filesystem plugin.
/// Implementing this trait is all you need to do - the SDK handles all FFI
/// binding generation automatically.
///
/// # Example
///
/// ```rust
/// use pfs_ffi::prelude::*;
///
/// #[derive(Default)]
/// struct MyFS {
///     initialized: bool,
/// }
///
/// impl FileSystem for MyFS {
///     fn name(&self) -> &str {
///         "my-fs"
///     }
///
///     fn readme(&self) -> &str {
///         "# My Filesystem Plugin\n\nA custom filesystem implementation."
///     }
///
///     fn initialize(&mut self, _config: &str) -> Result<()> {
///         self.initialized = true;
///         Ok(())
///     }
///
///     fn read(&self, path: &str, _offset: i64, _size: i64) -> Result<String> {
///         if path == "/hello" {
///             Ok("Hello, World!".to_string())
///         } else {
///             Err(FileSystemError::NotFound)
///         }
///     }
///
///     fn stat(&self, path: &str) -> Result<FileInfo> {
///         if path == "/" || path == "/hello" {
///             Ok(FileInfo::file("hello", 13, 0o644))
///         } else {
///             Err(FileSystemError::NotFound)
///         }
///     }
///
///     fn readdir(&self, _path: &str) -> Result<Vec<FileInfo>> {
///         Ok(vec![FileInfo::file("hello", 13, 0o644)])
///     }
/// }
/// ```
pub trait FileSystem: Default + Send + Sync {
    /// Get the plugin name
    fn name(&self) -> &str;

    /// Get the plugin README (markdown format)
    fn readme(&self) -> &str {
        "# Plugin\n\nNo documentation provided."
    }

    /// Validate plugin configuration (JSON string)
    fn validate(&self, _config: &str) -> Result<()> {
        Ok(())
    }

    /// Initialize the plugin with given configuration
    fn initialize(&mut self, _config: &str) -> Result<()> {
        Ok(())
    }

    /// Shutdown the plugin
    fn shutdown(&mut self) -> Result<()> {
        Ok(())
    }

    /// Read file contents
    ///
    /// # Arguments
    ///
    /// * `path` - File path to read
    /// * `offset` - Byte offset to start reading from
    /// * `size` - Maximum number of bytes to read (0 = read all)
    ///
    /// # Returns
    ///
    /// File contents as a string
    fn read(&self, path: &str, offset: i64, size: i64) -> Result<String>;

    /// Get file or directory information
    ///
    /// # Arguments
    ///
    /// * `path` - File or directory path
    ///
    /// # Returns
    ///
    /// FileInfo structure with metadata
    fn stat(&self, path: &str) -> Result<FileInfo>;

    /// List directory contents
    ///
    /// # Arguments
    ///
    /// * `path` - Directory path
    ///
    /// # Returns
    ///
    /// Vector of FileInfo for each entry in the directory
    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>>;

    /// Write data to a file
    ///
    /// Default implementation returns ReadOnly error.
    fn write(&self, _path: &str, _data: &[u8]) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Create a new file
    ///
    /// Default implementation returns ReadOnly error.
    fn create(&self, _path: &str) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Create a directory
    ///
    /// Default implementation returns ReadOnly error.
    fn mkdir(&self, _path: &str, _mode: u32) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Remove a file
    ///
    /// Default implementation returns ReadOnly error.
    fn remove(&self, _path: &str) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Remove a directory and all its contents
    ///
    /// Default implementation returns ReadOnly error.
    fn remove_all(&self, _path: &str) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Rename a file or directory
    ///
    /// Default implementation returns ReadOnly error.
    fn rename(&self, _old_path: &str, _new_path: &str) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }

    /// Change file or directory permissions
    ///
    /// Default implementation returns ReadOnly error.
    fn chmod(&self, _path: &str, _mode: u32) -> Result<()> {
        Err(FileSystemError::ReadOnly)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[derive(Default)]
    struct TestFS;

    impl FileSystem for TestFS {
        fn name(&self) -> &str {
            "test-fs"
        }

        fn read(&self, path: &str, _offset: i64, _size: i64) -> Result<String> {
            if path == "/test" {
                Ok("test content".to_string())
            } else {
                Err(FileSystemError::NotFound)
            }
        }

        fn stat(&self, path: &str) -> Result<FileInfo> {
            if path == "/" || path == "/test" {
                Ok(FileInfo::file("test", 12, 0o644))
            } else {
                Err(FileSystemError::NotFound)
            }
        }

        fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
            if path == "/" {
                Ok(vec![FileInfo::file("test", 12, 0o644)])
            } else {
                Err(FileSystemError::NotFound)
            }
        }
    }

    #[test]
    fn test_filesystem_trait() {
        let fs = TestFS::default();
        assert_eq!(fs.name(), "test-fs");
        assert!(fs.validate("{}").is_ok());

        let content = fs.read("/test", 0, 100).unwrap();
        assert_eq!(content, "test content");

        let info = fs.stat("/test").unwrap();
        assert_eq!(info.name, "test");

        let files = fs.readdir("/").unwrap();
        assert_eq!(files.len(), 1);
    }

    #[test]
    fn test_default_readonly_operations() {
        let fs = TestFS::default();
        assert!(matches!(fs.write("/test", b"data"), Err(FileSystemError::ReadOnly)));
        assert!(matches!(fs.create("/new"), Err(FileSystemError::ReadOnly)));
        assert!(matches!(fs.mkdir("/dir", 0o755), Err(FileSystemError::ReadOnly)));
    }
}
