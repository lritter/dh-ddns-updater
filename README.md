# DDNS Updater

A lightweight dynamic DNS updater for Dreamhost that runs as a system daemon. Automatically detects IP changes and updates your DNS records.

## Features

- Single binary with no dependencies
- Automatic IP change detection via ipinfo.io
- Multiple domain/record support
- Graceful error handling with retry logic
- Structured JSON logging
- Systemd integration
- Easy installation via Debian package

## Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/lritter/dh-ddns-updater/main/install.sh | bash
```

NOTE: requires `jq` for JSON parsing and `curl` for HTTP requests.

## Manual Installation

1. Download the latest `.deb` package from [releases](https://github.com/lritter/dh-ddns-updater/releases)
2. Install: `sudo dpkg -i dh-ddns-updater-*.deb`

## Configuration

Edit `/etc/dh-ddns-updater/config.yaml`:

```yaml
check_interval: 5m
log_level: info
dreamhost_api_key: "your_api_key_here"

domains:
  - name: "example.com"
    record: "home"  # Creates home.example.com
    type: "A"
  - name: "example.com"
    record: ""      # Updates example.com directly  
    type: "A"
```

### Getting Your Dreamhost API Key

1. Log into your Dreamhost panel
2. Go to [Advanced → API](https://panel.dreamhost.com/?tree=home.api)
3. Generate a new API key with `dns-add_record`, `dns-list_records`,
   `dns-remove_record` permissions
4. Copy it to your config file

## Usage

```bash
# Start the service
sudo systemctl start dh-ddns-updater

# Enable auto-start on boot
sudo systemctl enable dh-ddns-updater

# Check status
sudo systemctl status dh-ddns-updater

# View logs
sudo journalctl -u dh-ddns-updater -f

# Test configuration (run in foreground)
sudo -u dh-ddns-updater /usr/local/bin/dh-ddns-updater /etc/dh-ddns-updater/config.yaml
```

Note that you must restart the service after changing the configuration:

```bash
sudo systemctl restart dh-ddns-updater
```

## Building from Source

```bash
# For Raspberry Pi 5 (ARM64)
make build-arm64

# Create Debian package
make deb-arm64

# Development (run locally)
make dev
```

## Troubleshooting

**Service won't start:**

- Check config syntax: `sudo -u dh-ddns-updater /usr/local/bin/dh-ddns-updater /etc/dh-ddns-updater/config.yaml`
- Verify API key is correct
- Check logs: `sudo journalctl -u dh-ddns-updater`

**DNS not updating:**

- Verify domains exist in your Dreamhost panel
- Check that the API key has DNS management permissions
- Look for API errors in logs

**Permission errors:**

- Ensure `/var/lib/dh-ddns-updater` is owned by `dh-ddns-updater:dh-ddns-updater`
- Config file should be readable by the `dh-ddns-updater` user

## Security Notes

- The service runs as a non-privileged user (`dh-ddns-updater`)
- Config file contains your API key - keep it secure
- Uses systemd security features (NoNewPrivileges, ProtectSystem, etc.)
- Only requires network access and write access to state directory

## License

MIT License - see LICENSE file for details.
