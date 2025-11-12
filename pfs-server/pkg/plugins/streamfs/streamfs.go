package streamfs

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c4pt0r/pfs/pfs-server/pkg/filesystem"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin"
	"github.com/c4pt0r/pfs/pfs-server/pkg/plugin/config"
	log "github.com/sirupsen/logrus"
)

const (
	PluginName = "streamfs" // Name of this plugin
)

// parseSize parses a size string like "512KB", "1MB", "100MB" and returns bytes
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Handle pure numbers (bytes)
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val, nil
	}

	// Parse with unit suffix
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
	}

	for suffix, multiplier := range units {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)

			// Try parsing as float first (for "1.5MB")
			if val, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(val * float64(multiplier)), nil
			}
		}
	}

	return 0, fmt.Errorf("invalid size format: %s (expected format: 512KB, 1MB, etc)", s)
}

// formatSize formats bytes into human-readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}

// Reader represents a single reader with its channel and metadata
type Reader struct {
	id           string
	ch           chan []byte
	registered   time.Time
	droppedCount int64 // Number of chunks dropped due to slow consumption
	readIndex    int64 // Index of next chunk to read from ringBuffer (int64 to prevent overflow)
}

// streamReader wraps a registered reader and implements filesystem.StreamReader
type streamReader struct {
	sf       *StreamFile
	readerID string
	ch       <-chan []byte
}

// ReadChunk implements filesystem.StreamReader
func (sr *streamReader) ReadChunk(timeout time.Duration) ([]byte, bool, error) {
	return sr.sf.ReadChunk(sr.readerID, sr.ch, timeout)
}

// Close implements filesystem.StreamReader
func (sr *streamReader) Close() error {
	sr.sf.UnregisterReader(sr.readerID)
	return nil
}

// StreamFile represents a streaming file that supports multiple readers and writers
type StreamFile struct {
	name          string
	mu            sync.RWMutex
	offset        int64              // Total bytes written
	closed        bool               // Whether the stream is closed
	modTime       time.Time          // Last modification time
	readers       map[string]*Reader // All registered readers
	nextReaderID  int                // Auto-increment reader ID
	channelBuffer int                // Buffer size for each reader channel

	// Ring buffer for storing recent chunks (even when no readers)
	ringBuffer  [][]byte // Circular buffer for recent chunks
	ringSize    int      // Max number of chunks to keep
	writeIndex  int64    // Current write position in ring buffer (int64 to prevent overflow)
	totalChunks int64    // Total chunks written (for readIndex tracking)
}

// NewStreamFile creates a new stream file
func NewStreamFile(name string, channelBuffer int, ringSize int) *StreamFile {
	if channelBuffer <= 0 {
		channelBuffer = 100 // Default buffer size
	}
	if ringSize <= 0 {
		ringSize = 100 // Default ring buffer size
	}
	sf := &StreamFile{
		name:          name,
		modTime:       time.Now(),
		readers:       make(map[string]*Reader),
		nextReaderID:  0,
		channelBuffer: channelBuffer,
		ringBuffer:    make([][]byte, ringSize),
		ringSize:      ringSize,
		writeIndex:    0,
		totalChunks:   0,
	}
	return sf
}

// RegisterReader registers a new reader and returns reader ID and channel
// New readers will receive ALL available historical data from ring buffer
func (sf *StreamFile) RegisterReader() (string, <-chan []byte) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	readerID := fmt.Sprintf("reader_%d_%d", sf.nextReaderID, time.Now().UnixNano())
	sf.nextReaderID++

	// Calculate oldest available chunk in ring buffer
	historyStart := sf.totalChunks - int64(sf.ringSize)
	if historyStart < 0 {
		historyStart = 0
	}

	// New readers start from the beginning of available history
	reader := &Reader{
		id:           readerID,
		ch:           make(chan []byte, sf.channelBuffer),
		registered:   time.Now(),
		droppedCount: 0,
		readIndex:    historyStart, // Start from oldest available data
	}
	sf.readers[readerID] = reader

	log.Infof("[streamfs] Registered reader %s for stream %s (total readers: %d, starting at chunk %d, current chunk: %d)",
		readerID, sf.name, len(sf.readers), reader.readIndex, sf.totalChunks)

	// Send any available historical data from ring buffer
	go sf.sendHistoricalData(reader)

	return readerID, reader.ch
}

