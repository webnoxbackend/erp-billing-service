# Quick Start Guide

## Prerequisites

- Go 1.24+
- PostgreSQL 12+
- Protocol Buffers compiler (`protoc`)
- Docker (optional, for running with docker-compose)

## Setup Steps

### 1. Install Dependencies

```bash
go mod download
go mod tidy
```

### 2. Install Protobuf Tools

```bash
make proto-deps
# or manually:
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
```

### 3. Setup Database

**Option A: Using Docker (Recommended)**

```bash
docker-compose up -d postgres redis
```

**Option B: Local PostgreSQL**

```bash
createdb example_db
```

### 4. Configure Environment

```bash
cp .env.example .env
# Edit .env with your database credentials
```

### 5. Generate Protobuf Code

```bash
make proto
```

### 6. Run the Service

```bash
make run
# or
go run cmd/server/main.go
```

## Verify Installation

### Check gRPC Server

```bash
grpcurl -plaintext localhost:50051 list
```

### Check HTTP Server

```bash
curl http://localhost:8081/api/v1/examples
```

## Common Commands

```bash
# Build
make build

# Run tests
make test

# Format code
make fmt

# Clean build artifacts
make clean
```

## Next Steps

1. Customize the domain entities in `internal/domain/`
2. Add your business logic in `internal/application/`
3. Implement additional repositories in `internal/adapters/outbound/`
4. Add new endpoints in `proto/example.proto` and regenerate

## Troubleshooting

### Protobuf Generation Issues

If you get errors about missing `google/api/annotations.proto`:

```bash
# Download googleapis
git clone --depth 1 https://github.com/googleapis/googleapis.git /tmp/googleapis

# Then run make proto
```

### Database Connection Issues

- Check PostgreSQL is running: `pg_isready`
- Verify DATABASE_URL in `.env`
- Check database exists: `psql -l | grep example_db`

### Port Already in Use

Change ports in `.env`:
```env
GRPC_PORT=50052
HTTP_PORT=8082
```

