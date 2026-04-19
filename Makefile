.PHONY: build build-all test install clean checksums fmt lint help

# Version from git tags or default
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
OUTPUT_EXT ?=
ifeq ($(GOOS),windows)
	OUTPUT_EXT := .exe
endif
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT_SHA) -X main.buildDate=$(BUILD_DATE)"

DIST_DIR := dist
BINARY_NAME := shellai
BINARY_PATH := $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$(GOOS)-$(GOARCH)$(OUTPUT_EXT)

# Default target
all: build

# Build the binary
build:
	@mkdir -p $(DIST_DIR)
	@echo "Building $(BINARY_NAME) v$(VERSION) for $(GOOS)/$(GOARCH)"
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY_PATH) $(LDFLAGS) ./cmd
	@echo "✓ Built: $(BINARY_PATH)"

# Build for release platforms (Linux amd64/arm64)
build-all:
	@mkdir -p $(DIST_DIR)
	@echo "Building linux/amd64 and linux/arm64..."
	@GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64 $(LDFLAGS) ./cmd
	@GOOS=linux GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-arm64 $(LDFLAGS) ./cmd
	@echo "✓ All platforms built"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -timeout 30s ./...
	@echo "✓ Tests passed"

# Install the binary to /usr/local/bin
install: build
	@echo "Installing to /usr/local/bin"
	@sudo install -m 755 $(BINARY_PATH) /usr/local/bin/$(BINARY_NAME)
	@echo "✓ Installed: /usr/local/bin/$(BINARY_NAME)"
	@shellai --version

# Clean up build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(DIST_DIR)
	@echo "✓ Clean"

# Generate checksums for all binaries in dist/
checksums:
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && sha256sum * > SHA256SUMS && cat SHA256SUMS
	@echo "✓ Checksums written to $(DIST_DIR)/SHA256SUMS"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Formatted"

# Lint code
lint:
	@echo "Linting code..."
	@go vet ./...
	@echo "✓ Linted"

# Help target
help:
	@echo "ShellAI Makefile targets:"
	@echo ""
	@echo "  make build        - Build binary for current platform (default)"
	@echo "  make build-all    - Build Linux amd64 and arm64 binaries"
	@echo "  make test         - Run test suite"
	@echo "  make install      - Build and install to /usr/local/bin"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make checksums    - Generate SHA256 checksums for binaries"
	@echo "  make fmt          - Format code with go fmt"
	@echo "  make lint         - Lint code with go vet"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION           - Version string (default: git tag or 'dev')"
	@echo "  GOOS              - Target OS (default: current platform)"
	@echo "  GOARCH            - Target architecture (default: current platform)"
