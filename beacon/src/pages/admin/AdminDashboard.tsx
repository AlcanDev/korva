import { Brain, Clock, FileText, Users, TrendingUp, Database, Zap, LayoutDashboard } from 'lucide-react'
import { useAdminStats } from '@/api/admin'
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

  const topTypes = Object.entries(stats.by_type)
    .sort(([, a], [, b]) => b - a)

  const maxProjectCount = Math.max(...topProjects.map(([, v]) => v), 1)

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
        <KpiCard
          icon={<Brain size={18} className="text-[#388bfd]" />}
          label={t.dashboard.observations}
          value={stats.total_observations}
          color="blue"
        />
        <KpiCard
          icon={<Clock size={18} className="text-[#3fb950]" />}
          label={t.dashboard.sessions}
          value={stats.total_sessions}
          color="green"
        />
        <KpiCard
          icon={<FileText size={18} className="text-[#a371f7]" />}
          label={t.dashboard.savedPrompts}
          value={stats.total_prompts}
          color="purple"
        />
        <KpiCard
          icon={<Users size={18} className="text-[#f0883e]" />}
          label={t.dashboard.activeProjects}
          value={Object.keys(stats.by_project).length}
          color="orange"
        />
      </div>

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
                    <div
                      className="h-full bg-[#388bfd] rounded-full transition-all"
                      style={{ width: `${(count / maxProjectCount) * 100}%` }}
                    />
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
            <div className="space-y-2">
              {topTypes.map(([type, count]) => (
                <div key={type} className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className={`inline-block w-2 h-2 rounded-full ${typeColor(type)}`} />
                    <span className="text-xs text-[#e6edf3] capitalize">{type}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <div className="w-16 h-1 bg-[#21262d] rounded-full overflow-hidden">
                      <div
                        className={`h-full rounded-full ${typeColor(type)}`}
                        style={{ width: `${(count / stats.total_observations) * 100}%` }}
                      />
                    </div>
                    <span className="text-xs text-[#8b949e] w-6 text-right">{count}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

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

function KpiCard({ icon, label, value, color }: {
  icon: React.ReactNode
  label: string
  value: number
  color: 'blue' | 'green' | 'purple' | 'orange'
}) {
  const borders = {
    blue: 'border-[#388bfd30]',
    green: 'border-[#3fb95030]',
    purple: 'border-[#a371f730]',
    orange: 'border-[#f0883e30]',
  }

  return (
    <div className={`bg-[#161b22] border ${borders[color]} rounded-xl p-4`}>
      <div className="flex items-center justify-between mb-3">
        {icon}
      </div>
      <div className="text-2xl font-bold text-[#e6edf3]">{value.toLocaleString()}</div>
      <div className="text-xs text-[#8b949e] mt-0.5">{label}</div>
    </div>
  )
}

function typeColor(type: string): string {
  const map: Record<string, string> = {
    decision: 'bg-[#388bfd]',
    pattern: 'bg-[#3fb950]',
    bugfix: 'bg-[#f85149]',
    learning: 'bg-[#a371f7]',
    context: 'bg-[#8b949e]',
  }
  return map[type] ?? 'bg-[#8b949e]'
}
