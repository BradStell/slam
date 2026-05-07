.PHONY: build test lint run clean tidy

BINARY := slam
PKG    := ./cmd/slam

build:
	go build -o $(BINARY) $(PKG)

test:
	go test ./... -race

lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed; skipping"

run: build
	./$(BINARY) $(ARGS)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
	rm -rf dist/
