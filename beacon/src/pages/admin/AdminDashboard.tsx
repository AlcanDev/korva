import { Brain, Clock, FolderGit2, Zap, TrendingUp, Database, LayoutDashboard, ArrowRight } from 'lucide-react'
import { NavLink } from 'react-router'
import { useAdminStats, type DailyCount, type SessionRow } from '@/api/admin'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminDashboard() {
  const { data: stats, isLoading, error } = useAdminStats()
  const { t } = useI18n()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-[#388bfd]" />
      </div>
    )
  }

  if (error) {
    const msg = error.message
    const hint = msg.includes('403') || msg.includes('401')
      ? 'Admin key rejected — check ~/.korva/admin.key and re-login.'
      : msg.includes('500') || msg.includes('502') || msg.includes('503') || msg.includes('Failed to fetch')
        ? 'Vault server unreachable — run korva-vault (or korva vault start) and reload.'
        : `Unexpected error: ${msg}`
    return (
      <div className="m-6 bg-[#f8514912] border border-[#f8514930] rounded-xl p-5 space-y-1">
        <p className="text-[#f85149] text-sm font-medium">{t.dashboard.couldNotLoad}</p>
        <p className="text-[#8b949e] text-xs">{hint}</p>
      </div>
    )
  }

  if (!stats) return null

  const topProjects = Object.entries(stats.by_project)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 6)
  const topTypes = Object.entries(stats.by_type).sort(([, a], [, b]) => b - a)
  const maxProjectCount = Math.max(...topProjects.map(([, v]) => v), 1)
  // Estimated tokens: chars / 4 (standard approximation)
  const estTokens = Math.round((stats.total_content_len ?? 0) / 4)

  return (
    <div className="p-4 sm:p-6 space-y-6">
      <PageHeader
        icon={<LayoutDashboard size={17} />}
        iconColor="#388bfd"
        title={t.dashboard.title}
        description={t.dashboard.description}
        hint={{ command: 'korva status', label: t.dashboard.hintLabel }}
      />

      {/* KPI cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiCard icon={<Brain size={18} className="text-[#388bfd]" />}
          label={t.dashboard.observations} value={stats.total_observations} color="blue" />
        <KpiCard icon={<Clock size={18} className="text-[#3fb950]" />}
          label={t.dashboard.sessions} value={stats.total_sessions} color="green" />
        <KpiCard icon={<FolderGit2 size={18} className="text-[#f0883e]" />}
          label={t.dashboard.activeProjects} value={Object.keys(stats.by_project).length} color="orange" />
        <KpiCard
          icon={<Zap size={18} className="text-[#a371f7]" />}
          label={t.dashboard.contextTokens}
          value={estTokens}
          color="purple"
          hint={t.dashboard.contextTokensHint}
        />
      </div>

      {/* Activity timeline */}
      {stats.daily_activity && stats.daily_activity.length > 0 && (
        <ActivityChart data={stats.daily_activity} />
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Top projects */}
        <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <TrendingUp size={15} className="text-[#8b949e]" />
            <h3 className="text-sm font-medium text-[#e6edf3]">{t.dashboard.topProjects}</h3>
          </div>
          {topProjects.length === 0 ? (
            <p className="text-sm text-[#484f58]">{t.dashboard.noData}</p>
          ) : (
            <div className="space-y-3">
              {topProjects.map(([project, count]) => (
                <div key={project}>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs text-[#e6edf3] font-mono truncate max-w-[160px]">{project}</span>
                    <span className="text-xs text-[#8b949e]">{count}</span>
                  </div>
                  <div className="h-1.5 bg-[#21262d] rounded-full overflow-hidden">
                    <div className="h-full bg-[#388bfd] rounded-full transition-all"
                      style={{ width: `${(count / maxProjectCount) * 100}%` }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Observation types */}
        <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Database size={15} className="text-[#8b949e]" />
            <h3 className="text-sm font-medium text-[#e6edf3]">{t.dashboard.byType}</h3>
          </div>
          {topTypes.length === 0 ? (
            <p className="text-sm text-[#484f58]">{t.dashboard.noData}</p>
          ) : (
            <div className="space-y-2.5">
              {topTypes.map(([type, count]) => (
                <div key={type} className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className={`inline-block w-2 h-2 rounded-full ${typeDotColor(type)}`} />
                    <span className="text-xs text-[#e6edf3] capitalize">{type}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="w-20 h-1.5 bg-[#21262d] rounded-full overflow-hidden">
                      <div className={`h-full rounded-full ${typeDotColor(type)}`}
                        style={{ width: `${(count / stats.total_observations) * 100}%` }} />
                    </div>
                    <span className="text-xs text-[#8b949e] w-6 text-right tabular-nums">{count}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Recent sessions */}
      {stats.recent_sessions && stats.recent_sessions.length > 0 && (
        <RecentSessions sessions={stats.recent_sessions} />
      )}

      {/* Teams breakdown */}
      {Object.keys(stats.by_team).length > 0 && (
        <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Zap size={15} className="text-[#8b949e]" />
            <h3 className="text-sm font-medium text-[#e6edf3]">{t.dashboard.teams}</h3>
          </div>
          <div className="flex flex-wrap gap-2">
            {Object.entries(stats.by_team).map(([team, count]) => (
              <span key={team} className="px-3 py-1.5 bg-[#21262d] rounded-lg text-xs text-[#e6edf3]">
                {team} <span className="text-[#8b949e]">({count})</span>
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Activity Chart ────────────────────────────────────────────────────────────

function ActivityChart({ data }: { data: DailyCount[] }) {
  const { t } = useI18n()
  const max = Math.max(...data.map(d => d.count), 1)
  const total = data.reduce((s, d) => s + d.count, 0)

  return (
    <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <TrendingUp size={15} className="text-[#8b949e]" />
          <h3 className="text-sm font-medium text-[#e6edf3]">{t.dashboard.activityTitle}</h3>
        </div>
        <span className="text-xs text-[#8b949e]">{t.dashboard.activityTotal(total)}</span>
      </div>
      <div className="flex items-end gap-[3px] h-16">
        {data.map((d) => (
          <div key={d.date} className="flex-1 flex flex-col items-center gap-1 group relative"
            title={`${d.date}: ${d.count}`}>
            <div
              className="w-full bg-[#388bfd] rounded-sm opacity-80 hover:opacity-100 transition-opacity min-h-[2px]"
              style={{ height: `${Math.max((d.count / max) * 100, 4)}%` }}
            />
            <div className="absolute -top-7 left-1/2 -translate-x-1/2 bg-[#21262d] border border-[#30363d] rounded px-1.5 py-0.5 text-[10px] text-[#e6edf3] whitespace-nowrap opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none z-10">
              {d.date.slice(5)}: {d.count}
            </div>
          </div>
        ))}
      </div>
      <div className="flex justify-between mt-2 text-[10px] text-[#484f58]">
        <span>{data[0]?.date.slice(5)}</span>
        <span>{data[data.length - 1]?.date.slice(5)}</span>
      </div>
    </div>
  )
}

// ── Recent Sessions ───────────────────────────────────────────────────────────

function RecentSessions({ sessions }: { sessions: SessionRow[] }) {
  const { t } = useI18n()
  return (
    <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Clock size={15} className="text-[#8b949e]" />
          <h3 className="text-sm font-medium text-[#e6edf3]">{t.dashboard.recentSessions}</h3>
        </div>
        <NavLink to="sessions" relative="route"
          className="flex items-center gap-1 text-xs text-[#388bfd] hover:text-[#58a6ff] transition-colors">
          {t.dashboard.viewAll} <ArrowRight size={11} />
        </NavLink>
      </div>
      <div className="space-y-2">
        {sessions.map(s => (
          <div key={s.id} className="flex items-center gap-3 py-2 border-b border-[#21262d] last:border-0">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono text-[#388bfd] truncate max-w-[120px]">{s.project || '—'}</span>
                {s.agent && (
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#21262d] text-[#8b949e] shrink-0">{s.agent}</span>
                )}
              </div>
              {s.goal && (
                <p className="text-xs text-[#8b949e] truncate mt-0.5">{s.goal}</p>
              )}
            </div>
            <div className="flex items-center gap-3 shrink-0 text-right">
              <div className="text-xs text-[#484f58]">
                <span className="text-[#e6edf3] font-medium">{s.obs_count}</span> obs
              </div>
              {s.duration_min > 0 && (
                <div className="text-xs text-[#484f58]">
                  {s.duration_min < 60 ? `${s.duration_min}m` : `${Math.round(s.duration_min / 60)}h`}
                </div>
              )}
              <div className="text-[10px] text-[#484f58]">
                {formatRelative(s.started_at)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function KpiCard({ icon, label, value, color, hint }: {
  icon: React.ReactNode
  label: string
  value: number
  color: 'blue' | 'green' | 'purple' | 'orange'
  hint?: string
}) {
  const borders = {
    blue: 'border-[#388bfd30]', green: 'border-[#3fb95030]',
    purple: 'border-[#a371f730]', orange: 'border-[#f0883e30]',
  }
  return (
    <div className={`bg-[#161b22] border ${borders[color]} rounded-xl p-4`} title={hint}>
      <div className="flex items-center justify-between mb-3">{icon}</div>
      <div className="text-2xl font-bold text-[#e6edf3] tabular-nums">{value.toLocaleString()}</div>
      <div className="text-xs text-[#8b949e] mt-0.5">{label}</div>
    </div>
  )
}

function typeDotColor(type: string): string {
  const map: Record<string, string> = {
    decision: 'bg-[#388bfd]', pattern: 'bg-[#3fb950]',
    bugfix: 'bg-[#f85149]', learning: 'bg-[#a371f7]', context: 'bg-[#8b949e]',
  }
  return map[type] ?? 'bg-[#8b949e]'
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}
