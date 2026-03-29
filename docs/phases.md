# Roadmap

> 20 phases. Each phase = one session, one branch, one PR.
> Tracking issue: [#2](https://github.com/j33pguy/33-linux/issues/2)

---

## Phase 1: Documentation & README ✅

**Goal:** Document everything we've designed before writing more code.

- [x] 1.1 — Rewrite README with full project vision
- [x] 1.2 — Update LICENSE from MIT to BUSL-1.1
- [x] 1.3 — Write `docs/architecture.md` (ring model, boot sequence, data flow, modules)
- [x] 1.4 — Write `docs/security-model.md` (threat model, TPM+YubiKey, kernel hardening)
- [x] 1.5 — Write `docs/chunking-engine.md` (FastCDC, encryption pipeline, dedup, sync)
- [x] 1.6 — Write `docs/cloud-backend.md` (server arch, API, blob store, subscriptions)
- [x] 1.7 — Write `docs/roadmap.md` (5-phase high-level plan)
- [x] 1.8 — Write `docs/development.md` (build, dev mode, conventions, testing)
- [x] 1.9 — Populate GitHub wiki (7 pages + sidebar navigation)
- [x] 1.10 — Write `docs/phases.md` (detailed 20-phase breakdown)
- [x] 1.11 — Create master tracking issue (#2)
- [x] 1.12 — Update .gitignore (exclude bin/, grok-export/)
- [x] 1.13 — Remove CONTEXT.md (superseded by docs/)

---

## Phase 2: `gated` Scaffold — Ring 2 Gateway Proto & Skeleton

**Goal:** Define the API surface for the policy gateway. No logic — just the shape.

`gated` is the most critical service. Every request between rings flows through it. It validates, rebuilds, and routes. This phase defines *what* it does. Phase 3 implements *how*.

- [ ] 2.1 — Create `proto/gate/v1/gate.proto` with service definition
  - `RouteAuth(GateAuthRequest) → GateAuthResponse` — proxy auth operations
  - `RouteFile(GateFileRequest) → GateFileResponse` — proxy file operations
  - `RouteCrypto(GateCryptoRequest) → GateCryptoResponse` — proxy crypto operations
  - `RouteProc(GateProcRequest) → GateProcResponse` — proxy process operations
  - `RouteHW(GateHWRequest) → GateHWResponse` — proxy hardware operations
  - `RouteSync(GateSyncRequest) → GateSyncResponse` — proxy sync operations
  - `Health(HealthRequest) → HealthResponse` — gateway health check
  - `AuditLog(AuditLogRequest) → AuditLogResponse` — retrieve recent audit entries
- [ ] 2.2 — Design `GateRequest` wrapper message type
  - Contains: caller identity, session token, target ring, target service, operation name
  - Contains: serialized inner request (the actual Ring 1 request payload)
  - Contains: request metadata (timestamp, request ID, client version)
- [ ] 2.3 — Design `GateResponse` wrapper message type
  - Contains: status code, serialized inner response, audit trail ID
  - Contains: error details (sanitized — never leaks Ring 1 internals to Ring 3)
- [ ] 2.4 — Design `AuditEntry` message type
  - Fields: timestamp, caller_uid, caller_pid, target_service, operation, result, latency_ms
- [ ] 2.5 — Run `make proto` to generate Go code from gate.proto
- [ ] 2.6 — Create `internal/gated/gated.go` — service struct
  - Implement `UnimplementedGateServiceServer`
  - All RPC methods return `codes.Unimplemented` with "Phase 3" message
  - Constructor takes references to Ring 1 service clients (injected by initd)
- [ ] 2.7 — Create `internal/gated/config.go` — gateway configuration
  - Policy struct: allowed operations per caller type
  - Rate limit config: requests per second per caller
  - Audit config: log level, retention
- [ ] 2.8 — Register `gated` in `cmd/initd/main.go`
  - New socket path: `/run/33linux/ring2.sock` (dev: `/tmp/33linux-ring2.sock`)
  - Start after Ring 0, before Ring 1 (Ring 1 services register with gated)
- [ ] 2.9 — Add CLI subcommand `33 gate status`
  - Connects to Ring 2 socket, calls `Health()`, reports status
- [ ] 2.10 — Add CLI subcommand `33 gate audit`
  - Connects to Ring 2 socket, calls `AuditLog()`, displays recent entries
- [ ] 2.11 — Update `docs/architecture.md` with gated proto API reference
- [ ] 2.12 — Unit tests for proto compilation and service registration

---

## Phase 3: `gated` Logic — Validation, Rebuild, Routing

**Goal:** Make `gated` actually enforce policy. Requests validated, rebuilt from scratch, routed correctly.

- [ ] 3.1 — Implement request deserialization pipeline
  - Receive `GateRequest` → unmarshal inner payload → type-check against expected proto
  - Reject any request where inner payload doesn't match declared operation
- [ ] 3.2 — Implement session token validation
  - Extract token from `GateRequest` → call `authd.ValidateSession()` internally
  - Cache valid sessions briefly (5s TTL) to reduce authd round-trips
  - Expired/invalid token → reject with `codes.Unauthenticated`
- [ ] 3.3 — Implement request validation rules
  - Per-operation validation: required fields present, values in range, paths sanitized
  - Path traversal prevention: reject any path containing `..` or absolute paths
  - Size limits: reject requests exceeding configurable max payload size
- [ ] 3.4 — Implement request rebuild
  - Construct a *new* proto message from validated fields (never forward raw bytes)
  - Strip any fields that Ring 3 shouldn't be able to set (e.g., internal flags)
  - Add internal metadata: request ID, timestamp, caller identity from SO_PEERCRED
- [ ] 3.5 — Implement routing table
  - Map: `(operation_type, target_service)` → Ring 1 gRPC client connection
  - Route map is static (compiled in or loaded from signed config at startup)
  - Unknown route → reject with `codes.NotFound`
- [ ] 3.6 — Implement response sanitization
  - Strip internal error details before returning to Ring 3
  - Map internal errors to user-friendly messages
  - Include audit trail ID in response for debugging
- [ ] 3.7 — Implement audit logging
  - Every request through gated → append to audit log (in-memory ring buffer + periodic flush)
  - Log fields: timestamp, caller UID/PID, target service, operation, result, latency
  - Failed requests logged at higher verbosity
- [ ] 3.8 — Implement rate limiting
  - Token bucket per caller UID
  - Configurable limits per operation type
  - Rate-limited requests → reject with `codes.ResourceExhausted` + Retry-After hint
- [ ] 3.9 — Implement circuit breaker for Ring 1 services
  - If a Ring 1 service fails N times in M seconds → circuit opens → fast-fail without calling
  - Auto-reset after cooldown period
  - Health check probe to detect recovery
- [ ] 3.10 — Wire up existing CLI commands to go through gated
  - `33 auth login` → gated.RouteAuth → authd.Login
  - `33 file store` → gated.RouteFile → filed.StoreFile
  - `33 crypto encrypt` → gated.RouteCrypto → cryptd.Encrypt
  - etc.
- [ ] 3.11 — Integration test: Ring 3 CLI → gated → Ring 1 service → response
- [ ] 3.12 — Integration test: invalid request rejected at gated (never reaches Ring 1)
- [ ] 3.13 — Integration test: rate limiting kicks in under load
- [ ] 3.14 — Update wiki Architecture page with gated internals

---

## Phase 4: `SO_PEERCRED` Verification — Socket-Level Access Control

**Goal:** Kernel-enforced ring boundaries. Only the right UID/GID can connect to each socket.

- [ ] 4.1 — Create `internal/peercred/peercred.go`
  - `GetPeerCred(net.Conn) → (uid, gid, pid, error)` — extract SO_PEERCRED from Unix socket
  - Works with gRPC's `net.Listener` by wrapping accepted connections
- [ ] 4.2 — Create `internal/peercred/interceptor.go`
  - gRPC `UnaryServerInterceptor` that checks SO_PEERCRED on every RPC
  - Accepts a policy: `map[ring]AllowedUIDs`
  - Unauthorized caller → reject with `codes.PermissionDenied` + log alert
- [ ] 4.3 — Create `internal/peercred/policy.go`
  - Define per-ring access policy:
    - Ring 0 socket → only Ring 1 service UIDs
    - Ring 1 sockets → only Ring 2 (gated) UID
    - Ring 2 socket → only Ring 3 UIDs (CLI, desktop apps)
  - Policy loaded from config, validated at startup
- [ ] 4.4 — Update `internal/dispatcher/dispatcher.go`
  - Accept a `peercred.Policy` at construction
  - Attach interceptor to gRPC server options
  - Log all connection attempts (accepted and rejected)
- [ ] 4.5 — Define per-ring socket paths
  - `/run/33linux/ring0.sock` — cryptd, 33Vault
  - `/run/33linux/ring1/authd.sock` — authd
  - `/run/33linux/ring1/filed.sock` — filed
  - `/run/33linux/ring1/netd.sock` — netd
  - `/run/33linux/ring1/procsd.sock` — procsd
  - `/run/33linux/ring1/hwspawn.sock` — hwspawn
  - `/run/33linux/ring2.sock` — gated
  - Dev mode equivalents under `/tmp/33linux-*`
- [ ] 4.6 — Set socket file permissions
  - `0600` on each socket file
  - Owned by the ring's service user (ring0:ring0, ring1_authd:ring2, etc.)
- [ ] 4.7 — Update initd to create sockets with correct ownership
- [ ] 4.8 — Dev mode: log SO_PEERCRED but don't enforce (single process, same UID)
- [ ] 4.9 — Unit tests: mock connections with known UIDs, verify accept/reject
- [ ] 4.10 — Integration test: CLI (Ring 3 UID) → Ring 2 socket → accepted
- [ ] 4.11 — Integration test: CLI (Ring 3 UID) → Ring 0 socket → rejected
- [ ] 4.12 — Update docs with socket permission model

---

## Phase 5: Ring Isolation in initd — Process Separation

**Goal:** Each service is its own process with its own namespaces and resource limits.

- [ ] 5.1 — Design service binary strategy
  - Option A: single binary `initd --module=authd` (smaller image, shared binary)
  - Option B: separate binaries `cmd/authd/main.go` (more isolation, independent builds)
  - Decide and document rationale
- [ ] 5.2 — Create `internal/supervisor/supervisor.go`
  - Service launcher: fork/exec child process for each module
  - Pass configuration via environment variables or config file
  - Track child PIDs
- [ ] 5.3 — Implement service supervision
  - Monitor child processes (waitpid)
  - Restart on crash with exponential backoff (1s, 2s, 4s, 8s, max 60s)
  - After N consecutive crashes → mark service as failed, log alert
- [ ] 5.4 — Implement dependency ordering
  - Start order: Ring 0 (cryptd) → Ring 2 (gated) → Ring 1 (authd, filed, netd, procsd, hwspawn)
  - Each service signals "ready" via its Health() RPC
  - Next service starts only after dependency is healthy
- [ ] 5.5 — Implement namespace isolation per service
  - PID namespace: each service can't see other processes
  - Mount namespace: each service has minimal filesystem view
  - Network namespace: Ring 0 has no network; Ring 1 per-service rules
  - User namespace: each service runs as its own UID
- [ ] 5.6 — Implement cgroup v2 limits per service
  - Memory: cryptd 64MB, authd 128MB, filed 256MB, netd 128MB, procsd 256MB, hwspawn 64MB
  - CPU: proportional shares (cryptd gets priority for crypto operations)
  - I/O: netd gets throttled to prevent network saturation
- [ ] 5.7 — Create service health monitoring
  - initd polls each service's `Health()` RPC every 10s
  - Unhealthy service → restart attempt
  - Report overall system health via initd's own health endpoint
- [ ] 5.8 — Update initd `main.go` — replace goroutine-based module loading with supervisor
- [ ] 5.9 — Dev mode: still run in single process (goroutines) for easy debugging
- [ ] 5.10 — Production mode: full process separation with namespaces
- [ ] 5.11 — Integration test: kill a Ring 1 service → supervisor restarts it
- [ ] 5.12 — Integration test: Ring 1 service exceeds cgroup memory → OOM killed → restarted
- [ ] 5.13 — Integration test: full boot sequence with process separation
- [ ] 5.14 — Benchmark: startup time with process separation vs goroutines

---

## Phase 6: FastCDC — Content-Defined Chunking Engine

**Goal:** Pure Go implementation of FastCDC. The foundation of the storage system.

- [ ] 6.1 — Create `internal/chunker/gear.go`
  - Generate gear hash lookup table (256 entries, uint64)
  - Deterministic generation from seed (reproducible across builds)
- [ ] 6.2 — Create `internal/chunker/chunker.go`
  - `type Chunker struct` with configurable min/avg/max sizes
  - Constructor: `New(minSize, avgSize, maxSize int) *Chunker`
  - Validate params: min < avg < max, all powers of 2 (or close)
- [ ] 6.3 — Implement `Chunk(io.Reader) → ([]Chunk, error)` method
  - Streaming: reads from io.Reader in buffered chunks, doesn't load entire file
  - Uses gear hash rolling over sliding window
  - Boundary detection: `(hash & mask) == 0`
- [ ] 6.4 — Implement dual-mask normalization
  - Below average size: use larger mask (harder to match → discourages small chunks)
  - Above average size: use smaller mask (easier to match → encourages splitting)
  - This reduces chunk size variance for better dedup
- [ ] 6.5 — Define `Chunk` type
  - Fields: `Offset int64`, `Length int`, `Data []byte`, `Hash []byte`
  - Hash computed during chunking (SHA-256 of plaintext, used for local dedup only)
- [ ] 6.6 — Implement `ChunkBytes([]byte) → ([]Chunk, error)` convenience method
  - Wraps `Chunk()` with a bytes.Reader for in-memory use
- [ ] 6.7 — Handle edge cases
  - Empty input → return zero chunks
  - Input smaller than minSize → single chunk (the whole thing)
  - Input exactly minSize → single chunk
  - Input with long runs of identical bytes (pathological case for rolling hash)
- [ ] 6.8 — Unit tests: deterministic chunking
  - Same input → same chunks, every time, on every platform
  - Known test vectors (hardcoded inputs with expected chunk boundaries)
- [ ] 6.9 — Unit tests: chunk size distribution
  - Generate random data → chunk it → verify average chunk size ≈ configured average
  - Verify all chunks within [min, max] range
- [ ] 6.10 — Unit tests: insertion stability
  - Chunk a file → insert 1 byte at offset N → re-chunk → verify only 1-2 chunks changed
- [ ] 6.11 — Benchmark: throughput on x86_64 (target: >500 MB/s)
- [ ] 6.12 — Benchmark: throughput on ARM64 (target: >300 MB/s)
- [ ] 6.13 — Benchmark: memory allocation profile (should be minimal, streaming)
- [ ] 6.14 — Godoc comments on all exported types and methods

---

## Phase 7: Per-Chunk Encryption Pipeline

**Goal:** Wire chunking to encryption. Each chunk individually encrypted with a derived key.

- [ ] 7.1 — Create `internal/filed/pipeline.go`
  - `type Pipeline struct` — orchestrates chunk → encrypt → store
  - Constructor takes: session key, cache dir, queue dir
- [ ] 7.2 — Implement HKDF key derivation for per-chunk keys
  - `deriveChunkKey(sessionKey []byte, chunkIndex int) → []byte`
  - Uses `HKDF-SHA256(sessionKey, "chunk:" + strconv.Itoa(index))`
  - Returns 32-byte key for AES-256-GCM
- [ ] 7.3 — Implement per-chunk encryption
  - For each chunk from FastCDC:
    - Derive key → call `cryptd.EncryptBytes(chunk.Data, key)` → get `nonce || ciphertext`
    - This is the stored blob
- [ ] 7.4 — Implement content hash (of ciphertext)
  - `blobHash = SHA-256(nonce || ciphertext)` — hash of encrypted blob, NOT plaintext
  - This is the dedup key and storage filename
- [ ] 7.5 — Implement AAD (Additional Authenticated Data)
  - AAD = `manifestID || chunkIndex || version` (as bytes)
  - Passed to GCM Seal/Open — prevents chunk reordering and cross-manifest substitution
  - Requires updating `cryptd.EncryptBytes` to accept AAD parameter
- [ ] 7.6 — Update `cryptd` to support AAD
  - Add `aad []byte` parameter to `EncryptBytes` and `DecryptBytes`
  - Update proto if AAD needs to flow over gRPC
- [ ] 7.7 — Implement blob storage
  - Write encrypted blob to `{queueDir}/{blobHash}.enc`
  - Write encrypted blob to `{cacheDir}/{blobHash}.enc` (local cache)
  - If blob already exists (same hash) → skip write (dedup)
- [ ] 7.8 — Implement pipeline output
  - Returns: list of `(chunkIndex, blobHash, blobSize)` tuples — input for manifest creation
- [ ] 7.9 — Unit test: encrypt → decrypt round-trip for single chunk
- [ ] 7.10 — Unit test: same plaintext chunk → same ciphertext (deterministic key derivation)
- [ ] 7.11 — Unit test: different chunk index → different key → different ciphertext
- [ ] 7.12 — Unit test: AAD mismatch on decrypt → authentication failure
- [ ] 7.13 — Unit test: dedup — same chunk stored twice, only one blob on disk
- [ ] 7.14 — Integration test: file → chunk → encrypt → store → load → decrypt → reassemble → compare

---

## Phase 8: Manifest System — Version Tracking

**Goal:** Track which chunks make up each file version. Enable version history and rollback.

- [ ] 8.1 — Create `internal/manifest/types.go`
  - `type Manifest struct` — path hash, version, chunk list, timestamps, checksum
  - `type ChunkRef struct` — index, blob hash, size, offset in original file
  - JSON serialization for storage
- [ ] 8.2 — Create `internal/manifest/store.go`
  - `type Store struct` — filesystem-based manifest storage
  - Layout: `{manifestDir}/{pathHash}/v{version}.json`
- [ ] 8.3 — Implement `CreateManifest(pathHash, chunks []ChunkRef) → Manifest`
  - Auto-increment version number
  - Compute checksum: `SHA-256(concatenated chunk hashes)`
  - Set timestamps, link to previous version
- [ ] 8.4 — Implement `GetManifest(pathHash, version) → Manifest`
  - Load specific version from disk
  - Latest version if version=0
- [ ] 8.5 — Implement `ListVersions(pathHash) → []ManifestSummary`
  - Scan directory, return version numbers with timestamps and sizes
- [ ] 8.6 — Implement `GetLatestVersion(pathHash) → int`
  - Quick lookup without loading full manifest
- [ ] 8.7 — Update `filed.StoreFile` to use pipeline + manifest
  - Old flow: store raw file + queue encrypted copy
  - New flow: chunk → encrypt → store blobs → create manifest → queue manifest for sync
- [ ] 8.8 — Update `filed.LoadFile` to use manifest
  - Load manifest → for each chunk ref → load blob → decrypt → reassemble in order
  - Verify checksum after reassembly
- [ ] 8.9 — Add proto RPCs to `file/v1/file.proto`
  - `ListVersions(ListVersionsRequest) → ListVersionsResponse`
  - `GetManifest(GetManifestRequest) → GetManifestResponse`
- [ ] 8.10 — Add CLI commands: `33 file versions <path>`, `33 file manifest <path> [version]`
- [ ] 8.11 — Unit tests: create manifest → load → verify identical
- [ ] 8.12 — Unit tests: multiple versions → list returns all, ordered
- [ ] 8.13 — Integration test: store file twice → two manifests, shared chunks
- [ ] 8.14 — Integration test: store → load → compare → byte-identical

---

## Phase 9: Rollback & Garbage Collection

**Goal:** Roll back any file. Clean up orphaned chunks.

- [ ] 9.1 — Implement `Rollback(pathHash, targetVersion) → Manifest`
  - Load target version's manifest
  - Create a new version (version N+1) with same chunk references
  - All chunks must still exist; error if any are missing (GC'd too early)
- [ ] 9.2 — Add proto RPC: `Rollback(RollbackRequest) → RollbackResponse`
- [ ] 9.3 — Add CLI command: `33 file rollback <path> <version>`
- [ ] 9.4 — Create `internal/gc/gc.go` — garbage collector
  - `type GarbageCollector struct` — takes manifest store and blob store paths
- [ ] 9.5 — Implement reference counting
  - Scan all manifests across all files → build set of referenced blob hashes
  - Use bloom filter for memory efficiency at scale
- [ ] 9.6 — Implement mark phase
  - Any blob NOT in referenced set AND older than retention period → mark for deletion
  - Write marked hashes to a `.gc-pending` file
- [ ] 9.7 — Implement sweep phase (runs 24h after mark)
  - Re-check marked blobs (maybe a new manifest referenced them since marking)
  - Delete blobs that are still unreferenced
  - Two-pass prevents race with in-progress syncs
- [ ] 9.8 — Implement retention policy config
  - Default: all versions kept 30 days, weekly snapshots kept 1 year
  - GC only deletes chunks from expired versions
- [ ] 9.9 — Integrate GC into `filed` as periodic goroutine
  - Runs daily (configurable interval)
  - Logs: chunks scanned, chunks marked, chunks deleted, space reclaimed
- [ ] 9.10 — Add admin CLI: `33 gc run` (manual trigger), `33 gc status` (last run stats)
- [ ] 9.11 — Unit test: rollback restores correct content
- [ ] 9.12 — Unit test: GC doesn't delete referenced chunks
- [ ] 9.13 — Unit test: GC deletes unreferenced chunks after retention
- [ ] 9.14 — Unit test: two-pass prevents race condition

---

## Phase 10: End-to-End Storage Integration

**Goal:** Full pipeline works. No gaps. Performance validated.

- [ ] 10.1 — Integration test: store 5MB photo → chunk → encrypt → manifest → load → compare bytes
- [ ] 10.2 — Integration test: store photo → edit 100 bytes in middle → store again → verify only 1-2 new chunks
- [ ] 10.3 — Integration test: store → delete → rollback → verify restored
- [ ] 10.4 — Integration test: store same file twice (different paths) → verify chunks deduplicated
- [ ] 10.5 — Integration test: concurrent stores (3 files simultaneously) → no corruption
- [ ] 10.6 — Edge case test: empty file → store → load
- [ ] 10.7 — Edge case test: 1-byte file → store → load
- [ ] 10.8 — Edge case test: 1GB file → store → load (streaming, never full in memory)
- [ ] 10.9 — Edge case test: binary file (random bytes) → store → load → compare
- [ ] 10.10 — Performance test on Pi 5: 5MB file store pipeline <50ms
- [ ] 10.11 — Performance test on Pi 5: 5MB file load pipeline <30ms
- [ ] 10.12 — Performance test: chunk + encrypt throughput >200 MB/s on Pi 5
- [ ] 10.13 — Update CLI: `33 file store` and `33 file load` use full pipeline
- [ ] 10.14 — Update all docs affected by storage changes
- [ ] 10.15 — Manual smoke test: use CLI to store/load/version/rollback real files

---

## Phase 11: YubiKey FIDO2 Integration

**Goal:** Replace passwords with YubiKey fingerprint touch.

- [ ] 11.1 — Research: evaluate `go-libfido2` vs direct CTAP2 over USB HID
  - go-libfido2: CGo wrapper, more mature, external dep (libfido2)
  - Direct USB HID: pure Go, no CGo, more work, tighter control
  - Document decision with rationale
- [ ] 11.2 — Create `internal/authd/fido2.go` — FIDO2 module
- [ ] 11.3 — Implement credential creation (enrollment)
  - Detect YubiKey on USB → generate challenge → create credential → store public key
  - Credential bound to device (relying party = device hostname or UUID)
- [ ] 11.4 — Implement credential assertion (login)
  - Generate challenge → send to YubiKey → user touches/fingerprints → verify assertion
  - On success: create session with hardware-derived token
- [ ] 11.5 — Implement credential storage
  - Public keys stored in local encrypted file (sealed by current session key)
  - Backup to cloud backend (Phase 15+)
- [ ] 11.6 — Handle multiple YubiKeys (primary + backup)
  - Enrollment accepts up to N keys (default 2)
  - Any enrolled key can authenticate
- [ ] 11.7 — Handle YubiKey removal
  - Detect removal event → auto-lock screen (Phase 18 integration)
  - Session remains valid but UI locked until re-insertion + touch
- [ ] 11.8 — Fallback: dev mode allows password auth when no YubiKey present
  - Production mode: YubiKey required, no password fallback
- [ ] 11.9 — CLI: `33 auth enroll` — interactive YubiKey enrollment wizard
- [ ] 11.10 — CLI: `33 auth login` — "Touch your security key..."
- [ ] 11.11 — CLI: `33 auth keys` — list enrolled keys
- [ ] 11.12 — Unit tests with mock FIDO2 device
- [ ] 11.13 — Integration test: enroll → login → session valid
- [ ] 11.14 — Integration test: wrong key → login rejected

---

## Phase 12: TPM2 Key Sealing

**Goal:** Bind the master key to the hardware platform. Device theft = useless device.

- [ ] 12.1 — Research: evaluate `go-tpm` library (Google's pure Go TPM2 interface)
- [ ] 12.2 — Create `internal/tpm/tpm.go` — TPM2 interface
- [ ] 12.3 — Implement PCR reading
  - Read PCR values (firmware, bootloader, kernel hashes)
  - Display current PCR state for debugging
- [ ] 12.4 — Implement key sealing
  - Generate master key → seal to PCR policy → store sealed blob on disk
  - Only unsealable if PCRs match exactly
- [ ] 12.5 — Implement key unsealing
  - Load sealed blob → TPM unseal with PCR policy → master key in memory
  - Fail if PCRs don't match (tampered boot chain)
- [ ] 12.6 — Implement PCR policy definition
  - Configurable: which PCR registers to bind to
  - Default: PCR 0 (firmware), PCR 4 (bootloader), PCR 7 (secure boot state)
- [ ] 12.7 — Implement graceful fallback (no TPM present)
  - Detect TPM absence at boot
  - Fall back to YubiKey-only key derivation
  - Log warning: "No TPM detected — reduced security mode"
- [ ] 12.8 — Implement TPM health check
  - Verify TPM is functional, not in lockout, has available key slots
- [ ] 12.9 — CLI: `33 tpm status` — show TPM info, PCR values, sealed key status
- [ ] 12.10 — Unit tests with TPM simulator (`swtpm`)
- [ ] 12.11 — Integration test: seal → unseal → key matches
- [ ] 12.12 — Integration test: seal → tamper PCR → unseal fails

---

## Phase 13: Dual-Factor Boot Flow

**Goal:** TPM + YubiKey combined. Both must be present to unlock.

- [ ] 13.1 — Design combined key derivation
  - `masterKey = HKDF(tpmSealedKey || yubikeyResponse, "master-v1")`
  - Both factors contribute entropy; neither alone is sufficient
- [ ] 13.2 — Implement boot flow state machine
  - `INIT → TPM_UNSEAL → YUBIKEY_WAIT → YUBIKEY_VERIFY → UNLOCKED → SERVICES_START`
  - Timeout at `YUBIKEY_WAIT`: 30s → show "Insert security key" on display
  - Failure at any stage → halt with error message
- [ ] 13.3 — Implement recovery mode
  - Backup YubiKey (second enrolled key)
  - Recovery codes (8 single-use alphanumeric codes, printed at enrollment)
  - `33 auth recover` CLI for recovery code entry
- [ ] 13.4 — Implement session key derivation chain
  - `masterKey → HKDF("session:" + bootID) → sessionKey`
  - `sessionKey → HKDF("file:" + pathHash) → fileKey`
  - `sessionKey → HKDF("vault:" + entryID) → vaultKey`
- [ ] 13.5 — Implement boot audit log
  - Record: boot timestamp, TPM status, YubiKey serial, auth method, result
  - Stored locally (encrypted), synced to cloud when connected
- [ ] 13.6 — Integration test: full boot → TPM unseal → YubiKey auth → master key → session key
- [ ] 13.7 — Integration test: boot without YubiKey → timeout → insert → unlock
- [ ] 13.8 — Integration test: boot with wrong YubiKey → reject
- [ ] 13.9 — Integration test: recovery code flow

---

## Phase 14: Auth System Overhaul

**Goal:** Replace dev-mode auth with production system. No more hardcoded credentials.

- [ ] 14.1 — Remove hardcoded `admin/admin` user from authd
- [ ] 14.2 — Implement first-boot enrollment wizard
  - Detect: no users enrolled → enter enrollment mode
  - Create user identity bound to YubiKey
  - Generate and display recovery codes
  - Store encrypted credential file
- [ ] 14.3 — Rewrite session tokens to derive from hardware auth
  - Token = HMAC(sessionKey, timestamp + random)
  - Tokens are short-lived (1 hour), renewable via YubiKey touch
- [ ] 14.4 — Implement multi-user support (future-proofing)
  - User registry: list of enrolled users with their public keys
  - Each user has separate encryption keys (derived from their YubiKey)
  - User switching requires YubiKey swap
- [ ] 14.5 — Implement migration path
  - Dev mode → production: guided migration wizard
  - Re-encrypt existing data with hardware-derived keys
- [ ] 14.6 — Security review of entire auth flow
  - Document all auth paths, verify no bypass exists
  - Verify session invalidation on logout/timeout/YubiKey removal
- [ ] 14.7 — Update all integration tests to use new auth system
- [ ] 14.8 — Update docs with production auth flow

---

## Phase 15: Cloud Server Scaffold

**Goal:** Build the server that receives encrypted blobs and manages devices.

- [ ] 15.1 — Create `cmd/server/main.go` — server entry point
- [ ] 15.2 — Create `internal/server/config.go` — YAML configuration
  - Listen address, TLS cert paths, storage root, database path
- [ ] 15.3 — Implement auth service — device enrollment and JWT issuance
- [ ] 15.4 — Implement blob store — filesystem-based, `/data/users/{uid}/blobs/{hash}.enc`
- [ ] 15.5 — Implement manifest store — versioned manifests, `/data/users/{uid}/manifests/`
- [ ] 15.6 — Implement REST API routes
  - `PUT /api/v1/chunks/{hash}` — upload chunk
  - `GET /api/v1/chunks/{hash}` — download chunk
  - `HEAD /api/v1/chunks/{hash}` — check existence
  - `PUT /api/v1/manifests/{pathHash}` — upload manifest
  - `GET /api/v1/manifests/{pathHash}` — get latest manifest
  - `GET /api/v1/manifests/{pathHash}/versions` — list versions
- [ ] 15.7 — Implement health endpoint and metrics
- [ ] 15.8 — Implement SQLite metadata database (users, devices, quotas)
- [ ] 15.9 — Create Dockerfile and Docker Compose for self-hosted deployment
- [ ] 15.10 — Unit tests for each service
- [ ] 15.11 — Integration test: upload chunk → download → compare
- [ ] 15.12 — Update server docs

---

## Phase 16: Sync Protocol — Client-Server Communication

**Goal:** `netd` talks to the cloud server. Chunks flow.

- [ ] 16.1 — Rewrite `netd` — replace stubs with HTTP client to cloud server
- [ ] 16.2 — Implement chunk upload: scan queue → HEAD check → PUT if new → ack → delete from queue
- [ ] 16.3 — Implement chunk download: manifest refs → check cache → GET if missing → decrypt → cache
- [ ] 16.4 — Implement manifest push/pull
- [ ] 16.5 — Implement bandwidth throttling (configurable rate limit)
- [ ] 16.6 — Implement progress reporting (for future UI: "syncing 47/312...")
- [ ] 16.7 — Implement retry logic with exponential backoff
- [ ] 16.8 — Implement authentication: mTLS + JWT for all requests
- [ ] 16.9 — Integration test: client stores file → syncs → new client pulls → identical
- [ ] 16.10 — Integration test: large file sync (100MB)
- [ ] 16.11 — Integration test: interrupted sync → resume

---

## Phase 17: Offline Queue Drain & Conflict Resolution

**Goal:** Handle the hard case: two devices modify the same file offline.

- [ ] 17.1 — Implement connectivity monitoring in `netd`
- [ ] 17.2 — Implement persistent queue (survives reboot, encrypted on disk)
- [ ] 17.3 — Implement queue drain on reconnect (oldest-first, chunks before manifests)
- [ ] 17.4 — Implement conflict detection (server 409 on version mismatch)
- [ ] 17.5 — Implement server-wins resolution (default)
- [ ] 17.6 — Implement timestamp-wins resolution (configurable)
- [ ] 17.7 — Implement conflict copies (`{file}.conflict.{timestamp}`)
- [ ] 17.8 — Implement conflict notification (for future UI)
- [ ] 17.9 — Integration test: two clients modify same file offline → reconnect → resolve
- [ ] 17.10 — Integration test: one client offline 24h with 1000 queued chunks → reconnect → drain

---

## Phase 18: Wayland Compositor — Desktop Display

**Goal:** The Pi shows a desktop. Grandma sees something on screen.

- [ ] 18.1 — Evaluate compositors: `cage` (kiosk), `labwc` (stacking), custom
- [ ] 18.2 — Set up compositor with auto-start from initd
- [ ] 18.3 — Create login screen: "Touch your security key" prompt
- [ ] 18.4 — Create desktop shell: taskbar, app launcher, status indicators
- [ ] 18.5 — Implement Wayland socket passthrough for LXC containers
- [ ] 18.6 — Implement screen lock on YubiKey removal
- [ ] 18.7 — Implement display auto-detection (resolution, multi-monitor)
- [ ] 18.8 — Integration test: boot → compositor → login → desktop → app window
- [ ] 18.9 — User testing: someone non-technical navigates the desktop

---

## Phase 19: Browser in LXC — Containerized Chromium

**Goal:** Chromium runs in its own container. Compromise the browser, get nothing.

- [ ] 19.1 — Create LXC container template for Chromium
- [ ] 19.2 — Configure Wayland socket passthrough (display only)
- [ ] 19.3 — Configure network namespace (filtered internet, no local access)
- [ ] 19.4 — Disable DevTools and extension installation (consumer tier)
- [ ] 19.5 — Implement download integration: downloads go through `filed`
- [ ] 19.6 — Implement container lifecycle: start on click, stop on close, restart on crash
- [ ] 19.7 — Implement bookmark/history sync via cloud backend
- [ ] 19.8 — Performance testing: Chromium in LXC on Pi 5 (target: usable for basic browsing)
- [ ] 19.9 — Security testing: attempt container escape
- [ ] 19.10 — User testing: grandma browses the web

---

## Phase 20: Bootable Pi Image — Build Pipeline

**Goal:** An SD card that boots a Raspberry Pi into 33-Linux.

- [ ] 20.1 — Evaluate image build tools: `mkosi`, `debootstrap`, custom script
- [ ] 20.2 — Build minimal ARM64 kernel with hardening flags
- [ ] 20.3 — Create squashfs root image (initd, services, compositor, Chromium template)
- [ ] 20.4 — Implement A/B partition layout (two roots + data partition)
- [ ] 20.5 — Configure boot (U-Boot or Pi UEFI)
- [ ] 20.6 — Implement first-boot wizard (create user, enroll YubiKey, WiFi, cloud setup)
- [ ] 20.7 — Sign image with Ed25519
- [ ] 20.8 — Create SD card flasher script (or Pi Imager integration)
- [ ] 20.9 — GitHub Actions: build image on release tag
- [ ] 20.10 — Image size target: <500MB compressed
- [ ] 20.11 — End-to-end test: flash → boot → enroll → browse → sync → reboot → everything works
- [ ] 20.12 — Write user-facing quickstart guide

---

## Summary

| Phase | Tasks | Key Deliverable |
|-------|-------|----------------|
| 1 | 13 | ✅ Docs, wiki, license |
| 2 | 12 | `gated` proto + skeleton |
| 3 | 14 | `gated` validation + routing logic |
| 4 | 12 | Socket-level access control |
| 5 | 14 | Process separation + supervision |
| 6 | 14 | FastCDC chunking engine |
| 7 | 14 | Per-chunk encryption pipeline |
| 8 | 14 | Manifest version tracking |
| 9 | 14 | Rollback + garbage collection |
| 10 | 15 | End-to-end integration tests |
| 11 | 14 | YubiKey FIDO2 auth |
| 12 | 12 | TPM2 key sealing |
| 13 | 9 | Dual-factor boot flow |
| 14 | 8 | Production auth system |
| 15 | 12 | Cloud server foundation |
| 16 | 11 | Client-server sync protocol |
| 17 | 10 | Offline queue + conflicts |
| 18 | 9 | Wayland desktop |
| 19 | 10 | Containerized Chromium |
| 20 | 12 | Bootable Pi image |
| **Total** | **~242 tasks** | **Functional secure OS** |
