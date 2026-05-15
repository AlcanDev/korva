import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'

// ── Admin-authenticated fetch ─────────────────────────────────────────────────

async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const { key, sessionToken, authMode } = useAdminStore.getState()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authMode === 'session') {
    headers['X-Session-Token'] = sessionToken
  } else {
    headers['X-Admin-Key'] = key
  }
  if (options.headers) Object.assign(headers, options.headers)
  const res = await fetch(BASE + path, { ...options, headers })
  if (res.status === 401 || res.status === 403) {
    throw new Error('UNAUTHORIZED')
  }
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

async function adminPost<T>(path: string, body?: unknown): Promise<T> {
  return adminFetch<T>(path, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  })
}

// ── Types ─────────────────────────────────────────────────────────────────────

export interface DailyCount {
  date: string  // "2026-05-01"
  count: number
}

export interface SessionRow {
  id: string
  project: string
  goal: string
  agent: string
  obs_count: number
  started_at: string
  ended_at?: string
  duration_min: number
}

export interface AdminStats {
  total_observations: number
  total_sessions: number
  total_prompts: number
  total_content_len: number
  by_type: Record<string, number>
  by_project: Record<string, number>
  by_team: Record<string, number>
  by_country: Record<string, number>
  daily_activity: DailyCount[]
  recent_sessions: SessionRow[]
}

export interface Session {
  id: string
  project: string
  team: string
  country: string
  agent: string
  goal: string
  summary: string
  started_at: string
  ended_at: string | null
}

export interface Observation {
  id: string
  session_id?: string
  project: string
  team: string
  country: string
  type: string
  title: string
  content: string
  tags: string[]
  author: string
  created_at: string
}

export interface Prompt {
  id: string
  name: string
  content: string
  tags: string[]
  created_at: string
  updated_at: string
}

export interface PromptsResponse {
  prompts: Prompt[]
  total: number
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useAdminStats() {
  return useQuery({
    queryKey: ['admin', 'stats'],
    queryFn: () => adminFetch<AdminStats>('/admin/stats'),
    retry: false,
  })
}

export interface SearchResponse {
  results: Observation[]
  count: number
  total?: number   // present when query is empty (non-FTS path)
  limit: number
  offset: number
}

export function useAdminSearch(query: string, project = '', type = '', limit = 20, offset = 0) {
  return useQuery({
    queryKey: ['admin', 'search', query, project, type, limit, offset],
    queryFn: () =>
      adminFetch<SearchResponse>(
        `/api/v1/search?q=${encodeURIComponent(query)}&project=${encodeURIComponent(project)}&type=${encodeURIComponent(type)}&limit=${limit}&offset=${offset}`
      ),
    enabled: true,
  })
}

export function useAdminSessions() {
  return useQuery({
    queryKey: ['admin', 'sessions'],
    queryFn: () => adminFetch<{ sessions: Session[] }>('/api/v1/sessions/all'),
    retry: false,
  })
}

export function useAdminSessionsWithStats() {
  return useQuery({
    queryKey: ['admin', 'sessions', 'stats'],
    queryFn: () => adminFetch<{ sessions: SessionRow[]; total: number }>('/admin/sessions'),
    retry: false,
  })
}

export function useAdminDeleteObservation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      adminFetch<void>(`/admin/observations/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin'] })
    },
  })
}

export function useAdminPurge() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => adminPost<void>('/admin/purge'),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin'] }),
  })
}

export function useAdminPrompts() {
  return useQuery({
    queryKey: ['admin', 'prompts'],
    queryFn: () => adminFetch<PromptsResponse>('/admin/prompts'),
    retry: false,
  })
}

export function useAdminSavePrompt() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { name: string; content: string; tags: string[] }) =>
      adminPost<{ status: string }>('/api/v1/prompts', body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'prompts'] }),
  })
}

export function useAdminDeletePrompt() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (name: string) =>
      adminFetch<void>(`/admin/prompts/${encodeURIComponent(name)}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'prompts'] }),
  })
}

// ── Private Scrolls ───────────────────────────────────────────────────────────

export interface PrivateScroll {
  id: string
  name: string
  content: string
  created_at: string
  updated_at: string
}

export function useAdminPrivateScrolls() {
  return useQuery({
    queryKey: ['admin', 'scrolls', 'private'],
    queryFn: () => adminFetch<PrivateScroll[]>('/admin/scrolls/private'),
    retry: false,
  })
}

// ── Auth check ────────────────────────────────────────────────────────────────

export async function checkAdminKey(key: string): Promise<boolean> {
  const res = await fetch(`${BASE}/admin/stats`, {
    headers: { 'X-Admin-Key': key },
  })
  return res.ok
}

// checkSessionToken verifies a session token has role=admin.
// Calls GET /auth/me and checks the returned role field.
export async function checkSessionToken(token: string): Promise<boolean> {
  const res = await fetch(`${BASE}/auth/me`, {
    headers: { 'X-Session-Token': token },
  })
  if (!res.ok) return false
  const data = await res.json() as { role?: string }
  return data.role === 'admin'
}
