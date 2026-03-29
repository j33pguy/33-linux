# Chunking Engine

## How It Works

Files are split into variable-size chunks using FastCDC (content-defined boundaries), individually encrypted, and identified by their ciphertext hash.

```
File (5MB) → FastCDC → [chunk_a, chunk_b, chunk_c, ...]
  → Each chunk: encrypt(AES-256-GCM) → hash(SHA-256) → store
  → Manifest: ordered list of chunk hashes = one file version
```

## Why Content-Defined?

Fixed-size chunks break on any insertion (all boundaries shift). Content-defined boundaries are determined by the data itself, so insertions only affect nearby chunks. A 1-byte edit syncs ~8KB, not the entire file.

## Encryption Order

**Encrypt → then hash ciphertext.** Server never sees plaintext or plaintext hashes. No metadata leakage at the cost of no cross-user dedup.

## Version Control

```
v1: [A, B, C, D, E]     ← original
v2: [A, B, F, D, E]     ← chunk C changed
v3: [A, B, C, D, E]     ← rollback to v1 (free — chunks still exist)

Unique chunks: 6. Versions: 3. Storage: ~1.2x not 3x.
```

## Sync Protocol

1. Client chunks + encrypts file
2. For each chunk: check if server has it (HEAD), upload if not (PUT)
3. Upload manifest (new version)
4. Remove synced chunks from local queue

Conflict resolution: server-wins with conflict copy (configurable).

See the full [chunking engine document](https://github.com/j33pguy/33-linux/blob/main/docs/chunking-engine.md) for FastCDC parameters, performance benchmarks, and garbage collection.
