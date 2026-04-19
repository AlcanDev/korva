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

async function adminPost<T>(path: string, body?: unknown): Promise<T> {
  return adminFetch<T>(path, { method: 'POST', body: JSON.stringify(body) })
}

async function adminDelete<T>(path: string): Promise<T> {
  return adminFetch<T>(path, { method: 'DELETE' })
}

export interface Team {
  id: string
  name: string
  owner: string
  license_id: string
  created_at: string
}

export interface TeamMember {
  id: string
  team_id: string
  email: string
  role: string
  created_at: string
}

export function useTeams() {
  return useQuery({
    queryKey: ['admin', 'teams'],
    queryFn: () => adminFetch<{ teams: Team[] }>('/admin/teams'),
    retry: false,
  })
}

export function useTeamMembers(teamId: string) {
  return useQuery({
    queryKey: ['admin', 'teams', teamId, 'members'],
    queryFn: () => adminFetch<{ members: TeamMember[] }>(`/admin/teams/${teamId}/members`),
    enabled: !!teamId,
  })
}

export function useCreateTeam() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; owner: string }) =>
      adminPost<{ id: string }>('/admin/teams', data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams'] }),
  })
}

export function useAddMember(teamId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { email: string; role: string }) =>
      adminPost<{ id: string }>(`/admin/teams/${teamId}/members`, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams', teamId] }),
  })
}

export function useRemoveMember(teamId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (memberId: string) =>
      adminDelete<void>(`/admin/teams/${teamId}/members/${memberId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams', teamId] }),
  })
}

// ── Invites ───────────────────────────────────────────────────────────────────

export interface Invite {
  id: string
  email: string
  expires_at: string
  used_at?: string
  status: 'pending' | 'used' | 'expired'
  /** plaintext token — present only on creation, never on list */
  token?: string
}

export function useTeamInvites(teamId: string) {
  return useQuery({
    queryKey: ['admin', 'teams', teamId, 'invites'],
    queryFn: () => adminFetch<{ invites: Invite[]; count: number }>(`/admin/teams/${teamId}/invites`),
    enabled: !!teamId,
    refetchInterval: 15_000,
  })
}

export function useCreateInvite(teamId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (email: string) =>
      adminPost<Invite>(`/admin/teams/${teamId}/invites`, { email }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams', teamId, 'invites'] }),
  })
}

export function useRevokeInvite(teamId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (inviteId: string) =>
      adminDelete<void>(`/admin/teams/${teamId}/invites/${inviteId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams', teamId, 'invites'] }),
  })
}

// ── Sessions ──────────────────────────────────────────────────────────────────

export interface MemberSession {
  id: string
  member_id: string
  email: string
  created_at: string
  last_seen: string
  expires_at: string
  status: 'active' | 'expired'
}

export function useTeamSessions(teamId: string) {
  return useQuery({
    queryKey: ['admin', 'teams', teamId, 'sessions'],
    queryFn: () =>
      adminFetch<{ sessions: MemberSession[]; count: number }>(`/admin/teams/${teamId}/sessions`),
    enabled: !!teamId,
    refetchInterval: 20_000,
  })
}

export function useRevokeTeamSession(teamId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (sessionId: string) =>
      adminDelete<void>(`/admin/teams/${teamId}/sessions/${sessionId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'teams', teamId, 'sessions'] }),
  })
}
