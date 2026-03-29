# Roadmap

## Phase 1: Core Boot & Local System ← Current

**Milestone:** Pi boots → YubiKey tap → desktop → browser → grandma googles pie recipes.

- [x] Go init system, gRPC dispatcher, 6 core modules
- [ ] Ring 2 gateway (gated)
- [ ] FastCDC chunking engine
- [ ] YubiKey integration
- [ ] Wayland kiosk compositor
- [ ] Browser in LXC
- [ ] Bootable Pi image

## Phase 2: Cloud Integration & Sync

**Milestone:** Edit file on Device A → sync → appears on Device B.

- Cloud backend server
- Sync protocol with offline queue
- Provisioning tool
- Signed OS updates (A/B partition)
- Subscription infrastructure

## Phase 3: Desktop & Ecosystem

**Milestone:** Someone uses 33-Linux as their only computer for a week.

- Full desktop environment
- Per-app LXC containers
- App catalog
- 33Vault password manager
- Enterprise fleet management

## Phase 4: Hardening & Scale

- Formal security audit
- Post-quantum cryptography
- Reproducible builds

## Phase 5: Product & Market

- Pre-built hardware kits (Pi + YubiKey + case)
- Enterprise sales
- Community app marketplace

See the full [roadmap document](https://github.com/j33pguy/33-linux/blob/main/docs/roadmap.md) for detailed task lists and non-goals.
