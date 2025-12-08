#!/bin/bash

# release.sh - Release script for codechunking project
# Usage: ./release.sh [OPTIONS] VERSION

set -euo pipefail

# Get absolute path of script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.." || {
    echo "ERROR: Cannot change to project root directory" >&2
    exit 1
}

# Default values
VERSION=""
VERSION_FILE="${VERSION_FILE:-VERSION}"
RELEASE_DIR="${RELEASE_DIR:-releases}"
BUILD_DIR="${BUILD_DIR:-bin}"
DRY_RUN="${DRY_RUN:-false}"
NO_TAG=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1" >&2
}

print_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

# Function to show usage
show_usage() {
    cat << 'EOF'
Usage: ./release.sh [OPTIONS] VERSION

Create a release for codechunking project with version tagging and binary packaging.

OPTIONS:
    -d, --dry-run    Show what would be done without executing
    -n, --no-tag     Skip git tag creation
    -h, --help       Show this help message

ARGUMENTS:
    VERSION          Version string (e.g., v1.0.0, v2.1.0-beta)
                    Must be provided and follow semantic versioning

EXAMPLES:
    ./release.sh v1.0.0                    # Create release v1.0.0
    ./release.sh --dry-run v1.0.0          # Show what would be done
    ./release.sh --no-tag v1.0.0           # Create release without git tag
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

# Function to run command (or display in dry-run mode)
run_cmd() {
    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: $*"
    else
        print_info "Running: $*"
        "$@"
    fi
}

# Function to update VERSION file
update_version_file() {
    local version="$1"

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would update VERSION with version $version"
    else
        print_info "Updating VERSION with version $version"
        echo "$version" > "$VERSION_FILE"
    fi
}

# Function to run build
run_build() {
    local version="$1"

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would run build script with version $version"
    else
        print_info "Running build script with version $version"
        ./scripts/build.sh "$version"
    fi
}

# Function to create release directory
create_release_dirs() {
    local version="$1"
    local version_dir="$RELEASE_DIR/$version"

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would create directories $RELEASE_DIR and $version_dir"
    else
        print_info "Creating release directories"
        mkdir -p "$version_dir"
    fi
}

# Function to copy binaries with version names
copy_binaries() {
    local version="$1"
    local version_dir="$RELEASE_DIR/$version"

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would copy binaries to $version_dir with version suffixes"
    else
        print_info "Copying binaries with version names"

        local main_binary="$BUILD_DIR/codechunking"
        local client_binary="$BUILD_DIR/client"
        local versioned_main="$version_dir/codechunking-$version"
        local versioned_client="$version_dir/client-$version"

        # Copy main binary
        if [ ! -f "$main_binary" ]; then
            print_error "Main binary not found: $main_binary"
            exit 1
        fi
        cp "$main_binary" "$versioned_main"

        # Copy client binary
        if [ ! -f "$client_binary" ]; then
            print_error "Client binary not found: $client_binary"
            exit 1
        fi
        cp "$client_binary" "$versioned_client"
    fi
}

# Function to generate checksums
generate_checksums() {
    local version="$1"
    local version_dir="$RELEASE_DIR/$version"

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would generate checksums in $version_dir/checksums.txt"
    else
        print_info "Generating SHA256 checksums"
        cd "$version_dir"
        sha256sum codechunking-* client-* > checksums.txt
        cd - > /dev/null
    fi
}

# Function to create git tag
create_git_tag() {
    local version="$1"

    if [ "$NO_TAG" = true ]; then
        print_info "Skipping git tag creation (--no-tag specified)"
        return
    fi

    if [ "${CREATE_GIT_TAG:-false}" != "true" ]; then
        print_info "Skipping git tag creation (CREATE_GIT_TAG not set to true)"
        return
    fi

    if [ "$DRY_RUN" = true ]; then
        print_info "DRY: Would create git tag $version"
    else
        print_info "Creating git tag $version"
        if ! git rev-parse "refs/tags/$version" >/dev/null 2>&1; then
            run_cmd "git tag -a $version -m \"Release $version\""
            print_info "Git tag created: $version"
        else
            print_warn "Git tag $version already exists"
        fi
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -n|--no-tag)
            NO_TAG=true
            shift
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
        *)
            if [ -z "$VERSION" ]; then
                VERSION="$1"
            else
                print_error "Too many arguments. Only one VERSION argument is allowed."
                show_usage
                exit 1
            fi
            shift
            ;;
    esac
done

# Check if version is provided
if [ -z "$VERSION" ]; then
    print_error "Version argument is required"
    show_usage
    exit 1
fi

# Validate version format
validate_version "$VERSION"

# Print release information
print_info "Starting release for version: $VERSION"
if [ "$DRY_RUN" = true ]; then
    print_warn "DRY RUN MODE - No changes will be made"
fi

# Step 1: Update VERSION file
update_version_file "$VERSION"

# Step 2: Run build
run_build "$VERSION"

# Step 3: Create release directories
create_release_dirs "$VERSION"

# Step 4: Copy binaries
copy_binaries "$VERSION"

# Step 5: Generate checksums
generate_checksums "$VERSION"

# Step 6: Create git tag
create_git_tag "$VERSION"

# Success message
printf "${GREEN}✓${NC} Release created successfully\n"