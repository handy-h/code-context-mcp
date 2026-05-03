# Makefile for code-context-mcp

.PHONY: help build test clean lint vet fmt install docker-build docker-run

# Variables
BINARY_NAME := code-context-mcp
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@CGO_ENABLED=0 go install -v -trimpath -ldflags="$(LDFLAGS)" .

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and generate coverage report
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint: ## Run linters
	@echo "Running golangci-lint..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
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
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe coverage.out coverage.html
	@rm -rf dist/

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t code-context-mcp:$(VERSION) .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run --rm -it \
		-e OLLAMA_URL=http://host.docker.internal:11434 \
		-e ZILLIZ_URI=your_zilliz_uri_here \
		-e ZILLIZ_TOKEN=your_zilliz_token_here \
		-e PROJECT_PATH=/app/project \
		-v $(PWD):/app/project \
		code-context-mcp:$(VERSION)

release-snapshot: ## Create a snapshot release with GoReleaser
	@echo "Creating snapshot release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --clean; \
	else \
		echo "GoReleaser not installed. Installing..."; \
		go install github.com/goreleaser/goreleaser/v2@latest; \
		goreleaser release --snapshot --clean; \
	fi

check-deps: ## Check for outdated dependencies
	@echo "Checking for outdated dependencies..."
	@go list -u -m -json all | go-mod-outdated -update -direct

update-deps: ## Update all dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

# Development targets
dev: build ## Build and run in development mode
	@echo "Running in development mode..."
	@./$(BINARY_NAME) -index . || true

# Cross-compilation targets
build-linux: ## Build for Linux
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 .

build-darwin: ## Build for macOS (Intel)
	@echo "Building for macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-amd64 .

build-darwin-arm64: ## Build for macOS (Apple Silicon)
	@echo "Building for macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-darwin-arm64 .

build-windows: ## Build for Windows
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -v -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe .

build-all: build-linux build-darwin build-darwin-arm64 build-windows ## Build for all platforms