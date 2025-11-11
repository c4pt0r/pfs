"""Exception classes for pypfs"""


class PFSClientError(Exception):
    """Base exception for PFS client errors"""
    pass


class PFSConnectionError(PFSClientError):
    """Connection related errors"""
    pass


class PFSTimeoutError(PFSClientError):
    """Timeout errors"""
    pass


class PFSHTTPError(PFSClientError):
    """HTTP related errors"""

    def __init__(self, message, status_code=None):
        super().__init__(message)
        self.status_code = status_code
