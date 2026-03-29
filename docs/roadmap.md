# Roadmap

## Vision

33-Linux aims to be the most secure consumer operating system — one that your grandmother can use without thinking about security, and that enterprises can trust with their most sensitive data.

## Phase 1: Core Boot & Local System (Current)

**Goal:** A bootable system with working init, module communication, and encrypted local storage.

### Completed
- [x] Go-based init system running as PID 1
- [x] Immutable root filesystem (squashfs + overlayfs + tmpfs)
- [x] gRPC dispatcher over Unix sockets
- [x] Module scaffold: authd, cryptd, filed, netd (stub), procsd, hwspawn
- [x] CLI client (`33` command) with subcommands
- [x] Encrypted file queue (AES-256-GCM)
- [x] Dev mode for local development (no real mounts)
- [x] Cross-compilation for ARM64

### Remaining
- [ ] **Ring 2 gateway (`gated`)** — central policy enforcement
  - Deserialize/validate/rebuild request pattern
  - `SO_PEERCRED` caller verification
  - Audit logging
- [ ] **Content-defined chunking** — FastCDC implementation in `filed`
  - Rolling hash with gear table
  - Configurable min/avg/max chunk sizes
  - Per-chunk encryption pipeline
- [ ] **Manifest system** — version tracking for chunked files
- [ ] **Basic Wayland compositor** — `cage` or `labwc` in kiosk mode
- [ ] **Browser in LXC** — Chromium with DevTools disabled, containerized
- [ ] **YubiKey integration** — FIDO2 auth for login, challenge-response
- [ ] **Basic TPM support** — key sealing to PCR values (optional, graceful fallback)
- [ ] **Bootable Pi image** — custom ARM64 image that boots to desktop

### Milestone: "Grandma Demo"
Pi boots → YubiKey tap → desktop appears → browser opens → grandma googles pie recipes.

## Phase 2: Cloud Integration & Sync

**Goal:** Devices sync encrypted data to a cloud backend with offline support.

- [ ] **Cloud server** — Go monolith with auth, sync, version, fleet services
- [ ] **Sync protocol** — chunk upload/download with dedup
- [ ] **Offline queue drain** — automatic sync when connectivity returns
- [ ] **Conflict resolution** — server-wins with conflict copies
- [ ] **Provisioning tool (`33-prov`)** — new device enrollment, migration
- [ ] **Network discovery (`33-discover`)** — find local server, establish tunnels
- [ ] **Signed OS updates** — A/B partition scheme with automatic rollback
- [ ] **Self-hosted server deployment** — Docker Compose, single-command setup
- [ ] **Subscription infrastructure** — Stripe integration, tier enforcement

### Milestone: "Two Device Sync"
Edit a file on Device A (offline) → connect to WiFi → file appears on Device B.

## Phase 3: Desktop & Ecosystem

**Goal:** A polished desktop experience with per-app containerization.

- [ ] **Full Wayland desktop environment** — custom shell, app launcher, settings
- [ ] **Per-app LXC containers** — browser, email, media player each isolated
- [ ] **App catalog** — signed squashfs app images, install/update lifecycle
- [ ] **33Vault password manager** — hardware-encrypted credential store
- [ ] **Multi-device roaming** — seamless experience across Pi, desktop, laptop
- [ ] **Notification system** — cross-app notifications through Ring 2
- [ ] **Settings UI** — graphical configuration (network, display, accounts)
- [ ] **Enterprise fleet management** — admin dashboard, policy editor
- [ ] **MDM (Mobile Device Management)** — remote wipe, lock, policy push

### Milestone: "Daily Driver"
Someone uses 33-Linux as their only computer for a week without reaching for another device.

## Phase 4: Hardening & Scale

**Goal:** Production-ready security with third-party validation.

- [ ] **Formal security audit** — engage external firm
- [ ] **Fuzz testing** — every gRPC endpoint, every proto parser, especially `gated`
- [ ] **Post-quantum cryptography** — CRYSTALS-Kyber for key exchange, CRYSTALS-Dilithium for signatures
- [ ] **Reproducible builds** — identical binaries from identical source
- [ ] **Supply chain verification** — SLSA Level 3+
- [ ] **Performance optimization** — profiling, memory reduction for low-RAM devices

## Phase 5: Product & Market

**Goal:** A product people can buy.

- [ ] **Pre-built hardware kits** — Pi + YubiKey + case + SD card, preconfigured
- [ ] **Retail packaging** — box, quickstart guide, YubiKey included
- [ ] **Mobile variant** — phone/tablet version (major scope expansion)
- [ ] **Enterprise sales** — direct sales, volume licensing
- [ ] **Community marketplace** — third-party app submissions (signed, reviewed)

## Non-Goals (Explicit)

Things we are deliberately NOT building:

- **A general-purpose Linux distro.** We're not competing with Ubuntu. We're building a security appliance with a desktop.
- **A server OS.** The server component is a sync/management backend, not a server operating system.
- **A Kubernetes distribution.** Talos already did that. We're device-focused.
- **A package manager.** Apps are distributed as signed container images, not packages.
- **Backward compatibility with existing Linux apps.** If an app doesn't run in our container model, it doesn't run. Period.

## Target Timeline

| Phase | Target | Status |
|-------|--------|--------|
| Phase 1 | Q1-Q2 2026 | 🔧 In Progress |
| Phase 2 | Q3-Q4 2026 | 📋 Planned |
| Phase 3 | 2027 | 📋 Planned |
| Phase 4 | 2027-2028 | 🔮 Future |
| Phase 5 | 2028+ | 🔮 Future |

Timelines are aspirational. Quality and security over speed, always.
