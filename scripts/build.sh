#!/bin/bash

# build.sh - Build script for codechunking project
# Usage: ./build.sh [OPTIONS] [VERSION]

set -euo pipefail

# Change to project root directory
cd "$(dirname "$0")/.." || {
    echo "ERROR: Cannot change to project root directory" >&2
    exit 1
}

# Default values
VERSION=""
VERSION_FILE="${VERSION_FILE:-VERSION}"
OUTPUT_DIR="${OUTPUT_DIR:-bin}"
VERBOSE=false
CLEAN=false
PLATFORM=""
# Enable test mode optimizations
TEST_MODE="${TEST_MODE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    local msg="${1:-}"
    if [ "$VERBOSE" = true ]; then
        printf "${GREEN}[INFO]${NC} %s\n" "$msg" >&2
    fi
}

# Function to always print output (not just in verbose mode)
print_always() {
    local msg="${1:-}"
    printf "${GREEN}[INFO]${NC} %s\n" "$msg" >&2
}

print_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
}

print_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] [VERSION]

Build the codechunking binaries with version information.

OPTIONS:
    -v, --verbose    Enable verbose output
    -c, --clean      Clean build directory before building
    -p, --platform   Target platform (e.g., linux/amd64, darwin/arm64)
    -h, --help       Show this help message

ARGUMENTS:
    VERSION          Version string (e.g., v1.0.0, v2.1.0-beta)
                    If not provided, will read from VERSION file

ENVIRONMENT VARIABLES:
    VERSION_FILE     Path to VERSION file (default: VERSION)
    OUTPUT_DIR       Output directory for binaries (default: bin)

EXAMPLES:
    $0 v1.0.0                    # Build with version v1.0.0
    $0                           # Build using version from VERSION file
    $0 --verbose v1.0.0          # Build with verbose output
    $0 --clean --platform linux/amd64 v1.0.0  # Clean cross-compile build

EOF
}

# Function to validate version format
validate_version() {
    local version="${1:-}"
    if [ -z "$version" ]; then
        print_error "Version parameter is empty"
        return 1
    fi
    # Check version format: v<semver> (e.g., v1.0.0, v2.1.0-beta)
    if ! echo "$version" | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+([a-zA-Z0-9\.\-]*)?$' > /dev/null; then
        print_error "Invalid version format: $version"
        print_error "Version must match format: v1.0.0, v2.1.0-beta, etc."
        return 1
    fi
}

# Function to get version from VERSION file
# shellcheck disable=SC2120 # Function designed for optional argument, called without args
get_version() {
    local version="${1:-}"

    if [ -n "$version" ]; then
        validate_version "$version"
        echo "$version"
        return
    fi

    # Try to read from VERSION file
    if [ -f "$VERSION_FILE" ]; then
        version=$(cat "$VERSION_FILE" | tr -d '[:space:]')
        if [ -n "$version" ]; then
            validate_version "$version"
            print_info "Using version from $VERSION_FILE: $version"
            echo "$version"
            return
        fi
    fi

    print_error "No version specified and $VERSION_FILE file not found or empty"
    print_error "Either provide a version argument or create a $VERSION_FILE file"
    exit 1
}

# Function to check dependencies
check_dependencies() {
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi

    if ! command -v git >/dev/null 2>&1; then
        print_error "Git is not installed or not in PATH"
        exit 1
    fi
}

# Function to get git info
get_git_info() {
    local commit_hash
    local commit_date
    local is_dirty
    local build_time

    commit_hash=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    commit_date=$(git log -1 --format=%cd --date=iso8601 2>/dev/null || echo "unknown")
    is_dirty=$(git diff --quiet 2>/dev/null && echo "false" || echo "true")
    build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    # Return as a single string with quoted values to handle spaces
    echo "git_commit=${commit_hash}" "git_commit_date=${commit_date}" "git_build_time=${build_time}" "git_dirty=${is_dirty}"
}

