#!/bin/sh
# Costa CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/costa-app/costa-cli/main/install.sh | sh
set -e

REPO="costa-app/costa-cli"
BINARY_NAME="costa"
INSTALL_DIR="/usr/local/bin"

# Use ~/.local/bin if not root and /usr/local/bin isn't writable
if [ "$(id -u)" != "0" ] && [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
fi

info() { printf "\033[0;34m==>\033[0m %s\n" "$1"; }
error() { printf "\033[0;31merror:\033[0m %s\n" "$1" >&2; exit 1; }

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      error "Unsupported operating system: $OS" ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)   ARCH="arm64" ;;
    *)               error "Unsupported architecture: $ARCH" ;;
esac

# macOS uses a universal binary
if [ "$OS" = "darwin" ]; then
    ARCH="all"
fi

ARCHIVE="costa-cli_${OS}_${ARCH}.tar.gz"
CHECKSUM="${ARCHIVE}.sha256"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading ${ARCHIVE}..."
curl -fsSL "${BASE_URL}/${ARCHIVE}" -o "${TMPDIR}/${ARCHIVE}"
curl -fsSL "${BASE_URL}/${CHECKSUM}" -o "${TMPDIR}/${CHECKSUM}"

info "Verifying checksum..."
cd "$TMPDIR"
EXPECTED="$(cat "$CHECKSUM" | tr -d '[:space:]')"
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "$ARCHIVE" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')"
else
    echo "Warning: could not verify checksum (no sha256sum or shasum found)"
    ACTUAL="$EXPECTED"
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
    error "Checksum mismatch: expected $EXPECTED, got $ACTUAL"
fi

info "Extracting..."
tar xzf "$ARCHIVE"

info "Installing to ${INSTALL_DIR}..."
mkdir -p "$INSTALL_DIR"
mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

# Check if install dir is in PATH
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "  $INSTALL_DIR is not in your \$PATH."
        echo "  Add it by running:"
        echo ""
        echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        ;;
esac

# Optional XDG mode support
if [ "$COSTA_INSTALL_XDG" = "1" ]; then
    VERSION="$(${INSTALL_DIR}/${BINARY_NAME} version 2>/dev/null || echo "unknown")"
    DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/costa"
    INSTALL_DIR_XDG="$DATA_DIR/versions/$VERSION"
    SYMLINK_TARGET="$HOME/.local/bin/costa"

    mkdir -p "$INSTALL_DIR_XDG"
    mv "$INSTALL_DIR/$BINARY_NAME" "$INSTALL_DIR_XDG/$BINARY_NAME"

    mkdir -p "$HOME/.local/bin"
    ln -sf "$INSTALL_DIR_XDG/$BINARY_NAME" "$SYMLINK_TARGET"

    # Update INSTALL_DIR for PATH check
    INSTALL_DIR="$HOME/.local/bin"
fi

info "costa $(${INSTALL_DIR}/${BINARY_NAME} version 2>/dev/null || echo '(installed)') installed successfully!"
echo ""
if [ -t 1 ]; then
    echo ""
    info "Launching costa..."
    exec "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Run 'costa' to get started or 'costa --help' to see available commands."
fi
