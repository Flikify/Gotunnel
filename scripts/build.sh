#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$ROOT_DIR/build"
export GOCACHE="${GOCACHE:-$BUILD_DIR/.gocache}"

VERSION="${VERSION:-$(bash "$ROOT_DIR/scripts/resolve_version.sh" 2>/dev/null || echo v0.0.0-dev)}"
BUILD_TIME="$(date -u '+%Y-%m-%d %H:%M:%S')"
GIT_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"
USE_UPX="${USE_UPX:-true}"

DESKTOP_PLATFORMS="linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_upx() {
    command -v upx >/dev/null 2>&1
}

compress_binary() {
    local file=$1
    local os=$2

    if [ "$USE_UPX" != "true" ]; then
        return
    fi
    if ! check_upx; then
        log_warn "UPX not found, skipping compression"
        return
    fi
    if [ "$os" = "darwin" ]; then
        log_warn "Skipping UPX for macOS binary: $file"
        return
    fi

    log_info "Compressing $file with UPX..."
    upx -9 -q "$file" 2>/dev/null || log_warn "UPX compression failed for $file"
}

build_web() {
    log_info "Generating Swagger docs..."
    go generate "$ROOT_DIR/cmd/server"

    log_info "Building web UI..."
    rm -rf "$ROOT_DIR/web/dist"
    pushd "$ROOT_DIR/web" >/dev/null

    if [ ! -d "node_modules" ]; then
        log_info "Installing npm dependencies..."
        npm install
    fi
    npm run build

    popd >/dev/null

    log_info "Web UI built successfully"
}

output_name() {
    local component=$1
    local os=$2

    if [ "$os" = "windows" ]; then
        echo "${component}.exe"
    else
        echo "${component}"
    fi
}

build_binary() {
    local os=$1
    local arch=$2
    local component=$3

    local output_dir="$BUILD_DIR/${os}_${arch}"
    local output_file
    output_file="$(output_name "$component" "$os")"
    local output_path="$output_dir/$output_file"

    mkdir -p "$output_dir"
    log_info "Building $component for $os/$arch..."

    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build \
        -buildvcs=false \
        -trimpath \
        -ldflags "-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME' -X 'main.GitCommit=$GIT_COMMIT'" \
        -o "$output_path" \
        "$ROOT_DIR/cmd/$component"

    compress_binary "$output_path" "$os"
    log_info "  -> $output_path"
}

build_all() {
    local platforms="${1:-$DESKTOP_PLATFORMS}"
    local platform os arch

    for platform in $platforms; do
        os="${platform%/*}"
        arch="${platform#*/}"
        build_binary "$os" "$arch" server
        build_binary "$os" "$arch" client
    done
}

build_current() {
    local os
    local arch

    os="$(go env GOOS)"
    arch="$(go env GOARCH)"

    build_binary "$os" "$arch" server
    build_binary "$os" "$arch" client

    log_info "Binaries built in $BUILD_DIR/${os}_${arch}/"
}

build_android() {
    local output_dir="$BUILD_DIR/android_arm64"
    local android_lib_dir="$ROOT_DIR/android/app/libs"

    mkdir -p "$output_dir"
    log_info "Building client for android/arm64..."
    GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build \
        -buildvcs=false \
        -trimpath \
        -ldflags "-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME' -X 'main.GitCommit=$GIT_COMMIT'" \
        -o "$output_dir/client" \
        "$ROOT_DIR/cmd/client"

    if command -v gomobile >/dev/null 2>&1; then
        log_info "Building gomobile Android binding..."
        gomobile bind -target=android/arm64 -androidapi 21 -javapkg com.gotunnel.mobilebind -o "$output_dir/gotunnelmobile.aar" github.com/gotunnel/mobile/gotunnelmobile
        mkdir -p "$android_lib_dir"
        cp "$output_dir/gotunnelmobile.aar" "$android_lib_dir/gotunnelmobile.aar"
    else
        log_warn "gomobile not found, skipping Android AAR build"
    fi

    if [ -d "$ROOT_DIR/android" ]; then
        if [ -x "$ROOT_DIR/android/gradlew" ]; then
            log_info "Building Android debug APK..."
            (cd "$ROOT_DIR/android" && ./gradlew assembleDebug)
        else
            log_warn "android/gradlew not found, skipping APK build"
        fi
    else
        log_warn "Android host project not found, skipping APK build"
    fi
}

clean() {
    log_info "Cleaning build directory..."
    rm -rf "$BUILD_DIR"
    log_info "Clean completed"
}

show_help() {
    cat <<'EOF'
Usage: build.sh [command] [options]

Commands:
  all       Build web UI + all desktop platforms (default)
  current   Build web UI + current platform only
  web       Build web UI only
  server    Build server for current platform
  client    Build client for current platform
  android   Build android/arm64 client and optional Android artifacts
  clean     Clean build directory
  help      Show this help message

Environment variables:
  VERSION   Set version string (default: auto-resolved from tag or latest tag + commit)
  USE_UPX   Enable UPX compression (default: true)

Examples:
  ./scripts/build.sh current
  VERSION=1.0.0 ./scripts/build.sh all
EOF
}

main() {
    cd "$ROOT_DIR"

    case "${1:-current}" in
        all)
            build_web
            build_all "${2:-}"
            ;;
        current)
            build_web
            build_current
            ;;
        web)
            build_web
            ;;
        server)
            build_binary "$(go env GOOS)" "$(go env GOARCH)" server
            ;;
        client)
            build_binary "$(go env GOOS)" "$(go env GOARCH)" client
            ;;
        android)
            build_android
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac

    log_info "Done!"
}

main "$@"
