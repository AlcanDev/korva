import { describe, it, expect } from 'vitest'
import {
  safeParseFeatureList,
  countByStatus,
  type FeatureListFeature,
} from '@/api/harness'

describe('safeParseFeatureList', () => {
  it('parses a well-formed payload', () => {
    const raw = JSON.stringify({
      project: 'x',
      rules: {
        one_feature_at_a_time: true,
        require_tests_to_close: true,
        valid_status: ['pending', 'done'],
      },
      features: [{ id: 1, name: 'a', title: 'A', status: 'pending' }],
    })
    const got = safeParseFeatureList(raw)
    expect(got?.project).toBe('x')
    expect(got?.features).toHaveLength(1)
  })

  it('returns null on garbage input rather than throwing', () => {
    expect(safeParseFeatureList('{not json')).toBeNull()
    expect(safeParseFeatureList('')).toBeNull()
    expect(safeParseFeatureList('null')).toBeNull() // null parses but isn't a payload
  })
})

describe('countByStatus', () => {
  it('counts every status family + total', () => {
    const features: FeatureListFeature[] = [
      { id: 1, name: 'a', title: 'A', status: 'pending' },
      { id: 2, name: 'b', title: 'B', status: 'pending' },
      { id: 3, name: 'c', title: 'C', status: 'spec_ready' },
      { id: 4, name: 'd', title: 'D', status: 'in_progress' },
      { id: 5, name: 'e', title: 'E', status: 'done' },
      { id: 6, name: 'f', title: 'F', status: 'done' },
      { id: 7, name: 'g', title: 'G', status: 'blocked' },
    ]
    const c = countByStatus(features)
    expect(c).toEqual({
      pending: 2,
      spec_ready: 1,
      in_progress: 1,
      done: 2,
      blocked: 1,
      total: 7,
    })
  })

  it('returns zero counts for an empty list', () => {
    const c = countByStatus([])
    expect(c.total).toBe(0)
    expect(c.pending).toBe(0)
  })
})
