BINARY := apple-mail-mcp
PKG := ./cmd/apple-mail-mcp

.PHONY: build test lint install clean tools-docs

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./... -race -cover

lint:
	go vet ./...

install:
	go install ./cmd/apple-mail-mcp

clean:
	rm -f $(BINARY)

tools-docs:
	go run ./cmd/apple-mail-mcp tools-docs
