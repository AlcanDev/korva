---
id: error-handling
version: 1.0.0
team: backend
stack: Go errors, error wrapping, panic recovery, Result types
last_updated: 2026-04-30
---

# Scroll: Error Handling — Predictable Failure

## Triggers — load when:
- Files: `**/*.go`, `**/errors/**`, `**/recover*.ts`
- Keywords: error, errors.New, errors.Is, errors.As, panic, recover, Result, Either, try/catch
- Tasks: designing an error taxonomy, returning errors across layers, mapping internal errors to HTTP status, retrying on transient failures

## Context
The shape of an error tells callers what they can do about it. A string is unactionable — the caller can log it but cannot decide whether to retry, fall back, or surface to the user. A typed sentinel error is actionable — code can branch on it. The two patterns below cover the common cases: errors as values (Go-style) and errors as union types (TypeScript-style). Both prefer explicit failure handling over invisible exceptions.

---

## Rules

### 1. Sentinel errors for stable conditions

```go
// errors/errors.go
package errors

var (
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrConflict      = errors.New("conflict")
    ErrRateLimited   = errors.New("rate limited")
)

// usage
if errors.Is(err, errors.ErrNotFound) {
    return http.StatusNotFound
}
```

Sentinel errors are part of the public API. Once exported, they cannot be removed without a major version bump. Reserve them for the small set of conditions callers genuinely branch on.

### 2. Wrap with %w to preserve the chain

```go
// BAD — loses the original error
return fmt.Errorf("could not save observation: %v", err)

// GOOD — caller can errors.Is / errors.As against the cause
return fmt.Errorf("save observation %s: %w", id, err)
```

Wrapping turns a flat string into a chain that `errors.Is(err, ErrNotFound)` can walk. Without wrapping, the only thing the caller knows is that *something* failed.

### 3. errors.As for typed extraction

```go
type ValidationError struct {
    Field   string
    Message string
}
func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Message)
}

// caller
var verr *ValidationError
if errors.As(err, &verr) {
    return badRequest(verr.Field, verr.Message)
}
```

Typed errors carry structured data. Use them when the caller needs more than a yes/no — the field that failed validation, the rate-limit retry-after, the conflict's existing version.

### 4. Errors at the API boundary — status code mapping

```go
func toHTTPStatus(err error) int {
    switch {
    case errors.Is(err, ErrNotFound):     return http.StatusNotFound
    case errors.Is(err, ErrUnauthorized): return http.StatusUnauthorized
    case errors.Is(err, ErrConflict):     return http.StatusConflict
    case errors.Is(err, ErrRateLimited):  return http.StatusTooManyRequests
    default:                              return http.StatusInternalServerError
    }
}
```

Map at exactly one place — the HTTP middleware. Internal layers return domain errors; HTTP returns status codes. Don't sprinkle `c.JSON(404, ...)` across handlers.

### 5. Panic only for programmer errors

| Failure | Strategy |
|---------|----------|
| User typed an invalid input | return error |
| Database is down | return error |
| Map key missing that should have been set in init | panic |
| Nil pointer that "can't happen" | panic |

A panic crashes the process or unwinds the goroutine. Reserve it for invariants you actively defend against — never for routine failure modes. Once a panic reaches an HTTP handler, recover it and return 500:

```go
func recoverMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                logger.Error("panic", "value", rec, "stack", debug.Stack())
                http.Error(w, "internal server error", 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

Without recovery, one panic kills the entire HTTP server.

### 6. Retry only on transient errors

```go
type RetryableError struct{ Err error }
func (e *RetryableError) Error() string { return e.Err.Error() }
func (e *RetryableError) Unwrap() error { return e.Err }

func IsRetryable(err error) bool {
    var re *RetryableError
    return errors.As(err, &re)
}

// retry loop
for i := 0; i < maxAttempts; i++ {
    err := op()
    if err == nil { return nil }
    if !IsRetryable(err) { return err }   // permanent — don't retry
    time.Sleep(backoff(i))
}
```

Retrying a permanent error wastes capacity. Mark transient errors explicitly (timeout, 5xx, rate-limit) and never retry the rest.

### 7. TypeScript — Result<T,E> over exceptions

```typescript
type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E };

async function findUser(id: string): Promise<Result<User, NotFound | DBError>> {
  const row = await db.query("SELECT * FROM users WHERE id = ?", id);
  if (!row) return { ok: false, error: new NotFound("user", id) };
  return { ok: true, value: row };
}

// caller — exhaustive handling enforced by TS
const r = await findUser(id);
if (!r.ok) {
  if (r.error instanceof NotFound) return res.status(404).end();
  return res.status(500).end();
}
return res.json(r.value);
```

`throw`/`catch` makes failure invisible at the call site — callers don't know which functions throw what. `Result<T,E>` makes every failure path visible to the type system.

### 8. Errors carry context, not stack traces alone

```go
// Add what you know at this layer; let upper layers add more
return fmt.Errorf("loading config from %s for tenant %s: %w", path, tenantID, err)
```

When the operator reads the log, they see a chain:
```
loading config from /etc/foo.yml for tenant acme: parse: yaml: line 5: unknown field "bla"
```
Each segment was added at one layer. Together they pinpoint the failure without a debugger.

---

## Anti-Patterns

### BAD: stringly-typed errors
```go
if strings.Contains(err.Error(), "not found") { ... }
```
Brittle: a translation, a wording change, or an upstream library update breaks every caller.

### BAD: discarding errors
```go
data, _ := json.Marshal(payload)
```
You hid a real failure. When `payload` contains an unmarshalable type, `data` is the zero value and the bug manifests three layers away. Either handle the error or assert with a comment why it cannot fail.

### BAD: the "or panic" wrapper everywhere
```go
mustGet := func(k string) string {
    v, err := store.Get(k)
    if err != nil { panic(err) }
    return v
}
```
Convenient in one-shot scripts; catastrophic in services. The panic crosses goroutine boundaries and hides the real error path. Reserve `must*` for `init()` and tests.

### BAD: `catch (err) { logger.error(err); throw err; }`
Re-throwing after logging double-logs the same error at every layer. Pick one: log at the boundary OR return up the stack. Logging at every layer makes incident logs impossible to read.

### BAD: HTTP error response without correlation ID
```json
{ "error": "internal server error" }
```
The user reports the error; the operator can't find the matching log line. Always:
```json
{ "error": "internal server error", "request_id": "req-abc123" }
```
