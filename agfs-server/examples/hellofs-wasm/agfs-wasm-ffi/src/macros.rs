//! Macros for exporting WASM plugin functions

/// Export a FileSystem implementation as a WASM plugin
#[macro_export]
macro_rules! export_plugin {
    ($plugin_type:ty) => {
        static mut PLUGIN: Option<$plugin_type> = None;

        // Force type checking
        const _: fn() = || {
            fn assert_impl<T: $crate::FileSystem + Default>() {}
            assert_impl::<$plugin_type>();
        };

        #[no_mangle]
        pub extern "C" fn plugin_new() -> usize {
            unsafe {
                PLUGIN = Some(<$plugin_type>::default());
            }
            1
        }

        #[no_mangle]
        pub extern "C" fn plugin_name() -> *mut u8 {
            use $crate::memory::CString;
            use $crate::FileSystem;
            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                CString::new(<$plugin_type as $crate::FileSystem>::name(p)).into_raw()
            }
        }

        #[no_mangle]
        pub extern "C" fn plugin_get_readme() -> *mut u8 {
            use $crate::memory::CString;
            use $crate::FileSystem;
            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                CString::new(<$plugin_type as $crate::FileSystem>::readme(p)).into_raw()
            }
        }

        #[no_mangle]
        pub extern "C" fn plugin_validate(config_ptr: *const u8) -> *mut u8 {
            use $crate::ffi::{read_config, result_to_error_ptr};
            use $crate::FileSystem;
            let config = match read_config(config_ptr) {
                Ok(c) => c,
                Err(e) => return result_to_error_ptr::<()>(Err(e)),
            };
            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::validate(p, &config))
            }
        }

        #[no_mangle]
        pub extern "C" fn plugin_initialize(config_ptr: *const u8) -> *mut u8 {
            use $crate::ffi::{read_config, result_to_error_ptr};
            use $crate::FileSystem;
            let config = match read_config(config_ptr) {
                Ok(c) => c,
                Err(e) => return result_to_error_ptr::<()>(Err(e)),
            };
            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::initialize(p, &config))
            }
        }

        #[no_mangle]
        pub extern "C" fn plugin_shutdown() -> *mut u8 {
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;
            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::shutdown(p))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_read(path_ptr: *const u8, offset: i64, size: i64) -> u64 {
            use $crate::memory::{CString, Buffer, pack_u64};
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                match <$plugin_type as $crate::FileSystem>::read(p, &path, offset, size) {
                    Ok(data) => {
                        let len = data.len() as u32;
                        let buffer = Buffer::from_bytes(&data);
                        let ptr = buffer.into_raw() as u32;
                        pack_u64(ptr, len)
                    }
                    Err(_) => 0,
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_stat(path_ptr: *const u8) -> u64 {
            use $crate::memory::{CString, pack_u64};
            use $crate::ffi::fileinfo_to_json_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                match <$plugin_type as $crate::FileSystem>::stat(p, &path) {
                    Ok(info) => match fileinfo_to_json_ptr(&info) {
                        Ok(json_ptr) => pack_u64(json_ptr as u32, 0),
                        Err(e) => {
                            let err_ptr = CString::new(&e.to_string()).into_raw();
                            pack_u64(0, err_ptr as u32)
                        }
                    },
                    Err(e) => {
                        let err_ptr = CString::new(&e.to_string()).into_raw();
                        pack_u64(0, err_ptr as u32)
                    }
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_readdir(path_ptr: *const u8) -> u64 {
            use $crate::memory::{CString, pack_u64};
            use $crate::ffi::fileinfo_vec_to_json_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_ref().expect("Not initialized");
                match <$plugin_type as $crate::FileSystem>::readdir(p, &path) {
                    Ok(infos) => match fileinfo_vec_to_json_ptr(&infos) {
                        Ok(json_ptr) => pack_u64(json_ptr as u32, 0),
                        Err(e) => {
                            let err_ptr = CString::new(&e.to_string()).into_raw();
                            pack_u64(0, err_ptr as u32)
                        }
                    },
                    Err(e) => {
                        let err_ptr = CString::new(&e.to_string()).into_raw();
                        pack_u64(0, err_ptr as u32)
                    }
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_write(path_ptr: *const u8, data_ptr: *const u8, size: usize) -> u64 {
            use $crate::memory::{CString, Buffer, pack_u64};
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };
            let data = unsafe { std::slice::from_raw_parts(data_ptr, size) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                match <$plugin_type as $crate::FileSystem>::write(p, &path, data) {
                    Ok(response) => {
                        let len = response.len() as u32;
                        let buffer = Buffer::from_bytes(&response);
                        let ptr = buffer.into_raw() as u32;
                        pack_u64(ptr, len)
                    }
                    Err(_) => 0,
                }
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_create(path_ptr: *const u8) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::create(p, &path))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_mkdir(path_ptr: *const u8, perm: u32) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::mkdir(p, &path, perm))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_remove(path_ptr: *const u8) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::remove(p, &path))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_remove_all(path_ptr: *const u8) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::remove_all(p, &path))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_rename(old_path_ptr: *const u8, new_path_ptr: *const u8) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let old_path = unsafe { CString::from_ptr(old_path_ptr) };
            let new_path = unsafe { CString::from_ptr(new_path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::rename(p, &old_path, &new_path))
            }
        }

        #[no_mangle]
        pub extern "C" fn fs_chmod(path_ptr: *const u8, mode: u32) -> *mut u8 {
            use $crate::memory::CString;
            use $crate::ffi::result_to_error_ptr;
            use $crate::FileSystem;

            let path = unsafe { CString::from_ptr(path_ptr) };

            unsafe {
                let p = PLUGIN.as_mut().expect("Not initialized");
                result_to_error_ptr::<()>(<$plugin_type as $crate::FileSystem>::chmod(p, &path, mode))
            }
        }

        // Export malloc and free for Go compatibility
        #[no_mangle]
        pub extern "C" fn malloc(size: usize) -> *mut u8 {
            use std::alloc::{alloc, Layout};

            if size == 0 {
                return std::ptr::null_mut();
            }

            unsafe {
                let layout = Layout::from_size_align(size, 1).unwrap();
                alloc(layout)
            }
        }

        #[no_mangle]
        pub extern "C" fn free(ptr: *mut u8, size: usize) {
            use std::alloc::{dealloc, Layout};

            if ptr.is_null() || size == 0 {
                return;
            }

            unsafe {
                let layout = Layout::from_size_align(size, 1).unwrap();
                dealloc(ptr, layout);
            }
        }
    };
}
