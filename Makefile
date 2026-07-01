# Makefile for code-context-mcp

.PHONY: help build test clean lint vet fmt install package clean-build

# Variables
BINARY_NAME := code-context-mcp
BINARY_PATH := cmd/code-context-mcp/$(BINARY_NAME)
OUTPUT_PATH := $(BINARY_NAME)/$(BINARY_NAME)
DIST_DIR := dist
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GOLANGCI_LINT_VERSION := v1.64.8

# 自动签名（macOS），防止复制到其他目录被 Gatekeeper 拦截
AUTO_SIGN ?= 1

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_NAME)
	@CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT_PATH) ./cmd/code-context-mcp
	@$(call sign_binary,$(OUTPUT_PATH))

define sign_binary
	@if [ "$(AUTO_SIGN)" = "1" ] && command -v codesign >/dev/null 2>&1; then \
		echo "Signing $(1)..."; \
		codesign --force --deep -s - "$(1)"; \
	fi
endef

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@CGO_ENABLED=0 go install -v -trimpath -ldflags="$(LDFLAGS)" ./cmd/code-context-mcp
	@$(call sign_binary,$(shell which $(BINARY_NAME) 2>/dev/null || echo "$(shell go env GOPATH)/bin/$(BINARY_NAME)"))

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

clean: clean-build ## Clean build artifacts
	@echo "Cleaning dist..."
	@rm -rf $(DIST_DIR)

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

dev: build ## Build and run in development mode (index current directory)
	@./$(OUTPUT_PATH) -index .

# macOS 打包：构建 + 签名 + 清理 quarantine + 压缩包
# 输出到 $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64.tar.gz
PACKAGE_NAME := $(BINARY_NAME)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m)
.PHONY: package
package: clean-build build
	@echo "Packaging $(PACKAGE_NAME)..."
	@mkdir -p $(DIST_DIR)/$(PACKAGE_NAME)
	@cp $(OUTPUT_PATH) $(DIST_DIR)/$(PACKAGE_NAME)/
	@tar -czf $(DIST_DIR)/$(PACKAGE_NAME).tar.gz -C $(DIST_DIR) $(PACKAGE_NAME)
	@rm -rf $(DIST_DIR)/$(PACKAGE_NAME)
	@echo "Package created: $(DIST_DIR)/$(PACKAGE_NAME).tar.gz"
	@echo "NOTE: Extract and run directly; do NOT move across APFS volumes to preserve ad-hoc signature."

.PHONY: clean-build
clean-build: ## Clean build artifacts only
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_PATH) $(BINARY_PATH).exe $(OUTPUT_PATH) $(OUTPUT_PATH).exe coverage.out coverage.html
	@rm -rf $(BINARY_NAME)

