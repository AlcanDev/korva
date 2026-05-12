# Phase 1 — Exploration: Observatory

> Dashboard estadístico ampliado + edición de configuración desde web,
> integrado a Beacon. Reemplaza la mezcla actual de pantallas dispersas
> por una vista unificada de salud del sistema, métricas de uso
> y mutación segura de `korva.config.json`.

## Objetivo del feature

Que cualquier persona que instale Korva pueda, desde Beacon en local:

1. Ver el **estado de su instalación** de un vistazo (IDE detectado, Vault corriendo, Hive activo o no, Sentinel habilitado, scrolls/skills cargados, sesiones activas, observaciones almacenadas).
2. Ver **estadísticas reales** de uso: tokens consumidos, % de reducción de tokens vs baseline, observaciones por tipo, prompts ejecutados por proyecto, latencias MCP.
3. **Activar/desactivar features y ajustar configuración** desde la UI, persistido contra `korva.config.json` con validación y atomic write.
4. Ver un **timeline de actividad** estilo "en proyecto X corrió prompt Y a las HH:MM y devolvió Z" con tokens y duración.

## Found — qué ya existe

### Beacon (React 19 + Vite 6)

Ubicación: `/Users/AlcanDev/proyectos/korva/beacon/`.
Ya tiene 14 páginas admin enrutadas:

| Página | Archivo | Qué muestra |
|--------|---------|-------------|
| Dashboard | `src/pages/admin/AdminDashboard.tsx` | KPIs (obs, sesiones, proyectos), daily activity, top proyectos, tipos de obs |
| Vault | `src/pages/admin/AdminVault.tsx` | Search/filter/delete observaciones |
| Sessions | `src/pages/admin/AdminSessions.tsx` | Lista de sesiones con duration_min, obs_count |
| Prompts | `src/pages/admin/AdminPrompts.tsx` | CRUD de prompts con tags |
| Scrolls | `src/pages/admin/AdminScrolls.tsx` | Toggles **locales** (no persisten al backend) |
| Skills | `src/pages/admin/AdminSkills.tsx` | Editor + history + sync (Teams) |
| Interactions | `src/pages/admin/AdminInteractions.tsx` | Stats de tool calls (latency, success rate, by_tool) |
| Audit | `src/pages/admin/AdminAudit.tsx` | Log de mutaciones admin |
| Code Health | `src/pages/admin/AdminCodeHealth.tsx` | Score per-project |
| Teams / License / Scrolls Private | varios | menores |

API client en `src/api/`: `admin.ts`, `audit.ts`, `codeHealth.ts`, `interactions.ts`, `license.ts`, `skills.ts`, `teams.ts`, `vault.ts`.

### Vault HTTP API (puerto 7437)

Router en `vault/internal/api/router.go`:

- Públicos: `/healthz`, `/api/v1/status`, `/api/v1/metrics`, `/api/v1/stats`, `/api/v1/sessions`, `/api/v1/observations`, `/api/v1/search`, `/api/v1/timeline/{project}`, `/api/v1/summary/{project}`, `/api/v1/hive/status`.
- Admin (`X-Admin-Key`): `/admin/stats`, `/admin/sessions`, `/admin/prompts`, `/admin/skills`, `/admin/scrolls/private`, `/admin/audit`, `/admin/interactions`, `/admin/code-health`, `/admin/license/*`, `/admin/teams/*`, `/admin/hive/*`, `/admin/purge`, `/admin/export`.

### Modelo de datos SQLite (13 tablas core)

| Tabla | Campos relevantes para Observatory |
|-------|------------------------------------|
| `observations` | id, project, team, type, title, content, content_hash, created_at |
| `sessions` | id, project, team, **agent**, goal, summary, started_at, ended_at |
| `mcp_calls` | id, **tool**, project, author, status, **latency_ms**, error_msg, created_at |
| `prompts` | id, name, content, tags, created_at |
| `skill_activations` | id, skill_id, team_id, project, prompt_hash, match_score, activated_at |
| `quality_checkpoints` | id, project, phase, score, gate_passed, created_at |
| `audit_logs` | id, actor, action, target, before_hash, after_hash |
| `cloud_outbox` | id, observation_id, status (pending/sent/...), attempts |

Schema y migrations: `internal/db/migrations.go`. ULIDs vía `oklog/ulid`. Driver `modernc.org/sqlite` (pure Go).

### Config de Korva

- Archivo: `korva.config.json` en raíz del repo del usuario (también `.korva/config.json` global).
- Schema: `internal/config/schema.go` — struct `KorvaConfig` con campos `Project`, `Team`, `Country`, `Agent`, `Vault`, `Lore`, `Sentinel`, `Hive`, `License`.
- Loader: `internal/config/loader.go` (lee, no expone writer público hoy).
- Path resolver: `internal/config/PlatformPaths()`.

### Detección actual

- `vault/internal/detect/detect.go` detecta **proyecto** (git remote, git root, dir basename).
- **No** detecta IDEs instalados.
- `sessions.agent` se llena manual al hacer `vault_session_start` con valor `claude` / `cursor` / `copilot`.

