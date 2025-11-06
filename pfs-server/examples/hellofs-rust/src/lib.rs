//! HelloFS Rust Plugin using PFS FFI
//!
//! This is a simple example demonstrating how to use the PFS FFI library
//! to create a filesystem plugin with minimal boilerplate.

use pfs_ffi::prelude::*;

/// HelloFS - A simple read-only filesystem with a single file
#[derive(Default)]
pub struct HelloFS;

impl FileSystem for HelloFS {
    fn name(&self) -> &str {
        "hellofs-rust"
    }

    fn readme(&self) -> &str {
        r#"# HelloFS Rust Plugin

A simple read-only filesystem plugin written in Rust using the PFS Plugin SDK.

## Features
- Single file: /hello containing 'Hello from Rust dynamic library!'
- Demonstrates idiomatic Rust plugin interface for pfs-server
- Built with the PFS Plugin SDK for minimal boilerplate
"#
    }

    fn read(&self, path: &str, offset: i64, size: i64) -> Result<String> {
        match path {
            "/hello" => {
                let content = Self::hello_content();
                let content_len = content.len() as i64;

                // Handle offset beyond file size
                if offset >= content_len {
                    return Ok(String::new());
                }

                // Calculate actual read length
                let remaining = content_len - offset;
                let read_len = if size > 0 && size < remaining {
                    size
                } else {
                    remaining
                };

                let start = offset as usize;
                let end = (offset + read_len) as usize;
                Ok(content[start..end].to_string())
            }
            _ => Err(FileSystemError::NotFound),
        }
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        match path {
            "/" => Ok(FileInfo::directory("", 0o755).with_metadata(Self::dir_metadata())),
            "/hello" => {
                let content = Self::hello_content();
                Ok(FileInfo::file("hello", content.len() as i64, 0o644)
                    .with_metadata(Self::file_metadata()))
            }
            _ => Err(FileSystemError::NotFound),
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        match path {
            "/" => {
                let content = Self::hello_content();
                Ok(vec![FileInfo::file(
                    "hello",
                    content.len() as i64,
                    0o644,
                )
                .with_metadata(Self::file_metadata())])
            }
            _ => Err(FileSystemError::NotFound),
        }
    }
}

impl HelloFS {
    /// Get the content of the hello file
    fn hello_content() -> &'static str {
        "Hello from Rust dynamic library!\n"
    }

    /// Get file metadata
    fn file_metadata() -> FileMetadata {
        FileMetadata::new("hellofs-rust", "text", r#"{"language":"rust"}"#)
    }

    /// Get directory metadata
    fn dir_metadata() -> FileMetadata {
        FileMetadata::new("hellofs-rust", "directory", r#"{"language":"rust"}"#)
    }
}

// Helper method to add metadata to FileInfo
trait WithMetadata {
    fn with_metadata(self, metadata: FileMetadata) -> Self;
}

impl WithMetadata for FileInfo {
    fn with_metadata(mut self, metadata: FileMetadata) -> Self {
        self.metadata = metadata;
        self
    }
}

// Export the plugin using the SDK macro
export_plugin!(HelloFS);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_read_hello_file() {
        let fs = HelloFS::default();
        let result = fs.read("/hello", 0, 100);
        assert!(result.is_ok());
        let content = result.unwrap();
        assert_eq!(content, "Hello from Rust dynamic library!\n");
    }

    #[test]
    fn test_read_nonexistent_file() {
        let fs = HelloFS::default();
        let result = fs.read("/nonexistent", 0, 100);
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), FileSystemError::NotFound));
    }

    #[test]
    fn test_stat_root() {
        let fs = HelloFS::default();
        let result = fs.stat("/");
        assert!(result.is_ok());
        let info = result.unwrap();
        assert!(info.is_dir);
    }

    #[test]
    fn test_stat_hello() {
        let fs = HelloFS::default();
        let result = fs.stat("/hello");
        assert!(result.is_ok());
        let info = result.unwrap();
        assert_eq!(info.name, "hello");
        assert!(!info.is_dir);
        assert_eq!(info.size, 33);  // "Hello from Rust dynamic library!\n"
    }

    #[test]
    fn test_readdir_root() {
        let fs = HelloFS::default();
        let result = fs.readdir("/");
        assert!(result.is_ok());
        let files = result.unwrap();
        assert_eq!(files.len(), 1);
        assert_eq!(files[0].name, "hello");
    }

    #[test]
    fn test_read_with_offset() {
        let fs = HelloFS::default();
        let result = fs.read("/hello", 6, 100);
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), "from Rust dynamic library!\n");
    }

    #[test]
    fn test_write_fails() {
        let fs = HelloFS::default();
        let result = fs.write("/hello", b"new content");
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), FileSystemError::ReadOnly));
    }

    #[test]
    fn test_plugin_name() {
        let fs = HelloFS::default();
        assert_eq!(fs.name(), "hellofs-rust");
    }

    #[test]
    fn test_plugin_readme() {
        let fs = HelloFS::default();
        assert!(fs.readme().contains("HelloFS Rust Plugin"));
    }
}
