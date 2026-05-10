.PHONY: build build-all clean test install

VERSION := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Build for current platform
build:
	go build $(LDFLAGS) -o bin/contextsync ./cmd/contextsync

# Build for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/contextsync-darwin-amd64 ./cmd/contextsync
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/contextsync-darwin-arm64 ./cmd/contextsync
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/contextsync-linux-amd64 ./cmd/contextsync
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/contextsync-linux-arm64 ./cmd/contextsync
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/contextsync-windows-amd64.exe ./cmd/contextsync

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Install locally
install: build
	cp bin/contextsync /usr/local/bin/

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run ./...

# Run locally for development
run:
	go run ./cmd/contextsync

# Generate release archives
release: build-all
	cd bin && \
	zip contextsync-darwin-amd64.zip contextsync-darwin-amd64 && \
	zip contextsync-darwin-arm64.zip contextsync-darwin-arm64 && \
	zip contextsync-linux-amd64.zip contextsync-linux-amd64 && \
	zip contextsync-linux-arm64.zip contextsync-linux-arm64 && \
	zip contextsync-windows-amd64.zip contextsync-windows-amd64.exe

# Docker build
docker:
	docker build -t contextsync/cli:$(VERSION) .
