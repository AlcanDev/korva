# Phase 3 — Technical Design: Observatory

> Diseño técnico detallado: schemas SQL, contratos de endpoint con ejemplos,
> layout de archivos, algoritmos clave (atomic write, IDE probing), y el orden
> dependency-aware en que se implementa para llegar a smoke E2E lo antes posible.

## Estado de aprobación

- Phase 1: ✅
- Phase 2: ✅
- Phase 3 (esta): pendiente ✅ de Felipe.

---

## 1. Schema de base de datos

### 1.1 Tabla `interactions` (nueva)

Migration appended a `internal/db/migrations.go`. Idempotente.

```sql
CREATE TABLE IF NOT EXISTS interactions (
    id                TEXT PRIMARY KEY,                      -- ULID
    session_id        TEXT,                                  -- nullable, FK a sessions(id)
    project           TEXT NOT NULL DEFAULT '',
    team              TEXT NOT NULL DEFAULT '',
    agent             TEXT NOT NULL DEFAULT '',              -- claude|cursor|copilot
    model             TEXT NOT NULL DEFAULT '',              -- claude-opus-4-7, etc.
    prompt_excerpt    TEXT NOT NULL DEFAULT '',              -- max 8KiB tras filter
    response_excerpt  TEXT NOT NULL DEFAULT '',              -- max 8KiB tras filter
    input_tokens      INTEGER NOT NULL DEFAULT 0,
    output_tokens     INTEGER NOT NULL DEFAULT 0,
    cache_read        INTEGER NOT NULL DEFAULT 0,            -- cache_read_input_tokens
    cache_creation    INTEGER NOT NULL DEFAULT 0,            -- cache_creation_input_tokens
    duration_ms       INTEGER NOT NULL DEFAULT 0,
    tool_calls        TEXT NOT NULL DEFAULT '[]',            -- JSON array
    status            TEXT NOT NULL DEFAULT 'ok',            -- ok|error
    error_msg         TEXT NOT NULL DEFAULT '',
    estimated         INTEGER NOT NULL DEFAULT 0,            -- 0=real, 1=estimated
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_interactions_created_at  ON interactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_interactions_project     ON interactions(project, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_interactions_model       ON interactions(model);
CREATE INDEX IF NOT EXISTS idx_interactions_agent       ON interactions(agent);
CREATE INDEX IF NOT EXISTS idx_interactions_status      ON interactions(status);

-- FTS5 virtual table para búsqueda en prompts/responses
CREATE VIRTUAL TABLE IF NOT EXISTS interactions_fts USING fts5(
    prompt_excerpt, response_excerpt,
    content='interactions', content_rowid='rowid',
    tokenize='porter unicode61'
);

-- Triggers para mantener FTS sincronizada
CREATE TRIGGER IF NOT EXISTS interactions_ai AFTER INSERT ON interactions BEGIN
    INSERT INTO interactions_fts(rowid, prompt_excerpt, response_excerpt)
    VALUES (new.rowid, new.prompt_excerpt, new.response_excerpt);
END;
CREATE TRIGGER IF NOT EXISTS interactions_ad AFTER DELETE ON interactions BEGIN
    INSERT INTO interactions_fts(interactions_fts, rowid, prompt_excerpt, response_excerpt)
    VALUES ('delete', old.rowid, old.prompt_excerpt, old.response_excerpt);
END;
CREATE TRIGGER IF NOT EXISTS interactions_au AFTER UPDATE ON interactions BEGIN
    INSERT INTO interactions_fts(interactions_fts, rowid, prompt_excerpt, response_excerpt)
    VALUES ('delete', old.rowid, old.prompt_excerpt, old.response_excerpt);
    INSERT INTO interactions_fts(rowid, prompt_excerpt, response_excerpt)
    VALUES (new.rowid, new.prompt_excerpt, new.response_excerpt);
END;
```

