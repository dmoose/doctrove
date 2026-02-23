GOBIN  ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif
BINARY = doctrove
WORKSPACE = $(HOME)/.config/doctrove

.PHONY: build install uninstall test vet clean init-workspace

build:
	go build -o $(BINARY) ./cmd/doctrove

install: build
	go install ./cmd/doctrove
	@echo "Installed $(BINARY) to $(GOBIN)/"
	@echo "Run 'make init-workspace' to create default config"
	@echo "Run 'doctrove mcp-config' for agent integration"

uninstall:
	rm -f $(GOBIN)/$(BINARY)

init-workspace:
	@mkdir -p $(WORKSPACE)
	@if [ ! -f $(WORKSPACE)/doctrove.yaml ]; then \
		printf 'settings:\n  events_url: http://localhost:6060/events\n' > $(WORKSPACE)/doctrove.yaml; \
		echo "Created $(WORKSPACE)/doctrove.yaml"; \
	else \
		echo "$(WORKSPACE)/doctrove.yaml already exists — skipping"; \
	fi

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
