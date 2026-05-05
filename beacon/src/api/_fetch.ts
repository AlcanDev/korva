import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'

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
  const res = await fetch(BASE + path, { ...options, headers })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
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
