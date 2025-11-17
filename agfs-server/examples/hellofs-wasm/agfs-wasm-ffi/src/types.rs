//! Type definitions for AGFS filesystem operations

use serde::{Deserialize, Serialize};

/// Result type for filesystem operations
pub type Result<T> = std::result::Result<T, Error>;

/// Error type for filesystem operations
#[derive(Debug)]
pub enum Error {
    NotFound,
    PermissionDenied,
    AlreadyExists,
    IsDirectory,
    NotDirectory,
    ReadOnly,
    InvalidInput(String),
    Io(String),
    Other(String),
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::NotFound => write!(f, "file not found"),
            Error::PermissionDenied => write!(f, "permission denied"),
            Error::AlreadyExists => write!(f, "file already exists"),
            Error::IsDirectory => write!(f, "is a directory"),
            Error::NotDirectory => write!(f, "not a directory"),
            Error::ReadOnly => write!(f, "read-only filesystem"),
            Error::InvalidInput(msg) => write!(f, "invalid input: {}", msg),
            Error::Io(msg) => write!(f, "I/O error: {}", msg),
            Error::Other(msg) => write!(f, "{}", msg),
        }
    }
}

impl std::error::Error for Error {}

/// File information structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FileInfo {
    #[serde(rename = "Name")]
    pub name: String,
    #[serde(rename = "Size")]
    pub size: i64,
    #[serde(rename = "Mode")]
    pub mode: u32,
    #[serde(rename = "ModTime", serialize_with = "serialize_timestamp", deserialize_with = "deserialize_timestamp")]
    pub mod_time: i64,
    #[serde(rename = "IsDir")]
    pub is_dir: bool,
    #[serde(rename = "Meta")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<MetaData>,
}

// Serialize Unix timestamp to RFC3339 string
fn serialize_timestamp<S>(_timestamp: &i64, serializer: S) -> std::result::Result<S::Ok, S::Error>
where
    S: serde::Serializer,
{
    // Always serialize as zero time for simplicity
    serializer.serialize_str("0001-01-01T00:00:00Z")
}

// Deserialize RFC3339 string to Unix timestamp
fn deserialize_timestamp<'de, D>(deserializer: D) -> std::result::Result<i64, D::Error>
where
    D: serde::Deserializer<'de>,
{
    let _s = String::deserialize(deserializer)?;
    // Always return 0 for simplicity
    Ok(0)
}

impl FileInfo {
    /// Create a file info for a regular file
    pub fn file(name: impl Into<String>, size: i64, mode: u32) -> Self {
        Self {
            name: name.into(),
            size,
            mode,
            mod_time: 0,
            is_dir: false,
            meta: None,
        }
    }

    /// Create a file info for a directory
    pub fn dir(name: impl Into<String>, mode: u32) -> Self {
        Self {
            name: name.into(),
            size: 0,
            mode,
            mod_time: 0,
            is_dir: true,
            meta: None,
        }
    }

    /// Set metadata
    pub fn with_meta(mut self, meta: MetaData) -> Self {
        self.meta = Some(meta);
        self
    }

    /// Set modification time (Unix timestamp)
    pub fn with_mod_time(mut self, timestamp: i64) -> Self {
        self.mod_time = timestamp;
        self
    }
}

/// Metadata structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetaData {
    #[serde(rename = "Name")]
    pub name: String,
    #[serde(rename = "Type")]
    pub type_: String,
    #[serde(rename = "Content")]
    pub content: serde_json::Value,
}

impl MetaData {
    /// Create new metadata
    pub fn new(name: impl Into<String>, type_: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            type_: type_.into(),
            content: serde_json::Value::Object(serde_json::Map::new()),
        }
    }

    /// Set content from JSON value
    pub fn with_content(mut self, content: serde_json::Value) -> Self {
        self.content = content;
        self
    }
}

/// Configuration passed to plugin
#[derive(Debug, Clone, Deserialize)]
pub struct Config {
    #[serde(flatten)]
    pub inner: serde_json::Map<String, serde_json::Value>,
}

impl Config {
    /// Get a string value
    pub fn get_str(&self, key: &str) -> Option<&str> {
        self.inner.get(key)?.as_str()
    }

    /// Get an integer value
    pub fn get_i64(&self, key: &str) -> Option<i64> {
        self.inner.get(key)?.as_i64()
    }

    /// Get a boolean value
    pub fn get_bool(&self, key: &str) -> Option<bool> {
        self.inner.get(key)?.as_bool()
    }

    /// Check if a key exists
    pub fn contains(&self, key: &str) -> bool {
        self.inner.contains_key(key)
    }
}

impl From<serde_json::Value> for Config {
    fn from(value: serde_json::Value) -> Self {
        match value {
            serde_json::Value::Object(map) => Config { inner: map },
            _ => Config {
                inner: serde_json::Map::new(),
            },
        }
    }
}
