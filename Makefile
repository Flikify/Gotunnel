# GoTunnel Makefile

.PHONY: all build-frontend sync-frontend sync-only build-server build-client build-all-platforms build-current-platform build-android clean help

all: build-frontend sync-frontend build-current-platform

build-frontend:
	@echo "Building frontend..."
	cd web && npm ci && npm run build

sync-frontend:
	@echo "Syncing frontend to embed directory..."
ifeq ($(OS),Windows_NT)
	if exist internal\server\app\dist rmdir /s /q internal\server\app\dist
	xcopy /E /I /Y web\dist internal\server\app\dist
else
	rm -rf internal/server/app/dist
	cp -r web/dist internal/server/app/dist
endif

sync-only:
	@echo "Syncing existing frontend build..."
ifeq ($(OS),Windows_NT)
	if exist internal\server\app\dist rmdir /s /q internal\server\app\dist
	xcopy /E /I /Y web\dist internal\server\app\dist
else
	rm -rf internal/server/app/dist
	cp -r web/dist internal/server/app/dist
endif

build-server:
	@echo "Building server for current platform..."
	go build -buildvcs=false -trimpath -ldflags="-s -w" -o gotunnel-server ./cmd/server

build-client:
	@echo "Building client for current platform..."
	go build -buildvcs=false -trimpath -ldflags="-s -w" -o gotunnel-client ./cmd/client

build-current-platform:
	@echo "Building current platform binaries..."
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File scripts/build.ps1 current
else
	./scripts/build.sh current
endif

build-all-platforms:
	@echo "Building all desktop platform binaries..."
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File scripts/build.ps1 all -NoUPX
else
	./scripts/build.sh all
endif

build-android:
	@echo "Android build placeholder..."
ifeq ($(OS),Windows_NT)
	powershell -ExecutionPolicy Bypass -File scripts/build.ps1 android
else
	./scripts/build.sh android
endif

clean:
	@echo "Cleaning..."
ifeq ($(OS),Windows_NT)
	if exist gotunnel-server del gotunnel-server
	if exist gotunnel-client del gotunnel-client
	if exist gotunnel-server.exe del gotunnel-server.exe
	if exist gotunnel-client.exe del gotunnel-client.exe
	if exist gotunnel-server-* del gotunnel-server-*
	if exist gotunnel-client-* del gotunnel-client-*
	if exist build rmdir /s /q build
else
	rm -f gotunnel-server gotunnel-client gotunnel-server-* gotunnel-client-*
	rm -rf build
endif

help:
	@echo "Available targets:"
	@echo "  all                    - Build frontend, sync, and current platform binaries"
	@echo "  build-frontend         - Build frontend (npm)"
	@echo "  sync-frontend          - Sync web/dist to internal/server/app/dist"
	@echo "  sync-only              - Sync without rebuilding frontend"
	@echo "  build-server           - Build server for current platform"
	@echo "  build-client           - Build client for current platform"
	@echo "  build-current-platform - Build server/client into build/<os>_<arch>/"
	@echo "  build-all-platforms    - Build Windows/Linux/macOS server/client binaries"
	@echo "  build-android          - Android build placeholder"
	@echo "  clean                  - Remove build artifacts"