### 1.2 Tabla `config_snapshots` (nueva)

```sql
CREATE TABLE IF NOT EXISTS config_snapshots (
    id           TEXT PRIMARY KEY,                       -- ULID
    actor        TEXT NOT NULL DEFAULT '',               -- "admin" | session_id
    scope        TEXT NOT NULL,                          -- "local" | "global"
    file_path    TEXT NOT NULL,                          -- ./korva.config.json | ~/.korva/config.json
    before_hash  TEXT NOT NULL DEFAULT '',               -- sha256 antes
    after_hash   TEXT NOT NULL DEFAULT '',               -- sha256 después
    before_json  TEXT NOT NULL DEFAULT '',               -- contenido completo previo (gz si > 8KiB sería overkill — texto crudo)
    after_json   TEXT NOT NULL DEFAULT '',               -- contenido completo nuevo
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_config_snapshots_created_at ON config_snapshots(created_at DESC);
```

> El snapshot duplica con `audit_logs` solo en intención. Audit logs registra **el evento** (action, target, hashes); `config_snapshots` guarda **el contenido** para permitir rollback. Ambos coexisten — el evento se inserta en audit_logs vía la maquinaria existente.

### 1.3 Migrations append

Se añaden al final de `var migrations` en `internal/db/migrations.go`. Sin tocar las existentes (regla de Korva: append-only).

---

## 2. Layout de archivos (Go backend)

### Nuevo

```
internal/
├── detect/
│   ├── ide.go                    Detector cross-platform (probing)
│   └── ide_test.go
└── config/
    ├── writer.go                 Atomic write + validate (NUEVO — el loader actual solo lee)
    └── writer_test.go

vault/
└── internal/
    ├── api/
    │   ├── system_status.go      GET /admin/system-status
    │   ├── system_status_test.go
    │   ├── config.go             GET/PUT /admin/config + restart
    │   ├── config_test.go
    │   ├── tokens.go             GET /admin/tokens/stats
    │   ├── tokens_test.go
    │   ├── activity.go           GET /admin/activity
    │   ├── activity_test.go
    │   ├── interactions_ingest.go POST /api/v1/interactions
    │   ├── interactions_ingest_test.go
    │   ├── sentinel_rules.go     GET/PUT /admin/sentinel/rules + POST /admin/sentinel/test
    │   └── sentinel_rules_test.go
    └── store/
        ├── interactions.go       Store ops para tabla interactions
        ├── interactions_test.go
        ├── config_snapshots.go   Store ops para config_snapshots
        └── config_snapshots_test.go

sentinel/validator/
└── internal/rules/
    ├── loader.go                 LoadFromYAML(path) ([]Rule, error)
    └── loader_test.go
```

### Modificado

| Archivo | Cambio |
|---------|--------|
| `internal/db/migrations.go` | Append migrations 1.1 y 1.2 |
| `internal/config/loader.go` | Exportar `ConfigPath(scope)` para `writer.go` (sin cambio de comportamiento) |
| `vault/internal/api/router.go` | Registrar 9 nuevas rutas (ver §3) |
| `vault/cmd/korva-vault/main.go` | Wire `cfg.Sentinel.RulesPath` al validador (cambio que ya estaba pendiente) |
| `sentinel/validator/cmd/.../main.go` | Si `RulesPath != ""`, mergear reglas YAML con profile |

### No tocados

`observations`, `sessions`, `mcp_calls`, `prompts`, `audit_logs`, `skills`, `private_scrolls`, `quality_checkpoints`, `cloud_outbox`, etc. Observatory **agrega**, no muta.

---

## 3. Contratos de endpoints

Todos los endpoints admin requieren `X-Admin-Key`. CORS y rate limit ya existentes aplican.

### 3.1 `GET /admin/system-status`

Una sola request, todo el dashboard. Sin parámetros.

