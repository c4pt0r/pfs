#!/bin/sh
set -e

# PFS Server Installation Script
# This script downloads and installs the latest daily build of pfs-server

REPO="c4pt0r/pfs"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

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

# Download and install
install_binary() {
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

    echo "Installing to $INSTALL_DIR..."

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "$BINARY" "$INSTALL_DIR/pfs-server"
        chmod +x "$INSTALL_DIR/pfs-server"
    else
        echo "Installing to $INSTALL_DIR requires sudo..."
        sudo mv "$BINARY" "$INSTALL_DIR/pfs-server"
        sudo chmod +x "$INSTALL_DIR/pfs-server"
    fi

    # Clean up
    cd - > /dev/null
    rm -rf "$TMP_DIR"

    echo ""
    echo "✓ pfs-server installed successfully!"
    echo ""
    echo "Run 'pfs-server --help' to get started."

    # Check if install dir is in PATH
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            ;;
        *)
            echo ""
            echo "Note: $INSTALL_DIR is not in your PATH."
            echo "Add it to your PATH by running:"
            echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
            ;;
    esac
}

main() {
    detect_platform
    get_latest_tag
    install_binary
}

main
