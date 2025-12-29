# Example Service - Clean Architecture Implementation

A microservice boilerplate built with Go, following Clean Architecture principles. This serves as a template for creating new microservices.

## Architecture Overview

This service implements Clean Architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│                    Presentation Layer                    │
│              (gRPC Handlers / Adapters/Inbound)          │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                  Application Layer                       │
│         (Use Cases / Business Logic / DTOs)              │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│                    Domain Layer                         │
│              (Entities / Business Rules)                 │
└──────────────────────┬──────────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────────┐
│              Infrastructure Layer                        │
│    (PostgreSQL / Redis / Kafka / Adapters/Outbound)     │
└─────────────────────────────────────────────────────────┘
```

## Project Structure

```
example-service/
├── cmd/
│   └── server/              # Application entry point
│       └── main.go
├── internal/                # Private application code
│   ├── domain/              # Business entities and rules
│   ├── ports/               # Interfaces (repositories, services)
│   ├── adapters/            # Implementations
│   │   ├── inbound/         # gRPC/HTTP handlers
│   │   └── outbound/        # Database, Redis, Kafka
│   ├── application/         # Use cases and business logic
│   ├── config/              # Configuration management
│   └── database/            # Database initialization
├── pkg/                     # Public reusable libraries
│   ├── logger/              # Logging utilities
│   ├── errors/              # Error handling
│   └── validator/           # Validation utilities
├── proto/                   # gRPC protocol definitions
└── tests/                   # Test files
```

## Features

- ✅ Clean Architecture
- ✅ gRPC API
- ✅ HTTP REST API
- ✅ PostgreSQL for persistence
- ✅ Redis for caching (optional)
- ✅ Event Publishing (Kafka-ready)
- ✅ Auto migrations with GORM
- ✅ Docker support

## Prerequisites

- Go 1.24+
- PostgreSQL 12+
- Redis 6+ (optional)
- Protocol Buffers compiler

## Setup

### 1. Clone and Install Dependencies

```bash
cd example-service
go mod download
```

### 2. Setup Database

Create a PostgreSQL database:

```bash
createdb example_db
```

**Note**: Database schema is automatically created/updated by GORM when the service starts. No manual migrations are required.

### 3. Configure Environment

Copy `.env.example` to `.env` and update values:

```bash
cp .env.example .env
```

Edit `.env`:

```env
DATABASE_URL=postgres://postgres:postgres@localhost:5433/example_db?sslmode=disable
REDIS_URL=localhost:6380
GRPC_PORT=50051
HTTP_PORT=8081
JWT_SECRET=your-secret-key-change-in-production
```

### 4. Generate Protobuf Code

```bash
make proto
```

### 5. Run the Service

```bash
make run
# or
go run cmd/server/main.go
```

The service will start on:
- gRPC port: `50051` (default)
- HTTP port: `8081` (default)

## API Usage

### Using gRPC Client

```go
import (
    "google.golang.org/grpc"
    proto "example-service/example-service/proto"
)

conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := proto.NewExampleServiceClient(conn)

// Create Example
resp, _ := client.CreateExample(ctx, &proto.CreateExampleRequest{
    Name: "Example Name",
})
```

### Using HTTP REST API

```bash
# Create Example
curl -X POST http://localhost:8081/api/v1/examples \
  -H "Content-Type: application/json" \
  -d '{"name": "Example Name"}'

# Get Example
curl http://localhost:8081/api/v1/examples/1

# List Examples
curl http://localhost:8081/api/v1/examples

# Update Example
curl -X PUT http://localhost:8081/api/v1/examples/1 \
  -H "Content-Type: application/json" \
  -d '{"name": "Updated Name", "status": "active"}'

# Delete Example
curl -X DELETE http://localhost:8081/api/v1/examples/1
```

## Architecture Layers

### Domain Layer (`internal/domain/`)

Contains business entities and rules:
- `example.go` - Example entity
- `errors.go` - Domain errors
- `events.go` - Domain events

**Key Principle**: Zero dependencies on external frameworks.

### Ports Layer (`internal/ports/`)

Defines interfaces (contracts):
- `repositories/` - Data access interfaces
- `services/` - Business service interfaces
- `external/` - External service interfaces

**Key Principle**: Interfaces define what we need, not how it's implemented.

### Adapters Layer (`internal/adapters/`)

Implements the interfaces:
- `inbound/grpc/` - gRPC handlers
- `inbound/http/` - HTTP handlers
- `outbound/postgres/` - PostgreSQL implementation
- `outbound/redis/` - Redis implementation
- `outbound/kafka/` - Event publisher

**Key Principle**: Adapters implement ports, can be swapped easily.

### Application Layer (`internal/application/`)

Contains use cases and business logic:
- `example_service.go` - Example use cases
- `dto/` - Data Transfer Objects

**Key Principle**: Orchestrates domain entities and ports.

## Testing

### Unit Tests

```bash
go test ./internal/domain/...
go test ./internal/application/...
```

### Integration Tests

```bash
go test ./tests/integration/...
```

## Development

### Adding a New Feature

1. **Domain**: Add entities/rules in `internal/domain/`
2. **Ports**: Define interfaces in `internal/ports/`
3. **Application**: Implement use cases in `internal/application/`
4. **Adapters**: Implement interfaces in `internal/adapters/`
5. **Handler**: Wire up in `cmd/server/main.go`

### Code Generation

If you modify `proto/example.proto`:

```bash
make proto
```

## Docker

### Build and Run

```bash
docker-compose up -d
```

### View Logs

```bash
docker-compose logs -f example-service
```

## Deployment

### Build Binary

```bash
make build
```

### Docker Build

```bash
docker build -t example-service .
docker run -p 50051:50051 -p 8081:8081 --env-file .env example-service
```

## Contributing

1. Follow Clean Architecture principles
2. Write tests for new features
3. Update documentation
4. Follow Go best practices

## License

MIT

