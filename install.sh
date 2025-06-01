#!/bin/bash
set -e

# Get repo from GitHub URL or use default
REPO="${DDNS_REPO:-lritter/ddns-updater}"  # Set this to your actual repo
INSTALL_DIR="/tmp/ddns-updater-install"
ARCH=$(uname -m)

# Colors for pretty output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Map architecture names
case $ARCH in
    aarch64) DEB_ARCH="arm64" ;;
    x86_64) DEB_ARCH="amd64" ;;
    armv7l) DEB_ARCH="arm64" ;; # Pi 4 sometimes reports as armv7l
    *) 
        print_error "Unsupported architecture: $ARCH"
        echo "Supported architectures: aarch64 (arm64), x86_64 (amd64)"
        exit 1 
        ;;
esac

echo "🔧 Installing ddns-updater for $DEB_ARCH..."
print_info "Repository: $REPO"

# Check for required tools
for cmd in curl jq; do
    if ! command -v $cmd &> /dev/null; then
        print_error "$cmd is required but not installed"
        if [ "$cmd" = "jq" ]; then
            echo "Install with: sudo apt-get install jq"
        fi
        exit 1
    fi
done

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    print_warning "Running as root. This will install system-wide."
    SUDO=""
else
    SUDO="sudo"
fi

# Create temp directory
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Get latest release info
print_info "Fetching latest release information..."
RELEASE_DATA=$(curl -sf "https://api.github.com/repos/$REPO/releases/latest" || {
    print_error "Failed to fetch release information"
    echo "Please check:"
    echo "1. Internet connection"
    echo "2. Repository exists: https://github.com/$REPO"
    echo "3. Repository has releases"
    exit 1
})

LATEST_VERSION=$(echo "$RELEASE_DATA" | jq -r '.tag_name')
DOWNLOAD_URL=$(echo "$RELEASE_DATA" | jq -r ".assets[] | select(.name | contains(\"${DEB_ARCH}.deb\")) | .browser_download_url")

if [ -z "$DOWNLOAD_URL" ] || [ "$DOWNLOAD_URL" = "null" ]; then
    print_error "Could not find release for architecture $DEB_ARCH"
    echo "Available assets:"
    echo "$RELEASE_DATA" | jq -r '.assets[].name'
    exit 1
fi

print_info "Latest version: $LATEST_VERSION"
print_info "Download URL: $DOWNLOAD_URL"

# Download package
print_info "Downloading package..."
if ! curl -L -o "ddns-updater.deb" "$DOWNLOAD_URL"; then
    print_error "Failed to download package"
    exit 1
fi

# Verify download
if [ ! -f "ddns-updater.deb" ] || [ ! -s "ddns-updater.deb" ]; then
    print_error "Downloaded package is empty or missing"
    exit 1
fi

print_status "Package downloaded successfully"

# Install package
print_info "Installing package..."
if $SUDO dpkg -i ddns-updater.deb; then
    print_status "Package installed successfully"
else
    print_warning "Package installation failed, trying to fix dependencies..."
    $SUDO apt-get update
    $SUDO apt-get install -f -y
    print_status "Dependencies fixed"
fi

# Clean up
cd /
rm -rf "$INSTALL_DIR"

# Check if service is available
if systemctl list-unit-files | grep -q ddns-updater.service; then
    print_status "Service installed and available"
else
    print_warning "Service may not be properly installed"
fi

echo ""
echo "🎉 Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit configuration: $SUDO nano /etc/ddns-updater/config.yaml"
echo "2. Add your Dreamhost API key and domains"
echo "3. Start the service: $SUDO systemctl start ddns-updater"
echo "4. Enable auto-start: $SUDO systemctl enable ddns-updater"
echo "5. Check status: $SUDO systemctl status ddns-updater"
echo "6. View logs: $SUDO journalctl -u ddns-updater -f"
echo ""
echo "For help and documentation:"
echo "https://github.com/$REPO"
