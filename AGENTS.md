# AGENTS.md

This file provides guidance to AI coding agents working in this repository.

## Build Commands

```bash
# Build server and client binaries
go build -o server ./cmd/server
go build -o client ./cmd/client

# Run server
./server -c server.yaml

# Run client
./client -s <server>:7000 -t <token>

# Web UI development (in web/ directory)
cd web && npm install && npm run dev
cd web && npm run build          # production build → web/dist/

# Cross-platform build
./scripts/build.sh all           # Linux/Mac
.\scripts\build.ps1              # Windows
```

## Test Commands

```bash
# Run all Go tests
go test ./...

# Run tests in a specific package
go test ./internal/server/service/
go test ./pkg/protocol/

# Run a single test by name
go test -run TestClientServiceCreateClientPersistsRules ./internal/server/service/
go test -run TestRemoteControlFrameRoundTrip ./pkg/protocol/

# Verbose output
go test -v -run TestValidateProxyRuleLimit ./internal/server/runtime/

# Count and timeout
go test -count=1 -timeout=30s ./...

# No lint command configured; use `go vet` as baseline
go vet ./...
```

## Architecture Overview

GoTunnel is an intranet penetration tool (like frp) with **server-centric configuration**. Clients receive mapping rules from the server after authentication.

### Core Design

- **Yamux Multiplexing**: Single TCP connection carries control + data channels
- **Binary Protocol**: `[Type(1 byte)][Length(4 bytes)][Payload(JSON)]` — see `pkg/protocol/message.go`
- **TLS by Default**: Auto-generated self-signed ECDSA P-256 certificates
- **Embedded Web UI**: Vue.js SPA embedded via `//go:embed`
- **JS Plugin System**: goja JavaScript runtime

### Package Structure

```
cmd/server/              # Server entry point
cmd/client/              # Client entry point
internal/core/
  ├── client/            # Client domain model
  ├── rule/              # Proxy rule domain model
  └── domain/            # Aggregate type aliases
internal/server/
  ├── runtime/           # Tunnel runtime, session management
  ├── config/            # YAML configuration loading
  ├── storage/sqlite/    # SQLite storage adapters (ClientStore interface)
  ├── app/               # Web server, SPA handler
  ├── http/
  │   ├── handler/       # REST API handlers (Swagger documented)
  │   └── dto/           # Request/response DTOs
  ├── service/           # Business logic services
  ├── bootstrap/         # Server startup orchestration
  └── plugin/            # Server-side JS plugin manager
internal/client/
  └── tunnel/            # Client tunnel logic, auto-reconnect
pkg/
  ├── protocol/          # Message types and serialization
  ├── crypto/            # TLS certificate generation
  ├── relay/             # Bidirectional data relay (32KB buffers)
  ├── auth/              # JWT authentication
  ├── utils/             # Port availability checking
  ├── version/           # Version info, GitHub Releases API
  ├── update/            # Download, extract, binary replacement
  └── observability/     # Operational event storage
web/                     # Vue 3 + TypeScript (Vite)
scripts/                 # Build scripts
```

### Data Flow

External User → Server Port → Yamux Stream → Client → Local Service

### Configuration

- Server: YAML config + SQLite database for client rules
- Client: Command-line flags only (`-s` server address, `-t` token)
- Default ports: 7000 (tunnel), 7500 (web console)

## Code Style

### Imports

- Group imports in two blocks: stdlib first, then external (separated by blank line)
- Use import aliases when the package name is ambiguous or collides:
  - `domain "github.com/gotunnel/internal/core/domain"`
  - `db "github.com/gotunnel/internal/server/storage/sqlite"`
  - `coreclient "github.com/gotunnel/internal/core/client"`
- Use blank identifier `_` imports for side-effect-only imports (e.g., Swagger docs)

### Naming

- Exported types use PascalCase; unexported use camelCase
- Test fakes are named `fake<Interface>` (e.g., `fakeClientService`, `fakeClientRepository`)
- Constructor functions are `New<Type>` (e.g., `NewClientHandler`, `NewClientService`)
- Test functions follow `Test<Unit><Behavior>` (e.g., `TestClientServiceCreateClientPersistsRules`)
- Chinese comments are used for exported documentation; English is acceptable

### Error Handling

- Return `error` as the last return value
- Use sentinel errors for service-layer business rules (e.g., `ErrClientAlreadyExists`, `ErrClientNotOnline`)
- Use `errors.Is()` for sentinel error comparison in tests
- HTTP handlers map service errors to appropriate HTTP status codes via helper functions (`InternalError`, `NotFound`, `BindJSON`)
- Prefer `t.Fatalf` with descriptive format strings in tests: `t.Fatalf("unexpected status: got %d want %d", got, want)`

### Testing

- Tests live in `_test.go` files in the same package (white-box testing)
- Use standard `testing` package — no external test frameworks
- Define fakes as unexported structs implementing the required interfaces inline
- Use `net.Pipe()` for simulating network connections
- Use `httptest.NewRecorder()` and `gin.CreateTestContext()` for HTTP handler tests
- Set `gin.SetMode(gin.TestMode)` before handler tests
- Table-driven tests are welcome but not required

### General Conventions

- Prefer interfaces for dependencies to enable testability
- Lock/unlock mutexes explicitly (no `defer` for short critical sections is acceptable)
- Use `//go:embed` directives for static assets
- Swagger annotations on HTTP handlers: `// @Summary`, `// @Router`, etc.
- Validate and set defaults in config methods (see `ApplyRuntimeConfig`)