// sendHistoricalData sends historical chunks from ring buffer to a new reader
func (sf *StreamFile) sendHistoricalData(reader *Reader) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	// Calculate how many historical chunks are available
	historyStart := sf.totalChunks - int64(sf.ringSize)
	if historyStart < 0 {
		historyStart = 0
	}

	// If reader wants to start from the beginning and we have history
	if reader.readIndex < sf.totalChunks && sf.totalChunks > 0 {
		log.Debugf("[streamfs] Sending historical data to reader %s (from chunk %d to %d)",
			reader.id, historyStart, sf.totalChunks)

		// Send available historical chunks
		for i := historyStart; i < sf.totalChunks; i++ {
			ringIdx := int(i % int64(sf.ringSize))
			if sf.ringBuffer[ringIdx] != nil {
				select {
				case reader.ch <- sf.ringBuffer[ringIdx]:
					// Sent successfully
				default:
					// Channel full, will catch up with live data
					log.Warnf("[streamfs] Reader %s channel full during historical data send", reader.id)
					return
				}
			}
		}
	}
}

// UnregisterReader unregisters a reader and closes its channel
func (sf *StreamFile) UnregisterReader(readerID string) {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if reader, exists := sf.readers[readerID]; exists {
		close(reader.ch)
		delete(sf.readers, readerID)
		log.Infof("[streamfs] Unregistered reader %s for stream %s (dropped: %d chunks, total readers: %d)",
			readerID, sf.name, reader.droppedCount, len(sf.readers))
	}
}

// Write appends data to the stream and fanout to all readers
func (sf *StreamFile) Write(data []byte) error {
	sf.mu.Lock()

	if sf.closed {
		sf.mu.Unlock()
		return fmt.Errorf("stream is closed")
	}

	// Copy data to avoid external modification
	chunk := make([]byte, len(data))
	copy(chunk, data)

	sf.offset += int64(len(data))
	sf.modTime = time.Now()

	// Store in ring buffer (always, even if no readers)
	ringIdx := int(sf.writeIndex % int64(sf.ringSize))
	sf.ringBuffer[ringIdx] = chunk
	sf.writeIndex++
	sf.totalChunks++

	// Take a snapshot of all reader channels to avoid holding lock during send
	readerSnapshot := make([]*Reader, 0, len(sf.readers))
	for _, reader := range sf.readers {
		readerSnapshot = append(readerSnapshot, reader)
	}

	sf.mu.Unlock()

	// Fanout to all readers (non-blocking)
	successCount := 0
	dropCount := 0
	for _, reader := range readerSnapshot {
		select {
		case reader.ch <- chunk:
			successCount++
		default:
			// Channel is full - slow consumer, drop the chunk
			reader.droppedCount++
			dropCount++
			log.Warnf("[streamfs] Reader %s is slow, dropped chunk (total dropped: %d)", reader.id, reader.droppedCount)
		}
	}

	if len(readerSnapshot) == 0 {
		log.Debugf("[streamfs] Buffered %d bytes to ring (no readers, total chunks: %d)",
			len(data), sf.totalChunks)
	} else {
		log.Debugf("[streamfs] Fanout %d bytes to %d readers (success: %d, dropped: %d, total chunks: %d)",
			len(data), len(readerSnapshot), successCount, dropCount, sf.totalChunks)
	}

	return nil
}

// ReadChunk reads data from a reader's channel (blocking with timeout)
// Returns (data, eof, error)
// This method should be called after RegisterReader
func (sf *StreamFile) ReadChunk(readerID string, ch <-chan []byte, timeout time.Duration) ([]byte, bool, error) {
	select {
	case data, ok := <-ch:
		if !ok {
			// Channel closed - stream is closed or reader was unregistered
			return nil, true, io.EOF
		}
		return data, false, nil
	case <-time.After(timeout):
		// Check if stream is closed
		sf.mu.RLock()
		closed := sf.closed
		sf.mu.RUnlock()

		if closed {
			return nil, true, io.EOF
		}
		return nil, false, fmt.Errorf("read timeout")
	}
}

// Close closes the stream and all reader channels
func (sf *StreamFile) Close() error {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	sf.closed = true

	// Close all reader channels
	for id, reader := range sf.readers {
		close(reader.ch)
		log.Infof("[streamfs] Closed reader %s for stream %s (dropped: %d chunks)", id, sf.name, reader.droppedCount)
	}
	// Clear readers map
	sf.readers = make(map[string]*Reader)

	log.Infof("[streamfs] Stream %s closed", sf.name)
	return nil
}

