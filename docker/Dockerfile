# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o codechunking main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S codechunking && \
    adduser -u 1000 -S codechunking -G codechunking

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/codechunking /app/codechunking

# Copy configuration files
COPY --from=builder /app/configs /app/configs

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Change ownership
RUN chown -R codechunking:codechunking /app

# Switch to non-root user
USER codechunking

# Expose ports
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/app/codechunking"]

# Default command
CMD ["api"]