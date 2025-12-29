# Project Structure

```
example-service/
├── cmd/
│   └── server/
│       └── main.go                    # Application entry point
│
├── internal/                           # Private application code
│   ├── domain/                         # Domain Layer (Business Entities)
│   │   ├── example.go                  # Example entity with business logic
│   │   ├── errors.go                   # Domain-specific errors
│   │   └── events.go                   # Domain events
│   │
│   ├── ports/                          # Ports Layer (Interfaces)
│   │   ├── repositories/                # Repository interfaces
│   │   │   └── example_repository.go
│   │   ├── services/                   # Service interfaces
│   │   │   └── example_service.go
│   │   └── external/                   # External service interfaces
│   │       └── event_publisher.go
│   │
│   ├── application/                    # Application Layer (Use Cases)
│   │   ├── dto/                        # Data Transfer Objects
│   │   │   └── example_dto.go
│   │   └── example_service.go          # Service implementation
│   │
│   ├── adapters/                       # Adapters Layer (Implementations)
│   │   ├── inbound/                    # Inbound adapters (External → Internal)
│   │   │   ├── grpc/
│   │   │   │   └── handler.go          # gRPC handler
│   │   │   └── http/
│   │   │       └── handler.go          # HTTP handler
│   │   └── outbound/                   # Outbound adapters (Internal → External)
│   │       ├── postgres/
│   │       │   └── example_repository.go  # PostgreSQL repository
│   │       └── kafka/
│   │           └── event_publisher.go    # Kafka event publisher
│   │
│   ├── config/
│   │   └── config.go                   # Configuration management
│   │
│   └── database/
│       └── database.go                 # Database initialization
│
├── pkg/                                # Public reusable libraries
│   ├── logger/
│   │   └── logger.go                   # Logging utilities
│   ├── errors/
│   │   └── errors.go                   # Error handling
│   └── validator/
│       └── validator.go                # Validation utilities
│
├── proto/                              # Protocol Buffer definitions
│   └── example.proto                   # gRPC service definition
│
├── tests/                              # Test files
│   ├── unit/                           # Unit tests
│   └── integration/                    # Integration tests
│
├── cmd/
│   └── server/
│       └── main.go                     # Application entry point
│
├── .gitignore                          # Git ignore rules
├── .env.example                        # Environment variables template
├── docker-compose.yml                  # Docker Compose configuration
├── Dockerfile                          # Docker build file
├── go.mod                              # Go module definition
├── Makefile                            # Build commands
├── Makefile.proto                      # Protobuf generation commands
├── README.md                           # Project documentation
├── ARCHITECTURE.md                     # Architecture documentation
├── QUICK_START.md                      # Quick start guide
└── STRUCTURE.md                        # This file
```

## Layer Dependencies

```
┌─────────────────────────────────────┐
│  Adapters (Outermost)               │
│  - gRPC/HTTP Handlers               │
│  - PostgreSQL/Redis/Kafka           │
└──────────────┬──────────────────────┘
               │ depends on
┌──────────────▼──────────────────────┐
│  Application                        │
│  - Use Cases                        │
│  - DTOs                             │
└──────────────┬──────────────────────┘
               │ depends on
┌──────────────▼──────────────────────┐
│  Ports (Interfaces)                 │
│  - Repository interfaces            │
│  - Service interfaces               │
└──────────────┬──────────────────────┘
               │ depends on
┌──────────────▼──────────────────────┐
│  Domain (Innermost)                 │
│  - Entities                         │
│  - Business Rules                   │
│  - Events                           │
└─────────────────────────────────────┘
```

## File Naming Conventions

- **Domain entities**: `example.go`, `user.go`, etc.
- **Repositories**: `example_repository.go`
- **Services**: `example_service.go`
- **DTOs**: `example_dto.go`
- **Handlers**: `handler.go` (in respective directories)
- **Config**: `config.go`
- **Database**: `database.go`

## Adding New Features

1. **Add Domain Entity**: `internal/domain/your_entity.go`
2. **Add Repository Interface**: `internal/ports/repositories/your_repository.go`
3. **Add Service Interface**: `internal/ports/services/your_service.go`
4. **Implement Repository**: `internal/adapters/outbound/postgres/your_repository.go`
5. **Implement Service**: `internal/application/your_service.go`
6. **Add DTOs**: `internal/application/dto/your_dto.go`
7. **Add Handlers**: `internal/adapters/inbound/grpc/handler.go` (add methods)
8. **Update Proto**: `proto/example.proto` (add RPCs)
9. **Wire Up**: `cmd/server/main.go` (initialize and register)

