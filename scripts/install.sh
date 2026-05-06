#!/usr/bin/env sh
set -eu

BINARY_NAME="mapmystorage"
REPO="lautar0t/MapMyStorage"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

fail() {
  echo "error: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

need curl
need tar

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin|linux) ;;
  *) fail "unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) fail "unsupported architecture: $ARCH" ;;
esac

if [ "$OS" = "linux" ] && [ "$ARCH" != "amd64" ]; then
  fail "linux/$ARCH release assets are not published yet"
fi

VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
fi
[ -n "$VERSION" ] || fail "could not resolve latest release version"

ASSET="MapMyStorage_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $URL"
curl -fL "$URL" -o "$TMP_DIR/$ASSET"
tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"

[ -x "$TMP_DIR/$BINARY_NAME" ] || fail "archive did not contain $BINARY_NAME"

echo "Installing $BINARY_NAME to $INSTALL_DIR"
if [ -w "$INSTALL_DIR" ]; then
  cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
  chmod 0755 "$INSTALL_DIR/$BINARY_NAME"
else
  need sudo
  sudo cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
  sudo chmod 0755 "$INSTALL_DIR/$BINARY_NAME"
fi

"$INSTALL_DIR/$BINARY_NAME" -version
