#!/bin/sh
# Setup script for the recur project.
# Installs development dependencies and prepares the repository for building.
# POSIX-compliant for portability across shells and platforms.
#
# Usage: ./setup.sh [--dry-run]

set -e

DRY_RUN=false
for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=true ;;
        *) printf "Unknown option: %s\nUsage: ./setup.sh [--dry-run]\n" "$arg" >&2; exit 1 ;;
    esac
done

# Minimum required Go version
MIN_GO_VERSION="1.25"

log() {
    printf "[setup] %s\n" "$1"
}

error() {
    printf "[setup] ERROR: %s\n" "$1" >&2
    exit 1
}

run() {
    if [ "$DRY_RUN" = true ]; then
        log "(dry-run) $*"
    else
        "$@"
    fi
}

check_command() {
    command -v "$1" >/dev/null 2>&1
}

# Compare semver-style versions: returns 0 if $1 >= $2
version_gte() {
    if [ -z "$1" ] || [ -z "$2" ]; then
        return 1
    fi

    major1=$(echo "$1" | cut -d. -f1)
    minor1=$(echo "$1" | cut -d. -f2)
    major2=$(echo "$2" | cut -d. -f1)
    minor2=$(echo "$2" | cut -d. -f2)

    # Default to 0 if parsing fails
    major1=${major1:-0}
    minor1=${minor1:-0}
    major2=${major2:-0}
    minor2=${minor2:-0}

    if [ "$major1" -gt "$major2" ] 2>/dev/null; then
        return 0
    elif [ "$major1" -eq "$major2" ] 2>/dev/null && [ "$minor1" -ge "$minor2" ] 2>/dev/null; then
        return 0
    fi
    return 1
}

# --- Check Go ---
log "Checking for Go..."
if ! check_command go; then
    error "Go is not installed or not in PATH. Install Go ${MIN_GO_VERSION}+ from https://go.dev/dl/"
fi

GO_VERSION=$(go version | sed 's/.*go\([0-9]*\.[0-9]*\).*/\1/')
if ! version_gte "$GO_VERSION" "$MIN_GO_VERSION"; then
    error "Go ${MIN_GO_VERSION}+ is required, found ${GO_VERSION}"
fi
log "Found Go ${GO_VERSION}"

# Ensure GOBIN is in PATH for installed tools
GOBIN=$(go env GOBIN)
if [ -z "$GOBIN" ]; then
    GOBIN=$(go env GOPATH)/bin
fi
case ":${PATH}:" in
    *":${GOBIN}:"*) ;;
    *) export PATH="${GOBIN}:${PATH}" ;;
esac

# --- Install buf ---
log "Installing buf (protobuf toolchain)..."
if check_command buf; then
    log "buf already installed: $(buf --version)"
else
    run go install github.com/bufbuild/buf/cmd/buf@latest
    if [ "$DRY_RUN" = false ]; then
        log "buf installed: $(buf --version)"
    fi
fi

# --- Install task ---
log "Installing task (task runner)..."
if check_command task; then
    log "task already installed: $(task --version)"
else
    run go install github.com/go-task/task/v3/cmd/task@latest
    if [ "$DRY_RUN" = false ]; then
        log "task installed: $(task --version)"
    fi
fi

# --- Download Go module dependencies ---
log "Downloading Go module dependencies..."
run go mod download

# --- Generate protobuf code ---
log "Generating protobuf code..."
(cd api && run buf generate)

log ""
log "Setup complete. Available commands:"
log "  task          Build everything (generate + build)"
log "  task build    Build binaries"
log "  task test     Run unit tests"
log "  task test:all Run all tests"
log "  task --list   Show all available tasks"
log ""
log "Make sure $(go env GOPATH)/bin is in your PATH to use buf and task."