## Impact — qué tocaría este feature

### Backend (Go)

- **Nueva tabla** `interactions` (extiende lo que `mcp_calls` no captura): contenido del prompt, contenido de la respuesta, tokens (input/output/cache), modelo, duración. `mcp_calls` se mantiene tal cual — es ortogonal (uno es por tool MCP, el otro por prompt completo).
- **Nuevo paquete** `internal/detect/ide.go` con filesystem probing.
- **Nuevos endpoints** en `vault/internal/api/`:
  - `GET /admin/system-status` — agrega IDE + Vault + Hive + Sentinel + counts.
  - `GET /admin/config` — devuelve `korva.config.json` actual.
  - `PUT /admin/config` — valida schema, atomic write `.tmp + rename`.
  - `GET /admin/tokens/stats` — agrega de tabla `interactions` con buckets temporales.
  - `GET /admin/activity` — timeline filtrable por proyecto, modelo, fecha.
  - `POST /api/v1/interactions` — endpoint público que el cliente MCP llama tras cada prompt para registrar tokens.
- **Hot-reload** del config via `fsnotify` en el loader (post-MVP).

### Frontend (Beacon)

- **Nueva sección** `/observatory` en el sidebar de admin con 4 sub-rutas:
  - `/observatory` — System Health (cards de estado IDE/Vault/Hive/Sentinel + counts).
  - `/observatory/tokens` — Token analytics (gráfico tokens consumidos por día, % cache hit, % reducción vs baseline naive).
  - `/observatory/activity` — Timeline prompts con filtros.
  - `/observatory/config` — Form schema-driven con toggles para los settings principales.
- **Nuevos hooks** en `src/api/`: `observatory.ts` con `useSystemStatus`, `useConfig`, `useUpdateConfig`, `useTokenStats`, `useActivityTimeline`.
- **Nuevo componente** `ConfigForm` que renderiza desde JSON Schema (probablemente sin libs externas — el schema de Korva es chico).

### Cliente MCP (cambio externo)

Korva hoy es server MCP, no cliente. Para capturar tokens reales necesitamos que el IDE (Cursor/Claude Code/Copilot) reporte el `usage` que devuelve Anthropic. Tres opciones a decidir en Phase 2:

- **A. Extension del CLI `korva`** que envuelva al cliente MCP y parsee responses (intrusivo, pero portable).
- **B. Endpoint público `POST /api/v1/interactions`** que cualquier integración llame manualmente (simple, requiere instrumentación per-IDE).
- **C. Aproximación**: estimar tokens server-side desde el contenido de las observaciones y prompts inyectados (no real, pero no requiere cambio externo).

Recomendación inicial: **B** para el MVP (es el menos invasivo y permite que cualquier wrapper futuro lo use). Estimación server-side **C** se reserva para el % reducción donde no tenemos baseline real.

## Debt — issues existentes en el área

- `AdminScrolls.tsx` togglea **localStorage**, no hay endpoint que persista la lista activa. Esto se "arregla solo" con el endpoint `PUT /admin/config` ya que `lore.active_scrolls` es parte de `KorvaConfig`.
- `AdminDashboard.tsx:42` estima tokens como `total_content_len / 4`. Hay que reemplazar por `interactions.tokens_total`.
- No existe versionado/rollback de `korva.config.json`. Mitigación inicial: snapshot en `audit_logs` antes de cada `PUT`.
- `mcp_calls` no captura el prompt ni la respuesta — útil para latencia, no para timeline de actividad. La nueva tabla `interactions` no compite con ella, la complementa.

## Vault context — decisiones previas relevantes

- **Privacidad por default**: `internal/privacy/Filter()` debe aplicarse a TODO contenido antes de guardarlo en SQLite. Aplica al body de `interactions.prompt_excerpt` y `response_excerpt`.
- **No CGO**: `modernc.org/sqlite` para mantener single-binary. Cualquier query nueva debe usar `database/sql` estándar.
- **Privacidad cloud**: `*.key` y datos sensibles excluidos de Hive sync (ver `cloud_outbox` filter).
- **CORS**: `KORVA_CORS_ORIGIN` o default `http://localhost:5173` — Beacon ya está alineado.
- **Admin auth**: middleware `withAdminOrSessionAdmin` ya existe — los endpoints nuevos lo reutilizan.

## Restricciones del proyecto

- Go 1.26+, workspace `go.work` con módulos separados (`internal`, `vault`, `cli`, `sentinel/validator`).
- Tests table-driven, SQLite in-memory, fixtures en `testdata/`.
- Beacon: React 19, Vite 6, TanStack Query, Zustand, Tailwind v4, Biome, Vitest.
- `korva.config.json` puede vivir en CWD o `~/.korva/config.json` — el handler PUT debe respetar el origen.

## Próximo paso

Pasar a **Phase 2 — Specification**: cerrar criterios de aceptación del MVP, decidir entre opciones A/B/C de tracking de tokens, definir qué settings son toggleables en la primera versión. Requiere ✅ de Felipe.
