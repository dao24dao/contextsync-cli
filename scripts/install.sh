#!/bin/bash
# ContextSync Installation Script
# Usage: curl -fsSL https://contextsync.dev/install.sh | bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}"
echo "  ContextSync Installer"
echo -e "${NC}"

# Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

# Determine download URL
VERSION="${VERSION:-latest}"
BASE_URL="https://github.com/contextsync/cli/releases"

if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="$BASE_URL/latest/download/contextsync-${OS}-${ARCH}"
else
    DOWNLOAD_URL="$BASE_URL/download/${VERSION}/contextsync-${OS}-${ARCH}"
fi

# Determine install directory
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_PATH="$INSTALL_DIR/contextsync"

echo -e "  OS:        ${GREEN}$OS${NC}"
echo -e "  Arch:      ${GREEN}$ARCH${NC}"
echo -e "  Version:   ${GREEN}$VERSION${NC}"
echo -e "  Install:   ${GREEN}$BINARY_PATH${NC}"
echo

# Download binary
echo -e "${YELLOW}Downloading...${NC}"
if command -v curl &> /dev/null; then
    curl -fsSL "$DOWNLOAD_URL" -o "$BINARY_PATH"
elif command -v wget &> /dev/null; then
    wget -q "$DOWNLOAD_URL" -O "$BINARY_PATH"
else
    echo -e "${RED}Error: Neither curl nor wget found${NC}"
    exit 1
fi

# Make executable
chmod +x "$BINARY_PATH"

echo -e "${GREEN}Installation complete!${NC}"
echo
echo "  Get started:"
echo "    contextsync init"
echo
echo "  Documentation: https://contextsync.dev/docs"
echo
