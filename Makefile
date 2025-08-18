.PHONY: help test test-race test-cover build clean lint fmt vet tidy examples

# Default target
help:
	@echo "Available targets:"
	@echo "  test       - Run all tests"
	@echo "  test-race  - Run tests with race detection"
	@echo "  test-cover - Run tests with coverage report"
	@echo "  build      - Build all binaries"
	@echo "  clean      - Clean build artifacts"
	@echo "  lint       - Run golangci-lint (requires golangci-lint)"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  tidy       - Run go mod tidy"
	@echo "  examples   - Build all examples"

# Run all tests
test:
	go test -v ./...

# Run tests with race detection
test-race:
	go test -race -v ./...

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build all binaries
build:
	go build -o bin/ephemeral-issuer ./cmd/ephemeral-issuer
	go build -o bin/ws-minimal ./examples/ws-minimal
	go build -o bin/comprehensive ./examples/comprehensive

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f *.test

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin v1.54.2" && exit 1)
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Build examples
examples:
	go build -o bin/ws-minimal ./examples/ws-minimal
	go build -o bin/comprehensive ./examples/comprehensive

# Full check (format, vet, test, build)
check: fmt vet test build
	@echo "All checks passed!"

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest