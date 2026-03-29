# 33-Linux Project Context

> Last updated: 2026-03-29

## What Is 33-Linux?

A hardened, immutable Linux distribution designed for people who shouldn't have to think about security. Thick-client architecture — everything runs locally on ARM SBCs (Raspberry Pi 5+), with an optional cloud backend for encrypted sync, backup, and fleet management.

**Target audience:** Your grandmother. An Apple-like experience that's incredibly hard to hack.

**Business model:** Open source (BUSL-1.1). Self-host for free. Paid subscription for hosted cloud backend.

## Core Architecture Decisions

### Thick Client (not Thin Client)
- Everything runs locally: desktop, browser, apps, encryption
- Works fully offline — no internet dependency
- Cloud backend stores encrypted backups, configs, version history
- Changes queue locally and sync when connectivity returns

### Ring-Based Security (CPU Privilege Ring Model)
- **Ring 0:** cryptd, 33Vault — holds keys, performs crypto, only accepts Ring 1 calls
- **Ring 1:** authd, filed, netd, procsd, hwspawn — core services, isolated, cannot call each other
- **Ring 2:** gated — policy gateway, validates/rebuilds/routes all cross-service requests
- **Ring 3:** CLI, Wayland desktop, apps — user-facing, can only call Ring 2
- Each ring boundary = separate Unix socket, separate permissions, `SO_PEERCRED` verification
- `gated` is the highest-value target — must be minimal, hardened, fuzz-tested

### Content-Defined Chunking
- Files split into variable-size chunks (4-16KB) using FastCDC rolling hash
- Each chunk individually encrypted (AES-256-GCM)
- Chunks identified by hash of ciphertext (not plaintext — prevents metadata leakage)
- Only changed chunks sync to cloud — efficient delta updates
- Manifest per file version → rollback any file to any version
- Dedup within user (same content = same chunks = stored once)

### Dual Hardware Authentication
- **TPM:** Platform identity, measured boot, disk key sealing (proves "right device")
- **YubiKey Bio:** User identity, FIDO2/PIV, fingerprint (proves "right person")
- Both required — compromise one, still locked out
- Separated failure points — device and auth key are independent

### No Shell (Consumer Tier)
- No terminal, no developer tools in browser
- All system interaction via gRPC APIs (through the UI)
- Developer tier available separately with sandboxed shell
- Enterprise tier: configurable per-device policy

### Application Containerization
- Each app runs in its own LXC container
- Browser (biggest attack surface) always isolated
- Apps communicate with system only through Ring 3 → Ring 2 → Ring 1
- Wayland socket passthrough for display

### Immutable Root
- squashfs mounted read-only
- overlayfs with tmpfs upper layer for volatile writes
- Reboot = factory fresh (malware can't persist)
- OS updates: signed squashfs images, A/B partition swap with rollback

## Technology Stack

- **Language:** Go 1.25+ (stdlib + gRPC/protobuf only)
- **IPC:** gRPC over Unix sockets (local), gRPC over mTLS (cloud)
- **Encryption:** AES-256-GCM, HKDF-SHA256 for key derivation
- **Filesystem:** squashfs + overlayfs + tmpfs
- **Containers:** LXC with PID/Mount/Net/User namespaces + cgroups v2
- **Display:** Wayland (cage/labwc compositor)
- **Hardware target:** Raspberry Pi 5 (8GB), future ARM/x86 SBCs with TPM

## Inspiration

- **Talos Linux** — immutable, API-only, Go-based. We're device-focused instead of K8s-focused.
- **Qubes OS** — compartmentalization pioneer. We make it grandma-accessible.
- **ChromeOS** — security for everyone. We remove the Google dependency and add hardware auth.
- **Sidero Metal** (deprecated → Omni) — bare metal provisioning concepts for future fleet management.

## Licensing

- **BUSL-1.1** (Business Source License)
- Self-hosting: free, full features
- Commercial hosting: requires license
- Converts to Apache 2.0 after 4 years per version

## Go Conventions
- Go 1.25+, stdlib + gRPC/protobuf only (no other external deps)
- `cmd/` for binaries, `internal/` for module-specific, `proto/` for service definitions
- camelCase vars, CamelCase exported, UPPER_SNAKE constants
- No `unsafe` package
- `go fmt` always, godoc comments on exports
- Errors: always check, wrap with `fmt.Errorf("context: %w", err)`
- Concurrency: goroutines+channels preferred over mutexes
- Cross-compile: `GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w"`

## Current State (Phase 1)

### What Exists
- Go init system (PID 1) with dev mode
- gRPC dispatcher on Unix socket
- 6 modules: authd, cryptd, filed, netd (stub), procsd, hwspawn
- 6 proto service definitions
- CLI client (`33` command)
- Encrypted file queue
- Immutable root mounting code
- Makefile for build/proto/test/clean

### What's Next
- Ring 2 gateway (`gated`)
- FastCDC chunking in `filed`
- Manifest/version system
- YubiKey FIDO2 integration
- Wayland kiosk compositor
- Browser-in-LXC
- Bootable Pi image
