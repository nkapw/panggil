# Makefile for panggil - Terminal UI API Client

BINARY_NAME=panggil
VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
GO=go
INSTALL_DIR=/usr/local/bin
RELEASE_DIR=release

.PHONY: all build run clean install uninstall test lint fmt help version release

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'

## build: Build the binary
build:
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) .

## run: Build and run the application
run: build
	./$(BINARY_NAME)

## install: Install the binary to /usr/local/bin
install: build
	@if [ -w "$(INSTALL_DIR)" ]; then \
		cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "✓ Installed $(BINARY_NAME) to $(INSTALL_DIR)"
	@echo "  Run '$(BINARY_NAME)' to start the application"

## uninstall: Remove the binary from /usr/local/bin
uninstall:
	@if [ -w "$(INSTALL_DIR)" ]; then \
		rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "✓ Uninstalled $(BINARY_NAME) from $(INSTALL_DIR)"

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(RELEASE_DIR)
	$(GO) clean

## test: Run tests
test:
	$(GO) test -v ./...

## lint: Run linters
lint:
	$(GO) vet ./...
	@which staticcheck > /dev/null && staticcheck ./... || echo "staticcheck not installed"

## fmt: Format code
fmt:
	$(GO) fmt ./...

## deps: Download dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

## release: Build release binaries for all platforms (matches install.sh naming)
release: clean
	@mkdir -p $(RELEASE_DIR)
	@echo "Building release binaries v$(VERSION)..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64 .
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_linux_arm64 .
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_arm64 .
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_windows_amd64.exe .
	@echo ""
	@echo "✓ Release binaries created in $(RELEASE_DIR)/"
	@ls -la $(RELEASE_DIR)/

## build-linux: Cross-compile for Linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .

## build-darwin: Cross-compile for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .

## build-windows: Cross-compile for Windows
build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .

## build-all: Build for all platforms (dev naming)
build-all: build-linux build-darwin build-windows
	@echo "Built binaries for all platforms"

all: build

