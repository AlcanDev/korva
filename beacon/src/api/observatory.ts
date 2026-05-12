import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { adminFetch, adminPost } from './_fetch'

// ── Wire types ────────────────────────────────────────────────────────────────

export interface IDE {
  name: string
  version?: string
  config_path?: string
  has_korva_mcp: boolean
  is_default: boolean
}

export interface VaultStatus {
  running: boolean
  port: number
  pid: number
  uptime_sec: number
  version: string
}

export interface HiveStatus {
  enabled: boolean
  endpoint?: string
  phase?: string
  pending_outbox: number
  consecutive_errors: number
  last_sync_at?: string
  last_error?: string
  pull_count?: number
}

export interface SentinelStatus {
  enabled: boolean
  hooks_installed: string[]
  rules_total: number
  builtin_count: number
  custom_count: number
  rules_path: string
  profile: string
}

export interface LoreStatus {
  active_scrolls: string[]
  available_scrolls_count: number
}

export interface SkillsStatus {
  installed_count: number
  last_sync_at: string | null
  sync_status: string
}

export interface LicenseStatus {
  tier: string
  expiration_at: string | null
  seats_used: number
  seats_total: number
}

export interface SystemStatus {
  ide: IDE[]
  vault: VaultStatus
  hive: HiveStatus
  sentinel: SentinelStatus
  lore: LoreStatus
  skills: SkillsStatus
  license: LicenseStatus
  sessions: { total: number; active_24h: number }
  observations: { total: number; by_type: Record<string, number> }
  prompts: { total: number }
}

export interface KorvaConfig {
  version: string
  project: string
  team: string
  country: string
  agent: string
  vault: VaultConfigShape
  lore: LoreConfigShape
  sentinel: SentinelConfigShape
  hive: HiveConfigShape
  license: LicenseConfigShape
}

export interface VaultConfigShape {
  port: number
  auto_start: boolean
  sync_repo?: string
  sync_branch?: string
  auto_sync?: boolean
  sync_interval_minutes?: number
  private_patterns?: string[]
  retention_days?: number
  webhook_url?: string
}

export interface LoreConfigShape {
  active_scrolls: string[]
  scroll_priority?: string
}

export interface SentinelConfigShape {
  enabled: boolean
  hooks: string[]
  rules_path?: string
  block_on_violation?: boolean
}

export interface HiveConfigShape {
  enabled: boolean
  endpoint: string
  interval_minutes: number
  allowed_types: string[]
  reject_patterns?: string[]
}

export interface LicenseConfigShape {
  activation_url?: string
}

export interface ConfigResponse {
  scope: 'local' | 'global'
  path: string
  hash: string
  config: KorvaConfig
  schema_version: string
  exists: boolean
}

export interface UpdateConfigBody {
  scope: 'local' | 'global'
  expected_hash?: string
  config: KorvaConfig
}

export interface UpdateConfigResponse {
  status: string
  snapshot_id?: string
  hash: string
  restart_required: string[]
  path: string
  field?: string
}

export interface TokenStatsTotals {
  input_tokens: number
  output_tokens: number
  cache_read: number
  cache_creation: number
  interactions_count: number
  estimated_count: number
}

export interface TokenStatsBucket {
  input_tokens: number
  output_tokens: number
  cache_read: number
  count: number
}

export interface TokenStatsDaily {
  date: string
  input_tokens: number
  output_tokens: number
  cache_read: number
  estimated: boolean
}

export interface TokenStats {
  totals: TokenStatsTotals
  cache_hit_pct: number
  reduction_pct_estimated: number
  baseline_naive_tokens: number
  baseline_dir: string
  by_model: Record<string, TokenStatsBucket>
  by_project: Record<string, TokenStatsBucket>
  daily: TokenStatsDaily[]
}

export interface ActivityRow {
  id: string
  ts: string
  project: string
  team?: string
  agent: string
  model: string
  duration_ms: number
  input_tokens: number
  output_tokens: number
  cache_read: number
  cache_creation: number
  prompt_excerpt: string
  status: string
  estimated: boolean
}

export interface ActivityResponse {
  interactions: ActivityRow[]
  total?: number
  limit: number
  offset: number
}

export interface InteractionDetail extends ActivityRow {
  response_excerpt?: string
  tool_calls?: unknown
  error_msg?: string
  created_at?: string
}

export interface BuiltinSentinelRule {
  id: string
  description: string
  severity: string
}

export interface CustomSentinelRule {
  id: string
  description?: string
  severity?: string
  pattern: string
  paths_include?: string[]
  paths_exclude?: string[]
  message?: string
}

export interface SentinelRulesResponse {
  profile: string
  rules_path: string
  builtin: BuiltinSentinelRule[]
  custom: CustomSentinelRule[]
}

export interface UpdateSentinelRulesBody {
  profile?: string
  custom_rules: CustomSentinelRule[]
}

export interface UpdateSentinelRulesResponse {
  status: string
  rules_path: string
  rules_count: number
}

export interface TestSentinelRuleBody {
  rule: CustomSentinelRule
  code: string
  file_path: string
}

export interface TestSentinelRuleMatch {
  line: number
  column: number
  matched_text: string
  message: string
}

export interface TestSentinelRuleResponse {
  matches: TestSentinelRuleMatch[]
  applies: boolean
}

export interface RestartResponse {
  status: string
  old_pid?: number
  executable?: string
}

// ── Conflict judgment workflow types ────────────────────────────────────────

export type JudgmentStatus = 'pending' | 'judged' | 'orphaned' | 'ignored'

