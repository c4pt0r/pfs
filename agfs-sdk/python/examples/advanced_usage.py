"""Advanced usage examples for pyagfs"""

from pyagfs import AGFSClient, AGFSClientError
import time


def mount_example(client):
    """Example of mounting plugins"""
    print("=== Mount Management ===")

    # List current mounts
    print("Current mounts:")
    mounts = client.mounts()
    for mount in mounts:
        print(f"  {mount['path']} -> {mount['pluginName']}")
    print()

    # Mount a memory filesystem
    mount_path = "/test_mem"
    print(f"Mounting memfs at {mount_path}")
    try:
        client.mount("memfs", mount_path, {})
        print("Mount successful!")
    except AGFSClientError as e:
        print(f"Mount failed: {e}")
    print()

    # Use the mounted filesystem
    print("Testing mounted filesystem:")
    test_file = f"{mount_path}/test.txt"
    client.write(test_file, b"Data in memory filesystem")
    content = client.cat(test_file)
    print(f"  Wrote and read: {content.decode()}")
    print()

    # Unmount
    print(f"Unmounting {mount_path}")
    try:
        client.unmount(mount_path)
        print("Unmount successful!")
    except AGFSClientError as e:
        print(f"Unmount failed: {e}")
    print()


def grep_example(client):
    """Example of using grep functionality"""
    print("=== Grep Search ===")

    # Create test files with content
    test_dir = "/local/test_grep"
    client.mkdir(test_dir)

    # Write test files
    client.write(f"{test_dir}/file1.txt", b"This is a test file\nWith some error messages\n")
    client.write(f"{test_dir}/file2.txt", b"Another test file\nNo issues here\n")
    client.write(f"{test_dir}/file3.log", b"ERROR: Something went wrong\nWARNING: Be careful\n")

    # Search for pattern
    print(f"Searching for 'error' in {test_dir}:")
    result = client.grep(test_dir, "error", recursive=True, case_insensitive=True)
    print(f"Found {result['count']} matches:")
    for match in result['matches']:
        print(f"  {match['file']}:{match['line']}: {match['content'].strip()}")
    print()

    # Clean up
    client.rm(test_dir, recursive=True)


def streaming_example(client):
    """Example of streaming operations"""
    print("=== Streaming Operations ===")

    # Create a test file
    test_file = "/streamfs/test_stream.txt"
    large_content = b"Line %d\n" * 100
    lines = b"".join([b"Line %d\n" % i for i in range(100)])
    client.write(test_file, lines)

    # Stream read
    print(f"Streaming read from {test_file} (first 5 chunks):")
    response = client.cat(test_file, stream=True)
    chunk_count = 0
    for chunk in response.iter_content(chunk_size=100):
        if chunk_count < 5:
            print(f"  Chunk {chunk_count + 1}: {len(chunk)} bytes")
        chunk_count += 1
        if chunk_count >= 5:
            break
    print(f"  ... (total {chunk_count}+ chunks)")
    print()

    # Clean up
    client.rm(test_file)


def batch_operations(client):
    """Example of batch file operations"""
    print("=== Batch Operations ===")

    # Create multiple files
    batch_dir = "/local/test_batch"
    client.mkdir(batch_dir)

    print("Creating 10 files:")
    for i in range(10):
        filename = f"{batch_dir}/file_{i:02d}.txt"
        client.write(filename, f"File number {i}".encode())
        print(f"  Created {filename}")
    print()

    # List all files
    print(f"Files in {batch_dir}:")
    files = client.ls(batch_dir)
    for file in files:
        info = client.stat(f"{batch_dir}/{file['name']}")
        print(f"  {file['name']} - {info['size']} bytes")
    print()

    # Clean up
    print("Cleaning up...")
    client.rm(batch_dir, recursive=True)
    print("Done!")
    print()


def main():
    # Initialize client
    client = AGFSClient("http://localhost:8080")

    try:
        # Check connection
        health = client.health()
        print(f"Connected to AGFS server (version: {health.get('version', 'unknown')})")
        print()

        # Run examples
        mount_example(client)
        grep_example(client)
        streaming_example(client)
        batch_operations(client)

    except AGFSClientError as e:
        print(f"Error: {e}")


if __name__ == "__main__":
    main()