// GetInfo returns file info
func (sf *StreamFile) GetInfo() filesystem.FileInfo {
	sf.mu.RLock()
	defer sf.mu.RUnlock()

	// Remove leading slash from name for display
	name := sf.name
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}

	return filesystem.FileInfo{
		Name:    name,
		Size:    sf.offset, // Total bytes written
		Mode:    0644,
		ModTime: sf.modTime,
		IsDir:   false,
		Meta: filesystem.MetaData{
			Name: PluginName,
			Type: "stream",
			Content: map[string]string{
				"total_written":  fmt.Sprintf("%d", sf.offset),
				"active_readers": fmt.Sprintf("%d", len(sf.readers)),
			},
		},
	}
}

// StreamFS implements FileSystem interface for streaming files
type StreamFS struct {
	streams       map[string]*StreamFile
	mu            sync.RWMutex
	channelBuffer int // Default channel buffer size per reader
	ringSize      int // Ring buffer size for historical data
	pluginName    string
}

// NewStreamFS creates a new StreamFS
func NewStreamFS(channelBuffer int, ringSize int) *StreamFS {
	if channelBuffer <= 0 {
		channelBuffer = 100 // Default: 100 chunks per reader
	}
	if ringSize <= 0 {
		ringSize = 100 // Default: 100 chunks in ring buffer
	}
	return &StreamFS{
		streams:       make(map[string]*StreamFile),
		channelBuffer: channelBuffer,
		ringSize:      ringSize,
		pluginName:    PluginName,
	}
}

func (sfs *StreamFS) Create(path string) error {
	sfs.mu.Lock()
	defer sfs.mu.Unlock()

	if _, exists := sfs.streams[path]; exists {
		return fmt.Errorf("stream already exists: %s", path)
	}

	sfs.streams[path] = NewStreamFile(path, sfs.channelBuffer, sfs.ringSize)
	return nil
}

func (sfs *StreamFS) Mkdir(path string, perm uint32) error {
	return fmt.Errorf("streamfs does not support directories")
}

func (sfs *StreamFS) Remove(path string) error {
	sfs.mu.Lock()
	defer sfs.mu.Unlock()

	stream, exists := sfs.streams[path]
	if !exists {
		return fmt.Errorf("stream not found: %s", path)
	}

	stream.Close()
	delete(sfs.streams, path)
	return nil
}

func (sfs *StreamFS) RemoveAll(path string) error {
	return sfs.Remove(path)
}

// Read is not suitable for streaming, use ReadChunk instead
// This is here for compatibility with FileSystem interface
func (sfs *StreamFS) Read(path string, offset int64, size int64) ([]byte, error) {
	// README file can be read normally
	if path == "/README" {
		content := []byte(getReadme())
		return plugin.ApplyRangeRead(content, offset, size)
	}

	// Stream files must use --stream mode
	return nil, fmt.Errorf("use stream mode for reading stream files")
}

func (sfs *StreamFS) Write(path string, data []byte) ([]byte, error) {
	sfs.mu.Lock()
	stream, exists := sfs.streams[path]
	if !exists {
		// Auto-create stream on first write
		stream = NewStreamFile(path, sfs.channelBuffer, sfs.ringSize)
		sfs.streams[path] = stream
	}
	sfs.mu.Unlock()

	err := stream.Write(data)
	if err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("Written %d bytes to stream", len(data))), nil
}

func (sfs *StreamFS) ReadDir(path string) ([]filesystem.FileInfo, error) {
	if path != "/" {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	sfs.mu.RLock()
	defer sfs.mu.RUnlock()

	readme := filesystem.FileInfo{
		Name:    "README",
		Size:    int64(len(getReadme())),
		Mode:    0444,
		ModTime: time.Now(),
		IsDir:   false,
		Meta: filesystem.MetaData{
			Name: PluginName,
			Type: "doc",
		},
	}

	files := []filesystem.FileInfo{readme}
	for _, stream := range sfs.streams {
		files = append(files, stream.GetInfo())
	}

	return files, nil
}

func (sfs *StreamFS) Stat(path string) (*filesystem.FileInfo, error) {
	if path == "/" {
		info := &filesystem.FileInfo{
			Name:    "/",
			Size:    0,
			Mode:    0755,
			ModTime: time.Now(),
			IsDir:   true,
			Meta: filesystem.MetaData{
				Name: PluginName,
			},
		}
		return info, nil
	}

	if path == "/README" {
		readme := getReadme()
		info := &filesystem.FileInfo{
			Name:    "README",
			Size:    int64(len(readme)),
			Mode:    0444,
			ModTime: time.Now(),
			IsDir:   false,
			Meta: filesystem.MetaData{
				Name: PluginName,
				Type: "doc",
			},
		}
		return info, nil
	}

	sfs.mu.RLock()
	stream, exists := sfs.streams[path]
	sfs.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stream not found: %s", path)
	}

	info := stream.GetInfo()
	return &info, nil
}

