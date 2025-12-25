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
```

## Architecture Overview

GoTunnel is an intranet penetration tool (similar to frp) with **server-centric configuration** - clients require zero configuration and receive mapping rules from the server after authentication.

### Core Design

- **Yamux Multiplexing**: Single TCP connection carries both control (auth, config, heartbeat) and data channels
- **Binary Protocol**: `[Type(1 byte)][Length(4 bytes)][Payload(JSON)]` - see `pkg/protocol/message.go`
- **TLS by Default**: Auto-generated self-signed ECDSA P-256 certificates, no manual setup required
- **Embedded Web UI**: Vue.js SPA embedded in server binary via `//go:embed`

### Package Structure

```
cmd/server/          # Server entry point
cmd/client/          # Client entry point
internal/server/
  ├── tunnel/        # Core tunnel server, client session management
  ├── config/        # YAML configuration loading
  ├── db/            # SQLite storage (ClientStore interface)
  ├── app/           # Web server, SPA handler
  └── router/        # REST API endpoints
internal/client/
  └── tunnel/        # Client tunnel logic, auto-reconnect
pkg/
  ├── protocol/      # Message types and serialization
  ├── crypto/        # TLS certificate generation
  ├── proxy/         # SOCKS5 and HTTP proxy implementations
  ├── relay/         # Bidirectional data relay (32KB buffers)
  └── utils/         # Port availability checking
web/                 # Vue 3 + TypeScript frontend (Vite)
```

### Key Interfaces

- `ClientStore` (internal/server/db/): Database abstraction for client rules storage
- `ServerInterface` (internal/server/router/): API handler interface

### Proxy Types

1. **TCP** (default): Direct port forwarding (remote_port → local_ip:local_port)
2. **SOCKS5**: Full SOCKS5 protocol via `TunnelDialer`
3. **HTTP**: HTTP/HTTPS proxy through client network

### Data Flow

External User → Server Port → Yamux Stream → Client → Local Service

### Configuration

- Server: YAML config + SQLite database for client rules
- Client: Command-line flags only (server address, token, client ID)
- Default ports: 7000 (tunnel), 7500 (web console)
