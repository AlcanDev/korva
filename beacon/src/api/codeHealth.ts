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

export interface CodeHealthProject {
  project: string
  sdd_phase: string
  avg_score: number
  total_checkpoints: number
  last_score: number
  last_status: string
  last_phase: string
  last_checked_at: string
  gate_passed: boolean
  bugfix_count: number
  pattern_count: number
}

export interface RecentCheckpoint {
  id: string
  project: string
  phase: string
  status: string
  score: number
  gate_passed: boolean
  notes: string
  created_at: string
}

export interface CodeHealthData {
  projects: CodeHealthProject[]
  recent: RecentCheckpoint[]
  project_count: number
}

export function useCodeHealth() {
  return useQuery({
    queryKey: ['admin', 'code-health'],
    queryFn: () => adminFetch<CodeHealthData>('/admin/code-health'),
    retry: false,
    refetchInterval: 60_000,
  })
}
