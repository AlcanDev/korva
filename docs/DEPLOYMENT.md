# Despliegue del servidor compartido de Korva Vault

Esta guía explica cómo desplegar `korva-vault` como servidor central de equipo para que todos los developers compartan la misma base de conocimiento en tiempo real.

---

## Arquitectura de despliegue

```
Developer A (local)
  korva-vault (MCP local) ──▶ POST /api/v1/sync ──▶ Vault Server (Railway/VPS)
                                                          │
Developer B (local)                                       │ SQLite compartido
  korva-vault (MCP local) ──▶ GET  /api/v1/sync ──▶ Vault Server
                                                          │
Beacon (admin panel)      ────────────────────────▶ GET  /api/v1/*
http://tu-dominio.com/admin
```

**Sin servidor:** cada developer tiene su vault local en `~/.korva/vault/`. Funciona offline pero no comparte memoria entre el equipo.

**Con servidor:** el vault central es la fuente de verdad. Los vaults locales sincronizan con el servidor automáticamente en cada commit (hook `post-commit`).

---

## Opción A: Railway (recomendado — gratis hasta 5$/mes)

Railway es la opción más rápida. No necesitas saber Docker.

### 1. Crear cuenta en Railway

Ve a [railway.app](https://railway.app) y crea una cuenta con GitHub.

### 2. Crear el proyecto

```bash
# Instalar Railway CLI
npm install -g @railway/cli

# Login
railway login

# Desde el directorio del repo korva
cd ~/proyectos/korva
railway init
# → Selecciona "Empty project", nombre: "korva-vault"
```

### 3. Configurar variables de entorno

En el dashboard de Railway → tu proyecto → Variables:

```
PORT=7437
KORVA_VAULT_DB=/data/vault.db
KORVA_VAULT_MODE=http
```

Para la admin key, en tu máquina local:
```bash
# Leer tu admin key
cat ~/.korva/admin.key | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['key'])"
```

Agregar en Railway → Variables:
```
KORVA_ADMIN_KEY=el-valor-de-key-de-tu-archivo
```

### 4. Crear volumen persistente

En Railway → tu servicio → Storage → Add Volume:
- Mount path: `/data`
- Nombre: `korva-data`

### 5. Desplegar

```bash
railway up
```

Railway detecta el `Dockerfile` automáticamente y construye la imagen.

### 6. Obtener la URL pública

Railway → tu servicio → Settings → Domains → Generate Domain.

Obtendrás algo como: `https://korva-vault-production.up.railway.app`

### 7. Verificar que funciona

```bash
curl https://korva-vault-production.up.railway.app/healthz
# → {"status":"ok","service":"korva-vault"}
```

---

## Opción B: Fly.io (gratis con límites generosos)

```bash
# Instalar flyctl
curl -L https://fly.io/install.sh | sh

# Login
fly auth login

# Desde el directorio korva
fly launch
# → App name: korva-vault-tu-equipo
# → Region: iad (US East) o lax (US West)
# → No PostgreSQL needed
# → No Redis needed

# Crear volumen persistente
fly volumes create korva_data --region iad --size 1

# Configurar variables
fly secrets set KORVA_ADMIN_KEY="tu-admin-key-aqui"

# Desplegar
fly deploy
```

`fly.toml` generado automáticamente — agrega el mount del volumen:

```toml
# fly.toml
app = "korva-vault-tu-equipo"
primary_region = "iad"

[build]

[[services]]
  internal_port = 7437
  protocol = "tcp"

  [[services.ports]]
    port = 443
    handlers = ["tls", "http"]

  [[services.ports]]
    port = 80
    handlers = ["http"]

[[mounts]]
  source = "korva_data"
  destination = "/data"
```

```bash
fly deploy
fly open  # abre en el browser
```

---

## Opción C: VPS propio (Ubuntu/Debian) con Docker Compose

Para máximo control. Necesitas un servidor con IP pública y un dominio.

### 1. Instalar Docker en el VPS

```bash
# En el VPS (Ubuntu 22.04+)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

### 2. Copiar archivos al VPS

```bash
# Desde tu máquina local
scp docker-compose.yml user@tu-vps:/opt/korva/
```

### 3. Configurar variables de entorno

```bash
# En el VPS
cat > /opt/korva/.env << EOF
KORVA_PORT=7437
KORVA_DOMAIN=vault.tu-dominio.com
ACME_EMAIL=tu@email.com
EOF
```

Para la admin key, copia solo el campo `key` de tu `~/.korva/admin.key`:
```bash
# En tu máquina local
cat ~/.korva/admin.key | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['key'])"

