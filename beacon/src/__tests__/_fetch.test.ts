import { describe, it, expect, beforeEach, vi } from 'vitest'
import { adminFetch, adminPost, adminDelete } from '@/api/_fetch'
import { useAdminStore } from '@/stores/admin'

// Phase 20.A — direct coverage for the API client foundation.
// Every other src/api/*.ts module funnels through adminFetch; if
// it sends the wrong header, mishandles non-2xx, or forgets to
// JSON-encode bodies, every page in the dashboard breaks at once.

interface FetchCall {
  url: string
  init: RequestInit | undefined
}

function captureFetch(response: Response): FetchCall[] {
  const calls: FetchCall[] = []
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : (input as URL).toString()
    calls.push({ url, init })
    return response
  }) as unknown as typeof fetch
  return calls
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

beforeEach(() => {
  // Reset to a known clean state per test.
  useAdminStore.setState({
    key: '',
    sessionToken: '',
    authMode: 'key',
    isAuthenticated: false,
  })
})

describe('adminFetch', () => {
  it('prefixes the path with /vault-api', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({ ok: true }))
    await adminFetch('/admin/stats')
    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe('/vault-api/admin/stats')
  })

  it('sends X-Admin-Key when authMode is "key"', async () => {
    useAdminStore.setState({ key: 'admin-secret', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({}))
    await adminFetch('/admin/stats')
    const headers = calls[0].init?.headers as Record<string, string>
    expect(headers['X-Admin-Key']).toBe('admin-secret')
    expect(headers['X-Session-Token']).toBeUndefined()
  })

  it('sends X-Session-Token when authMode is "session"', async () => {
    useAdminStore.setState({
      key: '', sessionToken: 'session-token-xyz',
      authMode: 'session', isAuthenticated: true,
    })
    const calls = captureFetch(jsonResponse({}))
    await adminFetch('/admin/stats')
    const headers = calls[0].init?.headers as Record<string, string>
    expect(headers['X-Session-Token']).toBe('session-token-xyz')
    expect(headers['X-Admin-Key']).toBeUndefined()
  })

  it('always sets Content-Type: application/json', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({}))
    await adminFetch('/admin/stats')
    const headers = calls[0].init?.headers as Record<string, string>
    expect(headers['Content-Type']).toBe('application/json')
  })

  it('merges caller-supplied headers without dropping auth', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({}))
    await adminFetch('/admin/stats', { headers: { 'X-Custom': 'value' } })
    const headers = calls[0].init?.headers as Record<string, string>
    expect(headers['X-Custom']).toBe('value')
    expect(headers['X-Admin-Key']).toBe('k') // auth not lost in merge
  })

  it('decodes the JSON body and returns it typed', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    captureFetch(jsonResponse({ count: 42, items: ['a'] }))
    const got = await adminFetch<{ count: number; items: string[] }>('/admin/stats')
    expect(got).toEqual({ count: 42, items: ['a'] })
  })

  it('throws with the status code on non-2xx', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    captureFetch(new Response('forbidden', { status: 403 }))
    await expect(adminFetch('/admin/stats')).rejects.toThrowError('403')
  })

  it('throws on 5xx with the status code', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    captureFetch(new Response('boom', { status: 500 }))
    await expect(adminFetch('/admin/stats')).rejects.toThrowError('500')
  })
})

describe('adminPost', () => {
  it('uses POST and serializes the body to JSON', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({ id: 'abc' }))
    const got = await adminPost<{ id: string }>('/admin/observations', { title: 'x', content: 'y' })

    expect(calls[0].init?.method).toBe('POST')
    expect(calls[0].init?.body).toBe(JSON.stringify({ title: 'x', content: 'y' }))
    expect(got).toEqual({ id: 'abc' })
  })

  it('omits the body when not provided', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({}))
    await adminPost('/admin/restart')
    expect(calls[0].init?.body).toBeUndefined()
  })
})

describe('adminDelete', () => {
  it('uses DELETE and returns the JSON response', async () => {
    useAdminStore.setState({ key: 'k', authMode: 'key', isAuthenticated: true, sessionToken: '' })
    const calls = captureFetch(jsonResponse({ status: 'deleted' }))
    const got = await adminDelete<{ status: string }>('/admin/observations/123')
    expect(calls[0].init?.method).toBe('DELETE')
    expect(got).toEqual({ status: 'deleted' })
  })
})
