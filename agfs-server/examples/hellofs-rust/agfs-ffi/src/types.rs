//! Common type definitions for filesystem operations

use std::time::{SystemTime, UNIX_EPOCH};

/// Metadata about a file or directory
#[derive(Debug, Clone)]
pub struct FileInfo {
    /// File name (without path)
    pub name: String,
    /// File size in bytes
    pub size: i64,
    /// Unix file mode (permissions)
    pub mode: u32,
    /// Modification time (Unix timestamp)
    pub mod_time: i64,
    /// Whether this is a directory
    pub is_dir: bool,
    /// Plugin metadata
    pub metadata: FileMetadata,
}

impl FileInfo {
    /// Create a new FileInfo for a regular file
    pub fn file(name: impl Into<String>, size: i64, mode: u32) -> Self {
        Self::file_with_metadata(name, size, mode, FileMetadata::default())
    }

    /// Create a new FileInfo for a directory
    pub fn directory(name: impl Into<String>, mode: u32) -> Self {
        Self::directory_with_metadata(name, mode, FileMetadata::default())
    }

    /// Create a new FileInfo for a regular file with custom metadata
    pub fn file_with_metadata(
        name: impl Into<String>,
        size: i64,
        mode: u32,
        metadata: FileMetadata,
    ) -> Self {
        Self {
            name: name.into(),
            size,
            mode,
            mod_time: current_timestamp(),
            is_dir: false,
            metadata,
        }
    }

    /// Create a new FileInfo for a directory with custom metadata
    pub fn directory_with_metadata(
        name: impl Into<String>,
        mode: u32,
        metadata: FileMetadata,
    ) -> Self {
        Self {
            name: name.into(),
            size: 0,
            mode,
            mod_time: current_timestamp(),
            is_dir: true,
            metadata,
        }
    }

    /// Set the modification time
    pub fn with_mod_time(mut self, mod_time: i64) -> Self {
        self.mod_time = mod_time;
        self
    }
}

/// Plugin metadata attached to files
#[derive(Debug, Clone)]
pub struct FileMetadata {
    /// Plugin name
    pub name: String,
    /// File type description
    pub file_type: String,
    /// JSON content with additional metadata
    pub content: String,
}

impl FileMetadata {
    /// Create new metadata
    pub fn new(
        name: impl Into<String>,
        file_type: impl Into<String>,
        content: impl Into<String>,
    ) -> Self {
        Self {
            name: name.into(),
            file_type: file_type.into(),
            content: content.into(),
        }
    }
}

impl Default for FileMetadata {
    fn default() -> Self {
        Self {
            name: String::new(),
            file_type: String::new(),
            content: "{}".to_string(),
        }
    }
}

/// Get current Unix timestamp
pub fn current_timestamp() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time is before Unix epoch")
        .as_secs() as i64
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_file_info_creation() {
        let info = FileInfo::file("test.txt", 100, 0o644);
        assert_eq!(info.name, "test.txt");
        assert_eq!(info.size, 100);
        assert_eq!(info.mode, 0o644);
        assert!(!info.is_dir);
    }

    #[test]
    fn test_directory_info_creation() {
        let info = FileInfo::directory("testdir", 0o755);
        assert_eq!(info.name, "testdir");
        assert_eq!(info.size, 0);
        assert!(info.is_dir);
    }

    #[test]
    fn test_file_info_with_metadata() {
        let metadata = FileMetadata::new("myplugin", "text", r#"{"key":"value"}"#);
        let info = FileInfo::file_with_metadata("test.txt", 50, 0o644, metadata);
        assert_eq!(info.metadata.name, "myplugin");
        assert_eq!(info.metadata.file_type, "text");
    }

    #[test]
    fn test_current_timestamp() {
        let ts = current_timestamp();
        assert!(ts > 0);
    }
}