export interface ConflictRow {
  id: string
  source_id: string
  target_id: string
  relation: string
  project: string
  status: string
  judgment_status: JudgmentStatus
  confidence: number
  reason?: string
  evidence?: string
  marked_by_actor: string
  marked_by_kind: string
  marked_by_model?: string
  session_id?: string
  created_at: string
  judged_at?: string
}

export interface ConflictListResponse {
  conflicts: ConflictRow[]
  count: number
  status: string
  project?: string
}

export interface ConflictObservation {
  id: string
  type: string
  title: string
  content: string
  project: string
  created_at?: string
}

export interface ConflictDetailResponse {
  conflict: ConflictRow
  source: ConflictObservation | null
  target: ConflictObservation | null
}

export interface JudgeConflictBody {
  relation: string
  reason?: string
  evidence?: string
  confidence: number
  marked_by_actor?: string
  marked_by_kind?: string
  marked_by_model?: string
  session_id?: string
}

export interface IgnoreConflictBody {
  reason?: string
  session_id?: string
}

export interface ScanConflictsResponse {
  observation_id: string
  candidate_count: number
  judgment_ids: string[]
}

// ── Hooks ─────────────────────────────────────────────────────────────────────

export function useSystemStatus() {
  return useQuery({
    queryKey: ['observatory', 'system-status'],
    queryFn: () => adminFetch<SystemStatus>('/admin/system-status'),
    refetchInterval: 15_000,
    retry: false,
  })
}

export function useConfig(scope: 'local' | 'global' = 'local') {
  return useQuery({
    queryKey: ['observatory', 'config', scope],
    queryFn: () => adminFetch<ConfigResponse>(`/admin/config?scope=${scope}`),
    retry: false,
  })
}

export function useUpdateConfig() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: UpdateConfigBody) =>
      adminFetch<UpdateConfigResponse>('/admin/config', {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ['observatory', 'config', vars.scope] })
      qc.invalidateQueries({ queryKey: ['observatory', 'system-status'] })
    },
  })
}

export interface TokenStatsParams {
  from?: string
  to?: string
}

export function useTokenStats(params: TokenStatsParams = {}) {
  const qs = new URLSearchParams()
  if (params.from) qs.set('from', params.from)
  if (params.to) qs.set('to', params.to)
  const search = qs.toString()
  return useQuery({
    queryKey: ['observatory', 'tokens', search],
    queryFn: () => adminFetch<TokenStats>(`/admin/tokens/stats${search ? '?' + search : ''}`),
    retry: false,
  })
}

export interface ActivityParams {
  project?: string
  model?: string
  agent?: string
  status?: string
  q?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export function useActivity(params: ActivityParams = {}) {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') qs.set(k, String(v))
  }
  const search = qs.toString()
  return useQuery({
    queryKey: ['observatory', 'activity', search],
    queryFn: () => adminFetch<ActivityResponse>(`/admin/activity${search ? '?' + search : ''}`),
    retry: false,
  })
}

export function useInteraction(id: string) {
  return useQuery({
    queryKey: ['observatory', 'activity', 'detail', id],
    queryFn: () => adminFetch<InteractionDetail>(`/admin/activity/${id}`),
    enabled: !!id,
    retry: false,
  })
}

export function useSentinelRules() {
  return useQuery({
    queryKey: ['observatory', 'sentinel', 'rules'],
    queryFn: () => adminFetch<SentinelRulesResponse>('/admin/sentinel/rules'),
    retry: false,
  })
}

export function useUpdateSentinelRules() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: UpdateSentinelRulesBody) =>
      adminFetch<UpdateSentinelRulesResponse>('/admin/sentinel/rules', {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['observatory', 'sentinel', 'rules'] })
      qc.invalidateQueries({ queryKey: ['observatory', 'system-status'] })
    },
  })
}

export function useTestSentinelRule() {
  return useMutation({
    mutationFn: (body: TestSentinelRuleBody) =>
      adminPost<TestSentinelRuleResponse>('/admin/sentinel/test', body),
  })
}

export function useRestartVault() {
  return useMutation({
    mutationFn: () => adminPost<RestartResponse>('/admin/vault/restart'),
  })
}

// ── Conflict workflow hooks ─────────────────────────────────────────────────

export function useConflicts(status: JudgmentStatus = 'pending', project?: string, limit = 50) {
  const params = new URLSearchParams()
  params.set('status', status)
  if (project) params.set('project', project)
  params.set('limit', String(limit))
  const qs = params.toString()
  return useQuery({
    queryKey: ['observatory', 'conflicts', status, project ?? '', limit],
    queryFn: () => adminFetch<ConflictListResponse>(`/admin/conflicts?${qs}`),
    retry: false,
    refetchInterval: 30_000,
  })
}

export function useConflict(id: string | null) {
  return useQuery({
    queryKey: ['observatory', 'conflicts', 'detail', id],
    queryFn: () => adminFetch<ConflictDetailResponse>(`/admin/conflicts/${id}`),
    enabled: !!id,
    retry: false,
  })
}

export function useJudgeConflict() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: JudgeConflictBody }) =>
      adminFetch<ConflictRow>(`/admin/conflicts/${id}/judge`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['observatory', 'conflicts'] })
      qc.invalidateQueries({ queryKey: ['observatory', 'system-status'] })
    },
  })
}

export function useIgnoreConflict() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: IgnoreConflictBody }) =>
      adminFetch<{ status: string; id: string }>(`/admin/conflicts/${id}/ignore`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['observatory', 'conflicts'] })
    },
  })
}

export function useScanConflicts() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (observationID: string) =>
      adminPost<ScanConflictsResponse>(`/admin/observations/${observationID}/scan-conflicts`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['observatory', 'conflicts'] })
    },
  })
}
