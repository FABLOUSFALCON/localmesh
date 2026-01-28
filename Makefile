.PHONY: all build test lint security clean dev install help

# Variables
BINARY_NAME=localmesh
BUILD_DIR=./build
CMD_DIR=./cmd/localmesh
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Default target
all: lint test build

# Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "✅ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

# Run linter
lint:
	@echo "🔍 Running linter..."
	golangci-lint run --fix

# Security audit
security:
	@echo "🔐 Running security audit..."
	@echo "  → govulncheck..."
	govulncheck ./...
	@echo "  → gosec..."
	gosec -quiet ./...
	@echo "✅ Security audit complete"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "  → Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "  → Installing govulncheck..."
	go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "  → Installing gosec..."
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "✅ Dependencies installed"

# Run in development mode
dev:
	@echo "🚀 Starting in development mode..."
	go run $(CMD_DIR) start --dev

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Install binary to GOPATH/bin
install: build
	@echo "📥 Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

# Generate plugin scaffold
scaffold:
	@echo "🏗️ Generating plugin scaffold..."
	@read -p "Plugin name: " name; \
	mkdir -p plugins/$$name; \
	echo "package main" > plugins/$$name/main.go; \
	echo "✅ Created plugins/$$name"

# Format code
fmt:
	@echo "✨ Formatting code..."
	go fmt ./...
	goimports -w .

# Run pre-commit checks (lint + test + security)
precommit: lint test security
	@echo "✅ All pre-commit checks passed"

# Help
help:
	@echo "LocalMesh Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all           Run lint, test, and build (default)"
	@echo "  build         Build the binary"
	@echo "  test          Run tests"
	@echo "  test-coverage Run tests with coverage report"
	@echo "  lint          Run golangci-lint"
	@echo "  security      Run security audit (govulncheck + gosec)"
	@echo "  deps          Install dependencies and tools"
	@echo "  dev           Run in development mode"
	@echo "  clean         Clean build artifacts"
	@echo "  install       Install binary to GOPATH/bin"
	@echo "  scaffold      Generate plugin scaffold"
	@echo "  fmt           Format code"
	@echo "  precommit     Run all pre-commit checks"
	@echo "  help          Show this help"
