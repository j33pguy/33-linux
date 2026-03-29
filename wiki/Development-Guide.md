# Development Guide

## Quick Start

```bash
git clone https://github.com/j33pguy/33-linux.git
cd 33-linux
make build

# Run in dev mode
DEV_MODE=1 ./bin/initd

# In another terminal
DEV_MODE=1 ./bin/33 version
DEV_MODE=1 ./bin/33 auth login -u admin -p admin
DEV_MODE=1 ./bin/33 hw detect
```

## Prerequisites

- Go 1.25+
- protoc + Go plugins
- Make

## Code Conventions

- Go stdlib + gRPC/protobuf only (no other external deps)
- `go fmt` always
- Errors: always check, always wrap with context
- Goroutines + channels over mutexes
- No `unsafe` package

## Adding a Module

1. Define proto in `proto/{module}/v1/{module}.proto`
2. `make proto`
3. Implement in `internal/{module}/{module}.go`
4. Register in `cmd/initd/main.go`
5. Add CLI commands in `cmd/climain/main.go`

See the full [development guide](https://github.com/j33pguy/33-linux/blob/main/docs/development.md) for detailed conventions, testing, and cross-compilation.
