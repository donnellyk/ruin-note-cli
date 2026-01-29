# Ruin Note CLI - Makefile

# Variables
BINARY_NAME := ruin
BUILD_DIR := ./build
CMD_PATH := ./cmd/ruin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

# Go commands
GO := go
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOINSTALL := $(GO) install
GOFMT := $(GO) fmt
GOVET := $(GO) vet

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH)

# Build with race detector (for development)
.PHONY: build-race
build-race:
	$(GOBUILD) $(LDFLAGS) -race -o $(BINARY_NAME) $(CMD_PATH)

# Install to $GOPATH/bin
.PHONY: install
install:
	$(GOINSTALL) $(LDFLAGS) $(CMD_PATH)

# Run all tests
.PHONY: test
test:
	$(GOTEST) ./...

# Run tests with verbose output
.PHONY: test-v
test-v:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests with race detector
.PHONY: test-race
test-race:
	$(GOTEST) -race ./...

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Check formatting (fails if not formatted)
.PHONY: fmt-check
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:"; gofmt -l .; exit 1)

# Run go vet
.PHONY: vet
vet:
	$(GOVET) ./...

# Run golangci-lint (must be installed)
.PHONY: lint
lint:
	golangci-lint run

# Run all checks (fmt, vet, lint, test)
.PHONY: check
check: fmt-check vet test

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	rm -rf $(BUILD_DIR)

# Create/reset test vault
.PHONY: test-vault
test-vault:
	./scripts/test-vault.sh reset

# Clean test vault
.PHONY: test-vault-clean
test-vault-clean:
	./scripts/test-vault.sh clean

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

# Show help
.PHONY: help
help:
	@echo "Ruin Note CLI - Available targets:"
	@echo ""
	@echo "  Build:"
	@echo "    build        Build the binary (default)"
	@echo "    build-race   Build with race detector"
	@echo "    build-all    Build for all platforms (darwin, linux, windows)"
	@echo "    install      Install to \$$GOPATH/bin"
	@echo ""
	@echo "  Test:"
	@echo "    test         Run all tests"
	@echo "    test-v       Run tests with verbose output"
	@echo "    test-coverage Run tests with coverage report"
	@echo "    test-race    Run tests with race detector"
	@echo ""
	@echo "  Quality:"
	@echo "    fmt          Format code"
	@echo "    fmt-check    Check if code is formatted"
	@echo "    vet          Run go vet"
	@echo "    lint         Run golangci-lint"
	@echo "    check        Run fmt-check, vet, and test"
	@echo ""
	@echo "  Test Vault:"
	@echo "    test-vault       Create/reset test vault at /tmp/ruin-test-vault"
	@echo "    test-vault-clean Remove test vault"
	@echo ""
	@echo "  Other:"
	@echo "    clean        Remove build artifacts"
	@echo "    help         Show this help"
