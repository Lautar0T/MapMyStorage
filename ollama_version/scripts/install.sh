#!/bin/bash

# Installation script for real-disk-map

set -e

BINARY_NAME="rdm-cli"
INSTALL_DIR="/usr/local/bin"
REPO_URL="https://github.com/lautaror/real-disk-map"

echo "=== real-disk-map Installation Script ==="
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected: $OS/$ARCH"
echo ""

# Determine binary name
if [ "$OS" = "darwin" ]; then
    BINARY="${BINARY_NAME}-darwin-${ARCH}"
elif [ "$OS" = "linux" ]; then
    BINARY="${BINARY_NAME}-linux-${ARCH}"
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Check if we're building from source
if [ -f "bin/$BINARY" ] || [ -f "bin/$BINARY_NAME" ]; then
    echo "Building from source..."
    make build
    SOURCE="bin/$BINARY_NAME"
else
    echo "Downloading from GitHub releases..."
    VERSION=$(curl -s "$REPO_URL/releases/latest" | grep -o 'tag/v[0-9.]*' | head -1 | sed 's/tag\///')
    if [ -z "$VERSION" ]; then
        VERSION="v0.2.0"
    fi
    echo "Version: $VERSION"

    URL="$REPO_URL/releases/download/$VERSION/$BINARY"
    SOURCE="/tmp/$BINARY"

    echo "Downloading $URL..."
    curl -L -o "$SOURCE" "$URL" || {
        echo "Failed to download binary. Please build from source."
        exit 1
    }
fi

# Install
echo "Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    cp "$SOURCE" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Need sudo privileges to install to $INSTALL_DIR"
    sudo cp "$SOURCE" "$INSTALL_DIR/$BINARY_NAME"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
fi

# Verify installation
if command -v "$BINARY_NAME" >/dev/null 2>&1; then
    echo ""
    echo "Installation successful!"
    echo "Run '$BINARY_NAME --help' to get started."
else
    echo ""
    echo "Installation may have failed. Please check your PATH."
    exit 1
fi
