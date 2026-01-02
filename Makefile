# GoTunnel Makefile

.PHONY: all build-frontend sync-frontend build-server build-client clean help

# 默认目标
all: build-frontend sync-frontend build-server build-client

# 构建前端
build-frontend:
	@echo "Building frontend..."
	cd web && npm ci && npm run build

# 同步前端到 embed 目录
sync-frontend:
	@echo "Syncing frontend to embed directory..."
ifeq ($(OS),Windows_NT)
	if exist internal\server\app\dist rmdir /s /q internal\server\app\dist
	xcopy /E /I /Y web\dist internal\server\app\dist
else
	rm -rf internal/server/app/dist
	cp -r web/dist internal/server/app/dist
endif

# 仅同步（不重新构建前端）
sync-only:
	@echo "Syncing existing frontend build..."
ifeq ($(OS),Windows_NT)
	if exist internal\server\app\dist rmdir /s /q internal\server\app\dist
	xcopy /E /I /Y web\dist internal\server\app\dist
else
	rm -rf internal/server/app/dist
	cp -r web/dist internal/server/app/dist
endif

# 构建服务端（当前平台）
build-server:
	@echo "Building server..."
	go build -ldflags="-s -w" -o gotunnel-server ./cmd/server

# 构建客户端（当前平台）
build-client:
	@echo "Building client..."
	go build -ldflags="-s -w" -o gotunnel-client ./cmd/client

# 构建 Linux ARM64 服务端
build-server-linux-arm64: sync-only
	@echo "Building server for Linux ARM64..."
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gotunnel-server-linux-arm64 ./cmd/server

# 构建 Linux AMD64 服务端
build-server-linux-amd64: sync-only
	@echo "Building server for Linux AMD64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o gotunnel-server-linux-amd64 ./cmd/server

# 完整构建（包含前端）
full-build: build-frontend sync-frontend build-server build-client

# 开发模式：快速构建（假设前端已构建）
dev-build: sync-only build-server

# 清理构建产物
clean:
	@echo "Cleaning..."
ifeq ($(OS),Windows_NT)
	if exist gotunnel-server del gotunnel-server
	if exist gotunnel-client del gotunnel-client
	if exist gotunnel-server.exe del gotunnel-server.exe
	if exist gotunnel-client.exe del gotunnel-client.exe
	if exist gotunnel-server-* del gotunnel-server-*
	if exist gotunnel-client-* del gotunnel-client-*
else
	rm -f gotunnel-server gotunnel-client gotunnel-server-* gotunnel-client-*
endif

# 帮助
help:
	@echo "Available targets:"
	@echo "  all                    - Build frontend, sync, and build binaries"
	@echo "  build-frontend         - Build frontend (npm)"
	@echo "  sync-frontend          - Sync web/dist to internal/server/app/dist"
	@echo "  sync-only              - Sync without rebuilding frontend"
	@echo "  build-server           - Build server for current platform"
	@echo "  build-client           - Build client for current platform"
	@echo "  build-server-linux-arm64 - Cross-compile server for Linux ARM64"
	@echo "  build-server-linux-amd64 - Cross-compile server for Linux AMD64"
	@echo "  full-build             - Complete build with frontend"
	@echo "  dev-build              - Quick build (assumes frontend exists)"
	@echo "  clean                  - Remove build artifacts"
