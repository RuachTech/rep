# REP Gateway — Build & development targets.

VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.1.0-dev")
BINARY  := rep-gateway
GOFLAGS := -ldflags="-s -w -X main.version=$(VERSION)" -trimpath

.PHONY: all build test clean docker lint

all: build

## build: Compile the gateway binary for the current platform.
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -o bin/$(BINARY) ./cmd/rep-gateway

## build-linux: Cross-compile for Linux amd64 (container target).
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/rep-gateway

## build-all: Cross-compile for all supported platforms.
build-all:
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(GOFLAGS) -o bin/$(BINARY)-linux-amd64   ./cmd/rep-gateway
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(GOFLAGS) -o bin/$(BINARY)-linux-arm64   ./cmd/rep-gateway
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-amd64  ./cmd/rep-gateway
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(GOFLAGS) -o bin/$(BINARY)-darwin-arm64  ./cmd/rep-gateway

## test: Run all tests.
test:
	go test -v -race -count=1 ./...

## test-cover: Run tests with coverage report.
test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## fmt: Format
fmt:
	go fmt ./...

## lint: Run static analysis (requires golangci-lint).
lint:
	golangci-lint run ./...
	go vet ./...

## docker: Build the Docker image.
docker:
	docker build -t rep-gateway:$(VERSION) -t rep-gateway:latest .

## docker-push: Push to GitHub Container Registry.
docker-push: docker
	docker tag rep-gateway:$(VERSION) ghcr.io/ruachtech/rep-gateway:$(VERSION)
	docker tag rep-gateway:latest ghcr.io/ruachtech/rep-gateway:latest
	docker push ghcr.io/ruachtech/rep-gateway:$(VERSION)
	docker push ghcr.io/ruachtech/rep-gateway:latest

## clean: Remove build artifacts.
clean:
	rm -rf bin/ coverage.out coverage.html

## run-example: Run the gateway locally in embedded mode with example config.
run-example:
	REP_PUBLIC_API_URL="https://api.example.com" \
	REP_PUBLIC_FEATURE_FLAGS="dark-mode,new-checkout" \
	REP_PUBLIC_APP_VERSION="2.4.1" \
	REP_SENSITIVE_ANALYTICS_KEY="UA-12345-1" \
	REP_SERVER_INTERNAL_SECRET="never-sent-to-client" \
	go run ./cmd/rep-gateway --mode embedded --static-dir ./testdata/static --log-format text

## help: Show this help message.
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
