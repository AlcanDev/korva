import { useQuery } from '@tanstack/react-query'
import { adminFetch } from './_fetch'

// Phase 18.C — Beacon side of the editor-adoption telemetry.
//
// Reads from the vault's /admin/editor/adoption endpoint. The vault
// validates the editor id against harness.AllEditors before storing,
// so we can trust the rows we receive — but we still treat the empty
// string as the "anonymous / didn't opt in" bucket for display
// purposes.

export interface EditorAdoptionRow {
  editor: string
  count: number
  // Phase 19.D — count broken down by telemetry channel. `http` is
  // POST /api/v1/interactions with X-Korva-Editor; `mcp` is the
  // stdio MCP initialize.clientInfo.name path. Sums to `count`.
  by_channel: {
    http: number
    mcp: number
  }
}

export interface EditorAdoptionPayload {
  window_days: number
  total: number
  rows: EditorAdoptionRow[]
}

export function useEditorAdoption(windowDays: number = 7) {
  return useQuery({
    queryKey: ['admin', 'editor', 'adoption', windowDays],
    queryFn: () =>
      adminFetch<EditorAdoptionPayload>(`/admin/editor/adoption?days=${windowDays}`),
    retry: false,
    // 60s — adoption is a trailing indicator, no need to refresh aggressively.
    refetchInterval: 60_000,
  })
}
