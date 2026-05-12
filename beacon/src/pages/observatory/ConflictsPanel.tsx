import { useState } from 'react'
import {
  useConflicts,
  useConflict,
  useJudgeConflict,
  useIgnoreConflict,
  type ConflictRow,
  type JudgmentStatus,
} from '@/api/observatory'
import { Shield, X, Check, Trash2 } from 'lucide-react'

const STATUS_TABS: { key: JudgmentStatus; label: string }[] = [
  { key: 'pending', label: 'Pending' },
  { key: 'judged', label: 'Judged' },
  { key: 'ignored', label: 'Ignored' },
  { key: 'orphaned', label: 'Orphaned' },
]

const RELATIONS = [
  { value: 'supersedes', label: 'Supersedes', detail: 'source replaces target' },
  { value: 'conflicts_with', label: 'Conflicts with', detail: 'contradictory' },
  { value: 'related', label: 'Related', detail: 'topically linked' },
  { value: 'compatible', label: 'Compatible', detail: 'complementary' },
  { value: 'scoped', label: 'Scoped', detail: 'same topic, diff. context' },
]

export default function ConflictsPanel() {
  const [status, setStatus] = useState<JudgmentStatus>('pending')
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const { data, isLoading, error } = useConflicts(status)

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
          <Shield size={18} /> Conflicts
        </h1>
        <p className="text-xs text-[#8b949e] mt-1">
          Pending judgments from <code className="font-mono">vault_save</code> auto-scan.
          Resolve each pair as <em>supersedes / conflicts_with / related / compatible / scoped</em>, or dismiss.
        </p>
      </header>

      <nav className="flex border-b border-[#21262d]">
        {STATUS_TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setStatus(key)}
            className={`px-4 py-2 text-xs uppercase tracking-wider transition-colors ${
              status === key
                ? 'text-[#e6edf3] border-b-2 border-[#388bfd]'
                : 'text-[#8b949e] hover:text-[#e6edf3]'
            }`}
          >
            {label}
          </button>
        ))}
      </nav>

      {isLoading && <p className="text-xs text-[#8b949e]">Loading…</p>}
      {error && <p className="text-xs text-[#f85149]">Error: {String(error)}</p>}

      {data && (
        <>
          <p className="text-[10px] text-[#484f58]">
            {data.count} row(s) · status={data.status}
            {data.project ? ` · project=${data.project}` : ''}
          </p>
          {data.conflicts.length === 0 ? (
            <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-6 text-center">
              <p className="text-sm text-[#8b949e]">No conflicts in this state.</p>
              <p className="text-xs text-[#484f58] mt-1">
                Vault flags candidate overlaps automatically when an agent calls{' '}
                <code className="font-mono">vault_save</code>.
              </p>
            </div>
          ) : (
            <ConflictsTable rows={data.conflicts} onSelect={setSelectedID} />
          )}
        </>
      )}

      {selectedID && (
        <ConflictDrawer
          id={selectedID}
          onClose={() => setSelectedID(null)}
          allowAction={status === 'pending'}
        />
      )}
    </div>
  )
}

