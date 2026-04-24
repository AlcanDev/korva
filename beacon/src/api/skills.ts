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
  version: number
  updated_by: string
  scope: string
  created_at: string
  updated_at: string
}

export interface SkillHistoryEntry {
  id: string
  skill_id: string
  version: number
  body: string
  changed_by: string
  summary: string
  changed_at: string
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

export function useSkillHistory(skillId: string | null) {
  return useQuery({
    queryKey: ['admin', 'skills', skillId, 'history'],
    queryFn: () =>
      adminFetch<{ history: SkillHistoryEntry[]; skill_id: string; count: number }>(
        `/admin/skills/${skillId}/history`
      ),
    enabled: !!skillId,
    retry: false,
  })
}

export function useSaveSkill() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: {
      team_id: string
      name: string
      body: string
      tags?: string
      scope?: string
      summary?: string
    }) =>
      adminFetch<{ status: string; id: string; version: number }>('/admin/skills', {
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

export interface SyncStatusEntry {
  user_email: string
  last_sync: string
  skills_count: number
  target: string
  is_up_to_date: boolean
}

export function useSyncStatus(teamId?: string) {
  return useQuery({
    queryKey: ['admin', 'skills', 'sync-status', teamId],
    queryFn: () =>
      adminFetch<{ entries: SyncStatusEntry[]; latest_skill_at: string; count: number }>(
        `/admin/skills/sync-status${teamId ? `?team_id=${teamId}` : ''}`
      ),
    retry: false,
    refetchInterval: 30_000,
  })
}
