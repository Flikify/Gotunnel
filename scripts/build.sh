#!/bin/bash

set -e

# 项目根目录
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$ROOT_DIR/build"

# 版本信息
VERSION="${VERSION:-dev}"
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 默认目标平台
DEFAULT_PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

# 是否启用 UPX 压缩
USE_UPX="${USE_UPX:-true}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 UPX 是否可用
check_upx() {
    if command -v upx &> /dev/null; then
        return 0
    fi
    return 1
}

# UPX 压缩二进制
compress_binary() {
    local file=$1
    if [ "$USE_UPX" != "true" ]; then
        return
    fi
    if ! check_upx; then
        log_warn "UPX not found, skipping compression"
        return
    fi
    # macOS 二进制不支持 UPX
    if [[ "$file" == *"darwin"* ]]; then
        log_warn "Skipping UPX for macOS binary: $file"
        return
    fi
    log_info "Compressing $file with UPX..."
    upx -9 -q "$file" 2>/dev/null || log_warn "UPX compression failed for $file"
}

# 构建 Web UI
build_web() {
    log_info "Building web UI..."
    cd "$ROOT_DIR/web"
    if [ ! -d "node_modules" ]; then
        log_info "Installing npm dependencies..."
        npm install
    fi
    npm run build
    cd "$ROOT_DIR"

    # 复制到 embed 目录
    log_info "Copying dist to embed directory..."
    rm -rf "$ROOT_DIR/internal/server/app/dist"
    cp -r "$ROOT_DIR/web/dist" "$ROOT_DIR/internal/server/app/dist"

    log_info "Web UI built successfully"
}

# 构建单个二进制
build_binary() {
    local os=$1
    local arch=$2
    local component=$3  # server 或 client

    local output_name="${component}"
    if [ "$os" = "windows" ]; then
        output_name="${component}.exe"
    fi

    local output_dir="$BUILD_DIR/${os}_${arch}"
    mkdir -p "$output_dir"

    log_info "Building $component for $os/$arch..."

    GOOS=$os GOARCH=$arch go build \
        -ldflags "-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME' -X 'main.GitCommit=$GIT_COMMIT'" \
        -o "$output_dir/$output_name" \
        "$ROOT_DIR/cmd/$component"

    # UPX 压缩
    compress_binary "$output_dir/$output_name"
}

# 构建所有平台
build_all() {
    local platforms="${1:-$DEFAULT_PLATFORMS}"

    for platform in $platforms; do
        local os="${platform%/*}"
        local arch="${platform#*/}"
        build_binary "$os" "$arch" "server"
        build_binary "$os" "$arch" "client"
    done
}

# 仅构建当前平台
build_current() {
    local os=$(go env GOOS)
    local arch=$(go env GOARCH)

    build_binary "$os" "$arch" "server"
    build_binary "$os" "$arch" "client"

    log_info "Binaries built in $BUILD_DIR/${os}_${arch}/"
}

# 清理构建产物
clean() {
    log_info "Cleaning build directory..."
    rm -rf "$BUILD_DIR"
    log_info "Clean completed"
}

# 显示帮助
show_help() {
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  all       Build for all platforms (default: $DEFAULT_PLATFORMS)"
    echo "  current   Build for current platform only"
    echo "  web       Build web UI only"
    echo "  server    Build server for current platform"
    echo "  client    Build client for current platform"
    echo "  clean     Clean build directory"
    echo "  help      Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  VERSION   Set version string (default: dev)"
    echo "  USE_UPX   Enable UPX compression (default: true)"
    echo ""
    echo "Examples:"
    echo "  $0 current              # Build for current platform"
    echo "  $0 all                  # Build for all platforms"
    echo "  VERSION=1.0.0 $0 all    # Build with version"
}

# 主函数
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
            build_binary "$(go env GOOS)" "$(go env GOARCH)" "server"
            ;;
        client)
            build_binary "$(go env GOOS)" "$(go env GOARCH)" "client"
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
