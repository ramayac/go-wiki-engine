BINARY    := wiki-engine
CMD_DIR   := ./cmd/wiki-engine
BIN_DIR   := bin
VERSION   ?= dev
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w -X main.version=$(VERSION)-$(COMMIT)-$(BUILD_DATE)

.DEFAULT_GOAL := help

.PHONY: help build install test lint vet clean sync-scaffold

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build the binary to bin/
	@mkdir -p $(BIN_DIR)
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) $(CMD_DIR)
	@echo "built $(BIN_DIR)/$(BINARY)"

install: ## Install globally via go install
	go install -ldflags '$(LDFLAGS)' $(CMD_DIR)
	@echo "installed $(BINARY)"

test: ## Run all tests
	go test ./...

lint: vet ## Run go vet (alias: lint)

vet: ## Run go vet on all packages
	go vet ./...

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

sync-scaffold: ## Copy scaffold/ into internal/scaffold/files/ for embedding
	rm -rf internal/scaffold/files
	cp -r scaffold internal/scaffold/files
	@echo "synced scaffold → internal/scaffold/files"
