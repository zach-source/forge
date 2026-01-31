.PHONY: build test clean install fmt lint

BINARY_NAME=forge
VERSION?=0.1.0
BUILD_DIR=./bin

# Build flags
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/forge

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) ~/bin/$(BINARY_NAME)

test:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

fmt:
	gofumpt -w .
	goimports -w .

lint:
	golangci-lint run

# Development helpers
run:
	go run ./cmd/forge $(ARGS)

deps:
	go mod tidy
	go mod download
