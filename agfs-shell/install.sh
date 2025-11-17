#!/bin/bash
#
# Installation script for agfs-shell
# This script builds and installs the portable agfs distribution
#

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default installation directory
DEFAULT_INSTALL_DIR="$HOME/.local/agfs"
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
BIN_LINK_DIR="$HOME/.local/bin"

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_python() {
    if ! command -v python3 &> /dev/null; then
        print_error "python3 is not installed"
        exit 1
    fi
    print_info "Found python3: $(python3 --version)"
}

build_portable() {
    print_info "Building portable agfs distribution..."
    cd "$SCRIPT_DIR"

    # Check if uv is available (required)
    if ! command -v uv &> /dev/null; then
        print_error "uv is required for building"
        print_info "Install uv: curl -LsSf https://astral.sh/uv/install.sh | sh"
        exit 1
    fi

    # Run build script
    python3 build.py
}

install_portable() {
    print_info "Installing agfs to $INSTALL_DIR..."

    # Remove old installation if exists
    if [ -d "$INSTALL_DIR" ]; then
        print_warn "Removing old installation..."
        rm -rf "$INSTALL_DIR"
    fi

    # Create installation directory
    mkdir -p "$INSTALL_DIR"

    # Copy portable directory
    cp -r "$SCRIPT_DIR/dist/agfs-portable/"* "$INSTALL_DIR/"

    # Create symlink in bin directory
    mkdir -p "$BIN_LINK_DIR"
    ln -sf "$INSTALL_DIR/agfs" "$BIN_LINK_DIR/agfs"

    print_info "Installation complete!"
    print_info "Installed to: $INSTALL_DIR"
    print_info "Symlinked to: $BIN_LINK_DIR/agfs"
}

check_path() {
    if [[ ":$PATH:" != *":$BIN_LINK_DIR:"* ]]; then
        print_warn "$BIN_LINK_DIR is not in your PATH"
        print_warn "Add the following line to your ~/.bashrc or ~/.zshrc:"
        echo ""
        echo "    export PATH=\"$BIN_LINK_DIR:\$PATH\""
        echo ""
    fi
}

show_usage() {
    echo "agfs-shell installation script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -d, --dir DIR    Installation directory (default: $DEFAULT_INSTALL_DIR)"
    echo "  -h, --help       Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  INSTALL_DIR      Override installation directory"
    echo ""
    echo "Examples:"
    echo "  $0                          # Install to $DEFAULT_INSTALL_DIR"
    echo "  $0 -d /opt/agfs              # Install to /opt/agfs"
    echo "  INSTALL_DIR=~/apps/agfs $0   # Install to ~/apps/agfs"
    echo ""
    echo "Note: A symlink will be created in $BIN_LINK_DIR/agfs"
}

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    print_info "agfs-shell installer"
    print_info "Installation directory: $INSTALL_DIR"
    print_info "Symlink directory: $BIN_LINK_DIR"
    echo ""

    check_python
    build_portable
    install_portable
    check_path

    echo ""
    print_info "Run 'agfs --help' to get started"
    print_info "Requirements: Python 3.10+ (uses bundled dependencies)"
}

main "$@"
