import _React, { useState } from 'react'
import { Activity, RefreshCw, AlertCircle } from 'lucide-react'
import { useInteractions, useInteractionStats } from '@/api/interactions'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminInteractions() {
  const { t } = useI18n()
  const [statusFilter, setStatusFilter] = useState<'' | 'error'>('')
  const { data, isLoading, error, refetch } = useInteractions({ status: statusFilter || undefined, limit: 200 })
  const { data: stats } = useInteractionStats()

  const successRate = stats
    ? stats.Total > 0 ? Math.round(((stats.Total - stats.ErrorCount) / stats.Total) * 100) : 100
    : null

  return (
    <div className="p-4 sm:p-6 max-w-5xl">
      <PageHeader
        icon={<Activity size={17} />}
        iconColor="#58a6ff"
        title={t.interactions.title}
        description={t.interactions.description}
        actions={
          <button
            onClick={() => refetch()}
            className="text-[#8b949e] hover:text-[#e6edf3] transition-colors p-1 rounded hover:bg-[#21262d]"
            title={t.common.refresh}
          >
            <RefreshCw size={14} />
          </button>
        }
      />

      {/* Stats cards */}
      {stats && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-5">
          <StatCard label={t.interactions.totalCalls} value={stats.Total.toLocaleString()} color="#58a6ff" />
          <StatCard label={t.interactions.errorCount} value={stats.ErrorCount.toLocaleString()} color={stats.ErrorCount > 0 ? '#f85149' : '#3fb950'} />
          <StatCard label={t.interactions.avgLatency} value={`${Math.round(stats.AvgLatency)}ms`} color="#d29922" />
          <StatCard label="Success rate" value={successRate !== null ? `${successRate}%` : '—'} color={successRate !== null && successRate < 90 ? '#f85149' : '#3fb950'} />
        </div>
      )}

      {/* Top tools */}
      {stats && Object.keys(stats.ByTool).length > 0 && (
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-4 mb-5">
          <p className="text-[#8b949e] text-xs font-medium mb-3 uppercase tracking-wider">{t.interactions.topTools}</p>
          <div className="flex flex-wrap gap-2">
            {Object.entries(stats.ByTool)
              .sort(([, a], [, b]) => b - a)
              .slice(0, 12)
              .map(([tool, count]) => (
                <button
                  key={tool}
                  onClick={() => setStatusFilter('')}
                  className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-[#21262d] hover:bg-[#2d333b] border border-[#30363d] transition-colors"
                >
                  <span className="text-[#e6edf3] text-xs font-mono">{tool.replace('vault_', '')}</span>
                  <span className="text-[#484f58] text-[10px]">{count}</span>
                </button>
              ))}
          </div>
        </div>
      )}

      {/* Filter + table */}
      <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
        <div className="flex items-center justify-between gap-2 px-4 py-3 border-b border-[#21262d]">
          <div className="flex items-center gap-2">
            <Activity size={14} className="text-[#8b949e]" />
            <span className="text-[#e6edf3] text-sm font-medium">
              {t.interactions.callsTitle}{' '}
              {data && <span className="text-[#484f58] text-xs">({data.count})</span>}
            </span>
          </div>
          <div className="flex gap-1">
            <FilterBtn active={statusFilter === ''} onClick={() => setStatusFilter('')}>
              {t.interactions.filterAll}
            </FilterBtn>
            <FilterBtn active={statusFilter === 'error'} onClick={() => setStatusFilter('error')}>
              {t.interactions.filterErrors}
            </FilterBtn>
          </div>
        </div>

        {error && (
          <div className="px-4 py-3 flex items-center gap-2 text-[#f85149] text-xs">
            <AlertCircle size={13} />
            {t.interactions.errorLoad}
          </div>
        )}
        {isLoading && <p className="px-4 py-4 text-xs text-[#8b949e]">{t.common.loading}</p>}

        {/* Table header */}
        {!isLoading && !error && (
          <div className="hidden sm:grid grid-cols-12 gap-2 px-4 py-2 border-b border-[#21262d] bg-[#0d1117]">
            <HeaderCell className="col-span-3">{t.interactions.colTool}</HeaderCell>
            <HeaderCell className="col-span-2">{t.interactions.colAuthor}</HeaderCell>
            <HeaderCell className="col-span-2">{t.interactions.colProject}</HeaderCell>
            <HeaderCell className="col-span-2">{t.interactions.colStatus}</HeaderCell>
            <HeaderCell className="col-span-1">{t.interactions.colLatency}</HeaderCell>
            <HeaderCell className="col-span-2 text-right">{t.interactions.colTime}</HeaderCell>
          </div>
        )}

        <div className="divide-y divide-[#21262d]">
          {data?.calls.map(call => (
            <CallRow key={call.id} call={call} />
          ))}
          {data?.calls.length === 0 && !isLoading && !error && (
            <p className="px-4 py-4 text-xs text-[#484f58]">{t.interactions.noEntries}</p>
          )}
        </div>
      </div>
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] px-4 py-3">
      <p className="text-[#484f58] text-[10px] uppercase tracking-wider mb-1">{label}</p>
      <p className="text-xl font-mono font-semibold" style={{ color }}>{value}</p>
    </div>
  )
}

function FilterBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-2 py-0.5 rounded text-[11px] transition-colors ${
        active
          ? 'bg-[#21262d] text-[#e6edf3] border border-[#484f58]'
          : 'text-[#8b949e] hover:text-[#e6edf3] border border-transparent'
      }`}
    >
      {children}
    </button>
  )
}

function HeaderCell({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <span className={`text-[10px] text-[#484f58] uppercase tracking-wider ${className}`}>{children}</span>
  )
}

function CallRow({ call }: { call: import('@/api/interactions').CallEntry }) {
  const isError = call.status === 'error'
  return (
    <div className="px-4 py-2.5 flex flex-col sm:grid sm:grid-cols-12 gap-1 sm:gap-2 items-start sm:items-center hover:bg-[#161b22]/80">
      <div className="sm:col-span-3">
        <span className="text-[#e6edf3] text-xs font-mono">{call.tool}</span>
      </div>
      <div className="sm:col-span-2">
        <span className="text-[#8b949e] text-xs truncate block">{call.author || '—'}</span>
      </div>
      <div className="sm:col-span-2">
        <span className="text-[#8b949e] text-xs truncate block">{call.project || '—'}</span>
      </div>
      <div className="sm:col-span-2">
        <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono ${
          isError
            ? 'bg-[#f8514910] text-[#f85149] border border-[#f8514930]'
            : 'bg-[#3fb95010] text-[#3fb950] border border-[#3fb95030]'
        }`}>
          {isError && <AlertCircle size={9} />}
          {call.status}
        </span>
        {isError && call.error_msg && (
          <p className="text-[#f85149] text-[10px] mt-0.5 truncate max-w-[120px]" title={call.error_msg}>
            {call.error_msg}
          </p>
        )}
      </div>
      <div className="sm:col-span-1">
        <span className="text-[#8b949e] text-xs font-mono">{call.latency_ms}ms</span>
      </div>
      <div className="sm:col-span-2 sm:text-right">
        <span className="text-[#484f58] text-[10px]">
          {new Date(call.created_at).toLocaleString()}
        </span>
      </div>
    </div>
  )
}
