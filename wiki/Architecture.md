# Architecture

## Ring Model

33-Linux uses a CPU privilege ring-inspired architecture for service isolation:

```
Ring 3: CLI / Desktop / Apps (user-facing)
  ↓ gRPC, user token
Ring 2: gated (policy gateway — validate, rebuild, route)
  ↓ gRPC, service token, SO_PEERCRED
Ring 1: authd, filed, netd, procsd, hwspawn (isolated services)
  ↓ gRPC, Ring 1 service cert
Ring 0: cryptd, 33Vault (hardware-bound keys)
```

### Rules
- Ring 3 → Ring 2 only (never Ring 1 or Ring 0)
- Ring 2 deserializes, validates, rebuilds requests from scratch
- Ring 1 services cannot call each other (must go through Ring 2)
- Ring 1 → Ring 0 for crypto operations
- Ring 0 only accepts calls from Ring 1

### Why?
Compromising any single service gives you nothing. An attacker who owns `netd` can't access `filed`'s cache, `authd`'s sessions, or `cryptd`'s keys. They're stuck in a ring with no lateral movement.

## Boot Sequence

```
UEFI Secure Boot → Signed kernel + initrd
  → Go initd (PID 1)
    → Mount squashfs RO → overlayfs (tmpfs upper)
    → TPM unseal master key
    → YubiKey challenge-response
    → Start Ring 0 (cryptd, 33Vault)
    → Start Ring 2 (gated)
    → Start Ring 1 (authd, filed, netd, procsd, hwspawn)
    → Start Ring 3 (Wayland compositor, desktop)
```

## Filesystem

```
/              ← overlayfs (merged view)
├── /ro-root/  ← squashfs (immutable, dm-verity verified)
├── /tmp/upper ← tmpfs (volatile, lost on reboot)
└── /var/
    ├── cache/33linux/   ← plaintext file cache
    └── sync-queue/      ← encrypted chunks awaiting sync
```

## Module Reference

| Module | Ring | Responsibility |
|--------|------|---------------|
| `cryptd` | 0 | AES-256-GCM encryption/decryption |
| `33Vault` | 0 | Password/credential management |
| `authd` | 1 | Session management, key derivation |
| `filed` | 1 | File chunking, caching, sync queue |
| `netd` | 1 | Network management, cloud sync |
| `procsd` | 1 | Process/LXC container lifecycle |
| `hwspawn` | 1 | Hardware detection, device containers |
| `gated` | 2 | Policy enforcement, request routing |

See the full [architecture document](https://github.com/j33pguy/33-linux/blob/main/docs/architecture.md) for data flow diagrams, concurrency model, and build system details.
