"""Version information for pfs-cli"""

__version__ = "1.0.0"

# These will be populated during build time
__git_hash__ = "dev"
__build_date__ = "dev"

def get_version_string():
    """Get formatted version string"""
    if __git_hash__ == "dev":
        return f"pfs-cli {__version__} (dev)"
    return f"pfs-cli {__version__} (git: {__git_hash__}, built: {__build_date__})"
