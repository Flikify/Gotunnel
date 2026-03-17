# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Build server and client binaries
go build -o server ./cmd/server
go build -o client ./cmd/client

# Run server (zero-config, auto-generates token and TLS cert)
./server
./server -c server.yaml  # with config file

# Run client
./client -s <server>:7000 -t <token> -id <client-id>
./client -s <server>:7000 -t <token> -id <client-id> -no-tls  # disable TLS

# Web UI development (in web/ directory)
cd web && npm install && npm run dev    # development server
cd web && npm run build                  # production build (outputs to web/dist/)

# Cross-platform build (Windows PowerShell)
.\scripts\build.ps1

# Cross-platform build (Linux/Mac)
./scripts/build.sh all
```

## Architecture Overview

GoTunnel is an intranet penetration tool (similar to frp) with **server-centric configuration** - clients require zero configuration and receive mapping rules from the server after authentication.

### Core Design

- **Yamux Multiplexing**: Single TCP connection carries both control (auth, config, heartbeat) and data channels
- **Binary Protocol**: `[Type(1 byte)][Length(4 bytes)][Payload(JSON)]` - see `pkg/protocol/message.go`
- **TLS by Default**: Auto-generated self-signed ECDSA P-256 certificates, no manual setup required
- **Embedded Web UI**: Vue.js SPA embedded in server binary via `//go:embed`
- **JS Plugin System**: Extensible plugin system using goja JavaScript runtime

### Package Structure

```
cmd/server/          # Server entry point
cmd/client/          # Client entry point
internal/server/
  ├── tunnel/        # Core tunnel server, client session management
  ├── config/        # YAML configuration loading
  ├── db/            # SQLite storage (ClientStore, JSPluginStore interfaces)
  ├── app/           # Web server, SPA handler
  ├── router/        # REST API endpoints (Swagger documented)
  └── plugin/        # Server-side JS plugin manager
internal/client/
  └── tunnel/        # Client tunnel logic, auto-reconnect, plugin execution
pkg/
  ├── protocol/      # Message types and serialization
  ├── crypto/        # TLS certificate generation
  ├── relay/         # Bidirectional data relay (32KB buffers)
  ├── auth/          # JWT authentication
  ├── utils/         # Port availability checking
  ├── version/       # Version info and update checking (Gitea API)
  └── update/        # Shared update logic (download, extract tar.gz/zip)
web/                 # Vue 3 + TypeScript frontend (Vite + naive-ui)
scripts/             # Build scripts (build.sh, build.ps1)
```

### Key Interfaces

- `ClientStore` (internal/server/db/): Database abstraction for client rules storage
- `ServerInterface` (internal/server/router/handler/): API handler interface

### Proxy Types

1. **TCP** (default): Direct port forwarding (remote_port → local_ip:local_port)
2. **UDP**: UDP port forwarding
3. **HTTP**: HTTP proxy through client network
4. **HTTPS**: HTTPS proxy through client network
5. **SOCKS5**: SOCKS5 proxy through client network

### Data Flow

External User → Server Port → Yamux Stream → Client → Local Service

### Configuration

- Server: YAML config + SQLite database for client rules and JS plugins
- Client: Command-line flags only (server address, token, client ID)
- Default ports: 7000 (tunnel), 7500 (web console)

## API Documentation

The server provides Swagger-documented REST APIs at `/api/`.

### Key Endpoints

- `POST /api/auth/login` - JWT authentication
- `GET /api/clients` - List all clients
- `GET /api/client/{id}` - Get client details
- `PUT /api/client/{id}` - Update client config
- `POST /api/client/{id}/push` - Push config to online client
- `POST /api/client/{id}/plugin/{name}/{action}` - Plugin actions (start/stop/restart/delete)
- `GET /api/plugins` - List registered plugins
- `GET /api/update/check/server` - Check server updates
- `POST /api/update/apply/server` - Apply server update

## Update System

Both server and client support self-update from Gitea releases.

- Release assets are compressed archives (`.tar.gz` for Linux/Mac, `.zip` for Windows)
- The `pkg/update/` package handles download, extraction, and binary replacement
- Updates can be triggered from the Web UI at `/update` page