function ConflictsTable({
  rows,
  onSelect,
}: {
  rows: ConflictRow[]
  onSelect: (id: string) => void
}) {
  return (
    <div className="rounded-lg border border-[#30363d] overflow-hidden">
      <table className="w-full text-xs">
        <thead className="bg-[#161b22] text-[10px] uppercase tracking-wider text-[#8b949e]">
          <tr>
            <th className="text-left py-2 px-3">Time</th>
            <th className="text-left py-2 px-3">Project</th>
            <th className="text-left py-2 px-3">Source ↔ Target</th>
            <th className="text-left py-2 px-3">Status</th>
            <th className="text-left py-2 px-3">Relation</th>
            <th className="text-right py-2 px-3">Conf.</th>
            <th className="text-left py-2 px-3">Reason</th>
          </tr>
        </thead>
        <tbody className="bg-[#0d1117]">
          {rows.map((r) => (
            <tr
              key={r.id}
              onClick={() => onSelect(r.id)}
              className="border-t border-[#21262d] hover:bg-[#161b22] cursor-pointer"
            >
              <td className="py-2 px-3 font-mono text-[#8b949e]">{fmtTime(r.created_at)}</td>
              <td className="py-2 px-3 text-[#e6edf3]">{r.project}</td>
              <td className="py-2 px-3 font-mono text-[#8b949e]">
                {short(r.source_id)} ↔ {short(r.target_id)}
              </td>
              <td className="py-2 px-3">
                <StatusBadge status={r.judgment_status} />
              </td>
              <td className="py-2 px-3 text-[#e6edf3]">{r.relation || '—'}</td>
              <td className="py-2 px-3 text-right font-mono text-[#8b949e]">
                {r.confidence.toFixed(2)}
              </td>
              <td className="py-2 px-3 text-[#c9d1d9] truncate max-w-[280px]">
                {r.reason || ''}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function StatusBadge({ status }: { status: JudgmentStatus }) {
  const cls: Record<JudgmentStatus, string> = {
    pending: 'bg-[#d2992220] text-[#d29922] border-[#d2992240]',
    judged: 'bg-[#2ea04320] text-[#2ea043] border-[#2ea04340]',
    ignored: 'bg-[#21262d] text-[#484f58] border-[#30363d]',
    orphaned: 'bg-[#f8514920] text-[#f85149] border-[#f8514940]',
  }
  return (
    <span className={`text-[9px] uppercase px-1.5 py-0.5 rounded border ${cls[status]}`}>
      {status}
    </span>
  )
}

function ConflictDrawer({
  id,
  onClose,
  allowAction,
}: {
  id: string
  onClose: () => void
  allowAction: boolean
}) {
  const { data, isLoading } = useConflict(id)
  const judge = useJudgeConflict()
  const ignore = useIgnoreConflict()

  const [relation, setRelation] = useState('related')
  const [reason, setReason] = useState('')
  const [confidence, setConfidence] = useState(0.8)

  function submitJudge() {
    judge.mutate(
      { id, body: { relation, reason, confidence, marked_by_actor: 'user', marked_by_kind: 'manual' } },
      { onSuccess: () => onClose() },
    )
  }

  function submitIgnore() {
    ignore.mutate(
      { id, body: { reason: reason || 'not actually a conflict' } },
      { onSuccess: () => onClose() },
    )
  }

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <aside className="relative w-full max-w-3xl h-full bg-[#161b22] border-l border-[#30363d] overflow-y-auto p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold text-[#e6edf3]">Conflict detail</h3>
          <button onClick={onClose} className="text-[#8b949e] hover:text-[#e6edf3]" aria-label="Close">
            <X size={16} />
          </button>
        </div>
        {isLoading && <p className="text-xs text-[#8b949e]">Loading…</p>}
        {data && (
          <div className="space-y-5 text-xs">
            <div className="grid grid-cols-2 gap-4">
              <SmallField label="Project" value={data.conflict.project} />
              <SmallField label="Created" value={fmtTime(data.conflict.created_at)} mono />
              <SmallField label="Status" value={data.conflict.judgment_status} />
              <SmallField label="Confidence" value={data.conflict.confidence.toFixed(2)} mono />
              {data.conflict.relation && (
                <SmallField label="Relation" value={data.conflict.relation} />
              )}
              {data.conflict.judged_at && (
                <SmallField label="Judged at" value={fmtTime(data.conflict.judged_at)} mono />
              )}
            </div>

            {data.conflict.reason && (
              <div>
                <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">Reason</div>
                <p className="text-[#c9d1d9]">{data.conflict.reason}</p>
              </div>
            )}
            {data.conflict.evidence && (
              <div>
                <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">Evidence</div>
                <pre className="bg-[#0d1117] border border-[#30363d] rounded p-3 whitespace-pre-wrap text-[#c9d1d9] font-mono text-[11px]">
                  {data.conflict.evidence}
                </pre>
              </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <ObservationCard title="Source" obs={data.source} />
              <ObservationCard title="Target" obs={data.target} />
            </div>

            {allowAction && (
              <div className="rounded-lg border border-[#30363d] bg-[#0d1117] p-4 space-y-3">
                <h4 className="text-xs font-semibold text-[#e6edf3]">Record verdict</h4>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <label className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
                      Relation
                    </label>
                    <select
                      value={relation}
                      onChange={(e) => setRelation(e.target.value)}
                      className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3]"
                    >
                      {RELATIONS.map((r) => (
                        <option key={r.value} value={r.value}>
                          {r.label} — {r.detail}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
                      Confidence ({confidence.toFixed(2)})
                    </label>
                    <input
                      type="range"
                      min={0}
                      max={1}
                      step={0.05}
                      value={confidence}
                      onChange={(e) => setConfidence(Number(e.target.value))}
                      className="w-full accent-[#388bfd]"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
                    Reason
                  </label>
                  <input
                    type="text"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    placeholder="Short rationale (audit-visible)"
                    className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3]"
                  />
                </div>
                <div className="flex items-center gap-2 pt-2">
                  <button
                    onClick={submitJudge}
                    disabled={judge.isPending}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40"
                  >
                    <Check size={12} /> {judge.isPending ? 'Saving…' : 'Save verdict'}
                  </button>
                  <button
                    onClick={submitIgnore}
                    disabled={ignore.isPending}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d] disabled:opacity-40"
                  >
                    <Trash2 size={12} /> {ignore.isPending ? 'Ignoring…' : 'Ignore'}
                  </button>
                </div>
                {(judge.error || ignore.error) && (
                  <p className="text-xs text-[#f85149]">
                    {String(judge.error ?? ignore.error)}
                  </p>
                )}
              </div>
            )}
          </div>
        )}
      </aside>
    </div>
  )
}

function ObservationCard({ title, obs }: { title: string; obs: { id: string; type: string; title: string; content: string } | null }) {
  if (!obs) {
    return (
      <div className="rounded border border-[#30363d] bg-[#0d1117] p-3 text-xs text-[#484f58]">
        {title}: <span className="text-[#f85149]">(missing)</span>
      </div>
    )
  }
  return (
    <div className="rounded border border-[#30363d] bg-[#0d1117] p-3">
      <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">{title}</div>
      <div className="text-[#e6edf3] text-xs mb-1">
        <code className="font-mono text-[10px] text-[#8b949e] mr-1.5">[{obs.type}]</code>
        {obs.title}
      </div>
      <pre className="text-[11px] font-mono text-[#c9d1d9] whitespace-pre-wrap break-words">
        {obs.content}
      </pre>
    </div>
  )
}

function SmallField({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-0.5">{label}</div>
      <div className={`text-[#e6edf3] ${mono ? 'font-mono' : ''}`}>{value}</div>
    </div>
  )
}

function short(id: string): string {
  if (id.length <= 8) return id
  return id.slice(0, 8) + '…'
}

function fmtTime(iso: string): string {
  if (!iso) return ''
  try {
    return new Date(iso).toISOString().replace('T', ' ').slice(0, 19)
  } catch {
    return iso
  }
}
