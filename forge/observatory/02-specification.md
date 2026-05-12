# Phase 2 — Specification: Observatory

> Especificación funcional con alcance ampliado: paridad completa con
> el CLI para configuración, editor de reglas Sentinel desde web,
> tracking de tokens real (B) + fallback estimado (C), y suite de tests
> que garantice estabilidad.

## Estado de aprobación

- Phase 1 (Exploration): ✅ aprobada
- Phase 2 (esta): pendiente ✅ de Felipe antes de Phase 3.

## Alcance del MVP

Cinco capacidades end-to-end visibles en Beacon bajo `/observatory`:

### S1 — System Health
**Pantalla**: `/observatory`

Muestra el estado de la instalación en cards. Carga en una sola request a `GET /admin/system-status`.

| Card | Datos |
|------|-------|
| IDE detectados | lista de `[{name, version?, path, is_default}]` autodetectados por filesystem probing |
| Vault | running/stopped, port, PID, uptime_sec, version |
| Hive | enabled, endpoint, last_sync_at, pending_outbox, consecutive_errors |
| Sentinel | enabled, hooks_installed[], rules_total, custom_rules_count |
| Lore | active_scrolls[], available_scrolls_count |
| Skills | installed_count, last_sync_at, sync_status |
| Sessions | total, active_24h |
| Observations | total, by_type{...} |
| Prompts | total |
| License | tier (community/teams), expiration_at, seats_used/seats_total |

### S2 — Token Analytics
**Pantalla**: `/observatory/tokens`

Combina **tokens reales** (cuando el cliente reporta vía `POST /api/v1/interactions` con `usage` del SDK Anthropic) y **estimados** (fallback server-side).

| Visualización | Fuente |
|---------------|--------|
| Tokens hoy / 7d / 30d (input + output) | `interactions.input_tokens + output_tokens` agregados |
| % Cache hit ratio | `cache_read_input_tokens / (input_tokens + cache_read_input_tokens)` |
| Tokens ahorrados por cache | `sum(cache_read_input_tokens)` |
| Reducción vs baseline naive (estimado C) | `1 - (sum(input_tokens) / estimated_baseline)` donde baseline = bytes del repo en chars/4 |
| Tokens por modelo | `group by model` |
| Tokens por proyecto | `group by project` |
| Trend gráfico (line) | bucket diario últimos 30 días |

Los datos estimados se etiquetan como `estimated: true` en el response para que el UI los muestre con tinte distinto.

### S3 — Activity Timeline
**Pantalla**: `/observatory/activity`

Tabla virtualizada con feed de interacciones recientes.

Columnas: `ts | project | model | duration_ms | tokens (in/out/cache) | tool_calls_count | prompt_excerpt(120ch) | status`.

Filtros: project, model, agent, fecha desde/hasta, status (ok/error). Búsqueda full-text en `prompt_excerpt` y `response_excerpt` (FTS5 SQLite).

Click en row → drawer con prompt completo, response completo, tool_calls JSON formateado. Privacy filter aplicado al guardar (passwords/tokens enmascarados).

Endpoint: `GET /admin/activity?project=&model=&from=&to=&q=&limit=&offset=`.

### S4 — Configuration Editor
**Pantalla**: `/observatory/config`

Editor con paridad **completa** con `korva config set`. Sub-tabs por sección de `KorvaConfig`. Cada toggle/input dispara validación local (JSON Schema) y un `PUT /admin/config` que persiste con atomic write.

#### Tabs y campos

| Tab | Campo | Tipo | Default | Comentario |
|-----|-------|------|---------|------------|
| **General** | project | string | (cwd) | nombre proyecto |
| | team | string | "" | team_id |
| | country | string | "CL" | ISO-3166 |
| | agent | enum | copilot | copilot/claude/cursor |
| **Vault** | port | int | 7437 | restart_required: true |
| | auto_start | bool | true | |
| | sync_repo | string | "" | git URL |
| | sync_branch | string | "" | |
| | auto_sync | bool | false | |
| | sync_interval_minutes | int | 0 | Teams |
| | private_patterns | string[] | preset | chip-input |
| | retention_days | int | 0 | Teams |
| | webhook_url | string | "" | Teams |
| **Lore** | active_scrolls | string[] | ["forge-sdd"] | dual-list selector |
| | scroll_priority | enum | private_first | |
| **Sentinel** | enabled | bool | true | |
| | hooks | string[] | ["pre-commit"] | multi-select |
| | rules_path | string | "" | file picker |
| | block_on_violation | bool | true | |
| **Hive** | enabled | bool | true | |
| | endpoint | string | (preset) | |
| | interval_minutes | int | 15 | |
| | allowed_types | string[] | preset | chip-input |
| | reject_patterns | string[] | [] | |
| **License** | activation_url | string | (preset) | |
| **Profiles** | local_or_global | toggle | local | switch que selecciona archivo destino |

