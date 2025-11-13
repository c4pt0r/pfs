package filesystem

import (
	"path/filepath"
	"strings"
)

// NormalizePath normalizes a filesystem path to a canonical form.
// - Empty paths and "/" return "/"
// - Adds leading "/" if missing
// - Cleans the path (removes .., ., etc.)
// - Removes trailing slashes (except for root "/")
//
// This is used by most filesystem implementations (memfs, sqlfs, httpfs, etc.)
func NormalizePath(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	// Ensure leading slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Clean the path (resolve .., ., etc.)
	path = filepath.Clean(path)

	// filepath.Clean can return "." for some inputs
	if path == "." {
		return "/"
	}

	// Remove trailing slash (Clean might leave it in some cases)
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}

	return path
}

// NormalizeS3Key normalizes an S3 object key.
// S3 keys don't have a leading slash, so this:
// - Returns "" for empty paths or "/"
// - Removes leading "/"
// - Cleans the path
//
// This is used specifically by s3fs plugin.
func NormalizeS3Key(path string) string {
	if path == "" || path == "/" {
		return ""
	}

	// Remove leading slash (S3 keys don't have them)
	path = strings.TrimPrefix(path, "/")

	// Clean the path
	path = filepath.Clean(path)

	// filepath.Clean returns "." for empty/root paths
	if path == "." {
		return ""
	}

	return path
}
