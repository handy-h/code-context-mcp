# Makefile for code-context-mcp

.PHONY: help build test clean lint vet fmt install

# Variables
BINARY_NAME := code-context-mcp
BINARY_PATH := cmd/code-context-mcp/$(BINARY_NAME)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GOLANGCI_LINT_VERSION := v1.64.8

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_PATH) ./cmd/code-context-mcp

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@CGO_ENABLED=0 go install -v -trimpath -ldflags="$(LDFLAGS)" ./cmd/code-context-mcp

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and generate coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
		golangci-lint run ./...; \
	fi

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_PATH) $(BINARY_PATH).exe coverage.out coverage.html

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

dev: build ## Build and run in development mode (index current directory)
	@./$(BINARY_PATH) -index .

