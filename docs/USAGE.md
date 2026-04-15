# Guía de uso paso a paso — Korva

Esta guía cubre todo el ciclo de vida: instalación, configuración, uso diario, y gestión del equipo.

---

## Índice

1. [Instalación](#1-instalación)
2. [Primer inicio — `korva init`](#2-primer-inicio--korva-init)
3. [Conectar el Vault MCP a tu editor](#3-conectar-el-vault-mcp-a-tu-editor)
4. [Instalar hooks de Sentinel](#4-instalar-hooks-de-sentinel)
5. [Configurar un Team Profile](#5-configurar-un-team-profile-para-equipos)
6. [Uso diario con la IA](#6-uso-diario-con-la-ia)
7. [Vault HTTP API](#7-vault-http-api)
8. [Dashboard Beacon](#8-dashboard-beacon)
9. [Comandos de mantenimiento](#9-comandos-de-mantenimiento)
10. [Configuración avanzada](#10-configuración-avanzada)

---

## 1. Instalación

### macOS / Linux (recomendado: Homebrew)

```bash
brew tap AlcanDev/tap
brew install korva
```

### macOS / Linux (script)

```bash
curl -fsSL https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.sh | sh
```

### Windows (PowerShell — como Administrador recomendado)

```powershell
iwr -useb https://raw.githubusercontent.com/AlcanDev/korva/main/scripts/install.ps1 | iex
```

> Después del instalador en Windows, **reinicia tu terminal** para que el PATH se aplique.

### Verificar instalación

```bash
korva version
# Salida esperada: 0.1.0 (abc1234) built 2026-04-15

korva-vault --help
korva-sentinel --help
```

---

## 2. Primer inicio — `korva init`

```bash
# En cualquier directorio (configura el entorno global)
korva init
```

Esto hace:
- Crea `~/.korva/` (macOS/Linux) o `%APPDATA%\korva\` (Windows)
- Crea `~/.korva/config.json` con la configuración por defecto
- Crea los directorios `vault/`, `lore/`, `profiles/`, `logs/`

### Si eres el administrador del equipo

```bash
korva init --admin --owner=tu@email.com
```

Genera `~/.korva/admin.key` (permisos `0600` — solo tú puedes leerlo). Este archivo **nunca sale de tu máquina**.

---

## 3. Conectar el Vault MCP a tu editor

El Vault MCP server es la pieza central — permite que tu IA guarde y recupere memoria entre sesiones.

### Iniciar el servidor

```bash
# En background (recomendado):
korva-vault &

# O explícito:
korva-vault --mode=both --port=7437
```

El servidor queda escuchando en:
- `stdin/stdout` — para MCP (lo usa el editor)
- `http://localhost:7437` — para la REST API y Beacon

### VS Code + GitHub Copilot

Crea o edita `.vscode/mcp.json` en tu proyecto:

```json
{
  "servers": {
    "korva-vault": {
      "type": "stdio",
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

Reinicia VS Code. Copilot Chat ahora tiene acceso a las herramientas `vault_*`.

### Claude Code

Edita `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

O al nivel del proyecto, crea `.claude/settings.json`:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

### Cursor

Edita `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "korva-vault": {
      "command": "korva-vault",
      "args": ["--mode=mcp"]
    }
  }
}
```

### Verificar que el MCP funciona

En tu editor, pídele a la IA:

```
Usa vault_stats para mostrarme las estadísticas del vault.
```

Si responde con estadísticas (aunque sean en cero), el MCP está funcionando.

---

## 4. Instalar hooks de Sentinel

Sentinel valida cada commit contra las reglas de arquitectura de tu equipo.

```bash
# En el directorio de tu proyecto
cd ~/repos/mi-proyecto
korva sentinel install
```

Esto instala `.git/hooks/pre-commit` que ejecuta `korva-sentinel` en cada commit.

### Probar manualmente

```bash
# Analizar archivos staged
korva-sentinel --staged

# Analizar todo el proyecto
korva-sentinel ./src/

# Ver reglas activas
korva-sentinel --list-rules
```

### Reglas incluidas

| ID | Descripción |
|----|------------|
| `HEX-001` | No importar infra desde dominio |
| `HEX-002` | No importar app desde dominio |
| `HEX-003` | No importar infra desde aplicación |
| `HEX-004` | Importaciones circulares entre capas |
| `HEX-005` | Dependencias externas directas en dominio |
| `NAM-001` | Servicios de aplicación deben terminar en `UseCase` o `Service` |
| `NAM-002` | Repositorios deben terminar en `Repository` |
| `NAM-003` | Controladores deben terminar en `Controller` |
| `SEC-001` | Detectar secretos hardcodeados |
| `TEST-001` | Detectar `console.log` en producción |

---

## 5. Configurar un Team Profile (para equipos)

Los team profiles permiten compartir configuración privada (scrolls, reglas, instrucciones) sin tocar el repo público.

### Instalar el profile del equipo

```bash
korva init --profile https://github.com/TU-ORG/korva-team-profile.git
```

Esto:
1. Clona el profile privado a `~/.korva/profiles/`
2. Valida el `team-profile.json`
3. Aplica los overrides de configuración
4. Copia los scrolls privados a `~/.korva/lore/private/`
5. Inyecta las instrucciones en `.github/copilot-instructions.md` y `CLAUDE.md`

### Sincronizar cuando hay actualizaciones

```bash
korva sync --profile
```

### Crear tu propio team profile

1. Forkea (o inspírate en) [korva-team-profile](https://github.com/AlcanDev/korva-team-profile)
2. Hazlo privado en GitHub
3. Personaliza `team-profile.json`, scrolls e instrucciones
4. Comparte la URL con tu equipo

---

## 6. Uso diario con la IA

### Al inicio de cada sesión

Pídele a tu IA (Copilot, Claude, Cursor):

```
Usa vault_context con el proyecto "nombre-del-proyecto" para recuperar el contexto de trabajo anterior.
```

O si tienes un `.github/copilot-instructions.md` o `CLAUDE.md` bien configurado, el contexto se carga automáticamente.

### Guardar una decisión importante

```
Usa vault_save para guardar:
- project: home-api
- type: decision
- title: Usamos arquitectura hexagonal
- content: Decidimos adoptar ports & adapters para separar dominio de infraestructura...
```

### Buscar en la memoria del equipo

```
Usa vault_search con query "hexagonal" para buscar observaciones relacionadas.
```

### Al final de la sesión

```
Usa vault_session_end con el resumen de lo que implementamos hoy.
```

### Tipos de observaciones

| Tipo | Cuándo usarlo |
|------|--------------|
| `decision` | Decisiones de arquitectura o tecnología |
| `pattern` | Patrones de código que funcionaron bien |
| `bugfix` | Bugs resueltos y cómo los arreglamos |
| `learning` | Aprendizajes del equipo |
| `context` | Contexto general del proyecto |

---

## 7. Vault HTTP API

La API REST está disponible en `http://localhost:7437` mientras `korva-vault` está corriendo.

### Endpoints principales

```bash
# Health check
curl http://localhost:7437/healthz

# Guardar una observación
curl -X POST http://localhost:7437/api/v1/observations \
  -H "Content-Type: application/json" \
  -d '{"project":"home-api","type":"decision","title":"Hexagonal","content":"Adoptamos hexagonal..."}'

# Buscar
curl "http://localhost:7437/api/v1/search?q=hexagonal&project=home-api"

# Contexto del proyecto
curl http://localhost:7437/api/v1/context/home-api

# Estadísticas
curl http://localhost:7437/api/v1/stats

# Timeline de la última semana
curl "http://localhost:7437/api/v1/timeline/home-api"
```

### Endpoints de admin (requieren `admin.key`)

```bash
# Leer tu admin key
KEY=$(cat ~/.korva/admin.key | jq -r .key)

# Estadísticas completas
curl -H "X-Admin-Key: $KEY" http://localhost:7437/admin/stats
```

---

## 8. Dashboard Beacon

Beacon es el dashboard web que visualiza tu vault.

### Iniciar Beacon

```bash
cd beacon
npm install
npm run dev
```

Abre http://localhost:5173 en tu navegador.

> Requiere que `korva-vault --mode=http` esté corriendo en `:7437`.

---

## 9. Comandos de mantenimiento

```bash
# Estado general del sistema
korva status

# Diagnóstico completo
korva doctor

# Ver scrolls instalados
korva lore list

# Agregar un scroll curado al proyecto
korva lore add nestjs-hexagonal

# Actualizar todo (vault sync + profile sync)
korva sync

# Solo actualizar el profile del equipo
korva sync --profile
```

---

## 10. Configuración avanzada

### Archivo de configuración principal

`~/.korva/config.json` (macOS/Linux) o `%APPDATA%\korva\config.json` (Windows):

```json
{
  "vault": {
    "port": 7437,
    "sync_repo": "",
    "auto_sync": false,
    "private_patterns": ["password", "secret", "token", "Bearer "]
  },
  "sentinel": {
    "rules_path": "",
    "block_on_violation": true,
    "ignored_paths": ["node_modules", "dist", ".next"]
  },
  "lore": {
    "scroll_priority": "private_first",
    "active_scrolls": ["nestjs-hexagonal", "typescript"]
  }
}
```

### Variables de entorno

```bash
KORVA_HOME=/custom/path   # Sobreescribe ~/.korva/
KORVA_VAULT_PORT=8080     # Puerto del vault (default: 7437)
KORVA_LOG_LEVEL=debug     # Nivel de log
```

### Rotar la admin key

```bash
korva admin rotate-key
# Te pide la key actual por stdin (nunca como argumento de CLI)
```

---

## Referencia rápida

```
korva init                          — Inicializar Korva
korva init --profile <url>          — Inicializar con team profile
korva init --admin --owner=<email>  — Inicializar como admin
korva status                        — Ver estado del sistema
korva doctor                        — Diagnóstico y health checks
korva sync                          — Sincronizar todo
korva sync --profile                — Sincronizar solo el team profile
korva sentinel install              — Instalar hooks en el proyecto actual
korva lore list                     — Listar scrolls disponibles
korva lore add <scroll-id>          — Instalar un scroll
korva admin rotate-key              — Rotar la admin key

korva-vault --mode=both             — Iniciar servidor (MCP + HTTP)
korva-vault --mode=mcp              — Solo MCP (para editors)
korva-vault --mode=http             — Solo HTTP REST
korva-vault --port=7437             — Puerto HTTP

korva-sentinel ./src/               — Analizar directorio
korva-sentinel --staged             — Analizar solo archivos staged
korva-sentinel --list-rules         — Ver reglas activas
```
