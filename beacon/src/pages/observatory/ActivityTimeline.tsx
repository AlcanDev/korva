import { useState } from 'react'
import { useActivity, useInteraction, ActivityRow } from '@/api/observatory'
import { Search, Filter, X } from 'lucide-react'

export default function ActivityTimeline() {
  const [filters, setFilters] = useState({ project: '', model: '', agent: '', q: '' })
  const [selectedID, setSelectedID] = useState<string | null>(null)

  const { data, isLoading, error } = useActivity({
    project: filters.project || undefined,
    model: filters.model || undefined,
    agent: filters.agent || undefined,
    q: filters.q || undefined,
    limit: 100,
  })

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-[#e6edf3]">Activity Timeline</h1>
        <p className="text-xs text-[#8b949e] mt-1">
          Prompt round-trips reported by IDE clients · privacy filter applied
        </p>
      </header>

      <FilterBar filters={filters} onChange={setFilters} />

      {isLoading && <p className="text-xs text-[#8b949e]">Loading…</p>}
      {error && (
        <p className="text-xs text-[#f85149]">Error loading activity: {String(error)}</p>
      )}

      {data && (
        <>
          <p className="text-[10px] text-[#484f58]">
            {data.total !== undefined
              ? `${data.interactions.length} of ${data.total} interactions`
              : `${data.interactions.length} interactions (FTS query)`}
          </p>
          {data.interactions.length === 0 ? (
            <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-6 text-center">
              <p className="text-sm text-[#8b949e]">No interactions yet.</p>
              <p className="text-xs text-[#484f58] mt-1">
                IDE clients report tokens via{' '}
                <code className="font-mono">POST /api/v1/interactions</code>.
              </p>
            </div>
          ) : (
            <ActivityTable rows={data.interactions} onSelect={setSelectedID} />
          )}
        </>
      )}

      {selectedID && <DetailDrawer id={selectedID} onClose={() => setSelectedID(null)} />}
    </div>
  )
}

function FilterBar({
  filters,
  onChange,
}: {
  filters: { project: string; model: string; agent: string; q: string }
  onChange: (f: typeof filters) => void
}) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-3 flex flex-wrap items-center gap-2">
      <Filter size={14} className="text-[#8b949e]" />
      <SearchInput
        placeholder="Search prompt/response (FTS5)…"
        value={filters.q}
        onChange={(q) => onChange({ ...filters, q })}
        wide
      />
      <FilterInput placeholder="project" value={filters.project} onChange={(project) => onChange({ ...filters, project })} />
      <FilterInput placeholder="model" value={filters.model} onChange={(model) => onChange({ ...filters, model })} />
      <FilterInput placeholder="agent" value={filters.agent} onChange={(agent) => onChange({ ...filters, agent })} />
    </div>
  )
}

function FilterInput({
  placeholder,
  value,
  onChange,
}: {
  placeholder: string
  value: string
  onChange: (v: string) => void
}) {
  return (
    <input
      type="text"
      placeholder={placeholder}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd] w-28"
    />
  )
}

function SearchInput({
  placeholder,
  value,
  onChange,
  wide,
}: {
  placeholder: string
  value: string
  onChange: (v: string) => void
  wide?: boolean
}) {
  return (
    <div className="relative flex-1 min-w-[200px]">
      <Search size={11} className="absolute left-2 top-1/2 -translate-y-1/2 text-[#484f58]" />
      <input
        type="text"
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`bg-[#0d1117] border border-[#30363d] rounded pl-7 pr-2 py-1 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd] ${wide ? 'w-full' : ''}`}
      />
    </div>
  )
}

