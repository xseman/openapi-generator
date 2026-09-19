.PHONY: help test cover lint build clean

GOFUMPT ?= go run mvdan.cc/gofumpt@v0.12.0
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.0

OUTPUT_DIR ?= bin
VERSION ?= $(shell jq -r '."."' .github/.release-manifest.json 2>/dev/null || echo "dev")

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## test: go vet, then every test with the race detector (what CI runs)
test:
	go vet ./...
	go test -race ./...

## cover: Run tests with coverage
cover:
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

## cover-html: Generate HTML coverage report
cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: golangci-lint with .golangci.yml, the style gate CI runs; must be clean
lint:
	$(GOLANGCI) run

## fmt: gofumpt everything
fmt:
	$(GOFUMPT) -w .

## vet: Run go vet
vet:
	go vet ./...

## build: Build the binary
build:
	@mkdir -p $(OUTPUT_DIR)
	@CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/xseman/openapi-generator/internal/update.Version=$(VERSION)" -o $(OUTPUT_DIR)/openapi-generator ./cmd/openapi-generator

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
