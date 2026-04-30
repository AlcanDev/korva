# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ Current |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub Issues.**

Report security issues via **[GitHub Security Advisories](https://github.com/AlcanDev/korva/security/advisories/new)**.

You will receive a response within 48 hours. If the issue is confirmed, we will release a patch as soon as possible depending on complexity (typically within 7 days for critical issues).

## Security design

Korva is designed with these security properties:

### Admin key (`admin.key`)
- Generated locally with `korva init --admin`
- Stored at `~/.korva/admin.key` with permissions `0600` (owner read-only)
- **Never** committed to git, never synced, never logged
- Comparison uses `hmac.Equal()` (constant-time) to prevent timing attacks
- Rotate with `korva admin rotate-key` (requires current key via stdin)

### Privacy filter
- Applied to **all content** before it is written to SQLite
- Redacts: passwords, tokens, secrets, Bearer tokens, `<private>` tagged content
- Configurable via `vault.private_patterns` in `korva.config.json`

### 3 Kingdoms isolation
- **Kingdom 1** (public repo): zero knowledge of private team data
- **Kingdom 2** (private team profile): never merged into Kingdom 1
- **Kingdom 3** (local machine): admin keys and runtime data never leave the machine

### Network
- Vault HTTP server listens on `localhost:7437` only (not `0.0.0.0`)
- CORS restricted to `localhost:5173` (Beacon dev server)
- Admin endpoints require `X-Admin-Key` header

### Git Sync
- The sync manifest explicitly excludes `*.key` files
- Observations are privacy-filtered before export

---

*Last updated: 2026-04-30*