```jsonc
// Response 200
{
  "ide": [
    {"name": "VS Code", "version": "1.95.3", "config_path": "/Users/.../Library/Application Support/Code/User", "has_korva_mcp": true, "is_default": false},
    {"name": "Cursor",  "version": "0.42.0", "config_path": "/Users/.../Library/Application Support/Cursor/User", "has_korva_mcp": true, "is_default": true}
  ],
  "vault":    {"running": true, "port": 7437, "pid": 12345, "uptime_sec": 3723, "version": "0.7.2"},
  "hive":     {"enabled": true, "endpoint": "https://hive.korva.dev", "last_sync_at": "2026-05-06T14:22:00Z", "pending_outbox": 3, "consecutive_errors": 0},
  "sentinel": {"enabled": true, "hooks_installed": ["pre-commit"], "rules_total": 14, "builtin_count": 10, "custom_count": 4, "rules_path": ".korva/sentinel-rules.yaml", "profile": "standard"},
  "lore":     {"active_scrolls": ["forge-sdd"], "available_scrolls_count": 7},
  "skills":   {"installed_count": 3, "last_sync_at": "2026-05-05T09:10:00Z", "sync_status": "ok"},
  "sessions": {"total": 142, "active_24h": 6},
  "observations": {"total": 1843, "by_type": {"pattern": 800, "decision": 320, "bugfix": 210, "...":  0}},
  "prompts":  {"total": 12},
  "license":  {"tier": "community", "expiration_at": null, "seats_used": 0, "seats_total": 0}
}
```

Errores: `500` si Vault no puede leer la DB. `IDE probing` falla en silencio (devuelve `[]`, log warning).

### 3.2 `GET /admin/config`

```jsonc
// Query: ?scope=local|global  (default: local; resuelve a CWD/korva.config.json o ~/.korva/config.json)
// Response 200
{
  "scope": "local",
  "path":  "/Users/AlcanDev/proyectos/korva/korva.config.json",
  "config": { /* KorvaConfig completo */ },
  "schema_version": "1"
}
```

### 3.3 `PUT /admin/config`

```jsonc
// Body
{
  "scope": "local",
  "config": { /* KorvaConfig completo o parcial — merge profundo */ }
}
// Response 200
{
  "status":      "saved",
  "snapshot_id": "01J9XZQ...",   // ULID de config_snapshots
  "restart_required": ["vault.port"],   // campos que requieren restart
  "diff":        {"vault.auto_start": [true, false]}   // before/after por campo
}
// Response 400
{"error": "validation failed: vault.port out of range (1024-65535)"}
// Response 401 si admin key faltante
// Response 409 si on-disk hash != hash en último snapshot (concurrencia)
```

Algoritmo de write (ver §6 detalle):

1. Leer archivo actual + sha256 → `before_hash`.
2. Validar el body merge contra schema.
3. Insertar snapshot con `before_*` y `after_*`.
4. Atomic write `<path>.tmp` → `fsync` → `rename(<path>.tmp, <path>)`.
5. Insertar entry en `audit_logs`.
6. Calcular `restart_required` comparando campos críticos (lista hardcoded).

### 3.4 `POST /admin/vault/restart`

```jsonc
// Sin body
// Response 202
{"status": "restarting", "old_pid": 12345}
```

Implementación: lee `~/.korva/vault/vault.pid`, manda `SIGTERM`, espera 5s grace, si sigue vivo `SIGKILL`, luego `exec` el binario con mismo flags. La response se manda **antes** del SIGTERM al actual proceso. En la práctica el frontend hace polling de `/healthz` para saber cuándo volvió.

### 3.5 `GET /admin/tokens/stats`

