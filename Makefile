PREFIX ?= /usr/local
BINARY = llmshadow

.PHONY: build install uninstall test vet clean

build:
	go build -o $(BINARY) ./cmd/llmshadow

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/
	@echo "Installed $(BINARY) to $(PREFIX)/bin/"
	@echo "Workspace: ~/.config/llmshadow"
	@echo "Run 'llmshadow mcp-config' for agent integration"

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