# En el VPS (pega el valor)
echo "KORVA_ADMIN_KEY=pega-aqui" >> /opt/korva/.env
```

### 4. Iniciar el servidor

```bash
cd /opt/korva
docker compose up -d
docker compose logs -f  # verificar que arranca
```

### 5. Configurar HTTPS con Caddy (más simple que Traefik)

```bash
# Instalar Caddy
apt install caddy

# /etc/caddy/Caddyfile
echo 'vault.tu-dominio.com {
  reverse_proxy localhost:7437
}' > /etc/caddy/Caddyfile

systemctl reload caddy
```

HTTPS queda automático con Let's Encrypt.

---

## Conectar tu equipo al servidor central

Una vez que el servidor está en producción, cada developer configura su cliente:

### 1. Actualizar `~/.korva/config.json`

```json
{
  "vault": {
    "port": 7437,
    "sync_repo": "https://korva-vault-tu-equipo.up.railway.app",
    "auto_sync": true
  }
}
```

O simplemente actualiza el team profile y cada dev sincroniza con:
```bash
korva sync --profile
```

### 2. Configurar el MCP para apuntar al servidor remoto

En tu editor, en lugar de correr `korva-vault` localmente, también puedes apuntar el HTTP al servidor:

**`.vscode/mcp.json`** (workspace):
```json
{
  "servers": {
    "korva-vault": {
      "type": "stdio",
      "command": "korva-vault",
      "args": ["--mode=mcp", "--sync-url=https://korva-vault-tu-equipo.up.railway.app"]
    }
  }
}
```

### 3. Auto-sync en cada commit (ya instalado por `korva sentinel install`)

El hook `post-commit` ejecuta `korva sync --vault --quiet` después de cada commit. Funciona silenciosamente en background — nunca bloquea el commit.

---

## Monitoreo del servidor

### Logs en tiempo real

```bash
# Railway
railway logs --tail

# Fly.io
fly logs

# Docker Compose
docker compose logs -f vault
```

### Health check

```bash
curl https://tu-vault.railway.app/healthz
# {"status":"ok","service":"korva-vault"}
```

### Estadísticas via admin panel

```bash
KEY="tu-admin-key"
curl -H "X-Admin-Key: $KEY" https://tu-vault.railway.app/admin/stats
```

O abre el **Beacon dashboard** apuntando a la URL del servidor:

```bash
# Modifica el proxy en beacon/vite.config.ts
target: 'https://tu-vault.railway.app'
```

---

## Backup automático

```bash
# Cron en el VPS — backup diario del SQLite a S3/R2
0 3 * * * docker exec korva-vault sqlite3 /data/vault.db ".backup /data/backup-$(date +%Y%m%d).db" && \
          aws s3 cp /data/backup-$(date +%Y%m%d).db s3://tu-bucket/korva-backups/
```

---

## Resumen de costos estimados

| Opción | Costo/mes | Complejidad | Uptime |
|--------|-----------|-------------|--------|
| Railway (hobby) | $5 | ⭐ Muy fácil | 99.9% |
| Fly.io (free tier) | $0–3 | ⭐⭐ Fácil | 99.5% |
| VPS (DigitalOcean/Vultr) | $6–12 | ⭐⭐⭐ Media | 99.99% |

**Recomendación para empezar:** Railway — 10 minutos de setup, $5/mes, zero mantenimiento.
