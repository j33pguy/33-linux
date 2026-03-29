# Cloud Backend

## Role

The cloud server is the sync target, not the compute platform:
- Stores encrypted chunks (can't decrypt them)
- Tracks file version manifests
- Manages device enrollment and fleet policies
- Handles subscription/billing

## Deployment

| Option | Cost | Features |
|--------|------|----------|
| Self-hosted | Free | Full functionality, your hardware |
| Subscription | $5-30/mo | Hosted, managed, supported |

## API Overview

```
POST /api/v1/auth/enroll        — Device enrollment
POST /api/v1/auth/challenge     — YubiKey challenge for JWT
PUT  /api/v1/chunks/{hash}      — Upload encrypted chunk
GET  /api/v1/chunks/{hash}      — Download encrypted chunk
PUT  /api/v1/manifests/{path}   — Upload file manifest
GET  /api/v1/manifests/{path}   — Get current manifest
GET  /api/v1/manifests/{path}/versions — List all versions
POST /api/v1/manifests/{path}/rollback — Rollback to version
GET  /api/v1/devices/{id}/policy      — Get device policy
```

## Storage

Filesystem-based blob store:
```
/data/users/{uid}/blobs/{hash}.enc
/data/users/{uid}/manifests/{path_hash}/v{n}.json
```

The server stores opaque encrypted blobs. Even a full server compromise yields nothing useful.

See the full [cloud backend document](https://github.com/j33pguy/33-linux/blob/main/docs/cloud-backend.md) for transport security, storage estimation, and subscription tier details.
