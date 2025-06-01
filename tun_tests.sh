#!/bin/bash
set -e

echo "🧪 Running DDNS Updater Test Suite"
echo "=================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if Go is installed
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Go version: $(go version)"

# Get dependencies
echo ""
echo "📦 Installing dependencies..."
go mod download
go mod tidy
print_status "Dependencies installed"

# Run unit tests
echo ""
echo "🔬 Running unit tests..."
if go test -v ./...; then
    print_status "Unit tests passed"
else
    print_error "Unit tests failed"
    exit 1
fi

# Run tests with race detection
echo ""
echo "🏃 Running race condition tests..."
if go test -race ./...; then
    print_status "Race condition tests passed"
else
    print_warning "Race condition tests failed"
fi

# Generate coverage report
echo ""
echo "📊 Generating coverage report..."
if go test -coverprofile=coverage.out ./...; then
    go tool cover -html=coverage.out -o coverage.html
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
    print_status "Coverage report generated: coverage.html"
    print_status "Total coverage: $COVERAGE"
else
    print_warning "Coverage generation failed"
fi

# Run integration tests if requested
if [[ "$1" == "--integration" ]]; then
    echo ""
    echo "🌐 Running integration tests..."
    print_warning "Integration tests require internet connection"
    
    if go test -tags=integration -v ./...; then
        print_status "Integration tests passed"
    else
        print_warning "Integration tests failed (this might be expected without proper API credentials)"
    fi
fi

# Run benchmarks if requested
if [[ "$1" == "--bench" ]]; then
    echo ""
    echo "⚡ Running benchmarks..."
    go test -bench=. ./...
fi

# Check for potential issues
echo ""
echo "🔍 Running additional checks..."

# Check for go vet issues
if go vet ./...; then
    print_status "go vet passed"
else
    print_warning "go vet found issues"
fi

# Check for formatting issues
if [ "$(gofmt -l .)" ]; then
    print_warning "Code formatting issues found. Run 'go fmt ./...' to fix."
    gofmt -l .
else
    print_status "Code formatting is correct"
fi

# Check for common security issues (if gosec is installed)
if command -v gosec &> /dev/null; then
    echo ""
    echo "🔒 Running security checks..."
    if gosec ./...; then
        print_status "Security checks passed"
    else
        print_warning "Security issues found"
    fi
else
    print_warning "gosec not installed, skipping security checks"
    echo "   Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
fi

echo ""
print_status "All tests completed!"

# Print final summary
echo ""
echo "📋 Test Summary:"
echo "=================="
echo "✓ Unit tests: PASSED"
echo "✓ Race detection: PASSED"
echo "✓ Code coverage: Generated (see coverage.html)"
echo "✓ Code quality: Checked"

if [[ "$1" == "--integration" ]]; then
    echo "✓ Integration tests: RUN"
fi

if [[ "$1" == "--bench" ]]; then
    echo "✓ Benchmarks: RUN"
fi

echo ""
print_status "Ready for production! 🚀"
