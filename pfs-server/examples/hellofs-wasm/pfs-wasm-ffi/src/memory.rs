//! Safe memory management utilities for WASM FFI
//!
//! This module provides safe wrappers around raw pointer operations
//! needed for WASM<->Go communication.

use std::alloc::{alloc, dealloc, Layout};
use std::ptr;

/// A string allocated in WASM memory that can be passed to Go
pub struct CString {
    ptr: *mut u8,
    len: usize,
}

impl CString {
    /// Create a new C-compatible string from a Rust string
    pub fn new(s: &str) -> Self {
        if s.is_empty() {
            return Self {
                ptr: ptr::null_mut(),
                len: 0,
            };
        }

        let bytes = s.as_bytes();
        let len = bytes.len() + 1; // +1 for null terminator

        let ptr = unsafe {
            let layout = Layout::from_size_align(len, 1).unwrap();
            let ptr = alloc(layout);
            if ptr.is_null() {
                panic!("Failed to allocate memory");
            }
            ptr::copy_nonoverlapping(bytes.as_ptr(), ptr, bytes.len());
            *ptr.add(bytes.len()) = 0; // null terminator
            ptr
        };

        Self { ptr, len }
    }

    /// Convert to a raw pointer (consumes self, caller must free)
    pub fn into_raw(self) -> *mut u8 {
        let ptr = self.ptr;
        std::mem::forget(self); // Don't run destructor
        ptr
    }

    /// Get the raw pointer without consuming
    pub fn as_ptr(&self) -> *const u8 {
        self.ptr
    }

    /// Create a null pointer (represents None/empty)
    pub fn null() -> *mut u8 {
        ptr::null_mut()
    }

    /// Read a C string from a pointer into a Rust String
    pub unsafe fn from_ptr(ptr: *const u8) -> String {
        if ptr.is_null() {
            return String::new();
        }

        let mut len = 0;
        while *ptr.add(len) != 0 {
            len += 1;
        }

        if len == 0 {
            return String::new();
        }

        let slice = std::slice::from_raw_parts(ptr, len);
        String::from_utf8_lossy(slice).to_string()
    }
}

impl Drop for CString {
    fn drop(&mut self) {
        if !self.ptr.is_null() && self.len > 0 {
            unsafe {
                let layout = Layout::from_size_align(self.len, 1).unwrap();
                dealloc(self.ptr, layout);
            }
        }
    }
}

/// A buffer allocated in WASM memory
pub struct Buffer {
    ptr: *mut u8,
    len: usize,
}

impl Buffer {
    /// Allocate a new buffer of the given size
    pub fn new(size: usize) -> Self {
        if size == 0 {
            return Self {
                ptr: ptr::null_mut(),
                len: 0,
            };
        }

        let ptr = unsafe {
            let layout = Layout::from_size_align(size, 1).unwrap();
            let ptr = alloc(layout);
            if ptr.is_null() {
                panic!("Failed to allocate memory");
            }
            ptr
        };

        Self { ptr, len: size }
    }

    /// Create a buffer from bytes
    pub fn from_bytes(data: &[u8]) -> Self {
        let buf = Self::new(data.len());
        if !data.is_empty() {
            unsafe {
                ptr::copy_nonoverlapping(data.as_ptr(), buf.ptr, data.len());
            }
        }
        buf
    }

    /// Convert to raw pointer (consumes self, caller must free)
    pub fn into_raw(self) -> *mut u8 {
        let ptr = self.ptr;
        std::mem::forget(self);
        ptr
    }

    /// Get the pointer
    pub fn as_ptr(&self) -> *const u8 {
        self.ptr
    }

    /// Get the length
    pub fn len(&self) -> usize {
        self.len
    }

    /// Check if empty
    pub fn is_empty(&self) -> bool {
        self.len == 0
    }

    /// Create a null pointer
    pub fn null() -> *mut u8 {
        ptr::null_mut()
    }
}

impl Drop for Buffer {
    fn drop(&mut self) {
        if !self.ptr.is_null() && self.len > 0 {
            unsafe {
                let layout = Layout::from_size_align(self.len, 1).unwrap();
                dealloc(self.ptr, layout);
            }
        }
    }
}

/// Pack two u32 values into a u64
/// Used for returning multiple values from WASM functions
pub fn pack_u64(low: u32, high: u32) -> u64 {
    ((high as u64) << 32) | (low as u64)
}
