import { useQuery } from '@tanstack/react-query'
import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'

async function adminFetch<T>(path: string): Promise<T> {
  const key = useAdminStore.getState().key
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', 'X-Admin-Key': key },
  })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
}

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
