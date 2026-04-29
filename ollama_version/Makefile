# real-disk-map Makefile

BINARY_NAME = rdm-cli
GO = go
GOFLAGS =

# Build directories
BUILD_DIR = bin
DIST_DIR = dist

# Source directories
CMD_DIR = ./cmd/rdm-cli

# Platforms
PLATFORMS = darwin/amd64 darwin/arm64 windows/amd64 linux/amd64

# Colors for output
BLUE = \033[34m
GREEN = \033[32m
RED = \033[31m
RESET = \033[0m

.PHONY: all build clean test install uninstall fmt vet lint run help

# Default target
all: build

## help: Show this help message
help:
	@echo "real-disk-map - Real disk usage analyzer"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the binary for the current platform
build: fmt vet
	@echo "$(BLUE)Building $(BINARY_NAME)...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)Built: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"

## build-all: Build for all supported platforms
build-all: clean
	@echo "$(BLUE)Building for all platforms...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT=$(BUILD_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT=$$OUTPUT.exe; fi; \
		echo "Building $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build $(GOFLAGS) -o $$OUTPUT $(CMD_DIR); \
	done
	@echo "$(GREEN)All builds complete!$(RESET)"

## build-darwin: Build for macOS (universal binary)
build-darwin:
	@echo "$(BLUE)Building for macOS...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)
	@echo "$(GREEN)Darwin builds complete!$(RESET)"

## build-windows: Build for Windows
build-windows:
	@echo "$(BLUE)Building for Windows...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)
	@echo "$(GREEN)Windows build complete!$(RESET)"

## install: Install the binary to /usr/local/bin
install: build
	@echo "$(BLUE)Installing $(BINARY_NAME) to /usr/local/bin...$(RESET)"
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)Installed!$(RESET)"

## uninstall: Remove the binary from /usr/local/bin
uninstall:
	@echo "$(BLUE)Uninstalling $(BINARY_NAME)...$(RESET)"
	@rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)Uninstalled!$(RESET)"

## test: Run all tests
test:
	@echo "$(BLUE)Running tests...$(RESET)"
	$(GO) test -v ./...
	@echo "$(GREEN)Tests complete!$(RESET)"

## test-short: Run short tests
test-short:
	@echo "$(BLUE)Running short tests...$(RESET)"
	$(GO) test -v -short ./...

## coverage: Run tests with coverage
coverage:
	@echo "$(BLUE)Running tests with coverage...$(RESET)"
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(RESET)"

## fmt: Format all Go files
fmt:
	@echo "$(BLUE)Formatting...$(RESET)"
	$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "$(BLUE)Vetting...$(RESET)"
	$(GO) vet ./...

## lint: Run linter (golangci-lint if available)
lint:
	@if command -v golangci-lint >/dev/null 2>&amp;1; then \
		echo "$(BLUE)Running golangci-lint...$(RESET)"; \
		golangci-lint run ./...; \
	else \
		echo "$(RED)golangci-lint not installed, skipping...$(RESET)"; \
	fi

## clean: Remove build artifacts
clean:
	@echo "$(BLUE)Cleaning...$(RESET)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "$(GREEN)Cleaned!$(RESET)"

## run: Build and run the binary
run: build
	@echo "$(BLUE)Running $(BINARY_NAME)...$(RESET)"
	@$(BUILD_DIR)/$(BINARY_NAME)

## deps: Download and verify dependencies
deps:
	@echo "$(BLUE)Downloading dependencies...$(RESET)"
	$(GO) mod download
	$(GO) mod verify
	@echo "$(GREEN)Dependencies ready!$(RESET)"

## tidy: Tidy go modules
tidy:
	@echo "$(BLUE)Tidying modules...$(RESET)"
	$(GO) mod tidy

## raycast: Build the Raycast extension
raycast:
	@echo "$(BLUE)Building Raycast extension...$(RESET)"
	cd raycast-extension && npm install && npm run build
	@echo "$(GREEN)Raycast extension built!$(RESET)"

## raycast-dev: Develop the Raycast extension
raycast-dev:
	@echo "$(BLUE)Starting Raycast dev mode...$(RESET)"
	cd raycast-extension && npm install && npm run dev

## package: Create distribution packages
package: clean build-all
	@echo "$(BLUE)Creating distribution packages...$(RESET)"
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		ARCHIVE_NAME=$(BINARY_NAME)-v$(VERSION)-$$GOOS-$$GOARCH; \
		SOURCE=$(BUILD_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then SOURCE=$$SOURCE.exe; fi; \
		echo "Packaging $$ARCHIVE_NAME..."; \
		tar -czf $(DIST_DIR)/$$ARCHIVE_NAME.tar.gz -C $(BUILD_DIR) $$(basename $$SOURCE) 2>/dev/null || true; \
	done
	@echo "$(GREEN)Packages created in $(DIST_DIR)/$(RESET)"

## version: Show the current version
version:
	@echo "real-disk-map version $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')"

# Development shortcuts
.PHONY: dev darwin windows linux

## dev: Run in development mode with hot reload
dev:
	@echo "$(BLUE)Running in dev mode...$(RESET)"
	@if command -v air >/dev/null 2>&amp;1; then \
		air; \
	else \
		$(GO) run $(CMD_DIR); \
	fi

## darwin: Shortcut for build-darwin
darwin: build-darwin

## windows: Shortcut for build-windows
windows: build-windows

## linux: Shortcut for Linux build
linux:
	@echo "$(BLUE)Building for Linux...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	@echo "$(GREEN)Linux build complete!$(RESET)"

# Release version (override with VERSION=x.y.z)
VERSION ?= 0.2.0
