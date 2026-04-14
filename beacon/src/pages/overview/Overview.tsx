import { useVaultStats } from '@/api/vault'
import { Database, Clock, Bookmark, AlertTriangle } from 'lucide-react'

export default function Overview() {
  const { data: stats, isLoading, isError } = useVaultStats()

  if (isLoading) return <PageSkeleton />
  if (isError) return <VaultOffline />

  const typeColors: Record<string, string> = {
    decision: '#238636',
    pattern: '#1f6feb',
    bugfix: '#da3633',
    learning: '#9e6a03',
    context: '#30363d',
    antipattern: '#a371f7',
  }

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-6">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Overview</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">Your team's knowledge at a glance</p>
      </header>

      {/* Stats cards */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <StatCard
          icon={<Database size={18} />}
          label="Observations"
          value={stats?.total_observations ?? 0}
          color="#238636"
        />
        <StatCard
          icon={<Clock size={18} />}
          label="Sessions"
          value={stats?.total_sessions ?? 0}
          color="#1f6feb"
        />
        <StatCard
          icon={<Bookmark size={18} />}
          label="Prompts"
          value={stats?.total_prompts ?? 0}
          color="#9e6a03"
        />
      </div>

      {/* By type */}
      {stats && Object.keys(stats.by_type).length > 0 && (
        <section className="mb-8">
          <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3">
            By Type
          </h2>
          <div className="grid grid-cols-3 gap-3">
            {Object.entries(stats.by_type).map(([type, count]) => (
              <div
                key={type}
                className="border border-[#21262d] rounded-lg p-3 flex items-center gap-3"
              >
                <div
                  className="w-2 h-2 rounded-full flex-shrink-0"
                  style={{ background: typeColors[type] ?? '#8b949e' }}
                />
                <span className="text-sm text-[#8b949e] capitalize flex-1">{type}</span>
                <span className="text-sm font-semibold text-[#e6edf3]">{count}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* By project */}
      {stats && Object.keys(stats.by_project).length > 0 && (
        <section className="mb-8">
          <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3">
            By Project
          </h2>
          <div className="border border-[#21262d] rounded-lg overflow-hidden">
            {Object.entries(stats.by_project)
              .sort(([, a], [, b]) => b - a)
              .map(([project, count], i) => (
                <div
                  key={project}
                  className={`flex items-center justify-between px-4 py-2.5 text-sm ${
                    i > 0 ? 'border-t border-[#21262d]' : ''
                  }`}
                >
                  <span className="text-[#e6edf3]">{project}</span>
                  <span className="text-[#8b949e] tabular-nums">{count}</span>
                </div>
              ))}
          </div>
        </section>
      )}

      {/* By team */}
      {stats && Object.keys(stats.by_team).length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3">
            By Team
          </h2>
          <div className="border border-[#21262d] rounded-lg overflow-hidden">
            {Object.entries(stats.by_team)
              .sort(([, a], [, b]) => b - a)
              .map(([team, count], i) => (
                <div
                  key={team}
                  className={`flex items-center justify-between px-4 py-2.5 text-sm ${
                    i > 0 ? 'border-t border-[#21262d]' : ''
                  }`}
                >
                  <span className="text-[#e6edf3]">{team}</span>
                  <span className="text-[#8b949e] tabular-nums">{count}</span>
                </div>
              ))}
          </div>
        </section>
      )}
    </div>
  )
}

interface StatCardProps {
  icon: React.ReactNode
  label: string
  value: number
  color: string
}

function StatCard({ icon, label, value, color }: StatCardProps) {
  return (
    <div className="border border-[#21262d] rounded-lg p-4 bg-[#161b22]">
      <div className="flex items-center gap-2 mb-3" style={{ color }}>
        {icon}
        <span className="text-xs font-medium text-[#8b949e] uppercase tracking-wider">{label}</span>
      </div>
      <div className="text-3xl font-bold text-[#e6edf3] tabular-nums">{value}</div>
    </div>
  )
}

function PageSkeleton() {
  return (
    <div className="p-6 max-w-5xl">
      <div className="h-7 w-32 bg-[#21262d] rounded mb-6 animate-pulse" />
      <div className="grid grid-cols-3 gap-4">
        {[1, 2, 3].map((i) => (
          <div key={i} className="border border-[#21262d] rounded-lg p-4 h-24 animate-pulse bg-[#161b22]" />
        ))}
      </div>
    </div>
  )
}

function VaultOffline() {
  return (
    <div className="p-6 flex items-start gap-3">
      <AlertTriangle size={18} className="text-[#9e6a03] mt-0.5 flex-shrink-0" />
      <div>
        <p className="text-sm font-medium text-[#e6edf3]">Vault is offline</p>
        <p className="text-sm text-[#8b949e] mt-1">
          Start the Vault server with: <code className="text-[#79c0ff]">korva-vault</code>
        </p>
      </div>
    </div>
  )
}
