GOBIN   ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif
BINARY    = doctrove
WORKSPACE = $(HOME)/.config/doctrove
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install uninstall test lint fmt vet clean init-workspace help

build: ## Build the binary with version embedded
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/doctrove

install: build ## Install to $GOBIN
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/doctrove
	@echo "Installed $(BINARY) $(VERSION) to $(GOBIN)/"
	@echo "Run 'make init-workspace' to create default config"
	@echo "Run 'doctrove mcp-config' for agent integration"

uninstall: ## Remove installed binary
	rm -f $(GOBIN)/$(BINARY)

init-workspace: ## Create default workspace config
	@mkdir -p $(WORKSPACE)
	@if [ ! -f $(WORKSPACE)/doctrove.yaml ]; then \
		printf 'settings:\n  events_url: http://localhost:6060/events\n' > $(WORKSPACE)/doctrove.yaml; \
		echo "Created $(WORKSPACE)/doctrove.yaml"; \
	else \
		echo "$(WORKSPACE)/doctrove.yaml already exists — skipping"; \
	fi

test: ## Run tests with race detector
	go test -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code
	gofmt -w .

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -f $(BINARY)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
