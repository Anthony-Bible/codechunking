# Build stage - use Debian-based image to avoid musl libc issues
FROM golang:1.24-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make gcc g++ && \
    rm -rf /var/lib/apt/lists/*

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

# Final stage - use Debian slim for glibc compatibility
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates tzdata libstdc++6 git && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -g 1000 codechunking && \
    useradd -u 1000 -g codechunking -m -s /bin/bash codechunking

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