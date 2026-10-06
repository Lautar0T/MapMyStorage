# MapMyStorage Makefile

BINARY_NAME := mapmystorage
CMD_DIR := ./cmd/mapmystorage
BUILD_DIR := bin
DIST_DIR := dist
VERSION ?= 0.3.0-dev
PREFIX ?= $(HOME)/.local
INSTALL_DIR ?= $(PREFIX)/bin
LDFLAGS := -s -w -X main.version=$(VERSION)

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 windows/amd64

.PHONY: all help build build-all clean test test-short coverage fmt vet lint install uninstall run deps tidy package version

all: build

## help: Show this help message
help:
	@echo "MapMyStorage - Real disk usage analyzer"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /'

## build: Build the binary for the current platform
build: fmt vet
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

## build-all: Build binaries for supported platforms
build-all:
	@mkdir -p $(BUILD_DIR)
	@set -e; for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT=$(BUILD_DIR)/$(BINARY_NAME)-$$GOOS-$$GOARCH; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT=$$OUTPUT.exe; fi; \
		echo "Building $$GOOS/$$GOARCH"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o $$OUTPUT $(CMD_DIR); \
	done

## test: Run all tests
test:
	go test ./...

## test-short: Run short tests
test-short:
	go test -short ./...

## coverage: Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## fmt: Format Go files
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint when installed
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; skipping"; \
	fi

## install: Install to ~/.local/bin (override INSTALL_DIR or PREFIX)
install: build
	install -d "$(INSTALL_DIR)"
	install -m 0755 "$(BUILD_DIR)/$(BINARY_NAME)" "$(INSTALL_DIR)/$(BINARY_NAME)"

## uninstall: Remove the binary from INSTALL_DIR
uninstall:
	rm -f "$(INSTALL_DIR)/$(BINARY_NAME)"

## run: Build and run the binary
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

## deps: Download and verify dependencies
deps:
	go mod download
	go mod verify

## tidy: Tidy go modules
tidy:
	go mod tidy

## package: Create local archives using GoReleaser
package:
	goreleaser release --snapshot --clean

## version: Show the current source version
version:
	@echo "MapMyStorage $(VERSION)"

## clean: Remove generated artifacts
clean:
	@if [ -d .gomodcache ]; then chmod -R u+w .gomodcache; fi
	rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html .gocache .gomodcache
