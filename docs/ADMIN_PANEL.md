# Panel de Administración — Korva Beacon

El panel de admin es una sección privada del Beacon dashboard, accesible únicamente con tu `admin.key`. Nadie más en el equipo tiene acceso.

---

## Acceso

### Desde Beacon (recomendado)

```bash
# Iniciar el vault
korva-vault

# En otra terminal, iniciar Beacon
cd beacon && npm run dev
```

Abre http://localhost:5173 → en la barra lateral, clic en **Admin** (icono de escudo, abajo de todo).

O accede directamente: http://localhost:5173/admin

### Obtener tu admin key para el login

```bash
# macOS/Linux
cat ~/.korva/admin.key | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['key'])"

# Windows (PowerShell)
(Get-Content "$env:APPDATA\korva\admin.key" | ConvertFrom-Json).key
```

Pega ese valor en el campo de login del panel.

> La sesión se guarda en `sessionStorage` — expira cuando cierras el tab. No se guarda en cookies ni localStorage permanente.

---

## Funcionalidades del panel

### Dashboard (`/admin/dashboard`)

Vista general del sistema en tiempo real:

- **KPIs**: total de observaciones, sesiones, prompts guardados, proyectos activos
- **Top Projects**: qué proyectos tienen más actividad en el vault
- **Por tipo**: distribución de decisions/patterns/bugfixes/learnings
- **Teams**: desglose por equipo

### Vault Browser (`/admin/vault`)

Explorador completo de todas las observaciones:

- Búsqueda full-text por contenido
- Filtro por proyecto, tipo
- Vista detallada de cada observación (contenido completo, tags, fecha, ID)
- **Eliminar observaciones** (con confirmación)

### Scrolls & Instructions (`/admin/scrolls`)

Gestión del conocimiento inyectado en los editores:

**Pestaña Scrolls:**
- Activar/desactivar scrolls individuales
- Los scrolls activos se cargan en el contexto de la IA en cada sesión
- Cambios se aplican via `korva sync --profile`

**Pestaña Instructions:**
- Editar `copilot-extensions.md` (se inyecta en `.github/copilot-instructions.md`)
- Editar `claude-extensions.md` (se inyecta en `CLAUDE.md`)
- Los cambios se distribuyen al equipo con `korva sync --profile`

---

## Añadir nuevas skills / instrucciones

### Opción 1: Via panel admin (UI)

1. Admin panel → Scrolls & Instructions → Instructions tab
2. Clic en "Edit" en el archivo que quieras modificar
3. Escribe las instrucciones en Markdown
4. Clic en "Save"
5. Ejecuta `korva sync --profile` para distribuir al equipo

### Opción 2: Directo en el repo privado

```bash
cd ~/proyectos/korva-team-profile

# Editar instrucciones de Copilot
nano instructions/copilot-extensions.md

# O agregar un nuevo scroll
mkdir scrolls/mi-nuevo-scroll
cat > scrolls/mi-nuevo-scroll/SCROLL.md << 'EOF'
---
id: mi-nuevo-scroll
title: Mi Nueva Skill
version: 1.0.0
triggers:
  - keywords: ["mi-framework", "mi-libreria"]
  - file_patterns: ["*.config.ts"]
---

# Mi Nueva Skill

## Contexto
...

## Reglas
...
EOF

# Commit y push
git add . && git commit -m "feat: add mi-nuevo-scroll"
git push

# El equipo actualiza con:
korva sync --profile
```

### Opción 3: Scroll curado para la comunidad

Si el scroll es genérico (no contiene info propietaria), contribuirlo al repo público:

```bash
cd ~/proyectos/korva
cp -r /ruta/a/mi-scroll lore/curated/mi-scroll/
git add lore/curated/mi-scroll/
git commit -m "feat(lore): add mi-scroll scroll"
# Abre PR en github.com/AlcanDev/korva
```

---

## Rotar la admin key

Si sospechas que tu key fue comprometida o simplemente quieres rotarla:

```bash
korva admin rotate-key
# → Te pide la key actual por stdin (no como argumento)
# → Genera una nueva key y actualiza admin.key
# → La key antigua queda inválida inmediatamente
```

Si el servidor está en Railway/Fly.io, actualiza también la variable de entorno:

```bash
# Railway
railway variables set KORVA_ADMIN_KEY="nueva-key"

# Fly.io
fly secrets set KORVA_ADMIN_KEY="nueva-key"
```

---

## Monitoreo avanzado via API

Además del panel visual, puedes consultar cualquier endpoint admin directamente:

```bash
KEY=$(cat ~/.korva/admin.key | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['key'])")
BASE="http://localhost:7437"

# Estadísticas completas
curl -s -H "X-Admin-Key: $KEY" $BASE/admin/stats | python3 -m json.tool

# Eliminar una observación específica
curl -X DELETE -H "X-Admin-Key: $KEY" $BASE/admin/observations/01ABC123

# Purge completo (DESTRUCTIVO — borra todo)
curl -X POST -H "X-Admin-Key: $KEY" $BASE/admin/purge
```

---

## Seguridad del panel

- La admin key **nunca sale de tu máquina** — vive en `~/.korva/admin.key` (permisos `0600`)
- El panel usa `sessionStorage` — la sesión expira al cerrar el tab
- Todas las llamadas a `/admin/*` incluyen el header `X-Admin-Key` en HTTPS
- Si el servidor está en producción, asegúrate de que use HTTPS (Railway/Fly.io lo hacen automáticamente)

*Last updated: 2026-04-30*