```jsonc
// Query: ?from=2026-04-01&to=2026-05-06&group_by=day|model|project
// Response 200
{
  "totals": {
    "input_tokens": 1280000,
    "output_tokens": 320000,
    "cache_read":   840000,
    "cache_creation": 120000,
    "interactions_count": 142,
    "estimated_count": 23
  },
  "cache_hit_pct": 0.66,
  "reduction_pct_estimated": 0.42,    // baseline naive vs actual
  "baseline_naive_tokens":  3050000,
  "by_model":   {"claude-opus-4-7": {...}, "claude-sonnet-4-6": {...}},
  "by_project": {"korva": {...}, "falabella-x": {...}},
  "daily": [
    {"date": "2026-05-01", "input_tokens": 12000, "output_tokens": 3000, "cache_read": 8000, "estimated": false},
    ...
  ]
}
```

`reduction_pct_estimated` se calcula con el `baseline_naive_tokens` que el server estima como `bytes_repo / 4` (sin contar `.git`, `node_modules`, `dist`). Es un proxy honesto, etiquetado `estimated`.

### 3.6 `GET /admin/activity`

```jsonc
// Query: ?project=&model=&agent=&q=&from=&to=&status=&limit=50&offset=0
// Response 200
{
  "interactions": [
    {
      "id": "01J9XZQ...",
      "ts": "2026-05-06T14:22:00Z",
      "project": "korva",
      "agent": "claude",
      "model": "claude-opus-4-7",
      "duration_ms": 1234,
      "input_tokens": 8200,
      "output_tokens": 950,
      "cache_read": 6100,
      "tool_calls_count": 3,
      "prompt_excerpt": "implementa el endpoint POST /admin/...",
      "status": "ok",
      "estimated": false
    }
  ],
  "total": 142,
  "limit": 50,
  "offset": 0
}
```

`q` usa FTS5 contra `interactions_fts`. Para el detalle completo (response, tool_calls), `GET /admin/activity/{id}` (no listado en spec, lo añado aquí — tabla 3.6.b).

### 3.6.b `GET /admin/activity/{id}` — detalle

```jsonc
// Response 200: el row completo + response_excerpt + tool_calls JSON
```

### 3.7 `POST /api/v1/interactions` (público, CORS, rate-limited)

Endpoint que cualquier IDE/wrapper llama tras un prompt completo.

```jsonc
// Body — campos de tokens opcionales (B con datos / C estimado server-side si faltan)
{
  "session_id": "01J...",       // opcional (vincula a sesión existente)
  "project":    "korva",
  "team":       "my-team",
  "agent":      "claude",
  "model":      "claude-opus-4-7",
  "prompt":     "implementa el endpoint POST /admin/...",   // se trunca a 8KiB tras filter
  "response":   "Listo. He creado...",                       // idem
  "usage": {
    "input_tokens":               8200,
    "output_tokens":              950,
    "cache_read_input_tokens":    6100,
    "cache_creation_input_tokens": 120
  },
  "duration_ms":  1234,
  "tool_calls":   [{"name": "Read", "args": {...}}, ...],
  "status":       "ok",
  "error_msg":    ""
}
// Response 201
{"id": "01J9XZQ...", "estimated": false}
// Response 400 si project/agent faltantes
```

Si `usage` ausente → server estima desde `len(prompt)/4` y `len(response)/4`, marca `estimated=1`.

Privacy filter aplicado a `prompt`, `response`, y a cada `args` de `tool_calls` antes de insertar. Truncamiento a 8 KiB para evitar abuso.

### 3.8 `GET /admin/sentinel/rules`

```jsonc
// Response 200
{
  "profile": "standard",
  "rules_path": ".korva/sentinel-rules.yaml",
  "builtin": [
    {"id": "HEX-001", "description": "Domain no importa de infrastructure/application", "severity": "error", "active_in_profile": true}
  ],
  "custom": [
    {"id": "CUSTOM-001", "description": "...", "severity": "error", "pattern": "...", "paths_include": [...], "paths_exclude": [...], "message": "..."}
  ]
}
```

### 3.9 `PUT /admin/sentinel/rules`

