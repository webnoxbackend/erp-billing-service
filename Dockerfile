# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git pkgconfig build-base librdkafka-dev

WORKDIR /app

# Copy shared packages
COPY efs-shared-events ./efs-shared-events
COPY efs-shared-kafka ./efs-shared-kafka

# Copy the service code
COPY erp-billing-service ./erp-billing-service

WORKDIR /app/erp-billing-service

# Download dependencies
ENV GOPROXY=https://goproxy.io,direct
RUN go mod download

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -tags dynamic -a -installsuffix cgo -o billing-service ./cmd/api

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates libc6-compat librdkafka

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/erp-billing-service/billing-service .

# Expose ports
EXPOSE 8088 50051

# Run the service
CMD ["./billing-service"]
