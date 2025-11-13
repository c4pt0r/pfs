package filesystem

import "io"

// WriteFunc is a function that writes data to a path and returns the result and any error.
// This is typically a FileSystem's Write method.
type WriteFunc func(path string, data []byte) ([]byte, error)

// BufferedWriter is a generic io.WriteCloser that buffers writes in memory
// and flushes them when Close() is called.
// This is useful for filesystem implementations that don't support streaming writes.
type BufferedWriter struct {
	path      string
	buf       []byte
	writeFunc WriteFunc
}

// NewBufferedWriter creates a new BufferedWriter that will write to the given path
// using the provided write function when Close() is called.
func NewBufferedWriter(path string, writeFunc WriteFunc) *BufferedWriter {
	return &BufferedWriter{
		path:      path,
		buf:       make([]byte, 0),
		writeFunc: writeFunc,
	}
}

// Write appends data to the internal buffer.
// It never returns an error, following the io.Writer contract.
func (w *BufferedWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// Close flushes the buffered data by calling the write function and returns any error.
func (w *BufferedWriter) Close() error {
	_, err := w.writeFunc(w.path, w.buf)
	return err
}

// Ensure BufferedWriter implements io.WriteCloser
var _ io.WriteCloser = (*BufferedWriter)(nil)
