# Graft Makefile
# AI-powered code review CLI

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=graft

# Default target
.DEFAULT_GOAL := build

# Build the application
.PHONY: build
build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/graft

# Run tests
.PHONY: test
test:
	$(GOTEST) ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run tests with race detector
.PHONY: test-race
test-race:
	$(GOTEST) -race ./...

# Install the binary
.PHONY: install
install:
	$(GOBUILD) -o $(GOPATH)/bin/$(BINARY_NAME) ./cmd/graft

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Run go mod tidy
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Format code
.PHONY: fmt
fmt:
	$(GOCMD) fmt ./...

# Run linter (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run ./...

# Check for vulnerabilities (requires govulncheck)
.PHONY: vuln
vuln:
	govulncheck ./...

# Build for all platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/graft
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/graft

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/graft
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/graft

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/graft

# Show help
.PHONY: help
help:
	@echo "Graft - AI-powered code review CLI"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  test           Run tests"
	@echo "  test-verbose   Run tests with verbose output"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  test-race      Run tests with race detector"
	@echo "  install        Install binary to GOPATH/bin"
	@echo "  clean          Remove build artifacts"
	@echo "  tidy           Run go mod tidy"
	@echo "  fmt            Format code"
	@echo "  lint           Run linter (requires golangci-lint)"
	@echo "  vuln           Check for vulnerabilities (requires govulncheck)"
	@echo "  build-all      Build for all platforms"
	@echo "  help           Show this help"
