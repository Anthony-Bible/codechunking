# Build zoekt binaries from source for multi-arch (arm64/amd64) compatibility
FROM golang:1.25-bookworm AS zoekt-builder
ARG ZOEKT_VERSION=v0.0.0-20260410121455-f469edd14f92
RUN go install github.com/sourcegraph/zoekt/cmd/zoekt-git-index@${ZOEKT_VERSION}

# Build stage - use Debian-based image to avoid musl libc issues
FROM golang:1.25-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git make gcc g++ && \
    rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy dependency manifests first for better layer caching
COPY go.mod go.sum ./

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# Download dependencies (cached unless go.mod/go.sum change)
RUN go mod download

# Copy remaining source code
COPY . .

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w \
    -X codechunking/cmd.Version=${VERSION} \
    -X codechunking/cmd.Commit=${COMMIT} \
    -X codechunking/cmd.BuildTime=${BUILD_TIME}" \
    -o codechunking main.go

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

# Copy zoekt indexing binary (built from source for arm64 compatibility)
COPY --from=zoekt-builder /go/bin/zoekt-git-index /usr/local/bin/zoekt-git-index

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