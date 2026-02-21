BINARY_NAME := memory
BUILD_DIR   := bin
MODULE      := github.com/go-ports/echovault
BUILDINFO   := $(MODULE)/internal/buildinfo

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

LDFLAGS := -X $(BUILDINFO).Version=$(VERSION) \
           -X $(BUILDINFO).BuildDate=$(BUILD_DATE) \
           -X $(BUILDINFO).GitCommit=$(GIT_COMMIT) \
           -X $(BUILDINFO).GitBranch=$(GIT_BRANCH)

# CGO is required for go-sqlite3 (FTS5) and sqlite-vec
export CGO_ENABLED := 1

.PHONY: build build-native test lint lint-fix clean deps help

build: ## Build the binary (current platform, optimised)
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/memory

build-native: ## Build without cross-compile flags (fastest local build)
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build -ldflags "$(LDFLAGS)" -gcflags="-e" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/memory

test: ## Run all tests
	CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...

lint: ## Run all linters
	@echo "Running nolintguard..."
	@go run github.com/go-extras/nolintguard/cmd/nolintguard@latest ./...
	@echo ""
	@echo "Running qtlint..."
	@go run github.com/go-extras/qtlint/cmd/qtlint@latest ./...
	@echo ""
	@echo "Running golangci-lint..."
	golangci-lint run ./... --timeout=10m

lint-fix: ## Run linters with auto-fix
	@echo "Running qtlint with auto-fix..."
	@go run github.com/go-extras/qtlint/cmd/qtlint@latest -fix ./...
	@echo ""
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix ./... --timeout=10m

clean: ## Remove build artefacts
	rm -rf $(BUILD_DIR)

deps: ## Download and tidy module dependencies
	go mod download
	go mod tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
