# Development Phases

> 20 phases, each a single focused session. One branch, one PR, one merge.
> Tracking issue: [#2](https://github.com/j33pguy/33-linux/issues/2)

---

## Core Infrastructure (Phases 1-5)

### Phase 1: Documentation & README ✅
**Status:** Complete (PR #1)

Comprehensive documentation covering architecture, security model, chunking engine, cloud backend, roadmap, and development guide. License updated from MIT to BUSL-1.1. GitHub wiki populated.

**Deliverables:**
- README.md rewrite
- 6 docs/ files (architecture, security, chunking, cloud, roadmap, development)
- GitHub wiki (7 pages + sidebar)
- BUSL-1.1 license

---

### Phase 2: `gated` Scaffold — Ring 2 Gateway Proto & Skeleton
**Status:** Planned

`gated` is the central policy gateway — the most critical service in the entire system. Every request between rings passes through it. This phase creates the protobuf definitions and service skeleton without any logic.

**What gets built:**
- `proto/gate/v1/gate.proto` — service definition with RPCs for routing requests to each Ring 1 service
- `internal/gated/gated.go` — service struct implementing the generated interface, all methods return "not implemented" stubs
- Registration in `cmd/initd/main.go`
- CLI subcommand `33 gate status` for health check

**Why this matters:**
`gated` is where the ring model becomes real. Without it, Ring 3 talks directly to Ring 1 — no policy enforcement, no request validation, no audit logging. Every security guarantee in the architecture depends on this service.

**Key design decisions:**
- `gated` does NOT proxy raw bytes. It deserializes every incoming request, validates it, and constructs a *new* request to forward. This prevents malformed/malicious requests from reaching Ring 1.
- `gated` is the thinnest possible service — minimal code surface area because it's the highest-value attack target.

---

### Phase 3: `gated` Logic — Validation, Rebuild, Routing
**Status:** Planned

Wire up the actual logic inside `gated`. This is where requests get validated, rebuilt, and routed to the correct Ring 1 service.

**What gets built:**
- Request validation layer — check session token, verify caller permissions, enforce rate limits
- Request deserialization → validation → rebuild pipeline (no passthrough of raw request bytes)
- Routing table — maps Ring 3 request types to Ring 1 service endpoints
- Audit logging — every request through `gated` is logged with caller, target, timestamp, result
- Error handling — `gated` never exposes internal errors to Ring 3, returns sanitized error messages

**Why this matters:**
This is the application-layer firewall. A request from Ring 3 saying "give me /etc/shadow" gets rejected here, not in `filed`. A malformed protobuf that would crash `authd` gets caught here, not in `authd`.

**Key design decisions:**
- Policy is compiled in or loaded from a signed config file (not dynamic/runtime-configurable from Ring 3)
- Failed validation = deny + log, never fail-open
- Request rebuilding means `gated` imports the proto types for every Ring 1 service

---

### Phase 4: `SO_PEERCRED` Verification — Per-Ring Socket Permissions
**Status:** Planned

Enforce that only the correct caller can connect to each ring's Unix socket. This is what makes the ring boundaries real at the OS level, not just convention.

**What gets built:**
- `internal/peercred/peercred.go` — helper to extract `SO_PEERCRED` (UID/GID/PID) from Unix socket connections
- gRPC interceptor that verifies caller identity on every connection
- Per-ring socket paths: `/run/33linux/ring0.sock`, `/run/33linux/ring1/*.sock`, `/run/33linux/ring2.sock`
- Socket ownership and permissions: `0600`, owned by the ring's service user
- Connection rejection with logging for unauthorized callers

**Why this matters:**
Without this, any process on the system can connect to any service socket. A compromised Ring 3 app could bypass `gated` and talk directly to `cryptd`. `SO_PEERCRED` verification ensures the kernel enforces the ring boundaries — not just our code.

**Key design decisions:**
- Verification happens at connection time (gRPC interceptor), not per-request
- In dev mode, verification is logged but not enforced (single user, single process)
- In production, unauthorized connections are immediately closed + alert generated

---

### Phase 5: Ring Isolation in initd — Separate Processes
**Status:** Planned

Currently all modules run as goroutines in a single initd process. This phase separates them into distinct processes with their own namespaces, each listening on their own Unix socket.

**What gets built:**
- Module launcher in initd — forks each service as a separate process
- Per-service namespace configuration (PID, mount, net at minimum)
- Per-service cgroup v2 limits (memory, CPU)
- Service supervision — restart crashed services, with backoff
- Dependency ordering — Ring 0 starts first, then Ring 2, then Ring 1 (Ring 1 needs gated to be ready)
- Health check protocol — each service exposes a `Health()` RPC, initd polls periodically

**Why this matters:**
Running everything in one process means a crash in `hwspawn` takes down `cryptd`. A memory leak in `netd` starves `authd`. Process separation + cgroups prevents blast radius from spreading.

**Key design decisions:**
- initd remains PID 1 and acts as supervisor (like systemd, but simpler)
- Each service binary is the same `initd` binary with a `--module=<name>` flag (single binary, multi-mode)
- OR each service gets its own binary under `cmd/` (more isolation, more build targets) — decide during implementation

---

## Chunking & Storage (Phases 6-10)

### Phase 6: FastCDC Implementation — Content-Defined Chunking
**Status:** Planned

Implement the FastCDC algorithm in pure Go. This is the engine that splits files into variable-size chunks based on content, enabling efficient delta sync and deduplication.

**What gets built:**
- `internal/chunker/chunker.go` — FastCDC implementation with gear hash
- Configurable parameters: min chunk (2KB), avg chunk (8KB), max chunk (32KB)
- Normalization (dual-mask) to reduce chunk size variance
- `Chunk(io.Reader) → []Chunk` — streaming interface, doesn't load full file into memory
- Unit tests with known test vectors to verify deterministic chunking
- Benchmark tests (target: >300 MB/s on ARM64)

**Why this matters:**
This is the foundation of the entire storage system. If chunking is wrong, dedup breaks, sync is inefficient, and version control wastes space. Getting this right and tested before building on top of it is critical.

**Key design decisions:**
- Pure Go, no CGo, no external deps
- Streaming (io.Reader) so we can chunk files larger than available RAM
- Gear hash (faster than Rabin fingerprint, similar dedup ratio)
- Deterministic: same input always produces same chunks (required for dedup)

---

### Phase 7: Per-Chunk Encryption Pipeline
**Status:** Planned

Wire chunking to encryption. Each chunk gets its own derived key and is independently encrypted using AES-256-GCM.

**What gets built:**
- `internal/filed/encrypt.go` — chunk encryption pipeline
- Key derivation: `HKDF-SHA256(session_key, "chunk:" + index)` → per-chunk key
- Nonce generation: 12 bytes from `crypto/rand` per chunk
- AAD (Additional Authenticated Data): manifest ID + chunk index + version (prevents reordering)
- Output format: `nonce || ciphertext` (self-contained blob)
- Content hash: `SHA-256(encrypted_blob)` — hash of ciphertext, not plaintext
- Integration with `cryptd` service for key operations

**Why this matters:**
Individual chunk encryption means compromising one chunk doesn't reveal others. The AAD binding prevents an attacker from reordering chunks (swapping chunk 3 and chunk 7 to corrupt a file). Hashing ciphertext (not plaintext) prevents the server from learning anything about file contents.

---

### Phase 8: Manifest System — Version Tracking
**Status:** Planned

Implement the manifest system that tracks which chunks make up each version of each file.

**What gets built:**
- `internal/manifest/manifest.go` — manifest CRUD operations
- Manifest schema: path hash, version number, ordered chunk list, timestamps, checksum
- Local manifest store (filesystem-based, under cache dir)
- `filed` integration: `StoreFile` now chunks → encrypts → creates manifest
- `LoadFile` reassembles file from manifest → chunk lookup → decrypt
- `ListVersions(path)` — show all versions of a file
- Proto updates for `filed` service: `ListVersions`, `GetManifest`

**Why this matters:**
Manifests are what make version control work. Without them, chunks are just random encrypted blobs with no way to reconstruct files or track history.

---

### Phase 9: Rollback & Garbage Collection
**Status:** Planned

Add the ability to roll back to any previous file version, and clean up chunks that are no longer referenced.

**What gets built:**
- `Rollback(path, version)` — creates a new manifest version pointing to old chunks
- Garbage collector: scans all manifests → builds referenced chunk set → deletes orphaned chunks
- Retention policy: configurable (default: keep all versions 30 days, weekly snapshots 1 year)
- GC runs as a periodic goroutine in `filed` (like the existing queue monitor)
- Proto updates: `Rollback` RPC, `GarbageCollect` RPC (admin only)
- Two-pass deletion: mark → wait 24h → delete (prevents race with in-progress syncs)

**Why this matters:**
Rollback is THE killer feature for grandma. "I accidentally deleted my photos" → rollback. "Something encrypted all my files" → rollback. Without GC, storage grows unbounded as old chunks accumulate.

---

### Phase 10: End-to-End Storage Integration
**Status:** Planned

Wire everything together: file in → chunk → encrypt → manifest → queue. Verify the full pipeline works.

**What gets built:**
- Integration tests: store file → verify chunks created → verify manifest → load file → verify identical
- Integration tests: store file → modify → store again → verify only changed chunks are new
- Integration tests: rollback → verify old content restored
- CLI updates: `33 file versions <path>`, `33 file rollback <path> <version>`
- Performance testing on Raspberry Pi 5 (target: 5MB photo stored in <50ms)
- Edge cases: empty files, huge files (>1GB), binary vs text, concurrent writes

**Why this matters:**
Individual components can all pass tests but fail when integrated. This phase catches integration bugs before we build cloud sync on top of a broken storage layer.

---

## Auth & Hardware (Phases 11-14)

### Phase 11: YubiKey FIDO2 Integration
**Status:** Planned

Replace the hardcoded username/password auth with YubiKey FIDO2 hardware authentication.

**What gets built:**
- `internal/authd/fido2.go` — FIDO2 credential creation and assertion
- Enrollment flow: insert YubiKey → create credential → bind to device
- Login flow: insert YubiKey → touch/fingerprint → challenge-response → session token
- Credential storage: local encrypted store (sealed by session key)
- Fallback: if no YubiKey detected, allow password login in dev mode only
- CLI: `33 auth enroll` (initial setup), `33 auth login` (touch to auth)

**Why this matters:**
This replaces passwords entirely for the consumer tier. Grandma doesn't type a password — she touches her fingerprint on the YubiKey. Phishing becomes impossible because FIDO2 is origin-bound and requires physical presence.

**Dependencies:** Need to evaluate `go-libfido2` vs direct `ctap2` over USB HID in Go stdlib.

---

### Phase 12: TPM2 Key Sealing
**Status:** Planned

Integrate TPM 2.0 for sealing the disk encryption master key to the platform's boot state.

**What gets built:**
- `internal/tpm/tpm.go` — TPM2 interface (seal, unseal, PCR read)
- Key sealing: bind master key to PCR values (firmware + bootloader + kernel hash)
- Key unsealing: only succeeds if PCRs match expected values (no boot tampering)
- Graceful fallback: if no TPM present, derive master key from YubiKey alone (less secure but functional)
- PCR policy: which registers to bind to (configurable for dev/prod)

**Why this matters:**
TPM is what makes device theft useless. Even with physical access to the storage, the master key can't be extracted without the exact same boot chain on the exact same TPM chip.

**Note:** Raspberry Pi 5 doesn't have a TPM. This phase targets x86 SBCs or Pi + Zymkey module. The graceful fallback ensures Pi 5 still works.

---

### Phase 13: Dual-Factor Boot Flow
**Status:** Planned

Combine TPM + YubiKey into the unified boot authentication flow.

**What gets built:**
- Boot flow: TPM unseal (automatic, silent) → YubiKey prompt (touch fingerprint) → master key derived from both
- Key derivation: `HKDF(tpm_sealed_key || yubikey_response, "master")` → combined master key
- Session key derivation from master key
- Timeout handling: YubiKey not present within 30s → show "Insert security key" screen
- Recovery mode: backup codes or secondary YubiKey
- Audit log: boot events recorded locally (who authenticated, when, which key)

**Why this matters:**
This is where the dual-factor model becomes real. Without this, TPM and YubiKey are independent — this phase makes them co-dependent, so both must be present simultaneously.

---

### Phase 14: Auth System Overhaul
**Status:** Planned

Replace the Phase 1 in-memory user store with a proper credential system backed by hardware auth.

**What gets built:**
- Remove hardcoded admin/admin user
- User enrollment: first-boot wizard creates user identity bound to YubiKey
- Session management overhaul: tokens derived from hardware auth, not random
- Key derivation chain: hardware root → master → session → per-file keys
- Multi-user support (future-proofing): each user has their own YubiKey enrollment
- Migration path from dev-mode auth to production auth

**Why this matters:**
The entire security model depends on the auth system. Everything before this was scaffolding. This phase makes auth production-real.

---

## Cloud Backend (Phases 15-17)

### Phase 15: Cloud Server Scaffold
**Status:** Planned

Build the cloud backend server that receives encrypted chunks and manages device fleet.

**What gets built:**
- `cmd/server/main.go` — server entry point
- Auth service: device enrollment, JWT issuance, mTLS setup
- Blob store: filesystem-based, `/data/users/{uid}/blobs/{hash}.enc`
- Manifest store: filesystem-based, versioned manifests per file path
- Health endpoint, metrics endpoint
- Configuration: YAML config for storage path, TLS certs, listen address
- Docker Compose for easy self-hosted deployment

**Why this matters:**
The server is what turns a single device into an ecosystem. Without it, the Pi is a standalone secure computer. With it, multiple devices sync, files are backed up, and recovery from hardware failure is possible.

**Key design decisions:**
- Go monolith (same repo, same binary, different mode)
- Filesystem blob store first (simple, portable), S3-compatible later
- SQLite for metadata (users, devices, manifests), PostgreSQL option later
- The server NEVER decrypts anything — it stores opaque blobs

---

### Phase 16: Sync Protocol — Chunk Upload/Download
**Status:** Planned

Implement the client-server sync protocol in `netd`.

**What gets built:**
- `netd` rewrite: replace stubs with real HTTP/gRPC client to cloud server
- Chunk upload: scan queue → HEAD check (exists?) → PUT if new → ack → remove from queue
- Chunk download: manifest references chunk → check local cache → GET if missing → decrypt → cache
- Manifest sync: push local manifests → pull remote manifests → merge
- Bandwidth throttling (configurable, don't saturate grandma's DSL)
- Progress reporting (for UI: "syncing 47/312 chunks...")
- Retry logic with exponential backoff

**Why this matters:**
This is the network layer that connects the Pi to the cloud. It must handle unreliable connections (grandma's WiFi drops mid-sync), large files, and concurrent syncs from multiple devices gracefully.

---

### Phase 17: Offline Queue Drain & Conflict Resolution
**Status:** Planned

Handle the hard cases: what happens when two devices modify the same file offline?

**What gets built:**
- Offline detection: `netd` monitors connectivity, queues all writes when offline
- Queue persistence: queue survives reboots (written to encrypted local storage, not just tmpfs)
- Queue drain on reconnect: oldest-first, prioritize manifests after chunks
- Conflict detection: server returns 409 when manifest version doesn't match expected
- Conflict resolution strategies: server-wins (default), timestamp-wins, manual
- Conflict copies: losing version saved as `{filename}.conflict.{timestamp}`
- Notification: UI alert when conflicts occur

**Why this matters:**
Offline-first means conflicts WILL happen. Two family members edit a shared document on different devices without internet. When they reconnect, the system must handle this gracefully without data loss. "Server wins + conflict copy" is safe but not ideal — this phase builds the foundation for smarter merge strategies later.

---

## Desktop & Image (Phases 18-20)

### Phase 18: Wayland Compositor — Kiosk Mode
**Status:** Planned

Set up the display layer. The Pi needs to show a desktop.

**What gets built:**
- Wayland compositor selection and integration: `cage` (single-app kiosk) or `labwc` (multi-window)
- Display configuration: resolution auto-detect, multi-monitor support
- Login screen: shows "Touch your security key" prompt
- Desktop shell: taskbar, app launcher, status indicators (online/offline, sync progress)
- Window management: app windows from LXC containers via Wayland socket passthrough
- Screen lock: YubiKey removal = auto-lock, touch to unlock

**Why this matters:**
This is what grandma sees. Everything else is invisible infrastructure. The desktop must be simple, responsive, and impossible to break.

**Key design decisions:**
- Start with `cage` (simplest, single fullscreen app) for the "grandma demo"
- Graduate to `labwc` or custom compositor for multi-window desktop
- GTK4 or custom for shell components (taskbar, launcher)

---

### Phase 19: Browser in LXC — Containerized Chromium
**Status:** Planned

The browser is the primary app and the biggest attack surface. It gets its own container.

**What gets built:**
- LXC container template for Chromium
- Wayland socket passthrough: container's Chromium renders to host compositor
- DevTools disabled (consumer tier), no extensions by default
- Network namespace: browser has filtered internet access, can't reach local services
- Filesystem isolation: browser can only access its own profile directory
- Download integration: downloads go through `filed` (chunked, encrypted, synced)
- Bookmark/history sync through the cloud backend
- Container lifecycle: start on click, stop when closed, restart on crash

**Why this matters:**
Browsers are where 90% of attacks happen: drive-by downloads, malicious JavaScript, phishing. Containerizing Chromium means even if the browser is fully compromised, the attacker is trapped in a container with no access to the rest of the system, no access to files, and no access to crypto keys.

---

### Phase 20: Bootable Pi Image — Build Pipeline
**Status:** Planned

Build the actual image that goes on an SD card and boots a Raspberry Pi into 33-Linux.

**What gets built:**
- Image build pipeline (likely using `mkosi`, `debootstrap`, or custom script)
- Custom minimal kernel for ARM64 (Pi 5) with hardening flags
- squashfs root image containing: initd, all services, Wayland compositor, Chromium container template
- A/B partition layout: two root partitions for safe updates + rollback
- Boot configuration: U-Boot or UEFI for Pi 5
- First-boot wizard: create user, enroll YubiKey, connect to WiFi, optional cloud setup
- SD card flasher script (or integration with Raspberry Pi Imager)
- Image signing with Ed25519
- CI/CD: GitHub Actions builds the image on every release tag

**Why this matters:**
This is the product. Everything before this was components. This phase assembles them into something you can hand to grandma and say "plug it in."

**Image size target:** <500MB (squashfs compressed). Should fit on a 2GB SD card with room for the sync queue.

---

## Summary

| Phase | Name | Key Deliverable |
|-------|------|----------------|
| 1 | Docs & README | ✅ Documentation, wiki, BUSL-1.1 |
| 2 | `gated` scaffold | Proto definitions, service skeleton |
| 3 | `gated` logic | Validation, rebuild, routing |
| 4 | `SO_PEERCRED` | Unix socket caller verification |
| 5 | Ring isolation | Separate processes, namespaces, cgroups |
| 6 | FastCDC | Content-defined chunking engine |
| 7 | Chunk encryption | Per-chunk AES-256-GCM pipeline |
| 8 | Manifests | File version tracking system |
| 9 | Rollback & GC | Version rollback, chunk garbage collection |
| 10 | Integration | End-to-end storage pipeline tests |
| 11 | YubiKey FIDO2 | Hardware authentication |
| 12 | TPM2 sealing | Platform-bound key management |
| 13 | Dual-factor boot | Combined TPM + YubiKey auth flow |
| 14 | Auth overhaul | Production credential system |
| 15 | Server scaffold | Cloud backend foundation |
| 16 | Sync protocol | Chunk upload/download, manifest sync |
| 17 | Offline & conflicts | Queue drain, conflict resolution |
| 18 | Wayland compositor | Desktop display layer |
| 19 | Browser in LXC | Containerized Chromium |
| 20 | Pi image | Bootable ARM64 image pipeline |

**Pace:** 2-3 phases per day max. One branch, one PR, one merge per phase.
**Timeline:** ~6-8 weeks for all 20 phases.
