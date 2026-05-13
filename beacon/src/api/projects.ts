import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminFetch, adminPost } from './_fetch'

// Fase 6 — wrapper TanStack Query para los endpoints `/admin/projects*`.
// Mantiene el mismo patrón que `@/api/observatory` (useConflicts, useJudge…):
// queries idempotentes + mutaciones que invalidan la lista para refrescar
// el panel sin recargas manuales.

// ── Wire types ────────────────────────────────────────────────────────────────

export interface ProjectStats {
  name: string
  observation_count: number
  session_count: number
}

export interface ConsolidationProposal {
  canonical: string
  variants: ProjectStats[]
}

export interface EmptyProject {
  project: string
  session_count: number
  prompt_count: number
}

export interface PruneResult {
  empty: EmptyProject[] | null
  sessions_removed: number
  prompts_removed: number
  dry_run: boolean
}

export interface ConsolidateResponse {
  status: string
  canonical: string
  sources: string[]
  observations_updated: number
  sessions_updated: number
  prompts_updated: number
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useProjects() {
  return useQuery({
    queryKey: ['projects', 'list'],
    queryFn: () =>
      adminFetch<{ projects: ProjectStats[] | null; count: number }>('/admin/projects'),
  })
}

export function useProjectSuggestions() {
  return useQuery({
    queryKey: ['projects', 'suggestions'],
    queryFn: () =>
      adminFetch<{ proposals: ConsolidationProposal[]; count: number }>(
        '/admin/projects/suggestions',
      ),
  })
}

export function useConsolidateProjects() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { canonical: string; sources: string[] }) =>
      adminPost<ConsolidateResponse>('/admin/projects/consolidate', body),
    onSuccess: () => {
      // Invalidate both: the merge changes counts in `list` and removes
      // proposals from `suggestions`.
      qc.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}

export function usePruneProjects() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: { apply: boolean }) =>
      adminPost<PruneResult>('/admin/projects/prune', body),
    onSuccess: (_, vars) => {
      // Apply mode mutates the store; dry-run doesn't.
      if (vars.apply) qc.invalidateQueries({ queryKey: ['projects'] })
    },
  })
}
