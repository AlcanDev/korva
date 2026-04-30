---
id: sqlite-concurrency
version: 1.0.0
team: backend
stack: SQLite, Go, write-queue, WAL
last_updated: 2026-04-30
---

# Scroll: SQLite under Concurrent Access

## Triggers — load when:
- Files: `**/store/**`, `**/db/**`, `*sqlite*`, `migrations/**/*.sql`
- Keywords: SQLite, WAL, busy_timeout, locked, concurrent, write queue, modernc, mattn
- Tasks: designing the data layer, fixing "database is locked" errors, scaling write throughput, optimising read latency

## Context
SQLite serialises every write at the database level — there is exactly one writer at a time, full stop. Multiple goroutines that hammer the database with concurrent INSERTs will trip over each other and produce `SQLITE_BUSY` / `database is locked` errors. The fix is not a connection pool the way it is in Postgres — it is an explicit application-level write queue. Reads can stay parallel; writes must be serialised.

---

## Rules

### 1. Always WAL mode + busy_timeout

```go
db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
```

| PRAGMA | Why |
|--------|-----|
| `journal_mode=WAL` | readers and one writer can coexist without blocking each other |
| `busy_timeout=5000` | wait up to 5s for the writer lock before returning SQLITE_BUSY |
| `synchronous=NORMAL` | safe for WAL — full fsync only at checkpoint, not every commit |
| `foreign_keys=ON` | enforce relational integrity (off by default in SQLite) |

WAL mode also requires the directory to be writable, not just the database file.

### 2. Separate read and write pools

```go
type Store struct {
    readDB  *sql.DB   // SetMaxOpenConns(N) — many parallel readers
    writeDB *sql.DB   // SetMaxOpenConns(1)  — single serialised writer
}

func New(path string) (*Store, error) {
    readDB, _ := sql.Open("sqlite", path+"?mode=ro&_journal_mode=WAL")
    readDB.SetMaxOpenConns(runtime.NumCPU())

    writeDB, _ := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
    writeDB.SetMaxOpenConns(1)
    writeDB.SetMaxIdleConns(1)

    return &Store{readDB: readDB, writeDB: writeDB}, nil
}
```

The read pool uses `mode=ro` so any accidental write panics rather than silently going to the wrong connection.

### 3. Application-level write queue for hot paths

When N goroutines (e.g. concurrent agents calling MCP `vault_save`) all try to write at once, even WAL+busy_timeout starts dropping requests. Funnel writes through one goroutine:

```go
type writeOp struct {
    fn   func(*sql.Tx) error
    done chan error
}

type Store struct {
    writeQ chan writeOp
}

// goroutine started in New():
func (s *Store) writeLoop(ctx context.Context) {
    for {
        select {
        case op := <-s.writeQ:
            tx, err := s.writeDB.Begin()
            if err != nil { op.done <- err; continue }
            if err := op.fn(tx); err != nil { tx.Rollback(); op.done <- err; continue }
            op.done <- tx.Commit()
        case <-ctx.Done():
            return
        }
    }
}

// public API:
func (s *Store) Save(ctx context.Context, fn func(*sql.Tx) error) error {
    op := writeOp{fn: fn, done: make(chan error, 1)}
    select {
    case s.writeQ <- op:
    case <-ctx.Done():
        return ctx.Err()
    }
    return <-op.done
}
```

Reads still go directly to `readDB`. Only writes pay the queue cost — and the queue cost is tiny compared to a SQLITE_BUSY retry loop.

### 4. Batch writes inside one transaction

Ten INSERTs in ten transactions = ten fsyncs.
Ten INSERTs in one transaction = one fsync.

```go
// BAD — 10 fsyncs
for _, item := range items {
    db.Exec("INSERT INTO obs (...) VALUES (...)", item.fields...)
}

// GOOD — 1 fsync
tx, _ := db.Begin()
for _, item := range items {
    tx.Exec("INSERT INTO obs (...) VALUES (...)", item.fields...)
}
tx.Commit()
```

Throughput goes from ~100 inserts/sec to ~50,000 inserts/sec on consumer SSDs.

### 5. Indexes — only on columns that filter or join

Every index doubles the write cost on that table. Profile first:

```sql
EXPLAIN QUERY PLAN
SELECT * FROM observations WHERE project = ? AND created_at > ?;
```

If you see `SCAN TABLE observations` instead of `SEARCH TABLE observations USING INDEX`, add the index. Don't add indexes preemptively.

### 6. Migrations — additive, idempotent, versioned

```go
var migrations = []string{
    // v1
    `CREATE TABLE IF NOT EXISTS observations (
        id TEXT PRIMARY KEY,
        project TEXT NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_obs_project ON observations(project);`,

    // v2 — add type column
    `ALTER TABLE observations ADD COLUMN type TEXT NOT NULL DEFAULT 'note';`,
}

func Migrate(db *sql.DB) error {
    var current int
    db.QueryRow("PRAGMA user_version").Scan(&current)
    for i := current; i < len(migrations); i++ {
        if _, err := db.Exec(migrations[i]); err != nil {
            return fmt.Errorf("migration v%d: %w", i+1, err)
        }
        db.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1))
    }
    return nil
}
```

Never rewrite a previous migration. To "fix" a migration, write a new one that corrects the previous result.

### 7. Pure-Go driver vs CGo driver

| Driver | Pros | Cons |
|--------|------|------|
| `modernc.org/sqlite` (pure Go) | no CGO, cross-compiles trivially, single static binary | ~30% slower on heavy write workloads |
| `mattn/go-sqlite3` (CGo) | fastest, full feature parity with C SQLite | requires `CGO_ENABLED=1`, complicates cross-compile |

For a CLI distributed via Homebrew + GoReleaser, prefer the pure-Go driver. For a backend service that owns its own Dockerfile, the CGo driver is fine.

---

## Anti-Patterns

### BAD: large connection pool for writes
```go
db.SetMaxOpenConns(20)  // for write workload
```
Every connection competes for the same SQLite write lock. You get SQLITE_BUSY at 21 concurrent writers; throughput collapses. The right answer is `SetMaxOpenConns(1)` for the write DB.

### BAD: retry loop on SQLITE_BUSY
```go
for i := 0; i < 10; i++ {
    _, err := db.Exec(...)
    if err == nil { break }
    time.Sleep(time.Duration(i*100) * time.Millisecond)
}
```
Treats the symptom; ignores the cause. Use a write queue so writers are serialised at the application layer and never see SQLITE_BUSY in the first place.

### BAD: forgetting to close rows
```go
rows, _ := db.Query(...)
for rows.Next() { ... }
// no rows.Close() — connection leaks until GC
```
SQLite holds a read lock while rows are open. Combined with WAL checkpointing, leaked rows can stall the writer indefinitely.

### BAD: ALTER TABLE … DROP COLUMN
SQLite versions before 3.35 don't support DROP COLUMN. Even on newer versions, prefer the recreate-table-rename pattern explicitly so the migration is portable across versions.

### BAD: storing JSON when relations would do
```sql
CREATE TABLE post (
    id TEXT PRIMARY KEY,
    metadata TEXT  -- {"author": "alice", "tags": ["go", "sqlite"]}
);
```
Now `WHERE author = 'alice'` requires `json_extract` — slow, no index. Make `author` a real column and `tags` a separate table.
