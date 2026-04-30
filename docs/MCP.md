# MCP Tools Reference

> Every tool the Korva vault exposes over MCP, with example calls and license tiers.

*Last updated: 2026-04-30*

---

## What is MCP?

The **Model Context Protocol** is an open standard that lets AI assistants talk to local services through a typed JSON-RPC 2.0 interface. Korva exposes its memory and knowledge layer as an MCP server on `localhost:7437`.

When you run `korva setup <editor>`, the editor's config file is updated to launch `korva-vault --mode mcp` as a subprocess and pipe MCP messages over stdio.

---

## Connecting

```jsonc
// Generic MCP client config — most editors derive from this
{
  "mcpServers": {
    "korva": {
      "command": "korva-vault",
      "args": ["--mode", "mcp"]
    }
  }
}
```

The `korva setup <editor>` commands write the editor-specific equivalent for you.

---

## Tools — Community tier

These are available with no license required.

### `vault_save`
Persist a structured observation (decision, pattern, bug fix, note).

```json
{
  "name": "vault_save",
  "arguments": {
    "project": "my-app",
    "type": "decision",
    "title": "Switch auth from sessions to JWT",
    "content": "Decision: rotate signing keys via JWKS. Refresh tokens 30d, access 15min.",
    "tags": ["auth", "security"]
  }
}
```

Returns the new observation's ULID.

### `vault_search`
Free-text search across all observations for a project.

```json
{ "name": "vault_search", "arguments": { "project": "my-app", "query": "JWT", "limit": 10 } }
```

### `vault_context`
Load a session-ready context bundle for a project — recent decisions, current SDD phase, and auto-loaded scrolls (Smart Skill Loader on Teams+).

```json
{ "name": "vault_context", "arguments": { "project": "my-app" } }
```

### `vault_timeline`
Time-bounded list of observations.

```json
{
  "name": "vault_timeline",
  "arguments": {
    "project": "my-app",
    "from": "2026-01-01T00:00:00Z",
    "to":   "2026-04-30T00:00:00Z"
  }
}
```

### `vault_summary`
Counts and high-signal aggregates for a project.

```json
{ "name": "vault_summary", "arguments": { "project": "my-app" } }
```

### `vault_purge`
Delete observations older than a date.

```json
{ "name": "vault_purge", "arguments": { "project": "my-app", "before": "2025-01-01" } }
```

### `vault_export`
Export all observations for a project as a JSON bundle.

```json
{ "name": "vault_export", "arguments": { "project": "my-app" } }
```

### `vault_import`
Import a previously-exported bundle.

```json
{ "name": "vault_import", "arguments": { "project": "my-app", "bundle": "..." } }
```

### `lore_list`
List all curated scrolls available locally.

```json
{ "name": "lore_list" }
```

### `lore_get`
Fetch the full text of one scroll.

```json
{ "name": "lore_get", "arguments": { "id": "skill-authoring" } }
```

### `lore_search`
Full-text search across scrolls.

```json
{ "name": "lore_search", "arguments": { "query": "JWT rotation" } }
```

### `sdd_phase_get` / `sdd_phase_set`
Read or update the current Spec-Driven Development phase for a project.

```json
{ "name": "sdd_phase_set", "arguments": { "project": "my-app", "phase": "implementation" } }
```

### `project_meta_get` / `project_meta_set`
Per-project specification metadata — stack, conventions, glossary.

---

## Tools — Teams tier

Require an activated Teams license.

### `vault_skill_match`
Run the smart skill matcher against an arbitrary task description.

```json
{
  "name": "vault_skill_match",
  "arguments": { "task": "rotate JWT signing keys with JWKS endpoint" }
}
```

Returns ranked scroll IDs with match scores.

### `vault_dedupe`
Detect duplicate observations and merge.

### `hive_status`
Show Hive sync state — last successful push, queue depth, recent errors.

### `hive_push_now`
Force an immediate Hive sync without waiting for the worker tick.

---

## Tools — Business tier

Require an activated Business license.

### `vault_code_health`
Aggregate health metrics for a codebase — duplication, conflict density, churn.

```json
{ "name": "vault_code_health", "arguments": { "project": "my-app" } }
```

### `vault_pattern_mine`
Mine recurring patterns from observations to surface emergent conventions.

```json
{ "name": "vault_pattern_mine", "arguments": { "project": "my-app", "min_support": 3 } }
```

---

## Error responses

Every error follows the same shape:

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "error": {
    "code": -32602,
    "message": "vault_skill_match requires a Korva Teams license. Run: korva license activate <key>"
  }
}
```

License-gated tools return a clear, actionable error pointing the user at the upgrade path. Other tools follow the JSON-RPC error code conventions:

| Code | Meaning |
|------|---------|
| `-32700` | Parse error |
| `-32600` | Invalid request |
| `-32601` | Method (tool) not found |
| `-32602` | Invalid params |
| `-32603` | Internal error |

---

## Rate limits

The vault is local-process; there are no rate limits at the MCP layer.

For Hive-backed tools that hit a remote server, the server enforces its own quota — surfaced as a JSON-RPC `-32603` with a `retry_after_seconds` hint.

---

## Privacy filter

Every tool that accepts user content (`vault_save`, `lore_search`, etc.) runs the **privacy filter** before persisting or transmitting:

- JWTs / API keys / OAuth tokens → `***REDACTED***`
- Email addresses → preserved domain, hashed local part
- IPv4 / IPv6 → preserved class, hashed last octet
- Configurable patterns via `~/.korva/config.json` → `privacy.patterns[]`

You can verify the filter on any input with the `privacy_check` tool:

```json
{ "name": "privacy_check", "arguments": { "text": "API_KEY=sk-12345" } }
// → "API_KEY=***REDACTED***"
```

---

## Scripting from the CLI

You don't have to use an MCP client. Every tool is also reachable from the CLI / curl:

```bash
# CLI
korva vault save --project my-app --type decision --title "..." --content "..."

# curl — direct REST
curl -X POST http://localhost:7437/api/v1/observations \
  -H "Content-Type: application/json" \
  -d '{ "project":"my-app", "type":"decision", "title":"...", "content":"..." }'
```

→ See [CLI.md](CLI.md) for the full command surface and [the OpenAPI spec](../vault/internal/api/openapi.yaml) for the REST reference.