func (sfs *StreamFS) Rename(oldPath, newPath string) error {
	return fmt.Errorf("streamfs does not support rename")
}

func (sfs *StreamFS) Chmod(path string, mode uint32) error {
	return fmt.Errorf("streamfs does not support chmod")
}

func (sfs *StreamFS) Open(path string) (io.ReadCloser, error) {
	if path == "/README" {
		return io.NopCloser(bytes.NewReader([]byte(getReadme()))), nil
	}
	return nil, fmt.Errorf("use stream mode for reading stream files")
}

func (sfs *StreamFS) OpenWrite(path string) (io.WriteCloser, error) {
	return &streamWriter{sfs: sfs, path: path}, nil
}

// OpenStream implements filesystem.Streamer interface
func (sfs *StreamFS) OpenStream(path string) (filesystem.StreamReader, error) {
	sfs.mu.Lock()
	stream, exists := sfs.streams[path]
	if !exists {
		// Auto-create stream if it doesn't exist (for readers to connect before writer)
		stream = NewStreamFile(path, sfs.channelBuffer, sfs.ringSize)
		sfs.streams[path] = stream
		log.Infof("[streamfs] Auto-created stream %s for reader", path)
	}
	sfs.mu.Unlock()

	// Register a new reader
	readerID, ch := stream.RegisterReader()
	log.Infof("[streamfs] Opened stream %s with reader %s", path, readerID)

	return &streamReader{
		sf:       stream,
		readerID: readerID,
		ch:       ch,
	}, nil
}

// GetStream returns the stream for reading (deprecated, use OpenStream)
// Kept for backward compatibility
func (sfs *StreamFS) GetStream(path string) (interface{}, error) {
	sfs.mu.Lock()
	defer sfs.mu.Unlock()

	stream, exists := sfs.streams[path]
	if !exists {
		// Auto-create stream if it doesn't exist (for readers to connect before writer)
		stream = NewStreamFile(path, sfs.channelBuffer, sfs.ringSize)
		sfs.streams[path] = stream
		log.Infof("[streamfs] Auto-created stream %s for reader", path)
	}

	return stream, nil
}

type streamWriter struct {
	sfs  *StreamFS
	path string
}

