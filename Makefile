BINARY_NAME=ddns-updater
VERSION?=1.0.0
BUILD_DIR=build
DEB_DIR=$(BUILD_DIR)/deb

# Build for ARM64 (Pi 5)
build-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 .

# Build for AMD64 (testing)
build-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-amd64 .

# Create debian package for AMD64
deb-amd64: build-amd64
	mkdir -p $(DEB_DIR)-amd64/DEBIAN
	mkdir -p $(DEB_DIR)-amd64/usr/local/bin
	mkdir -p $(DEB_DIR)-amd64/etc/ddns-updater
	mkdir -p $(DEB_DIR)-amd64/var/lib/ddns-updater
	mkdir -p $(DEB_DIR)-amd64/etc/systemd/system
	
	# Copy binary
	cp $(BUILD_DIR)/$(BINARY_NAME)-amd64 $(DEB_DIR)-amd64/usr/local/bin/$(BINARY_NAME)
	chmod +x $(DEB_DIR)-amd64/usr/local/bin/$(BINARY_NAME)
	
	# Copy config template
	cp config.yaml $(DEB_DIR)-amd64/etc/ddns-updater/config.yaml.template
	
	# Copy systemd service
	cp ddns-updater.service $(DEB_DIR)-amd64/etc/systemd/system/
	
	# Create control file
	echo "Package: ddns-updater" > $(DEB_DIR)-amd64/DEBIAN/control
	echo "Version: $(VERSION)" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo "Section: net" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo "Priority: optional" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo "Architecture: amd64" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo "Maintainer: Your Name <your.email@example.com>" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo "Description: Dynamic DNS updater for Dreamhost" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo " A lightweight daemon that automatically updates DNS records" >> $(DEB_DIR)-amd64/DEBIAN/control
	echo " when your public IP address changes." >> $(DEB_DIR)-amd64/DEBIAN/control
	
	# Copy scripts from ARM64 version (they're identical)
	cp $(DEB_DIR)/DEBIAN/postinst $(DEB_DIR)-amd64/DEBIAN/postinst
	cp $(DEB_DIR)/DEBIAN/prerm $(DEB_DIR)-amd64/DEBIAN/prerm
	
	# Build package
	dpkg-deb --build $(DEB_DIR)-amd64 $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-amd64.deb
	mkdir -p $(DEB_DIR)/DEBIAN
	mkdir -p $(DEB_DIR)/usr/local/bin
	mkdir -p $(DEB_DIR)/etc/ddns-updater
	mkdir -p $(DEB_DIR)/var/lib/ddns-updater
	mkdir -p $(DEB_DIR)/etc/systemd/system
	
	# Copy binary
	cp $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(DEB_DIR)/usr/local/bin/$(BINARY_NAME)
	chmod +x $(DEB_DIR)/usr/local/bin/$(BINARY_NAME)
	
	# Copy config template
	cp config.yaml $(DEB_DIR)/etc/ddns-updater/config.yaml.template
	
	# Copy systemd service
	cp ddns-updater.service $(DEB_DIR)/etc/systemd/system/
	
	# Create control file
	echo "Package: ddns-updater" > $(DEB_DIR)/DEBIAN/control
	echo "Version: $(VERSION)" >> $(DEB_DIR)/DEBIAN/control
	echo "Section: net" >> $(DEB_DIR)/DEBIAN/control
	echo "Priority: optional" >> $(DEB_DIR)/DEBIAN/control
	echo "Architecture: arm64" >> $(DEB_DIR)/DEBIAN/control
	echo "Maintainer: Your Name <your.email@example.com>" >> $(DEB_DIR)/DEBIAN/control
	echo "Description: Dynamic DNS updater for Dreamhost" >> $(DEB_DIR)/DEBIAN/control
	echo " A lightweight daemon that automatically updates DNS records" >> $(DEB_DIR)/DEBIAN/control
	echo " when your public IP address changes." >> $(DEB_DIR)/DEBIAN/control
	
	# Create postinst script
	echo "#!/bin/bash" > $(DEB_DIR)/DEBIAN/postinst
	echo "set -e" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "# Create user and group" >> $(DEB_DIR)/DEBIAN/postinst
	echo "if ! getent group ddns-updater >/dev/null; then" >> $(DEB_DIR)/DEBIAN/postinst
	echo "    groupadd --system ddns-updater" >> $(DEB_DIR)/DEBIAN/postinst
	echo "fi" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "if ! getent passwd ddns-updater >/dev/null; then" >> $(DEB_DIR)/DEBIAN/postinst
	echo "    useradd --system --gid ddns-updater --home-dir /var/lib/ddns-updater --shell /bin/false ddns-updater" >> $(DEB_DIR)/DEBIAN/postinst
	echo "fi" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "# Set permissions" >> $(DEB_DIR)/DEBIAN/postinst
	echo "chown ddns-updater:ddns-updater /var/lib/ddns-updater" >> $(DEB_DIR)/DEBIAN/postinst
	echo "chmod 755 /var/lib/ddns-updater" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "# Copy config if it doesn't exist" >> $(DEB_DIR)/DEBIAN/postinst
	echo "if [ ! -f /etc/ddns-updater/config.yaml ]; then" >> $(DEB_DIR)/DEBIAN/postinst
	echo "    cp /etc/ddns-updater/config.yaml.template /etc/ddns-updater/config.yaml" >> $(DEB_DIR)/DEBIAN/postinst
	echo "    echo 'Config template copied to /etc/ddns-updater/config.yaml'" >> $(DEB_DIR)/DEBIAN/postinst
	echo "    echo 'Please edit it with your API key and domains before starting the service.'" >> $(DEB_DIR)/DEBIAN/postinst
	echo "fi" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "# Reload systemd and enable service" >> $(DEB_DIR)/DEBIAN/postinst
	echo "systemctl daemon-reload" >> $(DEB_DIR)/DEBIAN/postinst
	echo "systemctl enable ddns-updater.service" >> $(DEB_DIR)/DEBIAN/postinst
	echo "" >> $(DEB_DIR)/DEBIAN/postinst
	echo "echo 'ddns-updater installed successfully!'" >> $(DEB_DIR)/DEBIAN/postinst
	echo "echo 'Edit /etc/ddns-updater/config.yaml and then: systemctl start ddns-updater'" >> $(DEB_DIR)/DEBIAN/postinst
	
	chmod +x $(DEB_DIR)/DEBIAN/postinst
	
	# Create prerm script
	echo "#!/bin/bash" > $(DEB_DIR)/DEBIAN/prerm
	echo "set -e" >> $(DEB_DIR)/DEBIAN/prerm
	echo "systemctl stop ddns-updater.service || true" >> $(DEB_DIR)/DEBIAN/prerm
	echo "systemctl disable ddns-updater.service || true" >> $(DEB_DIR)/DEBIAN/prerm
	chmod +x $(DEB_DIR)/DEBIAN/prerm
	
	# Build package
	dpkg-deb --build $(DEB_DIR) $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)-arm64.deb

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	go test -v -race ./...

benchmark:
	go test -v -bench=. ./...

# Development
dev:
	go run . config.yaml

# Local release (for testing)
release-local: clean deb-arm64 deb-amd64
	@echo "Local release packages built:"
	@ls -la $(BUILD_DIR)/*.deb

# Make release script executable
setup-release:
	chmod +x scripts/release.sh

.PHONY: build-arm64 build-amd64 deb-arm64 deb-amd64 clean test test-coverage test-race benchmark dev release-local setup-release
