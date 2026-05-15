import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'
const TIMEOUT_MS = 10_000

function withTimeout(options: RequestInit): [RequestInit, () => void] {
  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), TIMEOUT_MS)
  const cancel = () => clearTimeout(timer)
  return [{ ...options, signal: ctrl.signal }, cancel]
}

/** Authenticated fetch for admin API routes. Sends X-Admin-Key or X-Session-Token
 *  depending on the active auth mode in the admin store. */
export async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { key, sessionToken, authMode } = useAdminStore.getState()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authMode === 'session') {
    headers['X-Session-Token'] = sessionToken
  } else {
    headers['X-Admin-Key'] = key
  }
  if (options.headers) Object.assign(headers, options.headers)
  const [opts, cancel] = withTimeout({ ...options, headers })
  try {
    const res = await fetch(BASE + path, opts)
    if (!res.ok) throw new Error(`${res.status}`)
    return res.json() as Promise<T>
  } finally {
    cancel()
  }
}

export async function adminPost<T>(path: string, body?: unknown): Promise<T> {
  return adminFetch<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
}

export async function adminDelete<T>(path: string): Promise<T> {
  return adminFetch<T>(path, { method: 'DELETE' })
}
