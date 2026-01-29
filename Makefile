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

# Benchmarking
BENCH_DIR := ./benchmarks
BENCH_TIME := 3s

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/commands/... 2>&1 | grep -v "^goos\|^goarch\|^pkg\|^cpu"

# Run realistic benchmarks only
.PHONY: bench-realistic
bench-realistic:
	$(GOTEST) -bench=Realistic -benchtime=$(BENCH_TIME) -benchmem ./internal/commands/...

# Run benchmarks and save with timestamp
.PHONY: bench-save
bench-save:
	@mkdir -p $(BENCH_DIR)
	$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/commands/... > $(BENCH_DIR)/$$(date +%Y-%m-%d-%H%M%S).txt
	@echo "Saved to $(BENCH_DIR)/$$(date +%Y-%m-%d-%H%M%S).txt"

# Save current results as baseline
.PHONY: bench-baseline
bench-baseline:
	@mkdir -p $(BENCH_DIR)
	$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/commands/... > $(BENCH_DIR)/baseline.txt
	@echo "Baseline saved to $(BENCH_DIR)/baseline.txt"

# Compare to baseline (requires benchstat: go install golang.org/x/perf/cmd/benchstat@latest)
.PHONY: bench-compare
bench-compare:
	@if [ ! -f $(BENCH_DIR)/baseline.txt ]; then echo "No baseline. Run 'make bench-baseline' first."; exit 1; fi
	@$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) -benchmem ./internal/commands/... > $(BENCH_DIR)/current.txt
	@echo "Comparing baseline vs current:"
	@benchstat $(BENCH_DIR)/baseline.txt $(BENCH_DIR)/current.txt || echo "Install benchstat: go install golang.org/x/perf/cmd/benchstat@latest"

# Create benchmark vaults
.PHONY: bench-vault-small bench-vault-medium bench-vault-large
bench-vault-small:
	./scripts/create-benchmark-vault.sh small /tmp/ruin-bench-small

bench-vault-medium:
	./scripts/create-benchmark-vault.sh medium /tmp/ruin-bench-medium

bench-vault-large:
	./scripts/create-benchmark-vault.sh large /tmp/ruin-bench-large

# Run search benchmark against real vault
.PHONY: bench-vault
bench-vault:
	@if [ ! -d /tmp/ruin-bench-medium ]; then $(MAKE) bench-vault-medium; fi
	@echo "Benchmarking against /tmp/ruin-bench-medium..."
	@time ./$(BINARY_NAME) --vault /tmp/ruin-bench-medium search "#daily" > /dev/null
	@time ./$(BINARY_NAME) --vault /tmp/ruin-bench-medium search "lorem" > /dev/null

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
	@echo "  Benchmarking:"
	@echo "    bench            Run all benchmarks"
	@echo "    bench-realistic  Run realistic vault benchmarks"
	@echo "    bench-save       Run benchmarks and save with timestamp"
	@echo "    bench-baseline   Save current results as baseline"
	@echo "    bench-compare    Compare current to baseline (needs benchstat)"
	@echo "    bench-vault-*    Create benchmark vault (small/medium/large)"
	@echo "    bench-vault      Run search on benchmark vault"
	@echo ""
	@echo "  Test Vault:"
	@echo "    test-vault       Create/reset test vault at /tmp/ruin-test-vault"
	@echo "    test-vault-clean Remove test vault"
	@echo ""
	@echo "  Other:"
	@echo "    clean        Remove build artifacts"
	@echo "    help         Show this help"