# Function to build binary
build_binary() {
    local main_path="$1"
    local output_name="$2"
    local version="$3"
    local git_info="$4"

    print_info "Building $output_name from $main_path"

    # Prepare ldflags with version and git info
    local ldflags="-X codechunking/cmd.Version=$version"

    # Add git info to ldflags if available
    for var in $git_info; do
        local key
        local value
        key=$(echo "$var" | cut -d= -f1)
        value=$(echo "$var" | cut -d= -f2-)
        # Map git info to version package variables based on type
        case "$key" in
            git_commit)
                ldflags="$ldflags -X codechunking/cmd.Commit=$value"
                ;;
            git_build_time)
                ldflags="$ldflags -X codechunking/cmd.BuildTime=$value"
                ;;
            *)
                # Skip other variables for now
                ;;
        esac
    done

    # Build the binary
    local build_args=()

    # Add platform if specified
    if [ -n "$PLATFORM" ]; then
        IFS='/' read -ra PLATFORM_PARTS <<< "$PLATFORM"
        local GOOS="${PLATFORM_PARTS[0]}"
        local GOARCH="${PLATFORM_PARTS[1]}"
        export GOOS="$GOOS"
        export GOARCH="$GOARCH"
        print_always "Cross-compiling for $GOOS/$GOARCH"
    fi

    # Add test mode optimizations (must be before adding ldflags to build_args)
    if [ "$TEST_MODE" = true ]; then
        # In test mode, use faster linking and disable some optimizations
        ldflags="$ldflags -w -s"
        # Use smaller build cache for tests
        export GOCACHE=/tmp/go-cache-test
        export GOMODCACHE=/tmp/go-mod-cache-test
    fi

    # Add output and ldflags
    build_args+=("-o" "$OUTPUT_DIR/$output_name")
    build_args+=("-ldflags" "$ldflags")

    # Show build command info (always show ldflags for tests)
    print_always "Building with -ldflags: $ldflags"

    # Add verbose flag if needed
    if [ "$VERBOSE" = true ]; then
        build_args+=("-v")
        echo "Running: go build ${build_args[*]} $main_path"
    fi

    # Execute build command
    go build "${build_args[@]}" "$main_path"

    # Unset platform-specific variables
    if [ -n "${GOOS:-}" ]; then
        unset GOOS
    fi
    if [ -n "${GOARCH:-}" ]; then
        unset GOARCH
    fi
}

# Function to clean build directory
clean_build_dir() {
    if [ -d "${OUTPUT_DIR:?}" ]; then
        print_always "Cleaning build directory: $OUTPUT_DIR"
        rm -rf "${OUTPUT_DIR:?}"/*
    else
        mkdir -p "$OUTPUT_DIR"
    fi
}

# Parse command line arguments
ARGS=()
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -p|--platform)
            if [ -z "${2:-}" ]; then
                print_error "Platform option requires a value (e.g., linux/amd64)"
                exit 1
            fi
            PLATFORM="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        -*)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
        "")
            # Empty argument - add as empty to be handled later (for validation tests)
            ARGS+=("")
            shift
            ;;
        *)
            ARGS+=("$1")
            shift
            ;;
    esac
done

# Restore arguments
set -- "${ARGS[@]}"

# Get version - don't fall back to VERSION file if provided argument is invalid
if [ $# -gt 0 ]; then
    # Check if the provided argument is empty first
    if [ -n "$1" ]; then
        # Validate version first, before capturing
        validate_version "$1" || {
            print_error "Version validation failed"
            exit 1
        }
        VERSION="$1"
    else
        print_error "Version argument cannot be empty"
        exit 1
    fi
else
    VERSION=$(get_version)
fi

# Check dependencies
check_dependencies

# Get git info
GIT_INFO=$(get_git_info)
print_always "Build info: $GIT_INFO"

# Clean build directory if requested
if [ "$CLEAN" = true ]; then
    clean_build_dir
fi

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Print build information
print_info "Starting build for version: $VERSION"
if [ -n "$PLATFORM" ]; then
    print_info "Target platform: $PLATFORM"
fi
print_info "Output directory: $OUTPUT_DIR"

# Build main binary
if [ "$PLATFORM" = "" ]; then
    # For main binary, require CGO_ENABLED=1 due to tree-sitter
    export CGO_ENABLED=1
    build_binary "./main.go" "codechunking" "$VERSION" "$GIT_INFO"
else
    build_binary "./main.go" "codechunking" "$VERSION" "$GIT_INFO"
fi

# Build client binary (static, no CGO)
export CGO_ENABLED=0
build_binary "./cmd/client/main.go" "client" "$VERSION" "$GIT_INFO"

# Success message
printf "${GREEN}✓${NC} Build completed successfully\n"
printf "Binaries created in: %s\n" "$OUTPUT_DIR"
printf "Version: %s\n" "$VERSION"