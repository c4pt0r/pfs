"""Helper functions usage examples for pyagfs"""

from pyagfs import AGFSClient, AGFSClientError, cp, upload, download
import tempfile
import os


def main():
    # Initialize client
    client = AGFSClient("http://localhost:8080")

    try:
        print("=== AGFS Helper Functions Examples ===\n")

        # Setup: Create test directory and files
        test_dir = "/local/test"
        print(f"Setting up test directory: {test_dir}")
        try:
            client.mkdir(test_dir)
        except AGFSClientError:
            # Directory might already exist
            pass

        # Create some test files
        print("Creating test files...")
        client.write(f"{test_dir}/file1.txt", b"This is file 1")
        client.write(f"{test_dir}/file2.txt", b"This is file 2")

        # Create a subdirectory with files
        client.mkdir(f"{test_dir}/subdir")
        client.write(f"{test_dir}/subdir/file3.txt", b"This is file 3 in subdir")
        client.write(f"{test_dir}/subdir/file4.txt", b"This is file 4 in subdir")
        print()

        # Example 1: Copy a single file within AGFS
        print("1. Copy single file within AGFS:")
        print(f"   cp(client, '{test_dir}/file1.txt', '{test_dir}/file1_copy.txt')")
        cp(client, f"{test_dir}/file1.txt", f"{test_dir}/file1_copy.txt")
        print("   ✓ File copied successfully")

        # Verify
        content = client.cat(f"{test_dir}/file1_copy.txt")
        print(f"   Content: {content.decode()}")
        print()

        # Example 2: Copy a directory recursively within AGFS
        print("2. Copy directory recursively within AGFS:")
        print(f"   cp(client, '{test_dir}/subdir', '{test_dir}/subdir_copy', recursive=True)")
        cp(client, f"{test_dir}/subdir", f"{test_dir}/subdir_copy", recursive=True)
        print("   ✓ Directory copied successfully")

        # Verify
        files = client.ls(f"{test_dir}/subdir_copy")
        print(f"   Files in copied directory: {[f['name'] for f in files]}")
        print()

        # Example 3: Upload a file from local filesystem to AGFS
        print("3. Upload file from local filesystem:")
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.txt') as f:
            local_file = f.name
            f.write("This is a local file to upload")

        print(f"   upload(client, '{local_file}', '{test_dir}/uploaded.txt')")
        upload(client, local_file, f"{test_dir}/uploaded.txt")
        print("   ✓ File uploaded successfully")

        # Verify
        content = client.cat(f"{test_dir}/uploaded.txt")
        print(f"   Content: {content.decode()}")

        # Clean up temp file
        os.unlink(local_file)
        print()

        # Example 4: Upload a directory from local filesystem to AGFS
        print("4. Upload directory from local filesystem:")
        with tempfile.TemporaryDirectory() as tmpdir:
            # Create local directory structure
            os.makedirs(os.path.join(tmpdir, "local_dir"))
            with open(os.path.join(tmpdir, "local_dir", "local1.txt"), 'w') as f:
                f.write("Local file 1")
            with open(os.path.join(tmpdir, "local_dir", "local2.txt"), 'w') as f:
                f.write("Local file 2")

            local_dir = os.path.join(tmpdir, "local_dir")
            print(f"   upload(client, '{local_dir}', '{test_dir}/uploaded_dir', recursive=True)")
            upload(client, local_dir, f"{test_dir}/uploaded_dir", recursive=True)
            print("   ✓ Directory uploaded successfully")

            # Verify
            files = client.ls(f"{test_dir}/uploaded_dir")
            print(f"   Files in uploaded directory: {[f['name'] for f in files]}")
        print()

        # Example 5: Download a file from AGFS to local filesystem
        print("5. Download file from AGFS to local filesystem:")
        with tempfile.TemporaryDirectory() as tmpdir:
            local_download = os.path.join(tmpdir, "downloaded.txt")
            print(f"   download(client, '{test_dir}/file2.txt', '{local_download}')")
            download(client, f"{test_dir}/file2.txt", local_download)
            print("   ✓ File downloaded successfully")

            # Verify
            with open(local_download, 'r') as f:
                content = f.read()
            print(f"   Content: {content}")
        print()

        # Example 6: Download a directory from AGFS to local filesystem
        print("6. Download directory from AGFS to local filesystem:")
        with tempfile.TemporaryDirectory() as tmpdir:
            local_dir_download = os.path.join(tmpdir, "downloaded_dir")
            print(f"   download(client, '{test_dir}/subdir', '{local_dir_download}', recursive=True)")
            download(client, f"{test_dir}/subdir", local_dir_download, recursive=True)
            print("   ✓ Directory downloaded successfully")

            # Verify
            files = os.listdir(local_dir_download)
            print(f"   Files in downloaded directory: {files}")

            # Read one file to verify content
            with open(os.path.join(local_dir_download, "file3.txt"), 'r') as f:
                content = f.read()
            print(f"   Content of file3.txt: {content}")
        print()

        # Example 7: Use streaming for large files
        print("7. Copy large file with streaming:")
        # Create a larger test file
        large_content = b"Large file content\n" * 1000  # ~19KB
        client.write(f"{test_dir}/large_file.txt", large_content)

        print(f"   cp(client, '{test_dir}/large_file.txt', '{test_dir}/large_copy.txt', stream=True)")
        cp(client, f"{test_dir}/large_file.txt", f"{test_dir}/large_copy.txt", stream=True)
        print("   ✓ Large file copied with streaming")

        # Verify size
        info = client.stat(f"{test_dir}/large_copy.txt")
        print(f"   Size: {info.get('size')} bytes")
        print()

        # Clean up
        print("Cleaning up test directory...")
        client.rm(test_dir, recursive=True)
        print("✓ Done!\n")

        print("=== All Examples Completed Successfully ===")

    except AGFSClientError as e:
        print(f"Error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
        # Try to clean up on error
        try:
            client.rm(test_dir, recursive=True)
        except:
            pass


if __name__ == "__main__":
    main()
