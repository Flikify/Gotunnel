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
  ├── router/        # REST API endpoints
  └── plugin/        # Server-side plugin manager
internal/client/
  ├── tunnel/        # Client tunnel logic, auto-reconnect
  └── plugin/        # Client-side plugin manager and cache
pkg/
  ├── protocol/      # Message types and serialization
  ├── crypto/        # TLS certificate generation
  ├── proxy/         # Legacy proxy implementations
  ├── relay/         # Bidirectional data relay (32KB buffers)
  ├── utils/         # Port availability checking
  └── plugin/        # Plugin system core
      ├── types.go       # ProxyHandler interface, PluginMetadata
      ├── registry.go    # Plugin registry
      ├── builtin/       # Built-in plugins (socks5, http)
      ├── wasm/          # WASM runtime (wazero)
      └── store/         # Plugin persistence (SQLite)
web/                 # Vue 3 + TypeScript frontend (Vite)
```

### Key Interfaces

- `ClientStore` (internal/server/db/): Database abstraction for client rules storage
- `ServerInterface` (internal/server/router/): API handler interface
- `ProxyHandler` (pkg/plugin/): Plugin interface for proxy handlers
- `PluginStore` (pkg/plugin/store/): Plugin persistence interface

### Proxy Types

**内置类型** (直接在 tunnel 中处理):
1. **TCP** (default): Direct port forwarding (remote_port → local_ip:local_port)
2. **UDP**: UDP port forwarding
3. **HTTP**: HTTP proxy through client network
4. **HTTPS**: HTTPS proxy through client network

**插件类型** (通过 plugin 系统提供):
- **SOCKS5**: Full SOCKS5 protocol (official plugin)

### Data Flow

External User → Server Port → Yamux Stream → Client → Local Service

### Configuration

- Server: YAML config + SQLite database for client rules
- Client: Command-line flags only (server address, token, client ID)
- Default ports: 7000 (tunnel), 7500 (web console)

## Plugin System

GoTunnel supports a WASM-based plugin system for extensible proxy handlers.

### Plugin Architecture

- **内置类型**: tcp, udp, http, https 直接在 tunnel 代码中处理
- **Official Plugin**: SOCKS5 作为官方 plugin 提供
- **WASM Plugins**: 自定义 plugins 可通过 wazero 运行时动态加载
- **Hybrid Distribution**: 内置 plugins 离线可用；WASM plugins 可从服务端下载

### ProxyHandler Interface

```go
type ProxyHandler interface {
    Metadata() PluginMetadata
    Init(config map[string]string) error
    HandleConn(conn net.Conn, dialer Dialer) error
    Close() error
}
```

### Creating a Built-in Plugin

See `pkg/plugin/builtin/socks5.go` as reference implementation.
