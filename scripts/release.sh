#!/bin/bash
set -e

# Release script for ddns-updater
# Usage: ./scripts/release.sh [version]

VERSION=${1:-}
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format v1.2.3"
    exit 1
fi

echo "🚀 Preparing release $VERSION"

# Check if we're in a git repo
if [ ! -d .git ]; then
    echo "Error: Not in a git repository"
    exit 1
fi

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: You have uncommitted changes. Please commit or stash them first."
    git status --short
    exit 1
fi

# Check if we're on main/master branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    echo "Warning: You're not on main/master branch (current: $CURRENT_BRANCH)"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Run tests
echo "🧪 Running tests..."
if ! make test; then
    echo "Error: Tests failed"
    exit 1
fi

# Build packages locally for verification
echo "🔨 Building packages..."
if ! make clean && make deb-arm64; then
    echo "Error: Build failed"
    exit 1
fi

echo "✅ Build successful"

# Check if tag already exists
if git tag -l | grep -q "^$VERSION$"; then
    echo "Error: Tag $VERSION already exists"
    exit 1
fi

# Create and push tag
echo "🏷️  Creating tag $VERSION"
git tag -a "$VERSION" -m "Release $VERSION"

echo "📤 Pushing tag to remote..."
git push origin "$VERSION"

echo ""
echo "🎉 Release $VERSION initiated!"
echo ""
echo "The GitHub Actions workflow will now:"
echo "  1. Run tests"
echo "  2. Build binaries for ARM64 and AMD64"
echo "  3. Create Debian packages"
echo "  4. Create a GitHub release"
echo ""
echo "Monitor the progress at:"
echo "https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')/actions"
echo ""
echo "Once complete, users can install with:"
echo "curl -sSL https://raw.githubusercontent.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\([^.]*\).*/\1/')/main/install.sh | bash"
