# Cloud Backend

## Overview

The 33-Linux cloud backend serves as the sync target, backup store, and fleet management platform. It is intentionally untrusted — all data arrives pre-encrypted from the client, and the server stores opaque blobs it cannot decrypt.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Cloud Server                       │
│                   (Go monolith)                      │
├──────────┬──────────┬──────────┬────────────────────┤
│  Auth    │  Sync    │ Version  │  Fleet             │
│  Service │  Service │ Service  │  Service           │
├──────────┴──────────┴──────────┴────────────────────┤
│                 Blob Store                           │
│           /data/users/{uid}/blobs/{hash}.enc         │
├─────────────────────────────────────────────────────┤
│                  Database                            │
│        Users, manifests, devices, policies           │
└─────────────────────────────────────────────────────┘
```

## Services

### Auth Service

Handles device enrollment and authentication.

**Enrollment Flow:**
1. Client generates keypair (YubiKey PIV slot)
2. Client sends public key + TPM attestation to server
3. Server verifies attestation, stores public key
4. Server issues signed device certificate
5. All subsequent requests use mTLS with this certificate

**Authentication:**
- mTLS on every request (client cert from YubiKey)
- JWT tokens for session management (short-lived, 15 min)
- Refresh via YubiKey challenge (hardware proof of presence)

### Sync Service

Receives and serves encrypted chunks.

**Upload:**
```
POST /api/v1/chunks/{sha256_hash}
Content-Type: application/octet-stream
Authorization: Bearer <jwt>

<encrypted chunk bytes>
```

- Server stores blob at `/data/users/{uid}/blobs/{hash}.enc`
- Dedup within a user: if hash exists, skip (idempotent)
- No cross-user dedup (different encryption keys = different hashes)
- Returns `201 Created` or `200 OK` (already exists)

**Download:**
```
GET /api/v1/chunks/{sha256_hash}
Authorization: Bearer <jwt>
```

- Returns encrypted blob
- 404 if chunk doesn't exist for this user

**Manifest Sync:**
```
PUT /api/v1/manifests/{path_hash}
Content-Type: application/json
Authorization: Bearer <jwt>

{
  "path": "<encrypted path>",
  "version": 42,
  "chunks": ["sha256_a", "sha256_b", ...],
  "created": "2026-03-29T09:50:00Z",
  "previous_version": 41
}
```

### Version Service

Manages file version history and rollback.

**List versions:**
```
GET /api/v1/manifests/{path_hash}/versions
Authorization: Bearer <jwt>
```

Returns all manifest versions for a file path. Each version references a set of chunks.

**Rollback:**
```
POST /api/v1/manifests/{path_hash}/rollback
Content-Type: application/json
Authorization: Bearer <jwt>

{ "target_version": 38 }
```

Creates a new manifest version that points to the chunks from version 38.

**Retention Policy:**
- Default: keep all versions for 30 days, then keep weekly snapshots for 1 year
- Configurable per subscription tier
- Garbage collection: chunks unreferenced by any manifest after retention period are purged

### Fleet Service

Manages devices and policies for multi-device and enterprise deployments.

**Device Registration:**
- Each device gets a unique ID (from TPM endorsement key)
- Devices are grouped by user, family, or organization
- Policies define per-device capabilities (app allowlist, network rules, etc.)

**Policy Distribution:**
```
GET /api/v1/devices/{device_id}/policy
Authorization: Bearer <jwt>
```

Returns the current policy document (signed by server). Client verifies signature before applying.

**Subscription Management:**
- Free tier: self-hosted, no server interaction needed
- Paid tiers: storage quotas, device limits, support level
- Subscription status checked on sync; grace period for lapsed accounts

## Deployment

### Self-Hosted

```bash
# Docker Compose (planned)
docker compose up -d

# Or build from source
go build -o 33-server ./cmd/server
./33-server --config /etc/33linux/server.yaml
```

Requirements:
- Go 1.25+ (build)
- TLS certificate (Let's Encrypt or self-signed for internal use)
- Storage: filesystem-based blob store (start with local disk, scale to S3-compatible)
- Database: SQLite (single-node) or PostgreSQL (scale)

### Hosted (Subscription)

Managed by the 33-Linux team. Users create an account, register devices, and sync automatically.

## Storage Model

### Blob Store Layout

```
/data/
  users/
    {user_id}/
      blobs/
        {sha256_hash_1}.enc
        {sha256_hash_2}.enc
        ...
      manifests/
        {path_hash_1}/
          v1.json
          v2.json
          ...
        {path_hash_2}/
          v1.json
          ...
      meta/
        devices.json
        policy.json
```

### Storage Estimation

Assuming average chunk size of 8KB:

| User Activity | Chunks/Day | Storage/Day | Storage/Month |
|--------------|------------|-------------|---------------|
| Light (email, browsing) | ~100 | ~800KB | ~24MB |
| Medium (photos, documents) | ~1,000 | ~8MB | ~240MB |
| Heavy (video, large files) | ~10,000 | ~80MB | ~2.4GB |

Deduplication reduces actual storage by 30-60% for typical usage.

### Garbage Collection

Periodic job (daily):
1. Scan all manifests for all users
2. Build set of referenced chunk hashes
3. Any blob not in the set AND older than retention period → delete
4. Log deleted chunks for audit

## Transport Security

### TLS Configuration

```yaml
tls:
  min_version: "1.3"
  cipher_suites:
    - TLS_AES_256_GCM_SHA384
    - TLS_CHACHA20_POLY1305_SHA256
  client_auth: "require_and_verify"
  client_ca: "/etc/33linux/ca.pem"
```

### Certificate Pinning

Clients embed the server's public key hash at build time:

```go
const serverPinSHA256 = "sha256/base64encodedpublickeyhash=="
```

On connection, the client verifies the server's certificate chain AND the leaf certificate's public key hash matches the pin. This prevents MITM even if a CA is compromised.

For self-hosted deployments, the pin is set during initial device enrollment.

## Subscription Tiers (Planned)

| Tier | Price | Users | Storage | Devices | Support |
|------|-------|-------|---------|---------|---------|
| Self-Hosted | Free | Unlimited | Your hardware | Unlimited | Community |
| Personal | $5/mo | 1 | 50GB | 3 | Email |
| Family | $15/mo | 5 | 500GB | 15 | Email |
| Enterprise | $30/mo/user | Unlimited | 1TB/user | Unlimited | Priority |

Self-hosters get the full feature set. Paid tiers fund development and provide hosted infrastructure.
