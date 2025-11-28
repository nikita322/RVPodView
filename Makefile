.PHONY: build run clean deps test cross-build

# Binary name
BINARY=rvpodview

# Build for current platform
build:
	go build -o $(BINARY) ./cmd/rvpodview

# Run the application
run: build
	./$(BINARY)

# Install dependencies
deps:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -f $(BINARY)-*

# Build for RISC-V Linux (Orange Pi RV2)
build-riscv:
	CGO_ENABLED=1 GOOS=linux GOARCH=riscv64 go build -o $(BINARY)-riscv64 ./cmd/rvpodview

# Build for ARM64 Linux
build-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -o $(BINARY)-arm64 ./cmd/rvpodview

# Build for AMD64 Linux
build-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o $(BINARY)-amd64 ./cmd/rvpodview

# Build all platforms
build-all: build-amd64 build-arm64 build-riscv

# Run tests
test:
	go test -v ./...

# Development mode with auto-reload (requires air)
dev:
	air

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build for current platform"
	@echo "  run         - Build and run"
	@echo "  deps        - Download dependencies"
	@echo "  clean       - Remove build artifacts"
	@echo "  build-riscv - Build for RISC-V Linux"
	@echo "  build-arm64 - Build for ARM64 Linux"
	@echo "  build-amd64 - Build for AMD64 Linux"
	@echo "  build-all   - Build for all platforms"
	@echo "  test        - Run tests"
	@echo "  fmt         - Format code"
	@echo "  lint        - Lint code"
