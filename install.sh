#!/bin/bash
#

set -e

REPO="nkapw/panggil"
APP_NAME="panggil"
INSTALL_DIR="/usr/local/bin"

info() {
    echo "[INFO] $1"
}

success() {
    echo "[SUCCESS] $1"
}

error() {
    echo "[ERROR] $1" >&2
    exit 1
}


info "Starting installation of '$APP_NAME'..."

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        ;;
    *)
        error "Unsupported operating system: $OS. Only macOS (darwin) and Linux are supported."
        ;;
esac

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64 | arm64)
        ARCH="arm64"
        ;;
    *)
        error "Unsupported architecture: $ARCH. Only x86_64/amd64 and aarch64/arm64 are supported."
        ;;
esac

info "Detected System: $OS/$ARCH"

info "Fetching the latest version..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
LATEST_VERSION=$(echo "$LATEST_TAG" | sed 's/^v//')


if [ -z "$LATEST_TAG" ]; then
    error "Could not fetch the latest version tag from GitHub."
fi

info "Latest version is $LATEST_TAG"


BINARY_NAME="${APP_NAME}_${LATEST_VERSION}_${OS}_${ARCH}"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$BINARY_NAME"

info "Downloading from: $DOWNLOAD_URL"
TEMP_FILE="/tmp/$APP_NAME"
curl -sSL -o "$TEMP_FILE" "$DOWNLOAD_URL"

chmod +x "$TEMP_FILE"
info "Binary downloaded to $TEMP_FILE"

info "Moving '$APP_NAME' to $INSTALL_DIR (may require sudo password)..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TEMP_FILE" "$INSTALL_DIR/$APP_NAME"
else
    sudo mv "$TEMP_FILE" "$INSTALL_DIR/$APP_NAME"
fi

INSTALLED_PATH=$(command -v $APP_NAME)
success "'$APP_NAME' installed successfully at: $INSTALLED_PATH"
info "You can now run '$APP_NAME' from your terminal."
echo "Run '$APP_NAME' to start the application."
