//! HelloFS WASM - Filesystem plugin with host fs access demo
//!
//! Returns a single file with "Hello World" content
//! Also demonstrates accessing the host filesystem

use pfs_wasm_ffi::prelude::*;

#[derive(Default)]
pub struct HelloFS {
    host_prefix: String,
}

impl FileSystem for HelloFS {
    fn name(&self) -> &str {
        "hellofs-wasm"
    }

    fn readme(&self) -> &str {
        "HelloFS WASM - Demonstrates host filesystem access\n\
         - /hello.txt - Returns 'Hello World'\n\
         - /host/* - Proxies to host filesystem (if configured)"
    }

    fn initialize(&mut self, config: &Config) -> Result<()> {
        // Get optional host_prefix from config
        if let Some(prefix) = config.get_str("host_prefix") {
            self.host_prefix = prefix.to_string();
        }
        Ok(())
    }

    fn read(&self, path: &str, offset: i64, size: i64) -> Result<Vec<u8>> {
        match path {
            "/hello.txt" => Ok(b"Hello World\n".to_vec()),
            p if p.starts_with("/host/") && !self.host_prefix.is_empty() => {
                // Proxy to host filesystem
                let host_path = p.strip_prefix("/host").unwrap();
                let full_path = format!("{}{}", self.host_prefix, host_path);
                HostFS::read(&full_path, offset, size)
                    .map_err(|e| Error::Other(format!("host fs: {}", e)))
            }
            _ => Err(Error::NotFound),
        }
    }

    fn stat(&self, path: &str) -> Result<FileInfo> {
        match path {
            "/" => Ok(FileInfo::dir("", 0o755)),
            "/hello.txt" => Ok(FileInfo::file("hello.txt", 12, 0o644)),
            "/host" if !self.host_prefix.is_empty() => {
                Ok(FileInfo::dir("host", 0o755))
            }
            p if p.starts_with("/host/") && !self.host_prefix.is_empty() => {
                // Proxy to host filesystem
                let host_path = p.strip_prefix("/host").unwrap();
                let full_path = format!("{}{}", self.host_prefix, host_path);
                let host_info = HostFS::stat(&full_path)
                    .map_err(|e| Error::Other(format!("host fs: {}", e)))?;

                // Convert and return
                Ok(FileInfo {
                    name: host_info.name,
                    size: host_info.size,
                    mode: host_info.mode,
                    mod_time: host_info.mod_time,
                    is_dir: host_info.is_dir,
                    meta: host_info.meta,
                })
            }
            _ => Err(Error::NotFound),
        }
    }

    fn readdir(&self, path: &str) -> Result<Vec<FileInfo>> {
        match path {
            "/" => {
                let mut entries = vec![FileInfo::file("hello.txt", 12, 0o644)];
                if !self.host_prefix.is_empty() {
                    entries.push(FileInfo::dir("host", 0o755));
                }
                Ok(entries)
            }
            "/host" if !self.host_prefix.is_empty() => {
                // Read from host filesystem root
                let host_infos = HostFS::readdir(&self.host_prefix)
                    .map_err(|e| Error::Other(format!("host fs: {}", e)))?;

                Ok(host_infos
                    .into_iter()
                    .map(|info| FileInfo {
                        name: info.name,
                        size: info.size,
                        mode: info.mode,
                        mod_time: info.mod_time,
                        is_dir: info.is_dir,
                        meta: info.meta,
                    })
                    .collect())
            }
            p if p.starts_with("/host/") && !self.host_prefix.is_empty() => {
                // Proxy to host filesystem
                let host_path = p.strip_prefix("/host").unwrap();
                let full_path = format!("{}{}", self.host_prefix, host_path);
                let host_infos = HostFS::readdir(&full_path)
                    .map_err(|e| Error::Other(format!("host fs: {}", e)))?;

                Ok(host_infos
                    .into_iter()
                    .map(|info| FileInfo {
                        name: info.name,
                        size: info.size,
                        mode: info.mode,
                        mod_time: info.mod_time,
                        is_dir: info.is_dir,
                        meta: info.meta,
                    })
                    .collect())
            }
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
