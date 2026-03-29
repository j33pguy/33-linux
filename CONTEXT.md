# 33-Linux Project Context

> Extracted from Grok conversation export, March 29, 2026

## What Is 33-Linux?

A custom, secure, immutable Linux distribution designed as a zero-trust thin client for cloud-backed operations. Inspired by Qubes OS, Tails, and Sidero Labs (Talos). Desktop/embedded/Pi/phone target.

## Core Principles

- **Immutable root** — squashfs mounted RO, overlayfs+tmpfs for volatile writes
- **Go-only userspace** — stdlib + gRPC/protobuf (sole external deps), everything else written by us
- **Zero-trust** — every RPC call authenticated, least privilege, fail-safe deny
- **Cloud as source of truth** — local is cache, data syncs to cloud backend
- **Compartmentalization** — LXC containers per module/device, namespace/cgroup isolation
- **Encryption everywhere** — AES-GCM, hardware-bound key hierarchy (YubiKey/TPM)

## Architecture

### Boot Sequence
```
Kernel (hardened) → Go initd (PID1) → Mount squashfs RO → Overlayfs (tmpfs upper) →
RPC Dispatcher (Unix socket) → Modules: authd → cryptd → filed → netd → procsd → hw-spawner →
CLI (climain) ready
```

### Module Dependency Graph
```
initd (root) → authd → cryptd → filed → netd
               ├→ procsd ← hw-spawner
               └→ 33Vault
```

### Core Modules
| Module | Purpose | Key RPCs |
|--------|---------|----------|
| authd | Auth, sessions, key derivation | Login, DeriveKey |
| cryptd | AES-GCM encrypt/decrypt | Encrypt, Decrypt |
| filed | File proxy, cache, sync queue | StoreFile, LoadFile |
| netd | Network, cloud sync, API calls | SyncQueue, APIGet |
| procsd | Process/LXC spawner | SpawnProc, SpawnLXC |
| hw-spawner | Hardware detect + auth + container spawn | DetectDevices, AuthDevice |
| climain | CLI frontend | Maps `33 <cmd>` to RPCs |
| 33-prov | Provisioning (new/existing) | Wipe/enroll/migrate |
| 33-discover | Network discovery + tunnels | Scan, tunnel establishment |
| 33Vault | Password manager | Store/retrieve/rotate creds |

### RPC Design
- gRPC over Unix sockets (local module-to-module) and TCP/TLS (client-to-cloud)
- Protobuf service definitions per module
- Auth tokens/metadata required in every request
- Only external deps: `google.golang.org/grpc` + `google.golang.org/protobuf`

### Data Flow
```
App write → filed.StoreFile → cryptd.Encrypt → queue in /var/sync-queue (encrypted tmpfs) →
netd drains queue → HTTPS POST to cloud → cloud acks → remove from queue
```

### Cloud Backend
- Go monolith (stdlib net/http)
- Auth: JWT from YubiKey/TPM challenge
- Storage: directory-based blob store (`/users/{id}/blobs/{uuid}.enc`)
- TLS 1.3 mutual auth, cert pinning
- Subscription enforcement on sync

## Security Model

### Threat Model
- Local: physical access, malware, evil maid
- Remote: MITM, API exploits, cloud compromise
- Supply chain: mitigated by zero deps + signed builds
- Insider/user errors

### Key Hierarchy
```
Master Key (hardware-bound: TPM/YubiKey)
  → Session Keys (ephemeral per boot, crypto/rand)
    → Per-File Keys (AES-256, derived from session + path hash)
```

### Isolation
- Per-module LXC with PID/mount/net/user namespaces
- Cgroups v2 for resource limits
- Capabilities dropped via syscall
- Unix socket perms (0600)

### On Compromise
- Module isolated, can't access others
- Data ephemeral (tmpfs), encrypted
- Recovery: reboot from immutable root, reprovision from cloud

### Boot Security
- UEFI Secure Boot: CA → Intermediate → signed kernel/initrd
- Kernel: CONFIG_SECURITY_YAMA, CONFIG_MODULE_SIG, stack protector
- Updates: signed squashfs images, verified before apply

## Go Conventions
- Go 1.25+, stdlib + gRPC/protobuf only (no other external deps)
- `cmd/` for binaries, `pkg/` for shared, `internal/` for module-specific
- camelCase vars, CamelCase exported, UPPER_SNAKE constants
- No `unsafe` package
- `go fmt` always, godoc comments on exports
- Errors: always check, wrap with `fmt.Errorf("context: %w", err)`
- Concurrency: goroutines+channels preferred over mutexes
- Cross-compile: `GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w"`

## Phases

### Phase 1 — Core Boot & Local Immutable System (MVP)
- Bootable ISO (squashfs + overlayfs)
- Go PID1, RPC dispatcher, all core modules
- CLI (`climain`)
- Encrypted write queue (local only)
- Hardened kernel, cross-build amd64+arm64
- **Target**: Q4 2025 – Q1 2026

### Phase 2 — Cloud Integration & Thin Client
- Full netd with sync, offline queuing, conflict detection
- Cloud backend (auth, sync, blob store)
- 33-prov provisioning, 33-discover
- 33Vault password manager
- Secure Boot chain, signed updates
- Subscription/licensing
- **Target**: Q2–Q3 2026

### Phase 3 — Usable Desktop & Ecosystem
- App marketplace (signed squashfs layers in LXC)
- Multi-device sync & roaming
- Advanced conflict resolution
- Wayland compositor / graphical login
- Self-hosted cloud option
- Enterprise MDM stubs
- **Target**: Late 2026 – mid 2027

### Phase 4+ — Hardening, Scale, Commercialization
- Formal verification, external audit
- Mobile variant, post-quantum crypto
- Marketplace revenue, enterprise licensing

## Licensing
- BUSL-1.1 (Business Source License)
- Self-hosted: free. Hosted cloud: paid subscription.
- Branding: "33-Linux" / "Project 33"

## Known Limitations / Open Items
- Stdlib crypto only — no scrypt/argon2, manual PBKDF2 with sha256
- Offline queue bounded by tmpfs RAM
- No multi-user yet (single session)
- LXC integration requires os/exec (no pure-Go container runtime)
- No formal security audit yet
