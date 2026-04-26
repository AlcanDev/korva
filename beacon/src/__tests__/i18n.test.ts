import { describe, it, expect } from 'vitest'
import { en } from '@/i18n/en'
import { es } from '@/i18n/es'

// Recursively gather all leaf paths from an object
function leafPaths(obj: unknown, prefix = ''): string[] {
  if (typeof obj === 'function') return [prefix]
  if (typeof obj !== 'object' || obj === null) return [prefix]
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) =>
    leafPaths(v, prefix ? `${prefix}.${k}` : k)
  )
}

describe('i18n translations', () => {
  const enPaths = leafPaths(en).sort()
  const esPaths = leafPaths(es).sort()

  it('EN and ES have the same number of keys', () => {
    expect(enPaths.length).toBe(esPaths.length)
  })

  it('every EN key exists in ES', () => {
    const missing = enPaths.filter(p => !esPaths.includes(p))
    expect(missing).toEqual([])
  })

  it('every ES key exists in EN', () => {
    const extra = esPaths.filter(p => !enPaths.includes(p))
    expect(extra).toEqual([])
  })

  it('function keys in EN return strings', () => {
    expect(en.vault.paginationLabel(10, 1, 3)).toContain('10')
    expect(en.codeHealth.checkpoints(1)).toBe('1 checkpoint')
    expect(en.codeHealth.checkpoints(3)).toBe('3 checkpoints')
    expect(en.skills.versionsCount(1)).toBe('(1 version)')
    expect(en.skills.versionsCount(5)).toBe('(5 versions)')
    expect(en.teams.activeCount(4)).toBe('4 active')
    expect(en.teams.seatsDisplay(3, 10)).toBe('3/10 seats')
    expect(en.teams.lastSeenMinutesAgo(15)).toBe('15m ago')
    expect(en.auth.rateLimited(25)).toContain('25')
  })

  it('function keys in ES return strings', () => {
    expect(es.vault.paginationLabel(1, 1, 1)).toContain('observación')
    expect(es.vault.paginationLabel(5, 1, 1)).toContain('observaciones')
    expect(es.codeHealth.patterns(1)).toContain('patrón')
    expect(es.codeHealth.patterns(3)).toContain('patrones')
    expect(es.teams.activeCount(2)).toContain('2')
    expect(es.teams.seatsDisplay(2, 5)).toContain('asientos')
    expect(es.teams.lastSeenMinutesAgo(10)).toContain('10')
    expect(es.auth.rateLimited(30)).toContain('30')
  })

  it('static strings are non-empty', () => {
    for (const path of enPaths) {
      const parts = path.split('.')
      let val: unknown = en
      for (const p of parts) val = (val as Record<string, unknown>)[p]
      if (typeof val === 'string') {
        expect(val, `EN key "${path}" is empty`).not.toBe('')
      }
    }
  })
})