```jsonc
// Body
{
  "profile": "custom",
  "custom_rules": [
    {"id": "CUSTOM-001", "description": "...", "severity": "error", "pattern": "...", "paths_include": [...], "paths_exclude": [...], "message": "..."}
  ]
}
// Response 200
{"status": "saved", "rules_path": ".korva/sentinel-rules.yaml", "rules_count": 4}
```

Atomic write a `RulesPath` (default `.korva/sentinel-rules.yaml`). Compila cada regex antes de escribir; si alguno es inválido → 400 con el `id` problemático.

### 3.10 `POST /admin/sentinel/test`

```jsonc
// Body
{
  "rule": { /* mismo shape que custom_rules item */ },
  "code": "console.log('hello')\nfunction foo() { ... }",
  "file_path": "src/utils/foo.ts"
}
// Response 200
{
  "matches": [
    {"line": 1, "column": 1, "matched_text": "console.log", "message": "console.log no permitido en src/"}
  ]
}
```

Sin tocar disco. Eval en memoria.

---

## 4. Schema YAML para Sentinel rules

Archivo: `<repo-root>/.korva/sentinel-rules.yaml`. Path configurable via `sentinel.rules_path` en `korva.config.json`.

```yaml
version: 1
profile: custom        # minimal | standard | strict | custom
rules:
  - id: CUSTOM-001
    description: "No usar console.log en producción"
    severity: error                   # error | warning | info
    pattern: 'console\.(log|debug|info)'
    paths_include:
      - 'src/**/*.ts'
      - 'src/**/*.tsx'
    paths_exclude:
      - 'src/**/*.spec.ts'
      - 'src/**/*.test.tsx'
    message: "console.log no permitido en src/"
  - id: CUSTOM-002
    description: "..."
    severity: warning
    pattern: '...'
    message: "..."
```

Validación al cargar:
- `id` único, `^[A-Z][A-Z0-9-]{2,30}$`.
- `pattern` compila con `regexp.Compile`.
- `severity` ∈ `{error, warning, info}`.
- `paths_include`/`paths_exclude` cada uno valida con `doublestar.Match`.

---

## 5. Algoritmo: detección de IDEs

Función: `detect.IDEs() ([]IDE, error)` con 60s caching en memoria.

```go
type IDE struct {
    Name        string  // "VS Code"
    Version     string  // "1.95.3" (best-effort)
    ConfigPath  string  // path al user config dir
    HasKorvaMCP bool    // detecta si korva está configurado en su mcp.json/settings
    IsDefault   bool    // first-found wins
}
```

Algoritmo:

```
ides := []IDE{}
candidates := []probe{
    {name: "Claude Code", paths: ["~/.claude"], binary: "claude"},
    {name: "Cursor",      paths: ["<APP_SUPPORT>/Cursor/User", "%APPDATA%\\Cursor\\User"], binary: "cursor"},
    {name: "VS Code",     paths: ["<APP_SUPPORT>/Code/User",   "%APPDATA%\\Code\\User"],   binary: "code"},
    {name: "JetBrains",   paths: ["<APP_SUPPORT>/JetBrains",   "%APPDATA%\\JetBrains"],     binary: ""},
    {name: "Zed",         paths: ["~/.config/zed",             "<APP_SUPPORT>/Zed"],         binary: "zed"},
    {name: "Neovim",      paths: [],                                                         binary: "nvim"},
}

for _, c := range candidates:
    found := false
    for _, p := range c.paths:
        if exists(expand(p)):
            ides = append(ides, IDE{Name: c.name, ConfigPath: expand(p), HasKorvaMCP: detectKorvaMCP(c.name, p)})
            found = true
            break
    if !found && c.binary != "":
        if which(c.binary) != "":
            ides = append(ides, IDE{Name: c.name, ConfigPath: ""})

mark first ide as IsDefault
return ides
```

