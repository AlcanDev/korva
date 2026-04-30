---
id: observability
version: 1.0.0
team: backend
stack: structured logs, metrics, traces, OpenTelemetry
last_updated: 2026-04-30
---

# Scroll: Observability — Logs, Metrics, Traces

## Triggers — load when:
- Files: `**/log*.go`, `**/tracer*.ts`, `**/middleware/log*.ts`, `*.observability.*`
- Keywords: log, logger, structured logging, metric, trace, span, OpenTelemetry, OTel, prom, prometheus
- Tasks: adding observability, debugging a production issue, instrumenting a service, designing log schemas

## Context
Three signals answer different questions: **logs** answer "what happened?", **metrics** answer "how often / how much?", **traces** answer "where did the time go?". A service that emits all three with a shared correlation ID can be debugged in minutes instead of hours. A service that emits only ad-hoc `console.log` lines has no production answer; you'll be guessing.

---

## Rules

### 1. Logs — structured JSON, never plain text

```go
// BAD
log.Printf("user %s purchased %s for $%d", userID, sku, price)

// GOOD
logger.Info("purchase",
    "user_id", userID,
    "sku", sku,
    "price_cents", price,
    "currency", "USD",
    "request_id", reqID,
)
```

JSON output is parseable by every log aggregator. Plain text requires a custom regex per message — guaranteed to break the first time someone changes the wording.

### 2. Required fields on every log line

| Field | Purpose |
|-------|---------|
| `timestamp` | ISO-8601 UTC, microsecond precision |
| `level` | `debug` \| `info` \| `warn` \| `error` |
| `service` | which app emitted the log |
| `version` | the app version (commit SHA or semver) |
| `request_id` | correlates one HTTP request across services |
| `trace_id` | correlates the log line to a distributed trace |
| `message` | short human-readable summary |

Anything missing one of these is invisible during an incident.

### 3. Log levels — strict semantics

- `error`: an operation failed and the user/caller will notice. Page-worthy.
- `warn`: degraded behaviour or unexpected state, but the operation succeeded. Investigate later.
- `info`: business events worth keeping (login, purchase, deploy). One per request is fine; ten per request is noise.
- `debug`: developer-only. Off in production.

If you find yourself logging `info` ten times per request, demote nine of them to `debug` or remove them. Logs cost money — both storage and the time of the human reading them at 3am.

### 4. Never log secrets — redact at the source

```go
type RedactedString string
func (RedactedString) String() string { return "***" }
func (r RedactedString) MarshalJSON() ([]byte, error) {
    return []byte(`"***"`), nil
}

type Config struct {
    APIKey RedactedString `json:"api_key"`
}
```

The redaction happens in the type, not in the logging callsite. That way the developer who adds a new log line three months from now can't accidentally print the secret.

PII (emails, phone numbers, addresses) gets the same treatment by default. A privacy regulator that finds an email in your logs costs more than the entire observability bill.

### 5. Metrics — RED for services, USE for resources

**RED** for any request-handling service:
- **R**ate: requests per second, by route + status
- **E**rrors: 4xx and 5xx counts, by route
- **D**uration: histogram of latency, by route. Always p50 / p95 / p99.

**USE** for any resource (CPU, memory, disk, DB connection pool):
- **U**tilisation: % of capacity used
- **S**aturation: queue depth / wait time
- **E**rrors: count of resource-exhaustion events

```go
// Prometheus example
httpDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_request_duration_seconds",
        Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
    },
    []string{"route", "method", "status"},
)
```

Buckets must cover both the median (~50ms) and the tail (>1s). Default buckets are usually wrong for HTTP services.

### 6. Traces — propagate context across boundaries

Use OpenTelemetry. The HTTP middleware extracts `traceparent` from incoming requests and injects it into outgoing ones, so a single trace_id ties together every span across services:

```go
ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
ctx, span := tracer.Start(ctx, "GET /users/:id")
defer span.End()
span.SetAttributes(attribute.String("user.id", userID))
```

A span you forget to close hides every child span from the trace. Always `defer span.End()`.

### 7. Sampling — head-based with tail override

Trace 100% in dev, 1-10% in production with a tail rule that always keeps:
- Any trace with `error` status
- Any trace with `duration > p99 baseline`
- Anything explicitly tagged `force_sample=true` (set by the on-call when reproducing)

Sampling everything floods storage; sampling uniformly at 1% loses the rare bug.

---

## Anti-Patterns

### BAD: `console.log("got here")`
You'll add it during a debugging session and forget to remove it. It ships to production. Now your log aggregator has 10M lines of "got here" with no context.

Use a structured logger from day one. Make `console.log` a lint error.

### BAD: one giant log line per request
```text
[INFO] req=abc123 user=u-456 path=/checkout method=POST body={"sku":"...","qty":3,"address":{...}} response={"status":"ok","order_id":"o-789"} duration=143ms
```
Hard to query, exceeds the typical 16KB log-line limit, leaks PII (the address). Split into entry/exit logs and keep the body in a separate event store if you need it.

### BAD: error logs with no stack
```go
log.Printf("error: %v", err)
```
Three months later you can't tell where this fired. Always:
```go
logger.Error("request failed",
    "error", err.Error(),
    "stack", fmt.Sprintf("%+v", err),  // or zap.Error / slog AnyValue
)
```

### BAD: metric cardinality explosion
```go
httpRequests.WithLabelValues(userID, sessionID, fullURL).Inc()
```
Every unique `(userID, sessionID, fullURL)` becomes a new time series. Prometheus dies. Use route templates (`/users/:id`) and bounded label sets only.

### BAD: alerting on log lines
"Page when the log contains the string `panic`" sounds reasonable until someone logs a sample panic message. Alert on metrics (`error_rate > 5%`) so the alert is bounded by the metric's units, not the open-ended set of strings the app might emit.
