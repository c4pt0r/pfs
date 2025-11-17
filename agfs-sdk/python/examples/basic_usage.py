"""Basic usage examples for pyagfs"""

from pyagfs import AGFSClient, AGFSClientError


def main():
    # Initialize client
    client = AGFSClient("http://localhost:8080")

    try:
        # Check server health
        print("Checking server health...")
        health = client.health()
        print(f"Server version: {health.get('version', 'unknown')}")
        print()

        # List directory contents
        print("Listing root directory:")
        files = client.ls("/")
        for file in files:
            file_type = "DIR " if file["isDir"] else "FILE"
            print(f"  [{file_type}] {file['name']}")
        print()

        # Create a test directory
        test_dir = "/test_pyagfs"
        print(f"Creating directory: {test_dir}")
        client.mkdir(test_dir)
        print()

        # Create and write to a file
        test_file = f"{test_dir}/hello.txt"
        content = b"Hello from pyagfs SDK!"
        print(f"Writing to file: {test_file}")
        client.write(test_file, content)
        print()

        # Read the file back
        print(f"Reading file: {test_file}")
        read_content = client.cat(test_file)
        print(f"Content: {read_content.decode()}")
        print()

        # Get file information
        print(f"Getting file info: {test_file}")
        info = client.stat(test_file)
        print(f"  Size: {info.get('size')} bytes")
        print(f"  Mode: {info.get('mode')}")
        print()

        # List the test directory
        print(f"Listing {test_dir}:")
        files = client.ls(test_dir)
        for file in files:
            print(f"  - {file['name']}")
        print()

        # Rename the file
        new_file = f"{test_dir}/renamed.txt"
        print(f"Renaming {test_file} to {new_file}")
        client.mv(test_file, new_file)
        print()

        # Clean up
        print(f"Removing directory: {test_dir}")
        client.rm(test_dir, recursive=True)
        print("Done!")

    except AGFSClientError as e:
        print(f"Error: {e}")


if __name__ == "__main__":
    main()
