# Clean Architecture Analysis - Example Service

## Overview

This document provides a deep analysis of the Clean Architecture implementation for the Example Service microservice boilerplate.

## Architecture Principles

### 1. Dependency Rule

**The fundamental rule of Clean Architecture:**

> Source code dependencies can only point inward. Nothing in an inner circle can know anything at all about something in an outer circle.

```
┌─────────────────────────────────────────┐
│  Frameworks & Drivers (Outermost)      │  ← gRPC, PostgreSQL, Redis
│  ┌───────────────────────────────────┐ │
│  │  Interface Adapters                │  ← Handlers, Repositories
│  │  ┌───────────────────────────────┐ │
│  │  │  Application Business Rules   │  ← Use Cases, Services
│  │  │  ┌───────────────────────────┐ │
│  │  │  │  Enterprise Business     │  ← Domain Entities
│  │  │  │  Rules (Innermost)        │  │
│  │  │  └───────────────────────────┘ │
│  │  └───────────────────────────────┘ │
│  └───────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### 2. Layer Responsibilities

#### Domain Layer (`internal/domain/`)
- **Purpose**: Core business logic and entities
- **Dependencies**: NONE (zero external dependencies)
- **Contains**:
  - Entities (Example)
  - Business rules (Example.IsActive())
  - Domain errors
  - Domain events

**Example:**
```go
// internal/domain/example.go
type Example struct {
    ID        int64
    Name      string
    Status    string
}

func (e *Example) IsActive() bool {
    return e.Status == "active"
}
```

#### Ports Layer (`internal/ports/`)
- **Purpose**: Define contracts/interfaces
- **Dependencies**: Domain layer only
- **Contains**:
  - Repository interfaces
  - Service interfaces
  - External service interfaces

**Example:**
```go
// internal/ports/repositories/example_repository.go
type ExampleRepository interface {
    Create(example *domain.Example) error
    FindByID(id int64) (*domain.Example, error)
    // ...
}
```

#### Application Layer (`internal/application/`)
- **Purpose**: Implement use cases and orchestrate domain logic
- **Dependencies**: Domain + Ports
- **Contains**:
  - Use case implementations
  - DTOs (Data Transfer Objects)
  - Service implementations

**Example:**
```go
// internal/application/example_service.go
type ExampleService struct {
    exampleRepo   repositories.ExampleRepository
    eventPublisher external.EventPublisher
}

func (s *ExampleService) CreateExample(req *dto.CreateExampleRequest) (*dto.ExampleResponse, error) {
    // 1. Validate input
    // 2. Create domain entity
    // 3. Save via repository
    // 4. Publish event
    // 5. Return DTO
}
```

#### Adapters Layer (`internal/adapters/`)
- **Purpose**: Implement ports, connect to external systems
- **Dependencies**: All inner layers
- **Contains**:
  - **Inbound**: gRPC/HTTP handlers (external → internal)
  - **Outbound**: PostgreSQL, Redis, Kafka implementations

**Example:**
```go
// internal/adapters/outbound/postgres/example_repository.go
type ExampleRepository struct {
    db *gorm.DB
}

func (r *ExampleRepository) Create(example *domain.Example) error {
    return r.db.Create(example).Error
}
```

## Data Flow

### Request Flow (Inbound)

```
1. gRPC/HTTP Request
   ↓
2. Handler (adapters/inbound/grpc or http)
   - Maps proto/JSON → DTO
   - Validates input
   ↓
3. Application Service (application/)
   - Orchestrates use case
   - Uses domain entities
   - Calls repositories via interfaces
   ↓
4. Domain Layer (domain/)
   - Business rules
   - Entity methods
   ↓
5. Repository Implementation (adapters/outbound/postgres)
   - Uses GORM for database operations
   - Maps DB → Domain entities
```

### Response Flow (Outbound)

```
1. Repository (adapters/outbound/postgres)
   - Queries database
   - Returns domain entities
   ↓
2. Application Service (application/)
   - Transforms entities → DTOs
   - Applies business logic
   ↓
