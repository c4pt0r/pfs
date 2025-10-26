S3FS Plugin - AWS S3-backed File System

This plugin provides a file system backed by AWS S3 object storage.

FEATURES:
  - Store files and directories in AWS S3
  - Support for S3-compatible services (MinIO, LocalStack, etc.)
  - Full POSIX-like file system operations
  - Automatic directory handling
  - Optional key prefix for namespace isolation

CONFIGURATION:

  AWS S3:
  [plugins.s3fs]
  enabled = true
  path = "/s3fs"

    [plugins.s3fs.config]
    region = "us-east-1"
    bucket = "my-bucket"
    access_key_id = "AKIAIOSFODNN7EXAMPLE"
    secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    prefix = "pfs/"  # Optional: all keys will be prefixed with this

  S3-Compatible Service (MinIO, LocalStack):
  [plugins.s3fs]
  enabled = true
  path = "/s3fs"

    [plugins.s3fs.config]
    region = "us-east-1"
    bucket = "my-bucket"
    access_key_id = "minioadmin"
    secret_access_key = "minioadmin"
    endpoint = "http://localhost:9000"
    disable_ssl = true

  Multiple S3 Buckets:
  [plugins.s3fs_prod]
  enabled = true
  path = "/s3/prod"

    [plugins.s3fs_prod.config]
    region = "us-east-1"
    bucket = "production-bucket"
    access_key_id = "..."
    secret_access_key = "..."

  [plugins.s3fs_dev]
  enabled = true
  path = "/s3/dev"

    [plugins.s3fs_dev.config]
    region = "us-west-2"
    bucket = "development-bucket"
    access_key_id = "..."
    secret_access_key = "..."

USAGE:

  Create a directory:
    pfs mkdir /s3fs/data

  Create a file:
    pfs write /s3fs/data/file.txt "Hello, S3!"

  Read a file:
    pfs cat /s3fs/data/file.txt

  List directory:
    pfs ls /s3fs/data

  Remove file:
    pfs rm /s3fs/data/file.txt

  Remove directory (must be empty):
    pfs rm /s3fs/data

  Remove directory recursively:
    pfs rm -r /s3fs/data

EXAMPLES:

  # Basic file operations
  pfs:/> mkdir /s3fs/documents
  pfs:/> echo "Important data" > /s3fs/documents/report.txt
  pfs:/> cat /s3fs/documents/report.txt
  Important data

  # List contents
  pfs:/> ls /s3fs/documents
  report.txt

  # Move/rename
  pfs:/> mv /s3fs/documents/report.txt /s3fs/documents/report-2024.txt

NOTES:
  - S3 doesn't have real directories; they are simulated with "/" in object keys
  - Large files may take time to upload/download
  - Permissions (chmod) are not supported by S3
  - Atomic operations are limited by S3's eventual consistency model

USE CASES:
  - Cloud-native file storage
  - Backup and archival
  - Sharing files across distributed systems
  - Cost-effective long-term storage
  - Integration with AWS services

ADVANTAGES:
  - Unlimited storage capacity
  - High durability (99.999999999%)
  - Geographic redundancy
  - Pay-per-use pricing
  - Versioning and lifecycle policies (via S3 bucket settings)

## License

Apache License 2.0
