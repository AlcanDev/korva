import { useQuery } from '@tanstack/react-query'
import { adminFetch } from './_fetch'

export interface CallEntry {
  id: string
  tool: string
  project: string
  author: string
  status: 'ok' | 'error'
  latency_ms: number
  error_msg?: string
  created_at: string
}

export interface CallStats {
  Total: number
  ErrorCount: number
  AvgLatency: number
  ByTool: Record<string, number>
  ByStatus: Record<string, number>
}

export interface ListCallsFilters {
  tool?: string
  project?: string
  author?: string
  status?: string
  limit?: number
  offset?: number
}

export function useInteractions(filters: ListCallsFilters = {}) {
  const params = new URLSearchParams()
  if (filters.tool)    params.set('tool', filters.tool)
  if (filters.project) params.set('project', filters.project)
  if (filters.author)  params.set('author', filters.author)
  if (filters.status)  params.set('status', filters.status)
  if (filters.limit)   params.set('limit', String(filters.limit))
  if (filters.offset)  params.set('offset', String(filters.offset))

  const qs = params.toString()
  return useQuery({
    queryKey: ['admin', 'interactions', filters],
    queryFn: () => adminFetch<{ calls: CallEntry[]; count: number }>(
      `/admin/interactions${qs ? '?' + qs : ''}`
    ),
    retry: false,
    refetchInterval: 15_000,
  })
}

export function useInteractionStats() {
  return useQuery({
    queryKey: ['admin', 'interactions', 'stats'],
    queryFn: () => adminFetch<CallStats>('/admin/interactions/stats'),
    retry: false,
    refetchInterval: 15_000,
  })
}