3. Handler (adapters/inbound/grpc or http)
   - Maps DTO → proto/JSON
   ↓
4. gRPC/HTTP Response
```

## Key Design Patterns

### 1. Dependency Injection

All dependencies are injected via constructors:

```go
func NewExampleService(
    exampleRepo repositories.ExampleRepository,
    eventPublisher external.EventPublisher,
) services.ExampleService {
    return &ExampleService{
        exampleRepo: exampleRepo,
        eventPublisher: eventPublisher,
    }
}
```

**Benefits:**
- Easy to test (mock dependencies)
- Easy to swap implementations
- Clear dependencies

### 2. Interface Segregation

Interfaces are small and focused:

```go
// Separate interfaces for different concerns
type ExampleRepository interface { ... }
type EventPublisher interface { ... }
```

### 3. Repository Pattern

Abstracts data access:

```go
// Port (interface)
type ExampleRepository interface {
    FindByID(id int64) (*domain.Example, error)
}

// Adapter (implementation)
type PostgresExampleRepository struct {
    db *gorm.DB
}
```

### 4. DTO Pattern

Separates internal domain from external API:

```go
// Domain entity (internal)
type Example struct {
    ID     int64
    Name   string
    Status string
}

// DTO (external)
type ExampleResponse struct {
    ID     int64  `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}
```

## Testing Strategy

### Unit Tests

Test each layer in isolation:

```go
// Test domain logic
func TestExample_IsActive(t *testing.T) {
    example := &domain.Example{Status: "active"}
    assert.True(t, example.IsActive())
}

// Test application service with mocks
func TestExampleService_CreateExample(t *testing.T) {
    mockRepo := &MockExampleRepository{}
    service := NewExampleService(mockRepo, nil)
    // Test create logic
}
```

### Integration Tests

Test adapters with real dependencies:

```go
// Test PostgreSQL repository
func TestPostgresExampleRepository_Create(t *testing.T) {
    db := setupTestDB()
    repo := postgres.NewExampleRepository(db)
    // Test with real database
}
```

## Benefits of This Architecture

### 1. Testability
- Domain logic has zero dependencies → easy to test
- Application layer uses interfaces → easy to mock
- Each layer can be tested independently

### 2. Maintainability
- Clear separation of concerns
- Easy to locate code
- Changes are isolated to specific layers

### 3. Flexibility
- Swap PostgreSQL for MongoDB → change adapter only
- Swap gRPC for REST → change handler only
- Domain and application logic unchanged

### 4. Scalability
- Easy to split into microservices
- Easy to add new features
- Easy to replace implementations

## Common Pitfalls to Avoid

### ❌ Don't: Import outer layers in inner layers

```go
// WRONG: Domain importing gRPC
import "google.golang.org/grpc"

type Example struct {
    // ...
}
```

### ✅ Do: Keep domain pure

```go
// CORRECT: Domain has no external imports
type Example struct {
    // ...
}
```

### ❌ Don't: Put business logic in handlers

```go
// WRONG: Business logic in handler
func (h *Handler) CreateExample(req *CreateExampleRequest) {
    example := h.db.Query("SELECT ...") // Direct DB access
    if example.IsActive() { // Business logic
        // ...
    }
}
```

### ✅ Do: Delegate to application layer

```go
// CORRECT: Handler delegates to service
func (h *Handler) CreateExample(req *CreateExampleRequest) {
    resp, err := h.exampleService.CreateExample(req)
    // ...
}
```

## Migration Path

If you have existing code, migrate layer by layer:

1. **Extract Domain**: Move entities to `domain/`
2. **Define Ports**: Create interfaces in `ports/`
3. **Implement Application**: Move business logic to `application/`
4. **Create Adapters**: Implement interfaces in `adapters/`
5. **Wire Up**: Connect everything in `main.go`

## Conclusion

This Clean Architecture implementation provides:

- ✅ Clear separation of concerns
- ✅ High testability
- ✅ Easy maintenance
- ✅ Flexible design
- ✅ Scalable structure

The architecture follows SOLID principles and Clean Architecture guidelines, making it suitable for production microservices.

