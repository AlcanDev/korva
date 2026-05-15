import { useQuery } from '@tanstack/react-query'
import { adminFetch } from './_fetch'

// Phase 14.3 — Beacon client for the harness REST surface introduced in
// Phase 14.2. The vault filters every read by the caller's team_id, so
// these hooks return only the projects the current session belongs to.

// ── wire types ─────────────────────────────────────────────────────────────

export interface HarnessProjectSummary {
  team_id: string
  project: string
  root: string
  updated_at: string                     // RFC3339
  last_transition_at?: string            // RFC3339, omitted when never
  last_transition_to?: string            // status name, omitted when never
}

export interface HarnessSnapshot {
  team_id: string
  project: string
  root: string
  payload: string                        // raw feature_list.json JSON string
  updated_at: string
}

export interface HarnessTransition {
  id: string
  team_id: string
  project: string
  root: string
  feature_id: number
  from_status: string
  to_status: string
  owner?: string
  occurred_at: string
}

export interface HarnessProjectsResponse {
  projects: HarnessProjectSummary[]
  count: number
}

export interface HarnessTransitionsResponse {
  transitions: HarnessTransition[]
  count: number
  limit: number
}

// ── parsed snapshot helper ─────────────────────────────────────────────────

// FeatureListPayload mirrors the harness package's wire shape. We define
// it locally rather than importing from a shared types package because
// the vault stores the raw JSON string and the dashboard parses on
// render — keeps the API surface stable even if the harness schema
// evolves field-by-field.
export interface FeatureListFeature {
  id: number
  name: string
  title: string
  description?: string
  acceptance?: string[]
  status: 'pending' | 'spec_ready' | 'in_progress' | 'done' | 'blocked'
  sdd?: boolean
  owner_agent?: string
  updated_at?: string
  // Phase 19.B — last persisted spec review verdict, set by
  // `korva harness review <id> --record` or its MCP equivalent.
  // Optional: only SDD features that have been reviewed at least
  // once carry it.
  review?: ReviewDecision
}

export type ReviewVerdict = 'approve' | 'needs_fixes' | 'reject'

export interface ReviewDecision {
  verdict: ReviewVerdict
  reviewer?: string
  at: string
  issue_count: number
  error_count: number
  note?: string
}

export interface FeatureListPayload {
  project: string
  description?: string
  rules: {
    one_feature_at_a_time: boolean
    require_tests_to_close: boolean
    require_approved_spec_to_implement?: boolean
    valid_status: string[]
  }
  features: FeatureListFeature[]
}

// safeParseFeatureList tries to parse a snapshot's payload string into
// the typed shape. Returns null on any parse error — callers render a
// fallback rather than crashing the page when the vault has stored a
// malformed snapshot (shouldn't happen but defensive).
export function safeParseFeatureList(raw: string): FeatureListPayload | null {
  try {
    return JSON.parse(raw) as FeatureListPayload
  } catch {
    return null
  }
}

// ── status counts (computed client-side from the snapshot) ─────────────────

export interface StatusCounts {
  pending: number
  spec_ready: number
  in_progress: number
  done: number
  blocked: number
  total: number
}

export function countByStatus(features: FeatureListFeature[]): StatusCounts {
  const c: StatusCounts = {
    pending: 0,
    spec_ready: 0,
    in_progress: 0,
    done: 0,
    blocked: 0,
    total: features.length,
  }
  for (const f of features) c[f.status]++
  return c
}

// ── hooks ──────────────────────────────────────────────────────────────────

// useHarnessProjects powers the dashboard grid. Refetches every 30s so
// the page reflects fresh transitions without a manual refresh.
export function useHarnessProjects() {
  return useQuery({
    queryKey: ['harness', 'projects'],
    queryFn: () => adminFetch<HarnessProjectsResponse>('/api/v1/harness/projects'),
    retry: false,
    refetchInterval: 30_000,
  })
}

// useHarnessProject returns the full snapshot for a single (project, root).
// Both args must be non-empty for the query to fire — undefined disables
// it (used during initial render before the route param resolves).
export function useHarnessProject(project: string | undefined, root: string | undefined) {
  return useQuery({
    queryKey: ['harness', 'project', project, root],
    queryFn: () => {
      const qs = new URLSearchParams({ root: root ?? '' }).toString()
      return adminFetch<HarnessSnapshot>(
        `/api/v1/harness/projects/${encodeURIComponent(project ?? '')}?${qs}`,
      )
    },
    enabled: Boolean(project && root),
    retry: false,
    refetchInterval: 30_000,
  })
}

// useHarnessTransitions returns the timeline. project optional (empty
// → team-wide). limit clamped server-side to [1, 1000]; we ask for 200
// here as a sane default for the timeline view.
export function useHarnessTransitions(project?: string, limit = 200) {
  return useQuery({
    queryKey: ['harness', 'transitions', project ?? 'all', limit],
    queryFn: () => {
      const qs = new URLSearchParams()
      if (project) qs.set('project', project)
      qs.set('limit', String(limit))
      return adminFetch<HarnessTransitionsResponse>(
        `/api/v1/harness/transitions?${qs.toString()}`,
      )
    },
    retry: false,
    refetchInterval: 30_000,
  })
}
