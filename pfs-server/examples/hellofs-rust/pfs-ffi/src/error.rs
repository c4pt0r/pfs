//! Error types for filesystem operations

/// Result type for filesystem operations
pub type Result<T> = std::result::Result<T, FileSystemError>;

/// Errors that can occur during filesystem operations
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FileSystemError {
    /// File or directory not found
    NotFound,
    /// Operation not supported (e.g., writes on read-only filesystem)
    ReadOnly,
    /// Invalid path
    InvalidPath,
    /// Permission denied
    PermissionDenied,
    /// File or directory already exists
    AlreadyExists,
    /// Not a directory
    NotADirectory,
    /// Is a directory (when file was expected)
    IsADirectory,
    /// Directory not empty
    DirectoryNotEmpty,
    /// General I/O error
    IoError(String),
    /// Custom error with message
    Custom(String),
}

impl std::fmt::Display for FileSystemError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            FileSystemError::NotFound => write!(f, "file not found"),
            FileSystemError::ReadOnly => {
                write!(f, "operation not supported: read-only filesystem")
            }
            FileSystemError::InvalidPath => write!(f, "invalid path"),
            FileSystemError::PermissionDenied => write!(f, "permission denied"),
            FileSystemError::AlreadyExists => write!(f, "file already exists"),
            FileSystemError::NotADirectory => write!(f, "not a directory"),
            FileSystemError::IsADirectory => write!(f, "is a directory"),
            FileSystemError::DirectoryNotEmpty => write!(f, "directory not empty"),
            FileSystemError::IoError(msg) => write!(f, "I/O error: {}", msg),
            FileSystemError::Custom(msg) => write!(f, "{}", msg),
        }
    }
}

impl std::error::Error for FileSystemError {}

impl From<std::io::Error> for FileSystemError {
    fn from(err: std::io::Error) -> Self {
        FileSystemError::IoError(err.to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_display() {
        assert_eq!(FileSystemError::NotFound.to_string(), "file not found");
        assert_eq!(
            FileSystemError::ReadOnly.to_string(),
            "operation not supported: read-only filesystem"
        );
        assert_eq!(
            FileSystemError::Custom("test error".to_string()).to_string(),
            "test error"
        );
    }

    #[test]
    fn test_io_error_conversion() {
        let io_err = std::io::Error::new(std::io::ErrorKind::NotFound, "test");
        let fs_err: FileSystemError = io_err.into();
        assert!(matches!(fs_err, FileSystemError::IoError(_)));
    }
}