Detección de `HasKorvaMCP`:
- VS Code/Cursor: leer `<config>/mcp.json` (si existe) o `<config>/settings.json` y buscar `"korva"` o `"korva-vault"` en keys.
- Claude Code: leer `~/.claude/settings.json` y buscar `mcpServers.korva-vault`.
- JetBrains/Zed/Vim: best-effort, devolver `false` si no hay convención clara.

`<APP_SUPPORT>` resuelve por OS:
- macOS: `~/Library/Application Support`
- Linux: `~/.config`
- Windows: `%APPDATA%`

Cache: `var cache atomic.Value` con `{ides []IDE; expiresAt time.Time}`. TTL 60s. Mutex para race safety.

Tests: fixtures temporales con `t.TempDir()` simulando cada estructura, table-driven.

---

## 6. Algoritmo: atomic write de config

Función: `config.Write(scope, path string, cfg KorvaConfig) error`.

```
1. Read current file → currentBytes (or "" si not exists)
2. computedBeforeHash = sha256(currentBytes)
3. Validate cfg (schema + invariantes:
     - port in [1024, 65535]
     - country en lista ISO-3166
     - scrolls existen on-disk
     - private_patterns bien-formados
   )
   → si falla: return ValidationError
4. newBytes = json.MarshalIndent(cfg, "", "  ")
5. computedAfterHash = sha256(newBytes)
6. tmpPath = path + ".tmp." + ulid()
7. f := os.OpenFile(tmpPath, O_WRONLY|O_CREATE|O_EXCL, 0644)
8. f.Write(newBytes)
9. f.Sync()        // fsync
10. f.Close()
11. os.Rename(tmpPath, path)   // atomic en POSIX y NTFS
12. return nil
```

Defer cleanup: `os.Remove(tmpPath)` si error después del paso 6.

Concurrencia: si `currentBytes` cambió entre que el frontend hizo GET y PUT, devolver `409 Conflict` con `expected_hash`/`actual_hash`. El frontend re-fetch y vuelve a intentar.

---

## 7. Layout Beacon (frontend)

### Nuevo

