#!/bin/sh
set -e

# PFS Installation Script
# This script downloads and installs the latest daily build of pfs-server and pfs-shell

REPO="c4pt0r/pfs"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
PFS_SHELL_DIR="${PFS_SHELL_DIR:-$HOME/.local/pfs}"
INSTALL_SERVER="${INSTALL_SERVER:-yes}"
INSTALL_CLIENT="${INSTALL_CLIENT:-yes}"

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw* | msys* | cygwin*)
            OS="windows"
            ;;
        *)
            echo "Error: Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64 | amd64)
            ARCH="amd64"
            ;;
        aarch64 | arm64)
            ARCH="arm64"
            ;;
        *)
            echo "Error: Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    echo "Detected platform: $OS-$ARCH"
}

# Get the latest daily build tag
get_latest_tag() {
    echo "Fetching latest daily build..."
    LATEST_TAG=$(curl -sL "https://api.github.com/repos/$REPO/releases" | \
        grep '"tag_name":' | \
        grep 'daily-' | \
        head -n 1 | \
        sed -E 's/.*"tag_name": "([^"]+)".*/\1/')

    if [ -z "$LATEST_TAG" ]; then
        echo "Error: Could not find latest daily build"
        exit 1
    fi

    echo "Latest daily build: $LATEST_TAG"
}

# Check Python version
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        echo "Warning: python3 not found. pfs-shell requires Python 3.10+"
        return 1
    fi

    PYTHON_VERSION=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:2])))')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)

    if [ "$PYTHON_MAJOR" -lt 3 ] || { [ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]; }; then
        echo "Warning: Python $PYTHON_VERSION found, but pfs-shell requires Python 3.10+"
        return 1
    fi

    echo "Found Python $PYTHON_VERSION"
    return 0
}

# Install pfs-server
install_server() {
    echo ""
    echo "Installing pfs-server..."

    DATE=$(echo "$LATEST_TAG" | sed 's/daily-//')

    if [ "$OS" = "windows" ]; then
        ARCHIVE="pfs-${OS}-${ARCH}-${DATE}.zip"
        BINARY="pfs-server-${OS}-${ARCH}.exe"
    else
        ARCHIVE="pfs-${OS}-${ARCH}-${DATE}.tar.gz"
        BINARY="pfs-server-${OS}-${ARCH}"
    fi

    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$ARCHIVE"

    echo "Downloading from: $DOWNLOAD_URL"

    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    if ! curl -fsSL -o "$ARCHIVE" "$DOWNLOAD_URL"; then
        echo "Error: Failed to download $ARCHIVE"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    echo "Extracting archive..."
    if [ "$OS" = "windows" ]; then
        unzip -q "$ARCHIVE"
    else
        tar -xzf "$ARCHIVE"
    fi

    if [ ! -f "$BINARY" ]; then
        echo "Error: Binary $BINARY not found in archive"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    # Install binary
    mv "$BINARY" "$INSTALL_DIR/pfs-server"
    chmod +x "$INSTALL_DIR/pfs-server"

    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"

    echo "✓ pfs-server installed to $INSTALL_DIR/pfs-server"
}

# Install pfs-shell
install_client() {
    echo ""
    echo "Installing pfs-shell..."

    # Check Python
    if ! check_python; then
        echo "Skipping pfs-shell installation (Python requirement not met)"
        return 1
    fi

    # Only build for supported platforms
    if [ "$OS" = "windows" ]; then
        if [ "$ARCH" != "amd64" ]; then
            echo "Skipping pfs-shell: Not available for $OS-$ARCH"
            return 1
        fi
        SHELL_ARCHIVE="pfs-shell-${OS}-${ARCH}.zip"
    else
        if [ "$ARCH" != "amd64" ] && ! { [ "$OS" = "darwin" ] && [ "$ARCH" = "arm64" ]; }; then
            echo "Skipping pfs-shell: Not available for $OS-$ARCH"
            return 1
        fi
        SHELL_ARCHIVE="pfs-shell-${OS}-${ARCH}.tar.gz"
    fi

    SHELL_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$SHELL_ARCHIVE"

    echo "Downloading from: $SHELL_URL"

    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    if ! curl -fsSL -o "$SHELL_ARCHIVE" "$SHELL_URL"; then
        echo "Warning: Failed to download pfs-shell, skipping client installation"
        rm -rf "$TMP_DIR"
        return 1
    fi

    echo "Extracting archive..."
    if [ "$OS" = "windows" ]; then
        unzip -q "$SHELL_ARCHIVE"
    else
        tar -xzf "$SHELL_ARCHIVE"
    fi

    if [ ! -d "pfs-portable" ]; then
        echo "Error: pfs-portable directory not found in archive"
        rm -rf "$TMP_DIR"
        return 1
    fi

    # Remove old installation
    rm -rf "$PFS_SHELL_DIR"
    mkdir -p "$PFS_SHELL_DIR"

    # Copy portable directory
    cp -r pfs-portable/* "$PFS_SHELL_DIR/"

    # Create symlink
    mkdir -p "$INSTALL_DIR"
    ln -sf "$PFS_SHELL_DIR/pfs" "$INSTALL_DIR/pfs"

    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"

    echo "✓ pfs-shell installed to $PFS_SHELL_DIR"
    echo "  Symlink created: $INSTALL_DIR/pfs"
}

show_completion() {
    echo ""
    echo "════════════════════════════════════════"
    echo "  ✓ Installation completed!"
    echo "════════════════════════════════════════"
    echo ""

    if [ "$INSTALL_SERVER" = "yes" ]; then
        echo "Server: pfs-server"
        echo "  Location: $INSTALL_DIR/pfs-server"
        echo "  Usage: pfs-server --help"
        echo ""
    fi

    if [ "$INSTALL_CLIENT" = "yes" ] && [ -f "$INSTALL_DIR/pfs" ]; then
        echo "Client: pfs"
        echo "  Location: $INSTALL_DIR/pfs"
        echo "  Usage: pfs --help"
        echo "  Interactive: pfs shell"
        echo ""
    fi

    # Check if install dir is in PATH
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            ;;
        *)
            echo "Note: $INSTALL_DIR is not in your PATH."
            echo "Add it to your PATH by adding this to ~/.bashrc or ~/.zshrc:"
            echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
            echo ""
            ;;
    esac

    echo "Quick Start:"
    echo "  1. Start server: pfs-server"
    echo "  2. Use client: pfs shell"
}

main() {
    echo "PFS Installer"
    echo "════════════════════════════════════════"

    detect_platform
    get_latest_tag

    if [ "$INSTALL_SERVER" = "yes" ]; then
        install_server
    fi

    if [ "$INSTALL_CLIENT" = "yes" ]; then
        install_client || true  # Don't fail if client install fails
    fi

    show_completion
}

main
