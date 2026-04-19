import _React from 'react'
import { ClipboardList, RefreshCw } from 'lucide-react'
import { useAuditLogs } from '@/api/audit'

export default function AdminAudit() {
  const { data, isLoading, error, refetch } = useAuditLogs()

  return (
    <div className="p-4 sm:p-6 max-w-4xl">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[#e6edf3] text-lg font-semibold">Audit Log</h1>
        <button
          onClick={() => refetch()}
          className="text-[#8b949e] hover:text-[#e6edf3] transition-colors"
        >
          <RefreshCw size={14} />
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-[#f85149] bg-[#f8514910] p-4 mb-4">
          <p className="text-[#f85149] text-xs">
            {String(error).includes('402')
              ? 'Audit log requires Korva for Teams license with audit_log feature.'
              : 'Failed to load audit log.'}
          </p>
        </div>
      )}

      <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
          <ClipboardList size={14} className="text-[#8b949e]" />
          <span className="text-[#e6edf3] text-sm font-medium">
            Recent actions {data && <span className="text-[#484f58] text-xs">({data.count})</span>}
          </span>
        </div>

        {isLoading && <p className="px-4 py-4 text-xs text-[#8b949e]">Loading…</p>}

        <div className="divide-y divide-[#21262d]">
          {data?.logs.map(entry => (
            <div key={entry.id} className="px-4 py-3 flex flex-col sm:grid sm:grid-cols-12 gap-1 sm:gap-2 items-start">
              <div className="sm:col-span-3 flex items-center gap-2">
                <span className="inline-block px-1.5 py-0.5 rounded text-[10px] font-mono bg-[#21262d] text-[#8b949e]">
                  {entry.action}
                </span>
                {/* Date shown inline on mobile, hidden on sm+ */}
                <span className="text-[10px] text-[#484f58] sm:hidden">
                  {new Date(entry.created_at).toLocaleDateString()}
                </span>
              </div>
              <div className="sm:col-span-5">
                <p className="text-[#e6edf3] text-xs truncate">{entry.target}</p>
                <p className="text-[#484f58] text-[10px]">{entry.actor}</p>
              </div>
              <div className="hidden sm:block sm:col-span-4 text-right">
                <p className="text-[#484f58] text-[10px]">
                  {new Date(entry.created_at).toLocaleString()}
                </p>
                {entry.before_hash && (
                  <p className="text-[10px] font-mono text-[#484f58]">
                    {entry.before_hash} → {entry.after_hash || '—'}
                  </p>
                )}
              </div>
            </div>
          ))}
          {data?.logs.length === 0 && !isLoading && !error && (
            <p className="px-4 py-4 text-xs text-[#484f58]">No audit entries yet.</p>
          )}
        </div>
      </div>
    </div>
  )
}
