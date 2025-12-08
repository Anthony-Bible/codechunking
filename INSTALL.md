# Installation Guide

This guide provides detailed installation instructions for CodeChunking, a production-grade semantic code search system.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
  - [Method 1: Go Install](#method-1-go-install)
  - [Method 2: Pre-built Binaries](#method-2-pre-built-binaries)
  - [Method 3: Build from Source](#method-3-build-from-source)
  - [Method 4: Docker](#method-4-docker)
- [Version Management](#version-management)
- [Binary Differences](#binary-differences)
- [Cross-Platform Builds](#cross-platform-builds)
- [Troubleshooting](#troubleshooting)
- [Environment Setup](#environment-setup)

## Prerequisites

### System Requirements

- **Go 1.24+** (required for building from source)
- **Git** (for repository cloning and version info)
- **CGO toolchain** (only for main binary)
  - Linux: `gcc` or `clang`
  - macOS: Xcode Command Line Tools
  - Windows: MinGW-w64 or TDM-GCC

### Optional Requirements

- **Docker** (for containerized deployment)
- **PostgreSQL** with pgvector extension
- **NATS** JetStream server
- **Google Gemini API key** (for embeddings)

## Installation Methods

### Method 1: Go Install

This method installs Go binaries directly from the repository.

#### Main Binary Installation

```bash
# Set CGO_ENABLED=1 for tree-sitter support
export CGO_ENABLED=1
go install github.com/Anthony-Bible/codechunking/cmd/codechunking@latest
```

#### Client Binary Installation

```bash
# Client binary doesn't require CGO
go install github.com/Anthony-Bible/codechunking/cmd/client@latest
```

#### Verify Installation

```bash
# Check main binary
codechunking version

# Check client binary
codechunking-client version
```

#### Go Install Troubleshooting

**Issue: CGO errors during installation**
```bash
# Ensure CGO is enabled
export CGO_ENABLED=1

# On Ubuntu/Debian, install build tools
sudo apt-get update
sudo apt-get install build-essential

# On macOS, install Xcode tools
xcode-select --install

# On Windows, install TDM-GCC and add to PATH
```

**Issue: Module path not found**
```bash
# Ensure you're using Go 1.24+
go version

# Try with explicit version
CGO_ENABLED=1 go install github.com/Anthony-Bible/codechunking/cmd/codechunking@v1.0.0
```

### Method 2: Pre-built Binaries

Download pre-compiled binaries from GitHub releases for your platform.

#### Finding Releases

Visit: https://github.com/Anthony-Bible/codechunking/releases

#### Downloading for Linux

```bash
# Determine your architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Download latest version
VERSION=$(curl -s https://api.github.com/repos/Anthony-Bible/codechunking/releases/latest | grep -o '"tag_name": "[^"]*' | cut -d'"' -f2)

# Download main binary
wget "https://github.com/Anthony-Bible/codechunking/releases/download/${VERSION}/codechunking-${ARCH}"
chmod +x "codechunking-${ARCH}"
sudo mv "codechunking-${ARCH}" /usr/local/bin/codechunking

# Download client binary
wget "https://github.com/Anthony-Bible/codechunking/releases/download/${VERSION}/client-${ARCH}"
chmod +x "client-${ARCH}"
sudo mv "client-${ARCH}" /usr/local/bin/codechunking-client
```

#### Downloading for macOS

```bash
# Using Homebrew (if available)
brew install codechunking

# Or manually download
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
esac

VERSION=$(curl -s https://api.github.com/repos/Anthony-Bible/codechunking/releases/latest | grep -o '"tag_name": "[^"]*' | cut -d'"' -f2)

curl -L "https://github.com/Anthony-Bible/codechunking/releases/download/${VERSION}/codechunking-darwin-${ARCH}" -o codechunking
chmod +x codechunking
sudo mv codechunking /usr/local/bin/
```

#### Downloading for Windows

```powershell
# Using PowerShell
$Version = (Invoke-RestMethod -Uri "https://api.github.com/repos/Anthony-Bible/codechunking/releases/latest").tag_name

# Download main binary
Invoke-WebRequest -Uri "https://github.com/Anthony-Bible/codechunking/releases/download/$Version/codechunking-windows-amd64.exe" -OutFile "codechunking.exe"
# Download client binary
Invoke-WebRequest -Uri "https://github.com/Anthony-Bible/codechunking/releases/download/$Version/client-windows-amd64.exe" -OutFile "codechunking-client.exe"

# Add to PATH or move to desired location
```

#### Verifying Checksums

Always verify downloaded binaries:

```bash
# Download checksums
wget "https://github.com/Anthony-Bible/codechunking/releases/download/${VERSION}/checksums.txt"

# Verify main binary
sha256sum codechunking-${ARCH} | grep -f checksums.txt

# Verify client binary
sha256sum client-${ARCH} | grep -f checksums.txt
```

### Method 3: Build from Source

Build binaries directly from source code.

#### Quick Build

```bash
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking
make build
```

The binaries will be created in `./bin/`:
- `bin/codechunking` - Main application
- `bin/client` - Client binary

#### Detailed Build Instructions

```bash
# Clone repository
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking

# Install dependencies
go mod download

# Build with version
./scripts/build.sh v1.0.0

# Or cross-compile
./scripts/build.sh --platform linux/amd64 v1.0.0
```

#### Build Script Options

The build script (`scripts/build.sh`) supports multiple options:

```bash
# Show help
./scripts/build.sh --help

# Clean build
./scripts/build.sh --clean v1.0.0

# Verbose build
./scripts/build.sh --verbose v1.0.0

# Cross-platform builds
./scripts/build.sh --platform linux/amd64 v1.0.0
./scripts/build.sh --platform darwin/arm64 v1.0.0
./scripts/build.sh --platform windows/amd64 v1.0.0

# Build with test optimizations
TEST_MODE=true ./scripts/build.sh v1.0.0
```

#### Make Commands

```bash
# Build both binaries (uses build script)
make build

# Build with specific version
make build-with-version VERSION=v1.0.0

# Build only main binary (legacy method)
make build-main

# Build only client binary (no CGO)
make build-client

# Install both binaries to GOPATH/bin
make install

# Install only client binary
make install-client

# Install development tools
make install-tools

# Clean build artifacts
make clean
```

### Method 4: Docker

#### Using Pre-built Docker Image

```bash
# Pull image
docker pull ghcr.io/anthony-bible/codechunking:latest

# Run container
docker run -p 8080:8080 ghcr.io/anthony-bible/codechunking:latest
```

#### Building Docker Image

```bash
# Clone repository
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking

# Build image
docker build -t codechunking .

# Or with version
docker build -t codechunking:v1.0.0 .

# Run with environment variables
docker run -p 8080:8080 \
  -e CODECHUNK_DATABASE_HOST=host.docker.internal \
  -e CODECHUNK_NATS_URL=nats://host.docker.internal:4222 \
  codechunking
```

#### Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Version Management

### Checking Version

```bash
# Full version information
codechunking version

# Output format:
codechunking version 1.0.0
commit: abc123def456
built: 2024-01-15T10:30:00Z

# Short version
codechunking version --short
# Output: 1.0.0
```

### Installing Specific Versions

```bash
# Go install specific version
CGO_ENABLED=1 go install github.com/Anthony-Bible/codechunking/cmd/codechunking@v1.0.0

# Download specific version binary
wget https://github.com/Anthony-Bible/codechunking/releases/download/v1.0.0/codechunking-linux-amd64
```

### Version Format

Versions follow Semantic Versioning: `v<MAJOR>.<MINOR>.<PATCH>`
- `v1.0.0` - Stable release
- `v1.1.0-beta` - Pre-release
- `v1.0.1` - Patch release

## Binary Differences

| Feature | Main Binary (`codechunking`) | Client Binary (`codechunking-client`) |
|---------|----------------------------|---------------------------------------|
| API Server | ✓ | ✗ |
| Worker | ✓ | ✗ |
| File Processing | ✓ (tree-sitter) | ✗ |
| Repository Management | ✓ | ✗ |
| API Client | ✗ | ✓ |
| CGO Required | ✓ | ✗ |
| Binary Size | ~15-20MB | ~5-8MB |
| Dependencies | tree-sitter, CGO | None |

### When to Use Which Binary

**Use Main Binary (`codechunking`) when:**
- Running the API server
- Processing repositories
- Running background workers
- Need full functionality

**Use Client Binary (`codechunking-client`) when:**
- Interacting with existing API
- CI/CD pipelines
- AI agent integration
- Minimal dependencies required

## Cross-Platform Builds

Building for multiple platforms:

```bash
# Build for all platforms
./scripts/build.sh --platform linux/amd64 v1.0.0
./scripts/build.sh --platform linux/arm64 v1.0.0
./scripts/build.sh --platform darwin/amd64 v1.0.0
./scripts/build.sh --platform darwin/arm64 v1.0.0
./scripts/build.sh --platform windows/amd64 v1.0.0

# Environment variables for cross-compilation
export GOOS=linux
export GOARCH=arm64
./scripts/build.sh v1.0.0
```

### Supported Platforms

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64 (Apple Silicon)
- **Windows**: amd64

## Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   chmod +x codechunking
   chmod +x codechunking-client
   ```

2. **Command Not Found**
   ```bash
   # Add to PATH
   echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
   source ~/.bashrc
   ```

3. **CGO Errors**
   ```bash
   # Install CGO dependencies
   # Ubuntu/Debian:
   sudo apt-get install build-essential

   # macOS:
   xcode-select --install

   # Windows:
   # Install MinGW-w64 or TDM-GCC
   ```

4. **Tree-sitter Build Failures**
   ```bash
   # Ensure CGO is enabled
   export CGO_ENABLED=1

   # Clean build
   ./scripts/build.sh --clean v1.0.0
   ```

5. **Version Information Missing**
   ```bash
   # Build with version
   ./scripts/build.sh v1.0.0

   # Or create VERSION file
   echo "v1.0.0" > VERSION
   ./scripts/build.sh
   ```

6. **Cross-compilation Issues**
   ```bash
   # For Windows, need MinGW
   sudo apt-get install gcc-mingw-w64-x86-64

   # For arm64, need cross-compiler
   sudo apt-get install gcc-aarch64-linux-gnu
   ```

### Getting Help

```bash
# Common help commands
codechunking --help
codechunking-client --help

# Specific command help
codechunking api --help
codechunking-client repos --help

# Version info for debugging
codechunking version --verbose
```

### Debug Mode

Enable debug logging:

```bash
# Set log level
export CODECHUNK_LOG_LEVEL=debug

# Or use flag
codechunking --log-level debug api
```

## Environment Setup

### Development Environment

```bash
# Clone repository
git clone https://github.com/Anthony-Bible/codechunking.git
cd codechunking

# Install development tools
make install-tools

# Set up environment
cp .env.example .env
# Edit .env with your configuration

# Start development services
make dev
make migrate-up

# Run tests
make test
```

### Production Environment

```bash
# Required environment variables
export CODECHUNK_DATABASE_HOST=localhost
export CODECHUNK_DATABASE_PORT=5432
export CODECHUNK_DATABASE_USER=codechunking
export CODECHUNK_DATABASE_PASSWORD=your_password
export CODECHUNK_DATABASE_NAME=codechunking

# Optional but recommended
export CODECHUNK_GEMINI_API_KEY=your_api_key
export CODECHUNK_LOG_LEVEL=info
export CODECHUNK_API_ENABLE_DEFAULT_MIDDLEWARE=true
```

### Client-Side Environment

For the client binary:

```bash
# Optional configuration
export CODECHUNK_CLIENT_API_URL=http://localhost:8080
export CODECHUNK_CLIENT_TIMEOUT=30s
export CODECHUNK_CLIENT_RETRY_ATTEMPTS=3
```

## Next Steps

After installation:

1. **Set up Database**: Configure PostgreSQL with pgvector
2. **Start Services**: Run API server and worker
3. **Configure**: Set up environment variables
4. **Test API**: Verify with health check endpoint
5. **Index Repository**: Add your first repository

For detailed configuration and usage, see the main [README.md](README.md) and project [wiki](wiki/).