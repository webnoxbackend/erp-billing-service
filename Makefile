.PHONY: build run test clean proto docker-build docker-up docker-down

# Build the application
build:
	go build -o bin/example-service ./cmd/server

# Run the application
run:
	go run ./cmd/server

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@if [ ! -d "/tmp/googleapis" ]; then \
		echo "Downloading googleapis..."; \
		git clone --depth 1 https://github.com/googleapis/googleapis.git /tmp/googleapis; \
	fi
	protoc -I. -I/tmp/googleapis --go_out=. --go-grpc_out=. proto/example.proto
	@echo "Protobuf code generated successfully"

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f example-service

# Development setup
setup:
	@echo "Setting up development environment..."
	@cp .env.example .env
	@echo "Please update .env with your configuration"
	@go mod download
	@echo "Setup complete!"

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

