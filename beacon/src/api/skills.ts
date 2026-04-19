import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'

async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const key = useAdminStore.getState().key
  const res = await fetch(BASE + path, {
    ...options,
    headers: { 'Content-Type': 'application/json', 'X-Admin-Key': key, ...options.headers },
  })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
}

export interface Skill {
  id: string
  team_id: string
  name: string
  body: string
  tags: string
  created_at: string
  updated_at: string
}

export function useSkills(teamId?: string) {
  return useQuery({
    queryKey: ['admin', 'skills', teamId],
    queryFn: () =>
      adminFetch<{ skills: Skill[]; count: number }>(
        `/admin/skills${teamId ? `?team_id=${teamId}` : ''}`
      ),
    retry: false,
  })
}

export function useSaveSkill() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { team_id: string; name: string; body: string; tags?: string }) =>
      adminFetch<{ status: string; id: string }>('/admin/skills', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'skills'] }),
  })
}

export function useDeleteSkill() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      adminFetch<void>(`/admin/skills/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'skills'] }),
  })
}
