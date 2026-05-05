import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminFetch, adminPost } from './_fetch'

export type LicenseTier = 'community' | 'teams' | 'business' | 'enterprise'

export interface LicenseStatus {
  tier: LicenseTier
  license_id?: string
  features: string[]
  seats?: number
  expires_at?: string
  last_heartbeat?: string
  grace_ok: boolean
  grace_remaining_hours?: number
}

/** Returns true for any paid tier that unlocks Teams features. */
export function isPaidTier(tier: LicenseTier | undefined): boolean {
  return tier === 'teams' || tier === 'business' || tier === 'enterprise'
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