func (sw *streamWriter) Write(p []byte) (n int, err error) {
	_, err = sw.sfs.Write(sw.path, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (sw *streamWriter) Close() error {
	return nil
}

// StreamFSPlugin wraps StreamFS as a plugin
type StreamFSPlugin struct {
	fs            *StreamFS
	channelBuffer int
	ringSize      int
}

// NewStreamFSPlugin creates a new StreamFS plugin
func NewStreamFSPlugin() *StreamFSPlugin {
	return &StreamFSPlugin{
		channelBuffer: 100, // Default: 100 chunks per reader channel
		ringSize:      100, // Default: 100 chunks in ring buffer
	}
}

func (p *StreamFSPlugin) Name() string {
	return PluginName
}

func (p *StreamFSPlugin) Validate(cfg map[string]interface{}) error {
	// Check for unknown parameters
	allowedKeys := []string{"channel_buffer_size", "ring_buffer_size", "mount_path"}
	if err := config.ValidateOnlyKnownKeys(cfg, allowedKeys); err != nil {
		return err
	}

	// Validate channel_buffer_size if provided
	if val, exists := cfg["channel_buffer_size"]; exists {
		switch v := val.(type) {
		case string:
			if _, err := config.ParseSize(v); err != nil {
				return fmt.Errorf("invalid channel_buffer_size: %w", err)
			}
		case int, int64, float64:
			// Valid numeric types
		default:
			return fmt.Errorf("channel_buffer_size must be a size string (e.g., '512KB') or number")
		}
	}

	// Validate ring_buffer_size if provided
	if val, exists := cfg["ring_buffer_size"]; exists {
		switch v := val.(type) {
		case string:
			if _, err := config.ParseSize(v); err != nil {
				return fmt.Errorf("invalid ring_buffer_size: %w", err)
			}
		case int, int64, float64:
			// Valid numeric types
		default:
			return fmt.Errorf("ring_buffer_size must be a size string (e.g., '1MB') or number")
		}
	}

	return nil
}

func (p *StreamFSPlugin) Initialize(config map[string]interface{}) error {
	const defaultChunkSize = 64 * 1024 // 64KB per chunk

	// Parse channel buffer size from config (support both bytes and string with units)
	channelBufferBytes := int64(6 * 1024 * 1024) // Default: 6MB
	if bufSizeStr, ok := config["channel_buffer_size"].(string); ok {
		if parsed, err := parseSize(bufSizeStr); err == nil {
			channelBufferBytes = parsed
		} else {
			log.Warnf("[streamfs] Invalid channel_buffer_size '%s': %v, using default", bufSizeStr, err)
		}
	} else if bufSize, ok := config["channel_buffer_size"].(int); ok {
		channelBufferBytes = int64(bufSize)
	} else if bufSizeFloat, ok := config["channel_buffer_size"].(float64); ok {
		channelBufferBytes = int64(bufSizeFloat)
	} else if bufSizeInt64, ok := config["channel_buffer_size"].(int64); ok {
		channelBufferBytes = bufSizeInt64
	}

	// Parse ring buffer size from config (support both bytes and string with units)
	ringBufferBytes := int64(6 * 1024 * 1024) // Default: 6MB
	if ringSizeStr, ok := config["ring_buffer_size"].(string); ok {
		if parsed, err := parseSize(ringSizeStr); err == nil {
			ringBufferBytes = parsed
		} else {
			log.Warnf("[streamfs] Invalid ring_buffer_size '%s': %v, using default", ringSizeStr, err)
		}
	} else if ringSize, ok := config["ring_buffer_size"].(int); ok {
		ringBufferBytes = int64(ringSize)
	} else if ringSizeFloat, ok := config["ring_buffer_size"].(float64); ok {
		ringBufferBytes = int64(ringSizeFloat)
	} else if ringSizeInt64, ok := config["ring_buffer_size"].(int64); ok {
		ringBufferBytes = ringSizeInt64
	}

	// Convert bytes to number of chunks
	p.channelBuffer = int(channelBufferBytes / defaultChunkSize)
	if p.channelBuffer < 1 {
		p.channelBuffer = 1
	}

	p.ringSize = int(ringBufferBytes / defaultChunkSize)
	if p.ringSize < 1 {
		p.ringSize = 1
	}

	p.fs = NewStreamFS(p.channelBuffer, p.ringSize)
	log.Infof("[streamfs] Initialized with channel buffer: %s (%d chunks), ring buffer: %s (%d chunks)",
		formatSize(channelBufferBytes), p.channelBuffer,
		formatSize(ringBufferBytes), p.ringSize)
	return nil
}

func (p *StreamFSPlugin) GetFileSystem() filesystem.FileSystem {
	return p.fs
}

func (p *StreamFSPlugin) GetReadme() string {
	return getReadme()
}

func (p *StreamFSPlugin) Shutdown() error {
	return nil
}

func getReadme() string {
	return `StreamFS Plugin - Streaming File System

This plugin provides streaming files that support multiple concurrent readers and writers
with real-time data fanout and ring buffer for late joiners.

FEATURES:
  - Multiple writers can append data to a stream concurrently
  - Multiple readers can consume from the stream independently (fanout/broadcast)
  - Ring buffer (1000 chunks) stores recent data for late-joining readers
  - Persistent streaming: readers wait indefinitely for new data (no timeout disconnect)
  - HTTP chunked transfer with automatic flow control
  - Memory-based storage with configurable channel buffer per reader

ARCHITECTURE:
  - Each stream maintains a ring buffer of recent chunks (default: last 1000 chunks)
  - New readers automatically receive all available historical data from ring buffer
  - Writers fanout data to all active readers via buffered channels
  - Readers wait indefinitely for new data (30s check interval, but never disconnect)
  - Slow readers may drop chunks if their channel buffer fills up

COMMAND REFERENCE:

  Write (Producer):
    cat file | pfs write --stream /streamfs/stream
    echo "data" | pfs write /streamfs/stream

  Read (Consumer):
    pfs cat --stream /streamfs/stream
    pfs cat --stream /streamfs/stream > output.dat
    pfs cat --stream /streamfs/stream | ffplay -

  Manage:
    pfs ls /streamfs
    pfs stat /streamfs/stream
    pfs rm /streamfs/stream

CONFIGURATION:

  [plugins.streamfs]
  enabled = true
  path = "/streamfs"

    [plugins.streamfs.config]
    # Channel buffer size per reader (supports units: KB, MB, GB or raw bytes)
    # Controls how much data each reader can buffer before dropping chunks
    # For live streaming: 256KB - 512KB (low latency)
    # For VOD/recording: 4MB - 8MB (smooth playback)
    # Default: 6MB
    # Examples: "512KB", "1MB", "6MB", or 524288 (bytes)
    channel_buffer_size = "512KB"

    # Ring buffer size for historical data (supports units: KB, MB, GB or raw bytes)
    # Stores recent data for late-joining readers
    # For live streaming: 512KB - 1MB (low latency, less memory)
    # For VOD: 4MB - 8MB (more history for seekable playback)
    # Default: 6MB
    # Examples: "1MB", "4MB", or 1048576 (bytes)
    ring_buffer_size = "1MB"

IMPORTANT NOTES:

  - Streams are in-memory only (not persistent across restarts)
  - Ring buffer stores recent data (configurable, default 6MB)
  - Late-joining readers receive historical data from ring buffer
  - Readers never timeout - they wait indefinitely for new data
  - Writer chunk size: 64KB (configured in CLI write --stream)
  - Channel buffer: configurable per reader (default 6MB)
  - Slow readers may drop chunks if they can't keep up
  - MUST use --stream flag for reading streams (cat --stream)
  - Regular cat without --stream will fail with error

MEMORY USAGE:

  File Size vs Memory Usage:
  - 'ls' and 'stat' show TOTAL BYTES WRITTEN (cumulative counter)
  - This is NOT the actual memory usage - just a throughput statistic
  - Example: Stream shows 1GB in 'ls', but only uses 6MB RAM (ring buffer)
  - The file size will continuously grow as data is written
  - This is similar to /dev/null - unlimited writes, fixed memory

  Actual Memory Footprint:
  - Ring buffer: Fixed at ring_buffer_size (default: 6MB)
  - Per reader channel: Fixed at channel_buffer_size (default: 6MB per reader)
  - Total memory = ring_buffer_size + (channel_buffer_size × number of readers)
  - Example with 3 readers: 6MB (ring) + 3×6MB (readers) = 24MB total
  - Old data in ring buffer is automatically overwritten (circular buffer)
  - No disk space is used - everything is in memory only

  Overflow Protection:
  - All counters use int64 to prevent overflow (max: 9.2 EB ≈ 292 years at 1GB/s)
  - Ring buffer index calculations are overflow-safe on both 32-bit and 64-bit systems
  - Stream can run indefinitely without counter overflow concerns

PERFORMANCE TIPS:

  - For live streaming: Use smaller buffers (256KB-512KB) to reduce latency
  - For VOD/recording: Use larger buffers (4MB-8MB) for smoother playback
  - For video streaming: Start writer first to fill ring buffer
  - Increase channel_buffer_size for high-bitrate streams
  - Decrease buffer sizes for interactive/live use cases
  - Monitor dropped chunks in logs (indicates slow readers)
  - Example low-latency config: channel=256KB, ring=512KB
  - Example high-throughput config: channel=8MB, ring=16MB

TROUBLESHOOTING:

  - Error "use stream mode": Use 'cat --stream' instead of 'cat'
  - Reader disconnects: Check if writer finished (readers wait indefinitely otherwise)
  - High memory usage: Reduce channel_buffer_size or limit concurrent readers

ARCHITECTURE DETAILS:

  - StreamFS implements filesystem.Streamer interface
  - Each reader gets a filesystem.StreamReader with independent position
  - Ring buffer enables time-shifting and late joining
  - Fanout is non-blocking: slow readers drop chunks, fast readers proceed
  - Graceful shutdown: closing stream sends EOF to all readers
`
}

// Ensure StreamFSPlugin implements ServicePlugin
var _ plugin.ServicePlugin = (*StreamFSPlugin)(nil)
