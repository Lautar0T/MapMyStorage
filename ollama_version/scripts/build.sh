#!/bin/bash

# Build script for real-disk-map
# Supports: macOS (amd64, arm64), Windows (amd64), Linux (amd64)

set -e

BINARY_NAME="rdm-cli"
VERSION=${VERSION:-"0.2.0"}
BUILD_DIR="bin"

echo "=== real-disk-map Build Script ==="
echo "Version: $VERSION"
echo ""

# Ensure dependencies
echo "Downloading dependencies..."
go mod download

# Create build directory
mkdir -p "$BUILD_DIR"

# Build function
build_platform() {
    local goos=$1
    local goarch=$2
    local output_suffix=""

    if [ "$goos" = "windows" ]; then
        output_suffix=".exe"
    fi

    local output="$BUILD_DIR/${BINARY_NAME}-${goos}-${goarch}${output_suffix}"

    echo "Building for $goos/$goarch..."
    GOOS="$goos" GOARCH="$goarch" go build \
        -ldflags "-s -w -X main.version=$VERSION" \
        -o "$output" \
        ./cmd/rdm-cli

    echo "  -> $output"
}

# Parse arguments
if [ $# -eq 0 ]; then
    # Build for current platform only
    echo "Building for current platform..."
    go build -o "$BUILD_DIR/$BINARY_NAME" ./cmd/rdm-cli
    echo "  -> $BUILD_DIR/$BINARY_NAME"
else
    # Build for specified platforms
    for platform in "$@"; do
        case "$platform" in
            darwin|macos|mac)
                build_platform darwin amd64
                build_platform darwin arm64
                ;;
            windows|win)
                build_platform windows amd64
                ;;
            linux|lin)
                build_platform linux amd64
                ;;
            all)
                build_platform darwin amd64
                build_platform darwin arm64
                build_platform windows amd64
                build_platform linux amd64
                ;;
            *)
                echo "Unknown platform: $platform"
                echo "Supported: darwin, windows, linux, all"
                exit 1
                ;;
        esac
    done
fi

echo ""
echo "Build complete!"
