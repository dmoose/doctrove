GOBIN  ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN = $(shell go env GOPATH)/bin
endif
BINARY = llmshadow

.PHONY: build install uninstall test vet clean

build:
	go build -o $(BINARY) ./cmd/llmshadow

install:
	go install ./cmd/llmshadow
	@echo "Installed $(BINARY) to $(GOBIN)/"
	@echo "Workspace: ~/.config/llmshadow"
	@echo "Run 'llmshadow mcp-config' for agent integration"

uninstall:
	rm -f $(GOBIN)/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
