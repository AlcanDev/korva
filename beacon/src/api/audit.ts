import { useQuery } from '@tanstack/react-query'
import { adminFetch } from './_fetch'

export interface AuditEntry {
  id: string
  actor: string
  action: string
  target: string
  before_hash: string
  after_hash: string
  created_at: string
}

export function useAuditLogs() {
  return useQuery({
    queryKey: ['admin', 'audit'],
    queryFn: () => adminFetch<{ logs: AuditEntry[]; count: number }>('/admin/audit'),
    retry: false,
    refetchInterval: 30_000,
  })
}
