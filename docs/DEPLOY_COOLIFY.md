# Deploy Korva en Coolify + Cloudflare + Resend

Runbook reproducible para el deploy de producción de Korva Vault:
**un solo contenedor expuesto en tres subdominios** (`app.`, `api.`, `mcp.`),
detrás del Traefik integrado de Coolify, con DNS en Cloudflare y emails por
Resend.

> Topología completa y razonamiento de la arquitectura: ver
> [ARCHITECTURE.md](ARCHITECTURE.md). Este documento solo cubre la ejecución.

---

## Prerrequisitos

- **VPS** con Coolify ya instalado y accesible vía su UI web.
- **Dominio `korva.dev`** (o equivalente) en Cloudflare como nameservers.
- **Cuenta Resend** (gratis hasta 3 000 emails/mes).
- **Repositorio GitHub** del Korva accesible — necesitarás una *deploy key*
  o un GitHub App de Coolify.
- **Cliente DNS de tu máquina** capaz de hacer `dig` para verificar
  propagación.

Todo lo demás (Traefik, certs Let's Encrypt) lo maneja Coolify.

---

## Paso 1 — DNS en Cloudflare

Tres registros A apuntando al VPS, más uno para el dominio de envío.

| Tipo  | Nombre  | Contenido          | Proxy   | TTL  |
|-------|---------|--------------------|---------|------|
| A     | `app`   | `<IP del VPS>`     | DNS only | Auto |
| A     | `api`   | `<IP del VPS>`     | DNS only | Auto |
| A     | `mcp`   | `<IP del VPS>`     | DNS only | Auto |
| CNAME | `send`  | *Resend te lo da en el Paso 2* | DNS only | Auto |

**Por qué "DNS only" (nube gris) y no proxied (nube naranja):**

- *DNS only* deja que Traefik en el VPS termine TLS con Let's Encrypt
  directamente. Es la ruta más simple para v1.
- *Proxied* mete a Cloudflare en el medio (CDN + WAF), pero requiere un
  Origin Certificate de Cloudflare instalado en Traefik y modo "Full
  (Strict)" en el dashboard. Vale la pena para v2 cuando quieras WAF/CDN;
  no para el primer deploy.

**Verificación:**

```bash
dig +short app.korva.dev api.korva.dev mcp.korva.dev
# Debe devolver la IP del VPS para cada uno.
```

---

## Paso 2 — Resend (dominio de envío)

1. Crea cuenta en [resend.com](https://resend.com).
2. **Domains → Add Domain → `send.korva.dev`**. Elige región `us-east-1` o
   `eu-west-1` según donde estén tus usuarios.
3. Resend muestra 3-4 registros DNS (MX, TXT con SPF, TXT con DKIM, opcional
   DMARC). Cópialos exactos a Cloudflare como registros separados (todos
   en "DNS only").
4. En Resend, dale **Verify Domain**. La verificación toma 1-10 min.
5. **API Keys → Create API Key** con permiso `Sending access` para
   `send.korva.dev`. Guarda el token `re_...` — solo se muestra una vez.

> ⚠ El subdominio `send.korva.dev` aísla los headers SPF/DKIM del email
> transaccional respecto al SPF que use el sitio de marketing en `korva.dev`.
> Mezclarlos rompe deliverability tarde o temprano.

---

## Paso 3 — Coolify: crear el proyecto

1. **Projects → New Project** → nombre `korva`.
2. **+ New → Resource → Docker Compose**.
3. **Source:** GitHub. Coolify pedirá conectar el repo. Usa una **deploy key
   de solo lectura** (Settings → Deploy keys en GitHub) o un GitHub App si
   ya lo tienes wireado en Coolify.
4. **Branch:** `main`. **Compose file:** `docker-compose.yml`.
5. **Build pack:** Docker Compose (autodetectado).

**No hagas deploy todavía.** Falta env y dominios.

---

## Paso 4 — Coolify: variables de entorno

En la pestaña **Environment Variables** del servicio `vault`, copia desde
[`.env.example`](../.env.example) y completa los valores reales. Mínimo
viable para producción:

```bash
# Hosts públicos
KORVA_APP_DOMAIN=app.korva.dev
KORVA_API_DOMAIN=api.korva.dev
KORVA_MCP_DOMAIN=mcp.korva.dev

# CORS
KORVA_CORS_ORIGIN=https://app.korva.dev

# MCP — default-deny ya viene activo; lo dejamos explícito.
KORVA_MCP_ALLOW_ANONYMOUS=false

# Resend
KORVA_EMAIL_API_KEY=re_xxxxxxxxxxxxxxxxxxxx
KORVA_EMAIL_FROM=no-reply@send.korva.dev
KORVA_EMAIL_FROM_NAME=Korva

# OIDC (cuando quieras habilitar SSO — opcional para el primer deploy)
# KORVA_OIDC_ISSUER_URL=https://accounts.google.com
# KORVA_OIDC_CLIENT_ID=...
# KORVA_OIDC_CLIENT_SECRET=...
# KORVA_OIDC_REDIRECT_URL=https://api.korva.dev/auth/oidc/callback
```

**Marca como "Build-time" solo las variables que no contienen secretos**;
las demás (API keys, client secrets) van como runtime env y Coolify las
inyecta en el contenedor sin escribirlas al filesystem.

---

## Paso 5 — Coolify: dominios y SSL

En la pestaña **Domains** del servicio `vault`, agrega los tres:

```
https://app.korva.dev
https://api.korva.dev
https://mcp.korva.dev
```

Coolify configura automáticamente:

- Los labels de Traefik (Coolify lee los que ya pusimos en
  `docker-compose.yml` y agrega los suyos para el routing interno).
- Let's Encrypt para cada subdominio vía HTTP-01.
- Redirect 80 → 443.

Si Cloudflare está en *DNS only* y los registros A ya propagaron, el cert
se emite en < 1 min por dominio.

---

## Paso 6 — Primer deploy

1. **Deploy** en Coolify. La primera build tarda 3-8 min (compila Beacon
   con npm, compila Go con CGO_ENABLED=0).
2. Coolify muestra logs en vivo. Espera el mensaje:
   `Korva Vault listening on http://0.0.0.0:7437 (Beacon UI embedded)`.

---

## Paso 7 — Smoke test (terminal local)

```bash
# Health del API — debe devolver {"status":"ok"}
curl -sS https://api.korva.dev/healthz | jq

# Beacon SPA — debe devolver index.html (200 + HTML)
curl -sSI https://app.korva.dev | head -3

# MCP sin auth — DEBE devolver 401 (no error real, prueba que el gate funciona)
curl -sS -X POST https://mcp.korva.dev/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  -w '\nHTTP %{http_code}\n'
# Esperado: HTTP 401 con body {"jsonrpc":"2.0",...,"error":{"code":-32001,...}}
```

Si los tres responden como se esperan, el deploy está vivo.

---

## Paso 8 — Crear el primer admin + token MCP

Desde tu máquina local, contra el servidor recién desplegado:

```bash
# Crea admin.key local (si no la tienes)
korva admin init

# Apunta el CLI al servidor remoto (esto vive en ~/.korva/config.json)
korva config set vault.endpoint https://api.korva.dev

# Crea un equipo
korva team create "Mi Equipo"

# Genera un magic link de invitación y obtén el session token
korva auth redeem
# → escribe el token en ~/.korva/session.token
```

**Verifica el MCP remoto autenticado:**

```bash
TOKEN=$(cat ~/.korva/session.token)
curl -sS -X POST https://mcp.korva.dev/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | jq '.result.tools | length'
# Esperado: 41 (todas las tools del profile agent)
```

---

## Paso 9 — Configurar tu editor con MCP remoto

### Claude Code

`~/.claude/mcp.json`:

```json
{
  "mcpServers": {
    "korva": {
      "url": "https://mcp.korva.dev/mcp",
      "headers": {
        "Authorization": "Bearer <tu-session-token>"
      }
    }
  }
}
```

### Cursor

`~/.cursor/mcp.json` con la misma estructura.

### Windsurf / Continue / otros

Mismo patrón: URL del endpoint + header `Authorization: Bearer`.

---

## Troubleshooting

### El cert de Let's Encrypt no se emite

- Verifica que el registro A esté en "DNS only" (no proxied).
- `dig +short app.korva.dev` debe devolver la IP del VPS, no la de
  Cloudflare. Si devuelve una IP `104.x` o `172.x`, está proxied.
- Revisa los logs de Coolify → Traefik service → busca "acme".

### `/mcp` devuelve 401 con token válido

- El token está hasheado en `member_sessions.token_hash`. Si el token se
  recreó (ej. `korva auth redeem` dos veces), el anterior queda invalidado.
- Verifica con `sqlite3 /data/vault.db "SELECT email, expires_at FROM
  member_sessions ORDER BY id DESC LIMIT 5"` desde el contenedor.

### CORS bloquea Beacon en producción

- `KORVA_CORS_ORIGIN` debe ser exactamente `https://app.korva.dev`, sin
  trailing slash, sin puerto.
- Si el navegador muestra el error, comprueba que el preflight (OPTIONS)
  retorne 204 con `Access-Control-Allow-Origin` correcto:
  ```bash
  curl -sSI -X OPTIONS https://api.korva.dev/api/v1/status \
    -H 'Origin: https://app.korva.dev' \
    -H 'Access-Control-Request-Method: GET'
  ```

### Resend marca emails como spam

- Verifica que SPF, DKIM y DMARC estén "Verified" en Resend Domains.
- El `From` debe ser `*@send.korva.dev`, no `*@korva.dev` — si pones el
  apex, SPF no matchea.
- Para producción seria, agrega un registro DMARC en Cloudflare:
  `_dmarc.korva.dev TXT "v=DMARC1; p=quarantine; rua=mailto:dmarc@korva.dev"`.

### El deploy de Coolify falla en `go build`

- Asegúrate que `go.work` esté commiteado en `main`.
- Coolify usa la imagen base `golang:1.26-alpine`. Si actualizaste a 1.27+
  en local, baja el go directive en los `go.mod` o sube la imagen en el
  Dockerfile.

---

## Rollback

Coolify guarda imágenes de las últimas N builds. **Project → Deployments →
selecciona el deployment verde anterior → Redeploy**. El SQLite del volumen
`korva-data` no se toca, así que el rollback es estado-de-código puro.

Para un rollback que requiera revertir migrations del SQLite, ver
[RUNBOOK.md](RUNBOOK.md#sqlite-recovery).

---

## Próximos pasos (post-deploy)

- **Backups del volumen `korva-data`** — sin esto pierdes toda la memoria
  del equipo si el VPS muere. Coolify tiene S3 backups; alternativa
  `litestream` apuntando a R2/B2.
- **Monitoring** — `/healthz` y `/readyz` ya existen; engancha un Uptime
  Kuma o Better Stack.
- **OIDC** — cuando quieras SSO, ver [SELF_HOSTING_OIDC.md](SELF_HOSTING_OIDC.md).
- **WAF + CDN** — pasar a proxied (Cloudflare orange cloud) con Origin
  Certificate cuando el tráfico justifique la complejidad adicional.