```
beacon/src/
├── api/
│   └── observatory.ts                Hooks TanStack Query
├── pages/
│   └── observatory/
│       ├── Observatory.tsx           Layout + sub-router
│       ├── SystemHealth.tsx          /observatory
│       ├── TokenAnalytics.tsx        /observatory/tokens
│       ├── ActivityTimeline.tsx      /observatory/activity
│       ├── ConfigEditor.tsx          /observatory/config
│       ├── SentinelRulesEditor.tsx   /observatory/sentinel
│       ├── components/
│       │   ├── StatusCard.tsx
│       │   ├── ConfigField.tsx       Render condicional por type
│       │   ├── RestartBanner.tsx
│       │   ├── InteractionRow.tsx
│       │   ├── InteractionDrawer.tsx
│       │   ├── RuleCard.tsx
│       │   ├── RuleEditorForm.tsx
│       │   └── RuleTestPlayground.tsx
│       └── __tests__/
│           ├── SystemHealth.test.tsx
│           ├── ConfigEditor.test.tsx
│           └── SentinelRulesEditor.test.tsx
└── (modificar)
    ├── pages/admin/Admin.tsx         Añade item "Observatory" al sidebar
    └── App.tsx                       Registra ruta `/observatory/*`
```

### Hook contracts (typed)

```ts
// observatory.ts
export const useSystemStatus = () => useQuery<SystemStatus>(...);
export const useConfig = (scope: "local" | "global") => useQuery<ConfigResponse>(...);
export const useUpdateConfig = () => useMutation<UpdateConfigResponse, Error, UpdateConfigBody>(...);
export const useTokenStats = (params: TokenStatsParams) => useQuery<TokenStats>(...);
export const useActivity = (params: ActivityParams) => useQuery<ActivityResponse>(...);
export const useInteraction = (id: string) => useQuery<InteractionDetail>(...);
export const useSentinelRules = () => useQuery<SentinelRules>(...);
export const useUpdateSentinelRules = () => useMutation<UpdateRulesResponse, Error, UpdateRulesBody>(...);
export const useTestSentinelRule = () => useMutation<TestRuleResponse, Error, TestRuleBody>(...);
export const useRestartVault = () => useMutation<RestartResponse, Error, void>(...);
```

Tipos generados manualmente (no autogen — son ~100 líneas y evitamos toolchain extra).

### ConfigField — render por tipo

| Tipo en schema | Componente |
|----------------|-----------|
| `bool` | `<input type="checkbox" />` |
| `int` (port, minutes) | `<input type="number" min max />` |
| `string` (free) | `<input type="text" />` |
| `string` (path) | `<input type="text">` con botón "browse" (deshabilitado v1) |
| `enum` | `<select>` |
| `[]string` (chips) | chip-input componente custom (sin libs) |
| `[]string` (dual-list) | dos `<select multiple>` lado-a-lado para active_scrolls |

---

## 8. Orden de implementación (dependency-aware)

Para llegar a smoke E2E lo antes posible, no en cascada.

| # | Item | Depende de | Habilita |
|---|------|-----------|----------|
| 1 | Migrations (interactions + config_snapshots) | nada | resto |
| 2 | `store/interactions.go` + tests | 1 | 4, 5 |
| 3 | `store/config_snapshots.go` + tests | 1 | 7 |
| 4 | `api/interactions_ingest.go` POST /api/v1/interactions | 2 | smoke parcial |
| 5 | `api/activity.go` GET /admin/activity | 2 | UI activity |
| 6 | `internal/detect/ide.go` + tests | nada | 8 |
| 7 | `internal/config/writer.go` + tests | 3 | 9 |
| 8 | `api/system_status.go` | 6 | UI health |
| 9 | `api/config.go` | 7 | UI config |
| 10 | `api/tokens.go` | 2 | UI tokens |
| 11 | `sentinel/validator/.../rules/loader.go` + tests | nada | 12, validador |
| 12 | `api/sentinel_rules.go` | 11 | UI sentinel |
| 13 | Wire `cfg.Sentinel.RulesPath` en validator main.go | 11 | E2E sentinel |
| 14 | `api/router.go` registra rutas | 4-13 | UI funcional |
| 15 | Beacon `observatory.ts` hooks | 14 | UI |
| 16 | Beacon SystemHealth.tsx + StatusCard | 15 | smoke S1 |
| 17 | Beacon ConfigEditor.tsx + ConfigField | 15 | smoke S4 |
| 18 | Beacon ActivityTimeline.tsx + Drawer | 15 | smoke S3 |
| 19 | Beacon TokenAnalytics.tsx | 15 | smoke S2 |
| 20 | Beacon SentinelRulesEditor.tsx + Playground | 15 | smoke S5 |
| 21 | `api/restart.go` POST /admin/vault/restart | 14 | UI restart |
| 22 | Smoke script `scripts/smoke-observatory.sh` | 14, 16-20 | AC verification |
| 23 | Tests Vitest Beacon | 16-20 | AC13 |
| 24 | E2E manual + 05-verification.md | 22 | AC19 |

**Smoke parcial alcanzable tras item 4**: `curl POST /api/v1/interactions` + verificar row en SQLite.
**Smoke completo de UI tras item 16-20**: cada pantalla independiente — no esperar a todo el frontend.

---

## 9. Compatibility matrix

| Cambio | Breaking? |
|--------|----------|
| Migrations añadidas | No (idempotentes) |
| Nuevos endpoints | No (rutas nuevas, no modifican existentes) |
| `KorvaConfig` schema | No campos removidos; los nuevos opcionales; el writer ignora keys desconocidas |
| Sentinel `RulesPath` activado | No (campo ya existía, ahora se respeta — comportamiento más generoso) |
| Beacon nueva sección | No (se añade al sidebar) |

Ningún cambio rompe instalaciones existentes ni el CLI.

---

## 10. Tests — checklist por archivo

### Backend Go

| Archivo | Casos mínimos |
|---------|---------------|
| `store/interactions_test.go` | save+get, save sin tokens (estimated=1), search FTS, paginación, privacy filter aplicado, retention purge |
| `store/config_snapshots_test.go` | insert, list paginado, latest by scope |
| `api/interactions_ingest_test.go` | 201 happy, 400 missing project, privacy mask en prompt, truncamiento 8KiB, estimated cuando usage falta |
| `api/activity_test.go` | filtros por project/model/agent, FTS query, paginación, detalle |
| `api/system_status_test.go` | 200 con todos los campos, IDE empty si nada detectado, hive nil-safe |
| `api/config_test.go` | GET local/global, PUT happy, PUT validación falla, PUT 409 si hash diff, atomic fault injection (kill antes de rename) |
| `api/tokens_test.go` | totals, group_by, baseline_naive_tokens calc, daily buckets vacíos |
| `api/sentinel_rules_test.go` | GET, PUT con regex inválido (400), POST test playground match correcto |
| `detect/ide_test.go` | macOS fixture, linux fixture, windows-style fixture (paths), cache TTL |
| `config/writer_test.go` | atomic happy, validation fail (port range), concurrent diff hash 409, fsync called |
| `sentinel/validator/.../loader_test.go` | YAML válido, YAML inválido (regex bad), missing file → []rule vacío + nil error, merge con profile |

### Frontend Beacon

| Archivo | Casos mínimos |
|---------|---------------|
| `api/observatory.test.ts` | mock fetch happy y error, retry off, types correctos |
| `pages/observatory/__tests__/SystemHealth.test.tsx` | render loading, error, loaded, IDE list rendered |
| `pages/observatory/__tests__/ConfigEditor.test.tsx` | render por sección, edit + submit invoca mutation, restart-banner aparece para vault.port |
| `pages/observatory/__tests__/SentinelRulesEditor.test.tsx` | add rule, delete rule, test playground submit |

### Integration

| Script | Comprueba |
|--------|----------|
| `scripts/smoke-observatory.sh` | Items: POST 3 interactions, PUT config, GET stats, ver `korva.config.json` modificado, ver fila en `config_snapshots` |

---

## 11. Documentos / artefactos derivados

- `forge/observatory/04-implementation.md` — log de cambios reales por commit (al final de Phase 4).
- `forge/observatory/05-verification.md` — checklist E2E manual con 20 ACs, marcado en Phase 5.
- README sección "Observatory" — pequeño párrafo + screenshot (Phase 5).

---

## 12. Riesgos técnicos detallados

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|--------------|---------|------------|
| `interactions` crece > 1GB en 6 meses | media | media | retention_days config + `VACUUM` automático; default 30 días |
| FTS5 corrupción si crash mid-trigger | baja | alto | `PRAGMA integrity_check` al boot del Vault; rebuild si falla |
| IDE detection falsos positivos (carpeta vacía abandonada) | media | bajo | requerir al menos 1 archivo conocido (ej. `User/settings.json`) |
| `Restart Vault` deja MCP huérfanos | baja | media | grace 5s + send signal a clients via channel close (tools.go) |
| YAML parse rompe si user mete BOM | baja | bajo | `bytes.TrimPrefix(\xef\xbb\xbf)` antes de unmarshal |
| Atomic rename no atómico en exFAT | baja | media | docs: bind solo a APFS/ext4/NTFS (no recomendado exFAT) |

---

## Próximo paso

Esperar ✅ de Felipe sobre este diseño. Tras aprobación arranco Phase 4
siguiendo el orden de §8, con commits pequeños por item para que puedas
revisar en pedazos digeribles.
