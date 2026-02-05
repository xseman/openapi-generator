.PHONY: help test cover lint build clean

OUTPUT_DIR ?= bin
VERSION ?= $(shell jq -r '."."' .github/.release-manifest.json 2>/dev/null || echo "dev")

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## test: Run all tests
test:
	go test -v -race ./...

## cover: Run tests with coverage
cover:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

## cover-html: Generate HTML coverage report
cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run linter
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...
	gofumpt -l -w .

## vet: Run go vet
vet:
	go vet ./...

## build: Build the binary
build:
	@mkdir -p $(OUTPUT_DIR)
	@CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(OUTPUT_DIR)/openapi-generator ./cmd/openapi-generator

## install: Install the binary
install:
	go install ./cmd/openapi-generator

## clean: Clean build artifacts
clean:
	rm -rf $(OUTPUT_DIR)
	rm -f coverage.out coverage.html
	rm -rf artifacts/
	rm -rf generated/

## tidy: Tidy go modules
tidy:
	go mod tidy

## update: Update dependencies
update:
	go get -u ./...
	go mod tidy

## generate: Run code generation (example usage)
generate:
	@echo "Generating code from sample spec..."
	./openapi-generator generate \
		-i samples/config.yaml \
		-g typescript-fetch \
		-o generated

## all: Run tests, lint, and build
all: test lint build
