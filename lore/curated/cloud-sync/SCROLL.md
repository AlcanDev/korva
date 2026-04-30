---
id: cloud-sync
version: 1.0.0
team: backend
stack: HTTP, content-addressed storage, conflict resolution, hashing
last_updated: 2026-04-30
---

# Scroll: Cloud Sync — Replication and Conflict Resolution

## Triggers — load when:
- Files: `**/sync*.go`, `**/replication/**`, `**/outbox/**`, `**/hive*`
- Keywords: sync, replication, chunk, hash, idempotent, content-addressed, eventual consistency, outbox, CDN
- Tasks: designing a client-server sync protocol, deduplicating uploads, resolving sync conflicts, implementing offline-first

## Context
Two clients writing to the same logical record produce conflicts. A client uploading the same content twice wastes bandwidth and storage. Both problems collapse into one solution: address content by its hash, not its name. Combined with an explicit outbox on the client and server-side validation of the canonical hash, you get an idempotent protocol that retries safely, deduplicates automatically, and detects tampering.

---

## Rules

### 1. Content-addressed chunks — name = hash

```text
Original: "the quick brown fox"
SHA-256:   2e7d2c03a9507ae265ecf5b5356885a53393a2029d241394997265a1a25aefc6

Storage path:
  chunks/2e/7d/2c03a9507ae265ecf5b5356885a53393a2029d241394997265a1a25aefc6
```

Two clients uploading the same content land on the same path — automatic deduplication. A corrupted byte changes the hash; the client's upload no longer matches the path it claims; server rejects.

### 2. Canonical serialization before hashing

```go
// BAD — map iteration order varies between Go versions / platforms
data, _ := json.Marshal(obj)
hash := sha256.Sum256(data)

// GOOD — sorted keys, no whitespace, fixed encoding
canonical, _ := json.MarshalIndent(obj, "", "")
canonical = canonicalizeJSON(canonical)  // sort keys recursively
hash := sha256.Sum256(canonical)
```

Both client and server MUST produce the same hash for the same logical content. Use a defined canonical form — sorted JSON keys, no whitespace, fixed Unicode normalization (NFC). Otherwise the same payload generates two hashes and dedup fails.

### 3. Client outbox — local queue first

```go
type Outbox struct {
    db *sql.DB
}

// Caller writes to local store + outbox in one transaction
func (o *Outbox) Enqueue(ctx context.Context, obj Object) error {
    tx, _ := o.db.Begin()
    tx.Exec("INSERT INTO observations (...) VALUES (...)", obj.fields...)
    tx.Exec("INSERT INTO outbox (id, payload, created_at) VALUES (?, ?, ?)",
        obj.ID, canonical(obj), time.Now())
    return tx.Commit()
}
```

The local write completes immediately; the outbox is the queue of "things to push to the server". A worker drains the outbox in the background and retries on failure.

If the network is down, the user keeps working. When it comes back, the outbox flushes.

### 4. Server-side hash validation — never trust the client's hash

```go
func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    canonical := canonicalize(body)

    serverHash := sha256.Sum256(canonical)
    clientHash := r.Header.Get("X-Content-Hash")

    if hex.EncodeToString(serverHash[:]) != clientHash {
        http.Error(w, "hash mismatch", 422)
        return
    }
    s.store(serverHash[:], canonical)
    w.WriteHeader(201)
}
```

The client sends its own hash for verification, but the server recomputes from canonical bytes. A buggy or malicious client cannot poison the storage with mismatched content.

### 5. Idempotent upload — same hash = noop

```go
func (s *Server) store(hash []byte, body []byte) error {
    path := chunkPath(hash)
    if _, err := os.Stat(path); err == nil {
        return nil  // already have it
    }
    return os.WriteFile(path, body, 0o644)
}
```

A client retries an upload after a partial network failure. Same hash → server returns 200 OK without writing. No duplicate, no conflict. The client can retry arbitrarily.

### 6. Conflict resolution — last-write-wins on metadata, never on chunks

Chunks are immutable (content-addressed). Conflicts only happen on **metadata** that points to chunks: "the latest version of file X is hash Y".

```text
Client A: file "report.md" → hash α (writes at 12:01)
Client B: file "report.md" → hash β (writes at 12:02)

Server resolution: B wins (later timestamp), α is preserved as a previous version.
Both chunks remain in storage; only the pointer flips.
```

This is the right default for personal sync. For collaborative editing, use CRDTs or operational transforms — but those are 100x harder; reserve for when the use case demands them.

### 7. Garbage collection of orphan chunks

A chunk that no metadata points to is unreachable. Run a periodic sweep:

```sql
DELETE FROM chunks
WHERE hash NOT IN (SELECT chunk_hash FROM metadata_versions);
```

GC must run with a grace period (e.g. delete chunks older than 7 days with no references) so concurrent writes don't have their freshly-uploaded chunk deleted before the metadata pointer commits.

### 8. Privacy filter at the boundary

If your sync service is multi-tenant or carries user content to a cloud you don't own, redact PII at the client BEFORE hashing:

```go
filtered := privacy.Redact(canonical)  // strip emails, JWTs, API keys
hash := sha256.Sum256(filtered)
```

Two consequences:
1. The cloud only ever sees redacted content
2. Hash is computed on the redacted form — server-side verification still works

The user gets explicit, auditable control over what leaves their machine.

---

## Anti-Patterns

### BAD: timestamp-based dedup
```go
if existing.Time == new.Time { skip }
```
Clock skew between client and server makes this unreliable. Two clients in different timezones produce "different" times for the same event. Hash content, not metadata.

### BAD: "delete" via DELETE request
```http
DELETE /chunks/abc123
```
A buggy client deletes shared content. Make storage append-only — deletes are tombstones in metadata, never destructive on chunks. GC is the only path to actual deletion.

### BAD: client computes hash on the wire format
```go
body, _ := json.Marshal(obj)         // ← non-canonical
hash := sha256.Sum256(body)
upload(hash, body)
```
A second client emits the same logical content with different key ordering — different hash, dedup fails, storage doubles. Always canonicalize before hashing.

### BAD: trusting `Content-Length` for retry
A client uploads a 1MB chunk; connection drops at 700KB. Retry sends a fresh 1MB. If the server keyed by `(filename, content-length)`, it'd think this is a different file. Hash-keyed storage doesn't have this problem; the partial 700KB never made it to a hash, so there's no orphan to clean up.

### BAD: synchronous sync on the user's hot path
```go
func Save(obj Object) error {
    localStore.Save(obj)
    return cloud.Upload(obj)  // blocks user until network OK
}
```
User's app freezes on slow network. Use the outbox pattern — local save returns immediately, cloud sync is a background worker.
