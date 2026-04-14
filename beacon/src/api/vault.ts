import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'

const BASE = '/vault-api'

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json() as Promise<T>
}

// Types
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

export interface VaultStats {
  total_observations: number
  total_sessions: number
  total_prompts: number
  by_type: Record<string, number>
  by_project: Record<string, number>
  by_team: Record<string, number>
}

export interface SearchResult {
  results: Observation[]
  count: number
}

export interface ProjectSummary {
  project: string
  observations: number
  sessions: number
  top_tags: string[]
  recent: Observation[]
  decisions: Observation[]
}

export interface HealthStatus {
  status: string
  service: string
}

// Health check (used for the status indicator in the header)
export function useVaultHealth() {
  return useQuery({
    queryKey: ['vault', 'health'],
    queryFn: () => fetchJSON<HealthStatus>('/healthz'),
    refetchInterval: 2000,
    retry: false,
  })
}

// Stats
export function useVaultStats() {
  return useQuery({
    queryKey: ['vault', 'stats'],
    queryFn: () => fetchJSON<VaultStats>('/api/v1/stats'),
  })
}

// Search observations
export function useSearch(query: string, filters: {
  project?: string
  team?: string
  country?: string
  type?: string
}) {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  if (filters.project) params.set('project', filters.project)
  if (filters.team) params.set('team', filters.team)
  if (filters.country) params.set('country', filters.country)
  if (filters.type) params.set('type', filters.type)

  return useQuery({
    queryKey: ['vault', 'search', query, filters],
    queryFn: () => fetchJSON<SearchResult>(`/api/v1/search?${params}`),
    enabled: true,
  })
}

// Project context
export function useContext(project: string) {
  return useQuery({
    queryKey: ['vault', 'context', project],
    queryFn: () => fetchJSON<{ context: Observation[]; project: string }>(
      `/api/v1/context/${encodeURIComponent(project)}`
    ),
    enabled: !!project,
  })
}

// Project summary
export function useProjectSummary(project: string) {
  return useQuery({
    queryKey: ['vault', 'summary', project],
    queryFn: () => fetchJSON<ProjectSummary>(
      `/api/v1/summary/${encodeURIComponent(project)}`
    ),
    enabled: !!project,
  })
}

// Save observation (mutation)
export function useSaveObservation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (obs: Partial<Observation>) =>
      postJSON<{ id: string }>('/api/v1/observations', obs),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['vault'] })
    },
  })
}
