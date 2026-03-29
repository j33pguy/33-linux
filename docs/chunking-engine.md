# Content-Defined Chunking Engine

## Overview

The chunking engine is the core of 33-Linux's file storage system. Files are split into variable-size chunks using content-defined boundaries, individually encrypted, deduplicated by content hash, and synced to the cloud backend. This enables efficient delta sync, space-efficient version control, and zero-knowledge server storage.

## Why Content-Defined Chunking?

### The Problem with Fixed-Size Chunks

If you split a file into 8KB fixed-size blocks and insert one byte at the beginning:

```
Before: [block_1: bytes 0-8191] [block_2: bytes 8192-16383] ...
After:  [block_1: bytes 0-8191] [block_2: bytes 8192-16383] ...
         ^^^^^^^^^^^^^^^^^^^^^^^^
         Every block shifted by 1 byte → every block changed
         → re-encrypt everything → re-upload everything
```

A 1-byte edit causes a full re-sync. Unacceptable.

### Content-Defined Boundaries

Content-Defined Chunking (CDC) uses a rolling hash over a sliding window. When the hash matches a pattern (e.g., low N bits are zero), that's a chunk boundary:

```
Before: [chunk_a: "Hello "] [chunk_b: "world, how"] [chunk_c: " are you?"]
Insert "dear " before "world":
After:  [chunk_a: "Hello "] [chunk_d: "dear world, how"] [chunk_c: " are you?"]
                              ^^^^^^^^^^^^^^^^^^^^^^^^
                              Only chunk_b changed → only re-encrypt and sync this one
```

Boundaries are determined by content, not position. Insertions only affect the chunk where the change occurs and potentially one neighbor.

## Algorithm: FastCDC