function ActivityTable({
  rows,
  onSelect,
}: {
  rows: ActivityRow[]
  onSelect: (id: string) => void
}) {
  return (
    <div className="rounded-lg border border-[#30363d] overflow-hidden">
      <table className="w-full text-xs">
        <thead className="bg-[#161b22] text-[10px] uppercase tracking-wider text-[#8b949e]">
          <tr>
            <th className="text-left py-2 px-3">Time</th>
            <th className="text-left py-2 px-3">Project</th>
            <th className="text-left py-2 px-3">Model</th>
            <th className="text-right py-2 px-3">Tokens</th>
            <th className="text-right py-2 px-3">Cache</th>
            <th className="text-right py-2 px-3">Duration</th>
            <th className="text-left py-2 px-3">Prompt</th>
            <th className="text-center py-2 px-3">Status</th>
          </tr>
        </thead>
        <tbody className="bg-[#0d1117]">
          {rows.map((row) => (
            <tr
              key={row.id}
              className="border-t border-[#21262d] hover:bg-[#161b22] cursor-pointer"
              onClick={() => onSelect(row.id)}
            >
              <td className="py-2 px-3 font-mono text-[#8b949e]">{fmtTime(row.ts)}</td>
              <td className="py-2 px-3 text-[#e6edf3] truncate max-w-[120px]">{row.project}</td>
              <td className="py-2 px-3 font-mono text-[#8b949e] truncate max-w-[120px]">{row.model}</td>
              <td className="py-2 px-3 text-right font-mono text-[#e6edf3]">
                {fmtNum(row.input_tokens + row.output_tokens)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-[#2ea043]">
                {fmtNum(row.cache_read)}
              </td>
              <td className="py-2 px-3 text-right font-mono text-[#8b949e]">
                {row.duration_ms}ms
              </td>
              <td className="py-2 px-3 text-[#c9d1d9] truncate max-w-[280px]">
                {row.prompt_excerpt}
              </td>
              <td className="py-2 px-3 text-center">
                {row.estimated && (
                  <span className="text-[9px] uppercase px-1.5 py-0.5 rounded bg-[#d2992220] text-[#d29922] mr-1">
                    est
                  </span>
                )}
                <span
                  className={`text-[9px] uppercase px-1.5 py-0.5 rounded ${
                    row.status === 'ok'
                      ? 'bg-[#2ea04320] text-[#2ea043]'
                      : 'bg-[#f8514920] text-[#f85149]'
                  }`}
                >
                  {row.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const { data, isLoading } = useInteraction(id)

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <aside className="relative w-full max-w-2xl h-full bg-[#161b22] border-l border-[#30363d] overflow-y-auto p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-[#e6edf3]">Interaction detail</h3>
          <button onClick={onClose} className="text-[#8b949e] hover:text-[#e6edf3]">
            <X size={16} />
          </button>
        </div>
        {isLoading && <p className="text-xs text-[#8b949e]">Loading…</p>}
        {data && (
          <div className="space-y-4 text-xs">
            <DetailField label="ID" value={data.id} mono />
            <DetailField label="Project" value={data.project} />
            <DetailField label="Model" value={data.model} mono />
            <DetailField label="Agent" value={data.agent} />
            <DetailField label="Tokens" value={`in ${data.input_tokens} · out ${data.output_tokens} · cache ${data.cache_read}`} mono />
            <DetailField label="Duration" value={`${data.duration_ms} ms`} mono />
            <div>
              <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">Prompt</div>
              <pre className="bg-[#0d1117] border border-[#30363d] rounded p-3 whitespace-pre-wrap text-[#c9d1d9] font-mono text-[11px]">
                {data.prompt_excerpt}
              </pre>
            </div>
            {data.response_excerpt && (
              <div>
                <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">Response</div>
                <pre className="bg-[#0d1117] border border-[#30363d] rounded p-3 whitespace-pre-wrap text-[#c9d1d9] font-mono text-[11px]">
                  {data.response_excerpt}
                </pre>
              </div>
            )}
            {data.tool_calls != null && (
              <div>
                <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">Tool calls</div>
                <pre className="bg-[#0d1117] border border-[#30363d] rounded p-3 whitespace-pre-wrap text-[#c9d1d9] font-mono text-[11px]">
                  {JSON.stringify(data.tool_calls, null, 2)}
                </pre>
              </div>
            )}
          </div>
        )}
      </aside>
    </div>
  )
}

function DetailField({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-0.5">{label}</div>
      <div className={`text-[#e6edf3] ${mono ? 'font-mono' : ''}`}>{value}</div>
    </div>
  )
}

function fmtTime(iso: string): string {
  try {
    return new Date(iso).toISOString().replace('T', ' ').slice(0, 19)
  } catch {
    return iso
  }
}

function fmtNum(n: number): string {
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return n.toLocaleString()
}
