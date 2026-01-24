#!/bin/bash

set -e

# TCC Bridge Installation Script for Raspberry Pi

echo "========================================"
echo "TCC-Matter Bridge Installation"
echo "========================================"

# Check if running on Raspberry Pi
if ! grep -q "Raspberry Pi" /proc/cpuinfo 2>/dev/null; then
    echo "Warning: This script is designed for Raspberry Pi"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check for root
if [ "$EUID" -eq 0 ]; then
    echo "Please do not run as root. The script will use sudo when needed."
    exit 1
fi

# Set installation directory
INSTALL_DIR="${HOME}/tcc-bridge"
DATA_DIR="${HOME}/.tcc-bridge"

echo "Installation directory: ${INSTALL_DIR}"
echo "Data directory: ${DATA_DIR}"
echo

# Create directories
echo "Creating directories..."
mkdir -p "${INSTALL_DIR}/bin"
mkdir -p "${INSTALL_DIR}/matter-bridge"
mkdir -p "${INSTALL_DIR}/web/dist"
mkdir -p "${DATA_DIR}"

# Install system dependencies
echo "Installing system dependencies..."
sudo apt-get update
sudo apt-get install -y \
    build-essential \
    git \
    curl \
    sqlite3

# Install Node.js if not present
if ! command -v node &> /dev/null; then
    echo "Installing Node.js..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

echo "Node.js version: $(node --version)"
echo "npm version: $(npm --version)"

# Install Go if not present
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    GO_VERSION="1.21.6"
    wget "https://go.dev/dl/go${GO_VERSION}.linux-arm64.tar.gz"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "go${GO_VERSION}.linux-arm64.tar.gz"
    rm "go${GO_VERSION}.linux-arm64.tar.gz"
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
fi

echo "Go version: $(go version)"

# Clone or update repository
if [ -d "${INSTALL_DIR}/.git" ]; then
    echo "Updating existing installation..."
    cd "${INSTALL_DIR}"
    git pull
else
    echo "Note: Copy your built files to ${INSTALL_DIR}"
fi

# Build instructions
echo
echo "========================================"
echo "Build Instructions"
echo "========================================"
echo
echo "1. Copy your project files to ${INSTALL_DIR}"
echo "2. Run: cd ${INSTALL_DIR}"
echo "3. Run: make build"
echo "4. Run: make install-service"
echo "5. Run: sudo systemctl start tcc-bridge"
echo
echo "Access the web UI at: http://$(hostname -I | awk '{print $1}'):8080"
echo

# Install systemd service
if [ -f "${INSTALL_DIR}/configs/systemd/tcc-bridge.service" ]; then
    echo "Installing systemd service..."
    sudo cp "${INSTALL_DIR}/configs/systemd/tcc-bridge.service" /etc/systemd/system/
    sudo systemctl daemon-reload
    sudo systemctl enable tcc-bridge
    echo "Service installed and enabled."
fi

echo
echo "Installation complete!"
echo
echo "To start the service: sudo systemctl start tcc-bridge"
echo "To view logs: sudo journalctl -u tcc-bridge -f"
