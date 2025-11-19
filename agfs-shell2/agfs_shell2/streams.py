"""Stream classes for Unix-style I/O handling"""

import sys
import io
from typing import Optional, Union, BinaryIO, TextIO, TYPE_CHECKING

if TYPE_CHECKING:
    from .filesystem import AGFSFileSystem


class Stream:
    """Base class for I/O streams"""

    def __init__(self, fd: Optional[Union[int, BinaryIO, TextIO]] = None, mode: str = 'r'):
        """
        Initialize a stream

        Args:
            fd: File descriptor (int), file object, or None
            mode: 'r' for read, 'w' for write, 'a' for append
        """
        self.mode = mode
        self._fd = fd
        self._file = None
        self._buffer = None

        if fd is None:
            # Use in-memory buffer
            if 'r' in mode:
                self._buffer = io.BytesIO()
            else:
                self._buffer = io.BytesIO()
        elif isinstance(fd, int):
            # File descriptor number
            self._file = open(fd, mode + 'b', buffering=0, closefd=False)
        else:
            # File-like object
            self._file = fd

    def get_file(self) -> BinaryIO:
        """Get the underlying file object"""
        if self._buffer is not None:
            return self._buffer
        return self._file

    def read(self, size: int = -1) -> bytes:
        """Read from stream"""
        f = self.get_file()
        return f.read(size)

    def readline(self) -> bytes:
        """Read a line from stream"""
        f = self.get_file()
        return f.readline()

    def readlines(self) -> list:
        """Read all lines from stream"""
        f = self.get_file()
        return f.readlines()

    def write(self, data: Union[bytes, str]) -> int:
        """Write to stream"""
        if isinstance(data, str):
            data = data.encode('utf-8')
        return self.get_file().write(data)

    def flush(self):
        """Flush the stream"""
        self.get_file().flush()

    def close(self):
        """Close the stream"""
        if self._file is not None and hasattr(self._file, 'close'):
            self._file.close()
        if self._buffer is not None:
            # Don't close buffer, might need to read from it
            pass

    def fileno(self) -> Optional[int]:
        """Get file descriptor number"""
        if self._fd is not None and isinstance(self._fd, int):
            return self._fd
        if self._file is not None and hasattr(self._file, 'fileno'):
            try:
                return self._file.fileno()
            except:
                pass
        return None

    def get_value(self) -> bytes:
        """Get the buffer contents (for buffer-based streams)"""
        if self._buffer is not None:
            pos = self._buffer.tell()
            self._buffer.seek(0)
            data = self._buffer.read()
            self._buffer.seek(pos)
            return data
        return b''


class InputStream(Stream):
    """Input stream (STDIN-like)"""

    def __init__(self, fd: Optional[Union[int, BinaryIO, TextIO]] = None):
        super().__init__(fd, mode='rb')

    @classmethod
    def from_stdin(cls):
        """Create from system stdin"""
        return cls(sys.stdin.buffer)

    @classmethod
    def from_bytes(cls, data: bytes):
        """Create from bytes data"""
        stream = cls(None)
        stream._buffer = io.BytesIO(data)
        return stream

    @classmethod
    def from_string(cls, data: str):
        """Create from string data"""
        return cls.from_bytes(data.encode('utf-8'))


class OutputStream(Stream):
    """Output stream (STDOUT-like)"""

    def __init__(self, fd: Optional[Union[int, BinaryIO, TextIO]] = None):
        super().__init__(fd, mode='wb')

    @classmethod
    def from_stdout(cls):
        """Create from system stdout"""
        return cls(sys.stdout.buffer)

    @classmethod
    def to_buffer(cls):
        """Create to in-memory buffer"""
        return cls(None)


class ErrorStream(Stream):
    """Error stream (STDERR-like)"""

    def __init__(self, fd: Optional[Union[int, BinaryIO, TextIO]] = None):
        super().__init__(fd, mode='wb')

    @classmethod
    def from_stderr(cls):
        """Create from system stderr"""
        return cls(sys.stderr.buffer)

    @classmethod
    def to_buffer(cls):
        """Create to in-memory buffer"""
        return cls(None)


class AGFSOutputStream(OutputStream):
    """Output stream that writes directly to AGFS file in streaming mode"""

    def __init__(self, filesystem: 'AGFSFileSystem', path: str, append: bool = False):
        """
        Initialize AGFS output stream

        Args:
            filesystem: AGFS filesystem instance
            path: Target file path in AGFS
            append: If True, append to file; if False, overwrite
        """
        # Don't call super().__init__ as we handle buffering differently
        self.mode = 'wb'
        self._fd = None
        self._file = None
        self._buffer = io.BytesIO()  # Temporary buffer
        self.filesystem = filesystem
        self.path = path
        self.append = append
        self._chunks = []  # Collect chunks
        self._total_size = 0

    def write(self, data: Union[bytes, str]) -> int:
        """Write data to buffer"""
        if isinstance(data, str):
            data = data.encode('utf-8')

        # Add to chunks
        self._chunks.append(data)
        self._total_size += len(data)

        # Also write to buffer for get_value() compatibility
        self._buffer.write(data)

        return len(data)

    def flush(self):
        """Flush accumulated data to AGFS"""
        if not self._chunks:
            return

        # Combine all chunks
        data = b''.join(self._chunks)

        # Write to AGFS
        try:
            self.filesystem.write_file(self.path, data, append=self.append)
            # After first write, switch to append mode for subsequent flushes
            self.append = True
            # Clear chunks
            self._chunks = []
            self._total_size = 0
        except Exception as e:
            # Re-raise to let caller handle
            raise

    def close(self):
        """Close stream and flush remaining data"""
        self.flush()
        if self._buffer is not None:
            self._buffer.close()
