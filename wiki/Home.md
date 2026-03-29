# 33-Linux Wiki

**Secure. Immutable. Yours.**

Welcome to the 33-Linux documentation wiki. 33-Linux is a hardened, immutable Linux distribution that makes security invisible to the user.

## Quick Links

- [[Architecture]] — Ring-based security model, boot sequence, data flow
- [[Security Model]] — Threat model, TPM+YubiKey auth, kernel hardening
- [[Chunking Engine]] — Content-defined chunking, encryption, deduplication
- [[Cloud Backend]] — Server architecture, sync protocol, subscription tiers
- [[Roadmap]] — 5-phase development plan
- [[Development Guide]] — Build, test, and contribute

## What Is 33-Linux?

A thick-client operating system built for people who shouldn't have to think about security:

- **Immutable root** — squashfs read-only, reboot fixes everything
- **Per-app containers** — browser can't read your email, apps can't read each other
- **Hardware auth** — TPM + YubiKey Bio, no passwords to remember
- **Offline-first** — works without internet, syncs when connected
- **Version-controlled files** — roll back any change, any time
- **Open source** — BUSL-1.1, self-host for free

## Repository

- Source: [github.com/j33pguy/33-linux](https://github.com/j33pguy/33-linux)
- License: BUSL-1.1 (self-hosting free, commercial hosting requires license)