We use [FastCDC](https://www.usenix.org/conference/atc16/technical-sessions/presentation/xia) (Fast Content-Defined Chunking), which is ~10x faster than the classic Rabin fingerprint approach.

### Parameters

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| Minimum chunk size | 2 KB | Prevents pathological small chunks |
| Average chunk size | 8 KB | Balance between dedup ratio and overhead |
| Maximum chunk size | 32 KB | Prevents pathological large chunks |
| Hash algorithm | Gear hash | Faster than Rabin, similar distribution |
| Normalization level | 2 | Reduces chunk size variance |

### Pseudocode

```
function chunk(data):
    chunks = []
    offset = 0

    while offset < len(data):
        # Skip minimum chunk size (no boundary possible here)
        pos = offset + MIN_CHUNK_SIZE

        # Scan for boundary
        hash = 0
        while pos < offset + MAX_CHUNK_SIZE and pos < len(data):
            hash = (hash << 1) + GEAR_TABLE[data[pos]]

            # Check if low bits match pattern (normalized)
            if pos - offset < AVERAGE_CHUNK_SIZE:
                mask = MASK_LARGE  # Harder to match → encourages larger chunks
            else:
                mask = MASK_SMALL  # Easier to match → encourages splitting

            if (hash & mask) == 0:
                break  # Found a boundary

            pos++

        chunk = data[offset:pos]
        chunks.append(chunk)
        offset = pos

    return chunks
```

The normalization trick (two masks) reduces chunk size variance, keeping most chunks close to the average size. This improves deduplication ratio.

## Encryption Pipeline

Each chunk is independently encrypted before storage:

```
Raw Chunk (plaintext, 4-32KB)
  │
  ▼ Derive key
  key = HKDF-SHA256(session_key, "chunk:" + sequence_index)
  │
  ▼ Encrypt
  nonce = crypto/rand (12 bytes)
  aad = {manifest_id, chunk_index, version}  ← prevents reordering
  ciphertext = AES-256-GCM(key, nonce, plaintext, aad)
  │
  ▼ Package
  stored_blob = nonce || ciphertext  (nonce prepended)
  │
  ▼ Identify
  blob_hash = SHA-256(stored_blob)  ← hash of CIPHERTEXT, not plaintext
  │
  ▼ Store
  /var/sync-queue/{blob_hash}.enc
```

### Why Hash Ciphertext (Not Plaintext)?

**Option A — Hash plaintext, then encrypt:**
- Pro: Same file across users = same hash = cross-user dedup
- Con: Server learns "User A and User B have the same file" → metadata leak

**Option B — Encrypt, then hash ciphertext:**
- Pro: Zero metadata leakage. Server sees only random-looking hashes.
- Con: No cross-user dedup. Different keys = different ciphertext = different hashes.

**We choose Option B.** For a security-focused OS, metadata privacy trumps storage efficiency. Within a single user's data, dedup still works because the same key produces the same ciphertext for identical chunks.

## Manifest Structure

Each file version is represented by a manifest:

```json
{
  "path_hash": "sha256(encrypted_path)",
  "version": 3,
  "created_at": "2026-03-29T09:50:00Z",
  "total_size": 5242880,
  "chunk_count": 412,
  "chunks": [
    {
      "index": 0,
      "hash": "a1b2c3d4...",
      "size": 4127,
      "offset": 0
    },
    {
      "index": 1,
      "hash": "e5f6a7b8...",
      "size": 8943,
      "offset": 4127
    }
  ],
  "previous_version": 2,
  "checksum": "sha256(concatenated_chunk_hashes)"
}
```

### Version Chain

```
v1: [A, B, C, D, E]           ← initial file
v2: [A, B, F, D, E]           ← chunk C changed to F
v3: [A, B, F, G, E]           ← chunk D changed to G
v4: [A, B, C, D, E]           ← rollback to v1 (same chunks!)

Unique chunks stored: A, B, C, D, E, F, G = 7
File versions stored: 4
Storage multiplier: ~1.4x (not 4x)
```

Rollback is free — it just creates a new manifest pointing to old chunks. The chunks already exist.

## Deduplication

### Within a User

Same content → same plaintext → same key derivation → same ciphertext → same hash. Stored once.

**Example:** User saves `photo.jpg` in two folders:
```
/photos/vacation/photo.jpg  → chunks [A, B, C]
/photos/backup/photo.jpg    → chunks [A, B, C]  ← same hashes, zero additional storage
```

Two manifests, same chunks. Storage cost: 1x, not 2x.

### Cross-User

Not supported (by design). Different users have different master keys, so identical files produce different ciphertext and different hashes. This is a conscious security tradeoff.

## Sync Protocol

### Upload (Client → Server)

```
1. Client has chunks in /var/sync-queue/
2. For each chunk:
   a. HEAD /api/v1/chunks/{hash}  → 200 (exists, skip) or 404 (upload)
   b. If 404: PUT /api/v1/chunks/{hash} with blob body
   c. Server stores, returns 201
3. After all chunks uploaded:
   a. PUT /api/v1/manifests/{path_hash} with manifest body
   b. Server stores new version
4. Client removes synced chunks from local queue
```

### Download (Server → Client)

```
1. Client requests manifest: GET /api/v1/manifests/{path_hash}
2. For each chunk in manifest:
   a. Check local cache: /var/cache/33linux/{hash}
   b. If missing: GET /api/v1/chunks/{hash}
   c. Decrypt chunk with derived key
   d. Store plaintext in local cache
3. Reassemble file from ordered chunks
```

### Conflict Resolution

When the same file is modified on two devices while offline:

```
Device A (offline): v1 → v2a (modified chunk 3)
Device B (offline): v1 → v2b (modified chunk 5)

Both come online:
  → Server has v1
  → Device A pushes v2a (accepted as v2)
  → Device B pushes v2b → CONFLICT (server has v2 from Device A)
```

**Resolution strategies (configurable):**

1. **Server wins:** v2a becomes canonical. Device B's changes saved as `{file}.conflict.{timestamp}`
2. **Timestamp wins:** Most recent modification wins. Other saved as conflict copy.
3. **Merge (future):** For text files, attempt 3-way merge. For binary, fall back to server-wins.
4. **Manual:** Flag conflict, let user choose in the UI.

Default: server-wins with conflict copy. Safe, predictable, no data loss.

## Garbage Collection

Chunks that are no longer referenced by any manifest become garbage:

```
Daily GC job:
  1. Build bloom filter of all chunk hashes from all current manifests
  2. Scan blob store
  3. Any blob not in the bloom filter AND older than retention_period → mark for deletion
  4. Second pass (next day): delete marked blobs if still unreferenced
```

Two-pass deletion prevents race conditions with in-progress syncs.

## Performance Considerations

### On Raspberry Pi 5 (ARM Cortex-A76, 8GB)

| Operation | Expected Performance |
|-----------|---------------------|
| FastCDC chunking | ~400 MB/s (single core) |
| AES-256-GCM encrypt | ~1.5 GB/s (ARM crypto extensions) |
| SHA-256 hash | ~800 MB/s (ARM crypto extensions) |
| Full pipeline (chunk + encrypt + hash) | ~300 MB/s |

For a 5MB photo: ~17ms to chunk, encrypt, and queue. Imperceptible to grandma.

### Memory Usage

- Chunking buffer: ~64KB (sliding window)
- Per-chunk encryption: ~32KB (chunk + ciphertext)
- Manifest in memory: ~50 bytes per chunk × 500 chunks = ~25KB per file
- Total overhead per file operation: < 1MB

### Storage Overhead

| Factor | Overhead |
|--------|----------|
| Nonce prepended to each chunk | 12 bytes per chunk |
| GCM auth tag | 16 bytes per chunk |
| Manifest metadata | ~100 bytes per file version |
| Total per 8KB chunk | 28 bytes = 0.3% overhead |
