import { describe, it, expect } from 'vitest'
import { formatVersionLabel } from '@/pages/admin/Admin'

// Fase 6.0 — el sidebar muestra la versión real del binario (vault.version
// vía /admin/system-status). formatVersionLabel mapea el string crudo a la
// etiqueta que vemos: "v1.9.0" para semver, "dev" tal cual para builds
// locales sin -ldflags, "—" cuando todavía no resolvió la query.
describe('formatVersionLabel', () => {
  it('prepends "v" to semver-shaped strings', () => {
    expect(formatVersionLabel('1.9.0')).toBe('v1.9.0')
    expect(formatVersionLabel('0.1.0-rc.1')).toBe('v0.1.0-rc.1')
  })

  it('strips an existing "v" prefix before re-adding it (idempotent)', () => {
    expect(formatVersionLabel('v1.9.0')).toBe('v1.9.0')
    expect(formatVersionLabel('V2.0.0')).toBe('v2.0.0')
  })

  it('returns dev-style strings verbatim (no misleading "v" prefix)', () => {
    expect(formatVersionLabel('dev')).toBe('dev')
    expect(formatVersionLabel('snapshot-abc123')).toBe('snapshot-abc123')
  })

  it('uses the em-dash placeholder when the query has not resolved', () => {
    expect(formatVersionLabel('—')).toBe('—')
    expect(formatVersionLabel('')).toBe('—')
  })

  it('trims surrounding whitespace before classifying', () => {
    expect(formatVersionLabel('  1.0.0  ')).toBe('v1.0.0')
    expect(formatVersionLabel('  dev  ')).toBe('dev')
  })
})
