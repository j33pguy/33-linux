# Development Guide

## Prerequisites

- **Go 1.25+** — [install](https://go.dev/dl/)
- **protoc** — Protocol Buffers compiler
- **protoc-gen-go** and **protoc-gen-go-grpc** — Go plugins for protoc
- **Make** — build orchestration

### Install Protobuf Tools

```bash
# Install protoc (Fedora)
sudo dnf install protobuf-compiler

# Install protoc (Ubuntu/Debian)
sudo apt install protobuf-compiler

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Ensure Go bin is in PATH
export PATH="$PATH:$(go env GOPATH)/bin"
```

## Building

```bash
# Clone
git clone https://github.com/j33pguy/33-linux.git
cd 33-linux

# Generate protobuf code + build binaries
make build

# Outputs:
#   bin/initd    — init system (PID 1)
#   bin/33       — CLI client
```

### Individual Targets

```bash
make proto    # Regenerate .pb.go files from .proto definitions
make build    # Proto + compile Go binaries
make test     # Run all tests
make clean    # Remove bin/ and generated .pb.go files
```

### Cross-Compilation

```bash
# Raspberry Pi 5 (ARM64)
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/initd-arm64 ./cmd/initd
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/33-arm64 ./cmd/climain

# x86_64 (default on most dev machines)
make build
```

## Running in Dev Mode

Dev mode skips real filesystem mounts and uses local directories. Safe to run on any Linux machine.

### Start the Init System

```bash
DEV_MODE=1 ./bin/initd
```

Expected output:
```
  ╔══════════════════════════════════╗
  ║         33-Linux v0.1.0          ║
  ║     Secure. Immutable. Yours.    ║
  ╚══════════════════════════════════╝

initd: all modules registered, starting gRPC server on /tmp/33linux-rpc.sock
```

### Use the CLI

In another terminal:

```bash
# Version check
DEV_MODE=1 ./bin/33 version

# Login (default dev credentials)
DEV_MODE=1 ./bin/33 auth login -u admin -p admin

# Detect hardware
DEV_MODE=1 ./bin/33 hw detect

# Store a file
echo "hello world" > /tmp/test.txt
DEV_MODE=1 ./bin/33 file store -path /tmp/test.txt

# Encrypt/decrypt
KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
DEV_MODE=1 ./bin/33 crypto encrypt -key $KEY -data "secret message"
DEV_MODE=1 ./bin/33 crypto decrypt -key $KEY -ct <ciphertext_hex> -nonce <nonce_hex>

# Trigger sync (Phase 1: stub)
DEV_MODE=1 ./bin/33 sync
```

### Dev Mode Paths

| Path | Purpose |
|------|---------|
| `/tmp/33linux-rpc.sock` | gRPC Unix socket |
| `/tmp/33linux-cache/` | Local file cache |
| `/tmp/33linux-queue/` | Sync queue (encrypted files) |

## Project Structure

```
33-linux/
├── cmd/
│   ├── initd/main.go           # Init system entry point
│   └── climain/main.go         # CLI client entry point
├── internal/
│   ├── authd/authd.go          # Authentication module
│   ├── cryptd/cryptd.go        # Encryption module
│   ├── dispatcher/dispatcher.go # gRPC server wrapper
│   ├── filed/filed.go          # File proxy module
│   ├── hwspawn/hwspawn.go      # Hardware detection module
│   ├── mount/mount.go          # Filesystem mount operations
│   ├── netd/netd.go            # Network/sync module (stub)
│   └── procsd/procsd.go        # Process/container spawner
├── proto/
│   ├── auth/v1/auth.proto      # Auth service definition
│   ├── crypto/v1/crypto.proto  # Crypto service definition
│   ├── file/v1/file.proto      # File service definition
│   ├── hw/v1/hw.proto          # Hardware service definition
│   ├── net/v1/net.proto        # Network service definition
│   └── proc/v1/proc.proto      # Process service definition
├── docs/                       # Documentation
├── Makefile                    # Build targets
├── go.mod                      # Go module definition
└── LICENSE                     # BUSL-1.1
```

## Code Conventions

### Go Style

- **Go 1.25+** features allowed
- **stdlib + gRPC/protobuf only** — no other external dependencies
- `go fmt` on every file (enforced by CI)
- godoc comments on all exported types and functions
- Errors: always check, always wrap with context

```go
// Good
if err := doThing(); err != nil {
    return fmt.Errorf("context for what failed: %w", err)
}

// Bad
doThing() // ignoring error
```

### Naming

- `camelCase` for unexported variables and functions
- `CamelCase` for exported types and functions
- `UPPER_SNAKE` for constants
- Package names: short, lowercase, no underscores

### Concurrency

- Goroutines + channels preferred over mutexes
- `sync.RWMutex` where channels are impractical (session stores, process tables)
- Always pass `context.Context` for cancellation
- Never use `unsafe` package

### Protobuf

- One `.proto` file per service
- Package naming: `{service}.v1`
- Go package: `github.com/j33pguy/33-linux/proto/{service}/v1`
- All fields use `snake_case` (proto convention)
- Generated code goes next to `.proto` files

## Adding a New Module

1. **Define the proto:**
   ```bash
   mkdir -p proto/mymodule/v1
   ```
   Create `proto/mymodule/v1/mymodule.proto` with service definition.

2. **Generate Go code:**
   ```bash
   make proto
   ```

3. **Implement the service:**
   ```bash
   mkdir -p internal/mymodule
   ```
   Create `internal/mymodule/mymodule.go` implementing the generated server interface.

4. **Register in initd:**
   Edit `cmd/initd/main.go` to import and register the new service with the dispatcher.

5. **Add CLI commands:**
   Edit `cmd/climain/main.go` to add the new subcommand and RPC calls.

6. **Test:**
   ```bash
   make test
   ```

## Testing

```bash
# Run all tests
make test

# Run tests for a specific package
go test ./internal/cryptd/...

# Run with verbose output
go test -v ./internal/filed/...

# Run with race detector
go test -race ./...
```

### Writing Tests

- Test files: `*_test.go` next to implementation
- Table-driven tests preferred
- Test both success and error paths
- For gRPC services, use `bufconn` for in-memory transport

```go
func TestEncryptDecrypt(t *testing.T) {
    svc := cryptd.NewService()
    ctx := context.Background()

    // Test encrypt
    resp, err := svc.Encrypt(ctx, &pb.EncryptRequest{
        Plaintext: []byte("hello"),
        Key:       make([]byte, 32), // test key
    })
    if err != nil {
        t.Fatalf("encrypt: %v", err)
    }

    // Test decrypt
    dec, err := svc.Decrypt(ctx, &pb.DecryptRequest{
        Ciphertext: resp.Ciphertext,
        Key:        make([]byte, 32),
        Nonce:      resp.Nonce,
    })
    if err != nil {
        t.Fatalf("decrypt: %v", err)
    }

    if string(dec.Plaintext) != "hello" {
        t.Errorf("got %q, want %q", dec.Plaintext, "hello")
    }
}
```
