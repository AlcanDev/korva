import { useTokenStats } from '@/api/observatory'
import { Coins, TrendingDown, Database, Zap } from 'lucide-react'

export default function TokenAnalytics() {
  const { data, isLoading, error } = useTokenStats()

  if (isLoading) return <div className="p-6 text-[#8b949e] text-sm">Loading token analytics…</div>
  if (error || !data) {
    return <div className="p-6 text-[#f85149] text-sm">Could not load token stats: {String(error)}</div>
  }

  const reductionPct = (data.reduction_pct_estimated * 100).toFixed(1)
  const cacheHitPct = (data.cache_hit_pct * 100).toFixed(1)
  const totalTokens = data.totals.input_tokens + data.totals.output_tokens

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      <header>
        <h1 className="text-xl font-semibold text-[#e6edf3]">Token Analytics</h1>
        <p className="text-xs text-[#8b949e] mt-1">
          Real tokens reported by IDE clients ({data.totals.interactions_count - data.totals.estimated_count})
          + estimated fallback ({data.totals.estimated_count}) · baseline scanned in
          <code className="ml-1 font-mono text-[10px] bg-[#21262d] px-1 py-0.5 rounded">
            {data.baseline_dir}
          </code>
        </p>
      </header>

      <section className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <KPICard
          icon={<Coins size={14} />}
          label="Total tokens"
          value={fmtNumber(totalTokens)}
          subtitle={`${fmtNumber(data.totals.input_tokens)} in / ${fmtNumber(data.totals.output_tokens)} out`}
        />
        <KPICard
          icon={<Database size={14} />}
          label="Cache hit"
          value={`${cacheHitPct}%`}
          subtitle={`${fmtNumber(data.totals.cache_read)} cached tokens`}
          accent="#2ea043"
        />
        <KPICard
          icon={<TrendingDown size={14} />}
          label="Token reduction"
          value={`${reductionPct}%`}
          subtitle={`vs baseline ${fmtNumber(data.baseline_naive_tokens)}`}
          accent="#388bfd"
        />
        <KPICard
          icon={<Zap size={14} />}
          label="Interactions"
          value={fmtNumber(data.totals.interactions_count)}
          subtitle={
            data.totals.estimated_count > 0
              ? `${data.totals.estimated_count} estimated`
              : 'all real'
          }
        />
      </section>

      <DailyTrend daily={data.daily} />

      <section className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <BucketTable title="By model" buckets={data.by_model} />
        <BucketTable title="By project" buckets={data.by_project} />
      </section>
    </div>
  )
}

function KPICard({
  icon,
  label,
  value,
  subtitle,
  accent,
}: {
  icon: React.ReactNode
  label: string
  value: string
  subtitle?: string
  accent?: string
}) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <div className="flex items-center gap-1.5 text-[#8b949e] mb-2">
        {icon}
        <span className="text-[10px] uppercase tracking-wider">{label}</span>
      </div>
      <div className="text-2xl font-semibold" style={{ color: accent ?? '#e6edf3' }}>
        {value}
      </div>
      {subtitle && <div className="text-[10px] text-[#8b949e] mt-1">{subtitle}</div>}
    </div>
  )
}

function DailyTrend({
  daily,
}: {
  daily: Array<{ date: string; input_tokens: number; output_tokens: number; cache_read: number }>
}) {
  if (!daily || daily.length === 0) {
    return (
      <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
        <h3 className="text-sm font-semibold text-[#e6edf3] mb-2">Daily trend</h3>
        <p className="text-xs text-[#8b949e]">No data yet — start sending interactions to see daily totals.</p>
      </div>
    )
  }
  const max = Math.max(
    1,
    ...daily.map((d) => d.input_tokens + d.output_tokens),
  )
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <h3 className="text-sm font-semibold text-[#e6edf3] mb-3">Daily trend (input + output)</h3>
      <div className="flex items-end gap-1 h-32">
        {daily.map((d) => {
          const total = d.input_tokens + d.output_tokens
          const heightPct = (total / max) * 100
          return (
            <div
              key={d.date}
              className="flex-1 bg-[#388bfd] hover:bg-[#58a6ff] transition-colors rounded-sm relative group"
              style={{ height: `${Math.max(heightPct, 2)}%` }}
              title={`${d.date}: ${total.toLocaleString()} tokens`}
            />
          )
        })}
      </div>
      <div className="flex justify-between text-[9px] text-[#484f58] mt-1 font-mono">
        <span>{daily[0]?.date}</span>
        <span>{daily[daily.length - 1]?.date}</span>
      </div>
    </div>
  )
}

function BucketTable({
  title,
  buckets,
}: {
  title: string
  buckets: Record<string, { input_tokens: number; output_tokens: number; cache_read: number; count: number }>
}) {
  const rows = Object.entries(buckets).sort(
    ([, a], [, b]) => b.input_tokens + b.output_tokens - (a.input_tokens + a.output_tokens),
  )
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <h3 className="text-sm font-semibold text-[#e6edf3] mb-3">{title}</h3>
      {rows.length === 0 ? (
        <p className="text-xs text-[#8b949e]">No data.</p>
      ) : (
        <table className="w-full text-xs">
          <thead>
            <tr className="text-[10px] uppercase tracking-wider text-[#8b949e]">
              <th className="text-left py-1">Name</th>
              <th className="text-right py-1">Input</th>
              <th className="text-right py-1">Output</th>
              <th className="text-right py-1">Cached</th>
              <th className="text-right py-1">Count</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(([name, b]) => (
              <tr key={name} className="border-t border-[#21262d]">
                <td className="py-1 text-[#e6edf3] truncate max-w-[180px]">{name}</td>
                <td className="py-1 text-right font-mono text-[#8b949e]">{fmtNumber(b.input_tokens)}</td>
                <td className="py-1 text-right font-mono text-[#8b949e]">{fmtNumber(b.output_tokens)}</td>
                <td className="py-1 text-right font-mono text-[#2ea043]">{fmtNumber(b.cache_read)}</td>
                <td className="py-1 text-right font-mono text-[#8b949e]">{b.count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function fmtNumber(n: number): string {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return n.toLocaleString()
}
