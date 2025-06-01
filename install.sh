#!/bin/bash
set -e

REPO="your-username/dh-ddns-updater"  # Update this
INSTALL_DIR="/tmp/dh-ddns-updater-install"
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    aarch64) DEB_ARCH="arm64" ;;
    x86_64) DEB_ARCH="amd64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Installing dh-ddns-updater for $DEB_ARCH..."

# Create temp directory
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Get latest release info
echo "Fetching latest release..."
LATEST_URL=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep "browser_download_url.*${DEB_ARCH}.deb" | cut -d '"' -f 4)

if [ -z "$LATEST_URL" ]; then
    echo "Error: Could not find release for architecture $DEB_ARCH"
    exit 1
fi

# Download package
echo "Downloading package..."
curl -L -o "dh-ddns-updater.deb" "$LATEST_URL"

# Install package
echo "Installing package..."
sudo dpkg -i dh-ddns-updater.deb

# Clean up
cd /
rm -rf "$INSTALL_DIR"

echo ""
echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit /etc/dh-ddns-updater/config.yaml with your API key and domains"
echo "2. Start the service: sudo systemctl start dh-ddns-updater"
echo "3. Check status: sudo systemctl status dh-ddns-updater"
echo "4. View logs: sudo journalctl -u dh-ddns-updater -f"