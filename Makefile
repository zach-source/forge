.PHONY: build build-all test clean install fmt lint

VERSION?=0.1.0
BUILD_DIR=./bin

# Build flags
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

build: build-forge build-foundry

build-forge:
	go build $(LDFLAGS) -o $(BUILD_DIR)/forge ./cmd/forge

build-foundry:
	go build $(LDFLAGS) -o $(BUILD_DIR)/foundry ./cmd/foundry

build-all: build

install: build
	cp $(BUILD_DIR)/forge ~/bin/forge
	cp $(BUILD_DIR)/foundry ~/bin/foundry

install-forge: build-forge
	cp $(BUILD_DIR)/forge ~/bin/forge

install-foundry: build-foundry
	cp $(BUILD_DIR)/foundry ~/bin/foundry

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
run-forge:
	go run ./cmd/forge $(ARGS)

run-foundry:
	go run ./cmd/foundry $(ARGS)

deps:
	go mod tidy
	go mod download