Cambios que requieren restart del Vault muestran banner "Cambios pendientes — reinicia Vault" con botón `Restart` que invoca `POST /admin/vault/restart` (kill + start vía PID en `~/.korva/vault/vault.pid`).

#### Restricciones de mutación

- `version`, `team` (cuando ya hay sesión activa), y `license.*` solo editables vía CLI (no expuestos en UI). Razón: invariantes críticos.
- `port`: validación `1024-65535`.
- `private_patterns`: cada item filtrado contra blacklist trivia (no aceptar `*` solo).
- `active_scrolls`: validar que cada nombre exista en `~/.korva/lore/{public,private}/`.

### S5 — Sentinel Rules Editor
**Pantalla**: `/observatory/sentinel`

Editor para reglas custom + selección de profile. Las 10 reglas built-in (HEX-001 a TEST-001) son **read-only** — se muestran con su descripción y severity, pero no se editan; eso requiere cambio en código Go.

Lo editable:
- **Profile**: `minimal | standard | strict | custom`. Si `custom`, se materializa el listado en `.korva/sentinel-rules.yaml`.
- **Custom rules**: lista YAML de reglas regex-based:
  ```yaml
  rules:
    - id: CUSTOM-001
      description: "No usar console.log en producción"
      severity: error
      pattern: 'console\.(log|debug|info)'
      paths_include: ['src/**/*.ts']
      paths_exclude: ['src/**/*.spec.ts']
      message: "console.log no permitido en src/"
  ```
- **Test playground**: textarea para pegar código, dropdown de regla, botón "Test" → llama `POST /admin/sentinel/test` con `{rule_id, code, file_path}` → devuelve matches.

#### Cambio de backend requerido

Hoy `SentinelConfig.RulesPath` está declarado en `internal/config/schema.go:62` pero el validador `korva-sentinel` ignora el field y siempre usa `rules.RulesForProfile()`. Cambio mínimo:

1. En `sentinel/validator/internal/rules/loader.go` (nuevo): función `LoadFromYAML(path string) ([]Rule, error)`.
2. En `sentinel/validator/cmd/.../main.go`: si `cfg.Sentinel.RulesPath != ""`, mergear las reglas YAML con las del profile.
3. Tests table-driven con fixtures YAML inválido/válido.

## Capacidades fuera del MVP (Post-Observatory v1)

Documentadas explícitamente para no scope-creep:

- Cost USD por modelo (requiere price table mantenida).
- Editor visual de reglas Sentinel con AST (las custom YAML del MVP son regex-only).
- Hot-reload con SSE/WebSocket (el MVP requiere refresh manual o navegación).
- Comparativa "sin Korva vs con Korva" como benchmark formal (requiere baseline run sin Korva).
- Multi-tenant view (varios installs visibles simultáneamente).
- Editor de skills inline (ya existe en `/admin/skills`, no se duplica).

## Endpoints nuevos (contrato preliminar — el detalle va en Phase 3)

| Método | Path | Auth | Body / Query | Respuesta |
|--------|------|------|--------------|-----------|
| GET | `/admin/system-status` | admin | — | `{ide:[], vault:{}, hive:{}, sentinel:{}, lore:{}, skills:{}, sessions:{}, observations:{}, prompts:{}, license:{}}` |
| GET | `/admin/config` | admin | `?scope=local|global` | `KorvaConfig` |
| PUT | `/admin/config` | admin | `KorvaConfig` parcial + scope | `{status, snapshot_id}` |
| POST | `/admin/vault/restart` | admin | — | `{status, new_pid}` |
| GET | `/admin/tokens/stats` | admin | `?from=&to=&group_by=` | `{totals, by_model, by_project, daily, estimated}` |
| GET | `/admin/activity` | admin | `?project=&model=&q=&limit=&offset=` | `{interactions:[], total}` |
| POST | `/api/v1/interactions` | público (CORS) | `{project, model, agent, prompt_excerpt, response_excerpt, input_tokens, output_tokens, cache_read_input_tokens?, cache_creation_input_tokens?, duration_ms, tool_calls?}` | `{id}` |
| GET | `/admin/sentinel/rules` | admin | — | `{builtin:[], custom:[], profile, rules_path}` |
| PUT | `/admin/sentinel/rules` | admin | `{profile?, custom_rules:[]}` | `{status, rules_path}` |
| POST | `/admin/sentinel/test` | admin | `{rule_id, code, file_path}` | `{matches:[{line, column, message}]}` |

