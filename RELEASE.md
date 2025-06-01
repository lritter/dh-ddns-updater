# Release Process

This document describes how to build and release dh-ddns-updater.

## Quick Release (Automated)

1. **Make sure everything is tested and committed:**
   ```bash
   make test
   git add .
   git commit -m "Release preparation"
   git push
   ```

2. **Create a release:**
   ```bash
   ./scripts/release.sh v1.0.0
   ```

This will automatically:
- Run tests
- Build packages locally for verification  
- Create and push a git tag
- Trigger GitHub Actions to build and publish the release

## Manual Process

### Local Development Build

```bash
# Test everything
make test

# Build for your current platform
go build -o dh-ddns-updater .

# Test locally
./dh-ddns-updater config.yaml
```

### Cross-Platform Build

```bash
# Build for Raspberry Pi (ARM64)
make build-arm64

# Build for x86_64 Linux
make build-amd64

# Build Debian packages
make deb-arm64
make deb-amd64

# Or build everything
make release-local
```

### GitHub Actions Release

The automated release process runs when you push a tag:

1. **Prerequisites:**
   - Repository must be on GitHub
   - GitHub Actions must be enabled
   - No failing tests

2. **Create a release:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

3. **Monitor the build:**
   - Go to your repository's Actions tab
   - Watch the "Release" workflow

4. **Release artifacts:**
   - Binary files: `dh-ddns-updater-arm64`, `dh-ddns-updater-amd64`
   - Debian packages: `dh-ddns-updater-1.0.0-arm64.deb`, `dh-ddns-updater-1.0.0-amd64.deb`

## Release Checklist

Before creating a release:

- [ ] All tests pass (`make test`)
- [ ] Code is properly formatted (`go fmt`)
- [ ] No race conditions (`make test-race`)
- [ ] Documentation is up to date
- [ ] Version number follows semantic versioning
- [ ] CHANGELOG.md is updated (if you maintain one)

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- `v1.0.0` - Major release (breaking changes)
- `v1.1.0` - Minor release (new features, backward compatible)
- `v1.0.1` - Patch release (bug fixes)

## Installation Testing

After a release, test the installation:

```bash
# Test the install script
curl -sSL https://raw.githubusercontent.com/your-username/dh-ddns-updater/main/install.sh | bash

# Or test manual installation
wget https://github.com/your-username/dh-ddns-updater/releases/latest/download/dh-ddns-updater-1.0.0-arm64.deb
sudo dpkg -i dh-ddns-updater-1.0.0-arm64.deb
```

## Troubleshooting Releases

### Build Failures

1. **Tests failing:**
   - Fix the failing tests
   - Commit and try again

2. **Cross-compilation issues:**
   - Check Go version compatibility
   - Verify CGO_ENABLED=0 for static builds

3. **Debian package issues:**
   - Check file permissions in the package
   - Verify systemd service file syntax
   - Test postinst/prerm scripts

### GitHub Actions Issues

1. **Workflow not triggering:**
   - Check that the tag was pushed: `git ls-remote --tags origin`
   - Verify workflow file syntax

2. **Permission errors:**
   - Check repository settings → Actions → General
   - Ensure "Read and write permissions" is enabled

3. **Missing artifacts:**
   - Check the build matrix in `.github/workflows/release.yml`
   - Verify artifact upload/download steps

## Manual Release Steps

If you need to create a release manually:

1. **Build everything locally:**
   ```bash
   make release-local
   ```

2. **Create GitHub release:**
   - Go to your repository on GitHub
   - Click "Releases" → "Create a new release"
   - Choose your tag or create a new one
   - Upload the `.deb` files from `build/`
   - Write release notes

3. **Update install script:**
   - Verify the install script works with the new release
   - Test on a clean system if possible

## Post-Release

After a successful release:

1. **Test installation** on target systems
2. **Update documentation** if needed
3. **Announce the release** (if applicable)
4. **Monitor for issues** in the first few days

## Development Releases

For testing prereleases:

```bash
# Create a prerelease tag
git tag v1.0.0-rc1
git push origin v1.0.0-rc1
```

Mark it as a "prerelease" in the GitHub release interface.
