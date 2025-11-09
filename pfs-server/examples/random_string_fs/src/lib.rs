//! RandomStringFS WASM - Generates random strings
//!
//! Write a number to /generate to set the length, then read to get a random string

use core::cell::Cell;
use pfs_wasm_ffi::prelude::*;

const CHARSET: &[u8] = b"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";

pub struct RandomStringFS {
    seed: Cell<u64>,
}

impl Default for RandomStringFS {
    fn default() -> Self {
        Self {
            seed: Cell::new(12345),
        }
    }
}

impl RandomStringFS {
    fn generate_random_string(&self, length: usize) -> Vec<u8> {
        let mut result = Vec::with_capacity(length);
        let mut seed = self.seed.get();

        for _ in 0..length {
            seed = seed.wrapping_mul(1103515245).wrapping_add(12345);
            let rand_byte = ((seed / 65536) % 256) as u8;
            result.push(CHARSET[(rand_byte as usize) % CHARSET.len()]);
        }

        // Update seed for next read
        self.seed.set(seed);

        result
    }
}

impl FileSystem for RandomStringFS {
    fn name(&self) -> &str {
        "random_string_fs"
    }

    fn readme(&self) -> &str {
        "RandomStringFS - Generate random strings [a-zA-Z0-9]\nWrite a number to /generate to set length (default: 6)\nRead from /generate to get a random string"
    }

    fn initialize(&mut self, _config: &Config) -> Result<()> {
        Ok(())
    }

    fn read(&self, path: &str, _offset: i64, _size: i64) -> Result<Vec<u8>> {
        match path {
            "/generate" => {
                // Default: return 6 character random string
                Ok(self.generate_random_string(6))
            }
            _ => Err(Error::NotFound),
        }
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        match path {
            "/" => Ok(FileInfo::dir("", 0o755)),
            "/generate" => Ok(FileInfo::file("generate", 0, 0o644)),
            _ => Err(Error::NotFound),
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        match path {
            "/" => Ok(vec![FileInfo::file("generate", 0, 0o644)]),
            _ => Err(Error::NotFound),
        }
    }

    fn write(&mut self, path: &str, data: &[u8]) -> Result<Vec<u8>> {
        match path {
            "/generate" => {
                let content = core::str::from_utf8(data)
                    .map_err(|_| Error::InvalidInput("invalid UTF-8".to_string()))?
                    .trim();

                let length = content.parse::<usize>()
                    .map_err(|_| Error::InvalidInput("not a valid number".to_string()))?;

                if length == 0 || length > 1024 {
                    return Err(Error::InvalidInput("length must be between 1 and 1024".to_string()));
                }

                // Generate and return random string directly
                Ok(self.generate_random_string(length))
            }
            _ => Err(Error::NotFound),
        }
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

export_plugin!(RandomStringFS);