## Nuevas tablas SQLite

| Tabla | Propósito |
|-------|-----------|
| `interactions` | timeline de prompts ejecutados con tokens reales/estimados |
| `config_snapshots` | snapshot de `korva.config.json` antes de cada PUT (para rollback) |

Schema detallado en Phase 3.

## Detección de IDEs (paths a probar)

Cross-platform, en orden de prioridad:

| IDE | macOS | Linux | Windows |
|-----|-------|-------|---------|
| VS Code | `~/Library/Application Support/Code/User/` + `which code` | `~/.config/Code/User/` | `%APPDATA%\Code\User\` |
| Cursor | `~/Library/Application Support/Cursor/User/` + `which cursor` | `~/.config/Cursor/User/` | `%APPDATA%\Cursor\User\` |
| Claude Code | `~/.claude/` (config) + `which claude` | idem | `%USERPROFILE%\.claude\` |
| JetBrains (IDEA/PyCharm/GoLand/WebStorm) | `~/Library/Application Support/JetBrains/` | `~/.config/JetBrains/` | `%APPDATA%\JetBrains\` |
| Zed | `~/.config/zed/` + `which zed` | idem | `%APPDATA%\Zed\` |
| Neovim/Vim | `which nvim` / `which vim` | idem | `where.exe nvim` |

Output: lista de objetos `{name, version (best effort via --version), config_path, has_korva_mcp_configured}`.

## Criterios de aceptación

Cada criterio es verificable manual o automáticamente.

| ID | Criterio | Cómo se verifica |
|----|----------|------------------|
| AC1 | `/observatory` carga y muestra al menos: IDE, Vault, Hive, Sentinel, Lore con valores no vacíos | Manual + smoke test |
| AC2 | Editar `vault.auto_start` desde UI persiste en `korva.config.json` y crea snapshot en `config_snapshots` | `curl PUT` + diff archivo |
| AC3 | Auto-detect identifica VS Code y Cursor presentes en macOS dev box | `go test detect/ide_test.go` |
| AC4 | `POST /api/v1/interactions` con `usage` válido aparece en `/observatory/tokens` < 2s | `curl POST` + reload |
| AC5 | `POST /api/v1/interactions` sin `input_tokens` → fallback estimado se etiqueta `estimated: true` | unit test |
| AC6 | Cambio en `private_patterns` desde UI bloquea efectivamente un save de observation con esa palabra | E2E test |
| AC7 | Custom rule YAML que matchea `console\.log` en `src/**/*.ts` falla pre-commit que contiene esa línea | E2E test sentinel |
| AC8 | `POST /admin/sentinel/test` con regla CUSTOM-001 + código de prueba devuelve matches correctos | unit test |
| AC9 | Restart Vault desde UI termina el PID actual y arranca uno nuevo, manteniendo DB | smoke test |
| AC10 | Atomic write: corrupción simulada (kill -9 entre `.tmp` write y rename) no deja archivo parcial | unit test fault-injection |
| AC11 | Beacon: editar config y refrescar la página muestra los nuevos valores (persistencia confirmada) | manual |
| AC12 | `go test github.com/alcandev/korva/...` pasa con cobertura ≥ 70% en archivos nuevos | `go test -cover` |
| AC13 | Beacon `npm test` pasa con ≥ 70% cobertura en archivos nuevos | `vitest --coverage` |
| AC14 | Privacy filter masking: prompt con "password=abc123" guardado en `interactions` aparece como "password=***" | unit test |
| AC15 | UI compatible con dark mode existente de Beacon | manual |
| AC16 | Acceso `/admin/config` sin `X-Admin-Key` retorna 401 | smoke test |
| AC17 | `PUT /admin/config` con JSON inválido retorna 400 con mensaje claro, no escribe nada | unit test |
| AC18 | UI muestra banner "restart required" si el campo cambiado lo requiere (`vault.port`, `sentinel.hooks`) | manual |
| AC19 | Documento de verificación `05-verification.md` con checklist marcado | revisión |
| AC20 | `golangci-lint run` y `biome lint` pasan sin warnings nuevos | CI |

## Estrategia de tests

### Backend Go (table-driven, in-memory SQLite)
- `vault/internal/store/interactions_test.go` — Save/Search/Stats con casos: tokens reales, estimados, missing fields, privacy filter aplicado.
- `vault/internal/api/system_status_test.go` — handler con stub de detect/ide y stub de hive worker.
- `vault/internal/api/config_test.go` — GET/PUT/scope local vs global, atomic write fault injection, validación schema.
- `vault/internal/api/activity_test.go` — paginación, filtros, FTS query.
- `vault/internal/api/sentinel_rules_test.go` — load YAML, test rule contra fixtures.
- `internal/detect/ide_test.go` — fixtures temp dirs por OS, table-driven matrix.
- `sentinel/validator/internal/rules/loader_test.go` — YAML válido/inválido/parcial.

### Frontend (Vitest + Testing Library)
- `beacon/src/api/observatory.test.ts` — hooks `useSystemStatus`, `useConfig`, `useUpdateConfig`, mocks de fetch.
- `beacon/src/pages/observatory/__tests__/SystemHealth.test.tsx` — render con states loading/error/loaded.
- `beacon/src/pages/observatory/__tests__/ConfigForm.test.tsx` — submit happy path, validación, restart-required banner.
- `beacon/src/pages/observatory/__tests__/SentinelRulesEditor.test.tsx` — add/edit/delete rule, test playground.

### Integration (docker-compose ya existe)
- Smoke script `scripts/smoke-observatory.sh`:
  1. `docker-compose up -d vault`
  2. `curl POST /api/v1/interactions` (3 con tokens, 2 sin)
  3. `curl GET /admin/tokens/stats` — assert totals
  4. `curl PUT /admin/config` con `vault.auto_start=false`
  5. `cat korva.config.json | jq .vault.auto_start` — assert `false`

### E2E manual (documentado en `05-verification.md`)
Flujo completo: `korva init` → abrir Beacon → ver Observatory → cambiar setting → ver reflejado en `~/.korva/config.json` → triggear interaction desde Claude Code → ver en activity timeline → cambiar regla Sentinel → triggear commit con violación → confirmar bloqueo.

## Dependencias y trabajo previo

- **No hay dependencias bloqueantes externas.**
- Bibliotecas Go a evaluar: `gopkg.in/yaml.v3` para Sentinel rules loader (ya transitivamente disponible vía dependencias actuales — confirmar en Phase 3).
- Bibliotecas Beacon: ninguna nueva. Forms se hacen con primitives + validación TypeScript del schema generado desde Go (script de export `make schema-export`).
- Migration SQLite: dos nuevas tablas (`interactions`, `config_snapshots`). Idempotente, sin tocar tablas existentes.

## Riesgos identificados

| Riesgo | Mitigación |
|--------|-----------|
| Inflación del schema de `KorvaConfig` rompe compatibilidad | Solo se añaden campos opcionales; `omitempty` por default |
| Privacy filter no enmascara campos custom de `interactions` | Aplicar `privacy.Filter` a `prompt_excerpt`, `response_excerpt` y `tool_calls` antes del INSERT |
| Endpoint público `POST /api/v1/interactions` se usa para spam | Reusar rate limiter existente (120 req/min por IP) |
| `PUT /admin/config` corrompe el archivo si la app crashea | Atomic write `.tmp + fsync + rename` + validación schema antes |
| IDE detection es lenta en máquinas con FS lento | Cachear resultado por 60s en memoria del Vault |
| Restart Vault desde UI deja sesiones MCP huérfanas | El restart respeta los clientes MCP conectados (timeout grace 5s) |
| `interactions` crece sin límite | retention_days del config aplica también a esta tabla; default 30 días si flag no está |

## Non-goals explícitos

- No reemplazar `/admin/sessions`, `/admin/interactions` (mcp_calls), `/admin/audit`, `/admin/skills`, `/admin/scrolls/private`, `/admin/code-health`, `/admin/teams`, `/admin/license` — siguen viviendo donde están. Observatory es una **agregación + nuevo nivel**.
- No reemplazar el CLI. Lo extiende con paridad para los flujos comunes; `korva config set` sigue siendo válido.
- No instala IDEs ni los configura — solo detecta y reporta. La instalación sigue siendo `korva setup`.
- No edita reglas built-in de Sentinel. Solo profile + custom YAML.

## Próximo paso

Esperar ✅ de Felipe sobre esta especificación. Tras aprobación, escribo
`03-design.md` con: schemas SQL completos, contratos JSON Schema de cada
endpoint, layout de archivos nuevos, plan de migration y orden de
implementación dependency-aware.
