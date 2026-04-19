import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAdminStore } from '@/stores/admin'

const BASE = '/vault-api'

async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const key = useAdminStore.getState().key
  const res = await fetch(BASE + path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Admin-Key': key,
      ...options.headers,
    },
  })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
}

async function adminPost<T>(path: string, body?: unknown): Promise<T> {
  return adminFetch<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
}

export interface LicenseStatus {
  tier: 'community' | 'teams'
  license_id?: string
  features: string[]
  seats?: number
  expires_at?: string
  last_heartbeat?: string
  grace_ok: boolean
  grace_remaining_hours?: number
}

export function useLicenseStatus() {
  return useQuery({
    queryKey: ['admin', 'license', 'status'],
    queryFn: () => adminFetch<LicenseStatus>('/admin/license/status'),
    retry: false,
    staleTime: 30_000,
  })
}

export function useLicenseActivate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (licenseKey: string) =>
      adminPost<{ status: string }>('/admin/license/activate', { license_key: licenseKey }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'license'] }),
  })
}

export function useLicenseDeactivate() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => adminPost<{ status: string }>('/admin/license/deactivate'),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'license'] }),
  })
}
