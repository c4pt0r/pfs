#!/bin/sh
set -e

# AGFS Installation Script
# This script downloads and installs the latest daily build of agfs-server and agfs-shell

REPO="c4pt0r/agfs"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
AGFS_SHELL_DIR="${AGFS_SHELL_DIR:-$HOME/.local/agfs}"
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

# Get the nightly build tag
get_latest_tag() {
    echo "Fetching nightly build..."
    LATEST_TAG="nightly"
    echo "Using nightly build"
}

# Check Python version
check_python() {
    if ! command -v python3 >/dev/null 2>&1; then
        echo "Warning: python3 not found. agfs-shell requires Python 3.10+"
        return 1
    fi

    PYTHON_VERSION=$(python3 -c 'import sys; print(".".join(map(str, sys.version_info[:2])))')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)

    if [ "$PYTHON_MAJOR" -lt 3 ] || { [ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -lt 10 ]; }; then
        echo "Warning: Python $PYTHON_VERSION found, but agfs-shell requires Python 3.10+"
        return 1
    fi

    echo "Found Python $PYTHON_VERSION"
    return 0
}

# Install agfs-server
install_server() {
    echo ""
    echo "Installing agfs-server..."

    # Get the date from the nightly release
    DATE=$(curl -sL "https://api.github.com/repos/$REPO/releases/tags/$LATEST_TAG" | \
        grep '"name":' | \
        head -n 1 | \
        sed -E 's/.*\(([0-9]+)\).*/\1/')

    if [ -z "$DATE" ]; then
        echo "Error: Could not determine build date from nightly release"
        exit 1
    fi

    if [ "$OS" = "windows" ]; then
        ARCHIVE="agfs-${OS}-${ARCH}-${DATE}.zip"
        BINARY="agfs-server-${OS}-${ARCH}.exe"
    else
        ARCHIVE="agfs-${OS}-${ARCH}-${DATE}.tar.gz"
        BINARY="agfs-server-${OS}-${ARCH}"
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
    mv "$BINARY" "$INSTALL_DIR/agfs-server"
    chmod +x "$INSTALL_DIR/agfs-server"

    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"

    echo "✓ agfs-server installed to $INSTALL_DIR/agfs-server"
}

# Install agfs-shell
install_client() {
    echo ""
    echo "Installing agfs-shell..."

    # Check Python
    if ! check_python; then
        echo "Skipping agfs-shell installation (Python requirement not met)"
        return 1
    fi

    # Only build for supported platforms
    if [ "$OS" = "windows" ]; then
        if [ "$ARCH" != "amd64" ] && [ "$ARCH" != "arm64" ]; then
            echo "Skipping agfs-shell: Not available for $OS-$ARCH"
            return 1
        fi
        SHELL_ARCHIVE="agfs-shell-${OS}-${ARCH}.zip"
    else
        if [ "$ARCH" != "amd64" ] && ! { [ "$OS" = "darwin" ] && [ "$ARCH" = "arm64" ]; } && ! { [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; }; then
            echo "Skipping agfs-shell: Not available for $OS-$ARCH"
            return 1
        fi
        SHELL_ARCHIVE="agfs-shell-${OS}-${ARCH}.tar.gz"
    fi

    SHELL_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$SHELL_ARCHIVE"

    echo "Downloading from: $SHELL_URL"

    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    if ! curl -fsSL -o "$SHELL_ARCHIVE" "$SHELL_URL"; then
        echo "Warning: Failed to download agfs-shell, skipping client installation"
        rm -rf "$TMP_DIR"
        return 1
    fi

    echo "Extracting archive..."
    if [ "$OS" = "windows" ]; then
        unzip -q "$SHELL_ARCHIVE"
    else
        tar -xzf "$SHELL_ARCHIVE"
    fi

    if [ ! -d "agfs-portable" ]; then
        echo "Error: agfs-portable directory not found in archive"
        rm -rf "$TMP_DIR"
        return 1
    fi

    # Remove old installation
    rm -rf "$AGFS_SHELL_DIR"
    mkdir -p "$AGFS_SHELL_DIR"

    # Copy portable directory
    cp -r agfs-portable/* "$AGFS_SHELL_DIR/"

    # Create symlink
    mkdir -p "$INSTALL_DIR"
    ln -sf "$AGFS_SHELL_DIR/agfs" "$INSTALL_DIR/agfs"

    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"

    echo "✓ agfs-shell installed to $AGFS_SHELL_DIR"
    echo "  Symlink created: $INSTALL_DIR/agfs"
}

show_completion() {
    echo ""
    echo "----------------------------------"
    echo "    Installation completed!"
    echo "----------------------------------"
    echo ""

    if [ "$INSTALL_SERVER" = "yes" ]; then
        echo "Server: agfs-server"
        echo "  Location: $INSTALL_DIR/agfs-server"
        echo "  Usage: agfs-server --help"
        echo ""
    fi

    if [ "$INSTALL_CLIENT" = "yes" ] && [ -f "$INSTALL_DIR/agfs" ]; then
        echo "Client: agfs"
        echo "  Location: $INSTALL_DIR/agfs"
        echo "  Usage: agfs --help"
        echo "  Interactive: agfs shell"
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
    echo "  1. Start server: agfs-server"
    echo "  2. Use client: agfs shell"
}

main() {
    echo ""
    echo "----------------------------------"
    echo "          AGFS Installer           "
    echo "----------------------------------"
    echo ""

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
