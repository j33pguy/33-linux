# Security Model

## Dual Hardware Authentication

Two independent hardware factors required:

### TPM 2.0 (Platform Identity)
- Measured boot: PCR values reflect exact boot chain
- Key sealing: master key only unseals on correct device + correct software
- Anti-cloning: endorsement key unique per chip

### YubiKey Bio (User Identity)
- FIDO2/WebAuthn for authentication challenges
- PIV certificate for mTLS with cloud backend
- Fingerprint verification — no passwords
- Physical presence required for sensitive operations

### Why Both?
| Scenario | TPM Alone | YubiKey Alone | Both |
|----------|-----------|---------------|------|
| Device stolen | ❌ | ✅ Blocked | ✅ |
| YubiKey stolen | ✅ | ❌ | ✅ |
| Remote exploit | Keys extractable | No presence | ✅ |

## Key Hierarchy

```
Hardware Root (TPM-sealed + YubiKey challenge)
  └─ Master Key (memory only, never persisted)
       ├─ Session Key = HKDF(master, "session:" + boot_id)
       ├─ File Key = HKDF(master, "file:" + path_hash)
       └─ Vault Key = HKDF(master, "vault:" + entry_id)
```

## Threat Model

| Threat | Mitigation |
|--------|-----------|
| Physical theft | TPM + YubiKey dual-factor; encrypted storage |
| Malware persistence | Immutable root; tmpfs volatile state |
| Lateral movement | Per-app LXC; ring isolation |
| Network MITM | mTLS + cert pinning |
| Cloud breach | Client-side encryption only |
| Credential phishing | No passwords — biometric YubiKey |
| Supply chain | Minimal deps; signed builds |

## App Containerization

Each application runs in its own LXC container:
- Separate PID, mount, network, user namespaces
- Cgroup v2 resource limits
- Wayland socket passthrough (display only)
- No inter-app filesystem access

## Kernel Hardening

Key compile-time options:
- `CONFIG_MODULE_SIG_FORCE=y` — reject unsigned modules
- `CONFIG_SECURITY_YAMA=y` — ptrace restrictions
- `CONFIG_LOCK_DOWN_KERNEL_FORCE_CONFIDENTIALITY=y`
- `CONFIG_INIT_ON_ALLOC_DEFAULT_ON=y` — zero memory on alloc/free

See the full [security model document](https://github.com/j33pguy/33-linux/blob/main/docs/security-model.md) for complete details.
