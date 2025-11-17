StreamFS Plugin - Streaming File System

This plugin provides streaming files that support multiple concurrent readers and writers
with real-time data fanout and ring buffer for late joiners.

DYNAMIC MOUNTING WITH AGFS SHELL:

  Interactive shell - Default settings:
  agfs:/> mount streamfs /stream
  agfs:/> mount streamfs /live

  Interactive shell - Custom buffer sizes:
  agfs:/> mount streamfs /stream channel_buffer_size=512KB ring_buffer_size=1MB
  agfs:/> mount streamfs /hq channel_buffer_size=8MB ring_buffer_size=16MB
  agfs:/> mount streamfs /lowlatency channel_buffer_size=256KB ring_buffer_size=512KB

  Direct command - Default:
  uv run agfs mount streamfs /stream

  Direct command - Custom settings:
  uv run agfs mount streamfs /video channel_buffer_size=4MB ring_buffer_size=8MB
  uv run agfs mount streamfs /live channel_buffer_size=512KB ring_buffer_size=1MB

CONFIGURATION PARAMETERS:

  Optional:
  - channel_buffer_size: Buffer per reader (default: "6MB")
    Supports units: KB, MB, GB or raw bytes (e.g., "512KB", "4MB", 524288)
    Controls how much data each reader can buffer before dropping chunks

  - ring_buffer_size: Historical data buffer (default: "6MB")
    Supports units: KB, MB, GB or raw bytes (e.g., "1MB", "8MB", 1048576)
    Stores recent data for late-joining readers

  Configuration examples by use case:
  # Live streaming (low latency)
  agfs:/> mount streamfs /live channel_buffer_size=256KB ring_buffer_size=512KB

  # VOD/Recording (smooth playback)
  agfs:/> mount streamfs /vod channel_buffer_size=8MB ring_buffer_size=16MB

  # Interactive streaming
  agfs:/> mount streamfs /interactive channel_buffer_size=512KB ring_buffer_size=1MB

  # High bitrate video
  agfs:/> mount streamfs /hd channel_buffer_size=16MB ring_buffer_size=32MB

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
    cat file | agfs write --stream /streamfs/stream
    echo "data" | agfs write /streamfs/stream

  Read (Consumer):
    agfs cat --stream /streamfs/stream
    agfs cat --stream /streamfs/stream > output.dat
    agfs cat --stream /streamfs/stream | ffplay -

  Manage:
    agfs ls /streamfs
    agfs stat /streamfs/stream
    agfs rm /streamfs/stream

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

## License

Apache License 2.0
