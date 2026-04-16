# Korva — Instrucciones para Claude Code

## Approach

- Think before acting. Read existing files before writing code.
- Be concise in output but thorough in reasoning.
- Prefer editing over rewriting whole files.
- Do not re-read files you have already read unless the file may have changed.
- Skip files over 100KB unless explicitly required.
- Suggest running /cost when a session is running long.
- Recommend starting a new session when switching to an unrelated task.
- Test your code before declaring done.
- No sycophantic openers or closing fluff.
- Keep solutions simple and direct.
- User instructions always override this file.

---

## Contexto del proyecto
Korva es un ecosistema de IA para equipos enterprise escrito en Go.
Repositorio: `github.com/alcandev/korva`

## Arquitectura del monorepo

```
korva/
├── internal/    → paquetes Go compartidos (db, config, privacy, admin, profile)
├── vault/       → servidor de memoria (SQLite + MCP + REST API :7437)
├── cli/         → CLI `korva` (Cobra + Bubbletea)
├── sentinel/    → pre-commit hooks + validador Go
├── lore/        → Scrolls de conocimiento (.md)
├── forge/       → workflow SDD de 5 fases (.md)
└── beacon/      → dashboard web (React 19 + Vite 6)
```

## Go workspace
El proyecto usa `go.work`. Cada componente tiene su propio `go.mod`:
- `github.com/alcandev/korva/internal`
- `github.com/alcandev/korva/vault`
- `github.com/alcandev/korva/cli`
- `github.com/alcandev/korva/sentinel/validator`

Para agregar dependencias a un módulo específico:
```bash
cd vault && go get <paquete>
```

## Reglas de desarrollo

### Go
- Go 1.26+
- SQLite via `modernc.org/sqlite` (pure Go, sin CGO)
- IDs de observaciones con ULID (`github.com/oklog/ulid/v2`)
- Paths siempre via `internal/config.PlatformPaths()` — nunca strings hardcodeadas
- Tests table-driven, SQLite in-memory para tests del store
- `go test ./...` desde la raíz del workspace

### Privacidad y seguridad
- `internal/privacy.Filter()` debe aplicarse a TODO contenido antes de guardarlo en SQLite
- `admin.key` nunca se serializa ni se logea — solo se lee de `~/.korva/admin.key`
- Los Team Profiles solo pueden modificar la whitelist: `vault`, `sentinel`, `lore`, `instructions`
- Git Sync excluye explícitamente `*.key` y datos sensibles

### Estructura de archivos Go
- Exports públicos en la raíz del paquete
- Implementaciones privadas en subdirectorios
- Tests junto al archivo (`store_test.go` junto a `store.go`)
- Fixtures en `testdata/` o `internal/testutil/`

## Comandos frecuentes
```bash
# Compilar vault (desde la raíz del workspace)
go build github.com/alcandev/korva/vault/cmd/korva-vault

# Compilar CLI
go build github.com/alcandev/korva/cli/cmd/korva

# Tests de todo el workspace (patrón correcto para go.work)
go test github.com/alcandev/korva/...

# Tests de un módulo específico
cd vault && go test ./...

# Agregar dependencia a un módulo específico
cd vault && go get github.com/charmbracelet/bubbletea
```

## Memoria del proyecto (Vault MCP)
Si el Vault MCP está configurado en este entorno:
- `vault_context` al inicio de sesión — recupera contexto previo
- `vault_save` después de cambios significativos — decisiones, patrones, bugs resueltos
- `vault_search "tema"` antes de proponer algo nuevo

## Contexto de negocio
Korva nace para resolver el problema de los equipos que usan IA sin instrucciones de arquitectura.
El equipo destino usa NestJS hexagonal + Fastify + Nx monorepo + deploy en K8s AKS.
El diseño de Korva debe ser genérico (open source MIT) pero el sistema de Team Profiles
permite configuración privada por equipo sin tocar el repo público.
