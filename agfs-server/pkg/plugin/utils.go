package plugin

import "io"

// ApplyRangeRead applies offset and size to data slice
// Returns io.EOF if offset+size >= len(data)
func ApplyRangeRead(data []byte, offset int64, size int64) ([]byte, error) {
	dataLen := int64(len(data))

	// Validate offset
	if offset < 0 {
		offset = 0
	}
	if offset >= dataLen {
		return nil, io.EOF
	}

	// Calculate end position
	var end int64
	if size < 0 {
		// Read all remaining data
		end = dataLen
	} else {
		end = offset + size
		if end > dataLen {
			end = dataLen
		}
	}

	result := data[offset:end]
	if end >= dataLen {
		return result, io.EOF
	}
	return result, nil
}
