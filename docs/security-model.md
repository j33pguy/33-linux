# Security Model

## Overview

33-Linux implements a zero-trust security architecture where every component is assumed hostile until proven otherwise. Security is enforced at multiple layers: hardware, boot chain, filesystem, service isolation, and network transport.

## Principles

1. **Deny by default.** No access is granted without explicit authentication and authorization.
2. **Hardware roots of trust.** Software-only secrets are insufficient. TPM and YubiKey provide hardware-bound authentication.
3. **Compartmentalize everything.** Services can't talk to each other. Apps can't read each other's data. Devices are isolated in containers.
4. **Immutability prevents persistence.** Malware can't survive a reboot because the root filesystem is read-only and volatile state is tmpfs.
5. **Client-side encryption.** The cloud server is untrusted. All data is encrypted before leaving the device.
6. **Minimal dependencies.** Every dependency is an attack surface. Go stdlib + gRPC/protobuf only.

## Hardware Authentication

### Dual-Factor Model

Two independent hardware factors are required for system access:

```mermaid
graph LR
    subgraph "Factor 1 — Platform"
        TPM["TPM 2.0<br/>Soldered to board"]
        TPM --> T1["Measured boot chain"]
        TPM --> T2["Key sealed to PCR values"]
        TPM --> T3["Cannot be removed"]
    end

    subgraph "Factor 2 — Person"
        YK["YubiKey Bio<br/>External, removable"]
        YK --> Y1["FIDO2 / PIV credential"]
        YK --> Y2["Biometric fingerprint"]
        YK --> Y3["Physical presence required"]
    end

    TPM & YK --> UNLOCK["Both required to unlock"]

    style UNLOCK fill:#0f3460,color:#fff,stroke:#e94560
```

#### Why Both?

| Scenario | TPM Alone | YubiKey Alone | Both Required |
|----------|-----------|---------------|--------------|
| Device stolen | ❌ Attacker has keys | ✅ Blocked | ✅ Blocked |
| YubiKey stolen | ✅ Works | ❌ Attacker has access | ✅ Blocked |
| Remote exploit | ✅ Can extract sealed keys | ✅ No physical presence | ✅ Blocked |
| Evil maid (boot tamper) | ❌ PCR bypass possible | N/A | ✅ PCR + presence required |

### Recovery

Lost YubiKey recovery options (configurable per deployment):

1. **Backup YubiKey:** Register a second key during enrollment. Keep it in a safe.
2. **Recovery codes:** 8 single-use alphanumeric codes printed at enrollment.
3. **Cloud-assisted recovery:** Identity verification through the subscription service (enterprise only).
4. **Emergency break-glass:** Physical access to the server + admin credentials (self-hosted only).

If both device AND YubiKey are lost: recover from cloud backup to a new device with a backup YubiKey or recovery codes.

## Boot Security

### Secure Boot Chain

```mermaid
flowchart TD
    FW["UEFI Firmware<br/>Platform Key"] --> BL
    BL["Bootloader<br/>Signed by 33-Linux CA"] --> KRN
    KRN["Kernel + initrd<br/>Signed by 33-Linux CA"] --> SQ
    SQ["squashfs root image<br/>dm-verity hash tree"] --> PCR
    PCR["TPM: Extend PCR values<br/>at each stage"] --> UNSEAL
    UNSEAL["Unseal master key<br/>Only if PCRs match expected values"]

    style UNSEAL fill:#0f3460,color:#fff,stroke:#e94560
```

### Kernel Hardening

Compile-time security options:

| Option | Purpose |
|--------|---------|
| `CONFIG_SECURITY_YAMA=y` | ptrace restrictions |
| `CONFIG_MODULE_SIG=y` | Only signed kernel modules |
| `CONFIG_MODULE_SIG_FORCE=y` | Reject unsigned modules |
| `CONFIG_STACKPROTECTOR_STRONG=y` | Stack buffer overflow protection |
| `CONFIG_FORTIFY_SOURCE=y` | Compile-time buffer overflow detection |
| `CONFIG_STRICT_DEVMEM=y` | Restrict /dev/mem access |
| `CONFIG_IO_STRICT_DEVMEM=y` | Restrict I/O memory access |
| `CONFIG_LOCK_DOWN_KERNEL_FORCE_CONFIDENTIALITY=y` | Kernel lockdown |
| `CONFIG_INIT_ON_ALLOC_DEFAULT_ON=y` | Zero memory on allocation |
| `CONFIG_INIT_ON_FREE_DEFAULT_ON=y` | Zero memory on free |

Runtime parameters (kernel command line):

```
lockdown=confidentiality init_on_alloc=1 init_on_free=1
page_alloc.shuffle=1 slab_nomerge vsyscall=none
```

### Update Verification

OS updates are distributed as signed squashfs images:

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Update Server
    participant D as Disk

    C->>S: Check for update
    S-->>C: New image + Ed25519 signature
    C->>C: Verify signature against pinned public key
    C->>C: Verify dm-verity hash tree
    C->>D: Write to inactive partition (A/B scheme)
    C->>D: Update bootloader → point to new partition
    C->>C: Reboot
    alt Boot succeeds
        C->>C: Mark new partition as good
    else Boot fails
        C->>D: Automatic rollback to previous partition
    end
```

## Service Isolation

### Ring Enforcement

Each ring runs with progressively fewer capabilities:

| Ring | User | Capabilities | Namespaces | Network |
|------|------|-------------|------------|---------|
| Ring 0 | `ring0` | `CAP_SYS_ADMIN` (crypto only) | All isolated | None |
| Ring 1 | `ring1_{service}` | Minimal per-service | PID, Mount, Net | Per-service |
| Ring 2 | `ring2` | None (pure routing) | PID, Mount | Loopback only |
| Ring 3 | `user` | None | Full isolation | Filtered |

### Unix Socket Permissions

```mermaid
graph TD
    subgraph "Ring 3 — User Space"
        CLI["CLI / Desktop"]
    end

    subgraph "Ring 2 — Gateway"
        R2S["/run/33linux/ring2.sock<br/>0600 ring2:ring3"]
    end

    subgraph "Ring 1 — Services"
        R1A["/run/33linux/ring1/authd.sock<br/>0600 ring1_authd:ring2"]
        R1F["/run/33linux/ring1/filed.sock<br/>0600 ring1_filed:ring2"]
        R1N["/run/33linux/ring1/netd.sock<br/>0600 ring1_netd:ring2"]
    end

    subgraph "Ring 0 — Crypto"
        R0S["/run/33linux/ring0.sock<br/>0600 ring0:ring0"]
    end

    CLI -->|SO_PEERCRED verified| R2S
    R2S -->|SO_PEERCRED verified| R1A & R1F & R1N
    R1A & R1F -->|SO_PEERCRED verified| R0S

    CLI -.->|"❌ DENIED"| R0S
    CLI -.->|"❌ DENIED"| R1A
```

`SO_PEERCRED` is checked on every connection to verify the calling process's UID/GID matches the expected ring.

### Application Containers

```mermaid
graph TD
    subgraph "Per-App LXC Containers"
        B["Browser<br/>Chromium"]
        E["Email<br/>App LXC"]
        F["Files<br/>App LXC"]
    end

    subgraph "Isolation per Container"
        NS["PID ns + Mount ns + Net ns + User ns + Cgroup v2"]
    end

    B & E & F --> NS
    B & E & F -->|"Wayland socket<br/>display only"| COMP["Wayland Compositor"]

    B x--x E
    E x--x F
    B x--x F

    style B fill:#533483,color:#fff
    style E fill:#0f3460,color:#fff
    style F fill:#1a1a2e,color:#fff
```

**Isolation guarantees:**
- App A cannot see App B's processes (PID namespace)
- App A cannot read App B's files (mount namespace)
- App A cannot sniff App B's network (network namespace)
- App A has resource limits (cgroup v2 — CPU, memory, I/O)
- Apps communicate with the system only through gRPC to Ring 3 → Ring 2

## Encryption

### At Rest

All persistent data is encrypted using AES-256-GCM:

- **Chunk-level encryption:** Each file chunk is independently encrypted with a derived key
- **Key derivation:** `HKDF-SHA256(master_key, chunk_content_hash)` → per-chunk key
- **Nonce:** Random 12 bytes from `crypto/rand` per encryption operation
- **AAD (Additional Authenticated Data):** Includes chunk index and manifest version to prevent reordering attacks

### In Transit

- **Client ↔ Server:** TLS 1.3 with mutual authentication (client cert from YubiKey PIV)
- **Certificate pinning:** Client embeds server public key hash; rejects any other cert
- **No plaintext ever leaves the device.** The server receives and serves encrypted blobs.

### Key Management

```mermaid
graph TD
    HW["TPM-sealed Master Key"] --> SK["Session Key<br/>HKDF(master, 'session:' + boot_id)<br/>Runtime only, never persists"]
    HW --> FK["File Key<br/>HKDF(master, 'file:' + path_hash)<br/>Per-file chunk encryption"]
    HW --> VK["Vault Key<br/>HKDF(master, 'vault:' + entry_id)<br/>33Vault password entries"]

    style HW fill:#e94560,color:#fff,stroke:#e94560
    style SK fill:#0f3460,color:#fff
    style FK fill:#0f3460,color:#fff
    style VK fill:#0f3460,color:#fff
```

**Phase 1 limitation:** Master key is generated randomly per session (no TPM integration yet). This means data encrypted in one session can't be decrypted in the next. Phase 2 adds TPM-sealed persistent keys.

## Threat Model

### In Scope

| Threat | Attack Vector | Mitigation |
|--------|--------------|-----------|
| Physical device theft | Boot from USB, read disk | TPM-sealed keys + YubiKey required; encrypted storage |
| Evil maid | Modify bootloader/kernel | Secure Boot; PCR measurements detect tampering |
| Malware/ransomware | Exploit in browser/app | App containerization; immutable root; reboot recovers |
| Lateral movement | Compromised app → other apps | Per-app LXC; ring isolation; no inter-service comms |
| Network eavesdropping | MITM on cloud sync | mTLS + cert pinning; all data pre-encrypted |
| Cloud server breach | Attacker accesses server DB | Client-side encryption; server stores opaque blobs |
| Credential phishing | Social engineering for passwords | No passwords — YubiKey biometric only |
| Supply chain | Malicious dependency | Minimal deps (stdlib + gRPC); signed builds |
| Privilege escalation | Kernel exploit from userspace | Hardened kernel; YAMA; signed modules only |

### Out of Scope (Phase 1)

- Side-channel attacks (timing, power analysis) — future hardening
- Post-quantum cryptography — planned for Phase 4
- Advanced persistent threats with physical lab access
- Compromised hardware supply chain (e.g., implanted TPM)

## Comparison

| Feature | 33-Linux | ChromeOS | Qubes OS | Standard Linux |
|---------|----------|----------|----------|---------------|
| Immutable root | ✅ | ✅ | ❌ | ❌ |
| App containerization | ✅ (LXC) | Partial (Crostini) | ✅ (Xen VMs) | ❌ |
| Hardware auth required | ✅ (TPM+YubiKey) | ❌ | ❌ | ❌ |
| Client-side encryption | ✅ | ❌ (Google has keys) | ✅ | Optional |
| Offline-capable | ✅ | Limited | ✅ | ✅ |
| Self-hostable backend | ✅ | ❌ | N/A | N/A |
| Grandma-friendly | ✅ (goal) | ✅ | ❌ | ❌ |
| Zero-trust services | ✅ (ring model) | Partial | ❌ | ❌ |
