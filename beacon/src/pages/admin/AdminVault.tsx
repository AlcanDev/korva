import { useState } from 'react'
import { Search, Trash2, Tag, Calendar, User, AlertCircle, ChevronLeft, ChevronRight } from 'lucide-react'
import { useAdminSearch, useAdminDeleteObservation, type Observation } from '@/api/admin'

const TYPE_OPTIONS = ['', 'decision', 'pattern', 'bugfix', 'learning', 'context', 'antipattern', 'task']
const PAGE_SIZE = 20

export default function AdminVault() {
  const [query, setQuery] = useState('')
  const [project, setProject] = useState('')
  const [type, setType] = useState('')
  const [offset, setOffset] = useState(0)
  const [selected, setSelected] = useState<Observation | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const { data, isLoading } = useAdminSearch(query, project, type, PAGE_SIZE, offset)
  const deleteMutation = useAdminDeleteObservation()

  const results = data?.results ?? []
  const total = data?.total ?? data?.count ?? 0
  const hasPrev = offset > 0
  const hasNext = data != null && offset + PAGE_SIZE < total

  function handleDelete(id: string) {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        setConfirmDelete(null)
        if (selected?.id === id) setSelected(null)
      },
    })
  }

  // Reset to first page whenever filters change
  function setQueryAndReset(v: string) { setQuery(v); setOffset(0) }
  function setProjectAndReset(v: string) { setProject(v); setOffset(0) }
  function setTypeAndReset(v: string) { setType(v); setOffset(0) }

  function handleDelete(id: string) {
    deleteMutation.mutate(id, {
      onSuccess: () => {
        setConfirmDelete(null)
        if (selected?.id === id) setSelected(null)
      },
    })
  }

  return (
    <div className="p-4 sm:p-6 space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-[#e6edf3]">Vault Browser</h2>
        <p className="text-sm text-[#8b949e] mt-0.5">
          {total > 0
            ? `${total} observation${total !== 1 ? 's' : ''} · page ${Math.floor(offset / PAGE_SIZE) + 1} of ${Math.ceil(total / PAGE_SIZE)}`
            : 'All observations'}
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-3">
        <div className="relative flex-1 min-w-[200px]">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#484f58]" />
          <input
            type="text"
            placeholder="Search observations..."
            value={query}
            onChange={e => setQueryAndReset(e.target.value)}
            className="w-full bg-[#161b22] border border-[#30363d] rounded-lg pl-9 pr-3 py-2 text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
          />
        </div>
        <input
          type="text"
          placeholder="Project filter..."
          value={project}
          onChange={e => setProjectAndReset(e.target.value)}
          className="bg-[#161b22] border border-[#30363d] rounded-lg px-3 py-2 text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd] w-full sm:w-40"
        />
        <select
          value={type}
          onChange={e => setTypeAndReset(e.target.value)}
          className="bg-[#161b22] border border-[#30363d] rounded-lg px-3 py-2 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
        >
          {TYPE_OPTIONS.map(t => (
            <option key={t} value={t}>{t || 'All types'}</option>
          ))}
        </select>
      </div>

      <div className="flex flex-col lg:flex-row gap-4">
        {/* List */}
        <div className="flex-1 space-y-2 min-w-0">
          {isLoading && (
            <div className="flex items-center justify-center h-32">
              <div className="animate-spin rounded-full h-6 w-6 border-t-2 border-[#388bfd]" />
            </div>
          )}
          {!isLoading && results.length === 0 && (
            <div className="text-center text-sm text-[#484f58] py-12">
              No observations found
            </div>
          )}
          {results.map(obs => (
            <div
              key={obs.id}
              onClick={() => setSelected(obs)}
              className={`bg-[#161b22] border rounded-lg p-4 cursor-pointer transition-colors group ${
                selected?.id === obs.id
                  ? 'border-[#388bfd]'
                  : 'border-[#21262d] hover:border-[#30363d]'
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <TypeBadge type={obs.type} />
                    <span className="text-xs text-[#8b949e] font-mono">{obs.project}</span>
                  </div>
                  <p className="text-sm text-[#e6edf3] truncate">{obs.title}</p>
                  <p className="text-xs text-[#8b949e] mt-0.5 truncate">{obs.content.slice(0, 100)}…</p>
                </div>
                <button
                  onClick={e => { e.stopPropagation(); setConfirmDelete(obs.id) }}
                  className="opacity-0 group-hover:opacity-100 text-[#484f58] hover:text-[#f85149] flex-shrink-0 p-1 rounded transition-all"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </div>
          ))}

          {/* Pagination controls */}
          {(hasPrev || hasNext) && (
            <div className="flex items-center justify-between pt-2">
              <button
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                disabled={!hasPrev}
                className="flex items-center gap-1 text-xs text-[#8b949e] hover:text-[#e6edf3] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              >
                <ChevronLeft size={13} /> Prev
              </button>
              <span className="text-xs text-[#484f58]">
                {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} of {total}
              </span>
              <button
                onClick={() => setOffset(offset + PAGE_SIZE)}
                disabled={!hasNext}
                className="flex items-center gap-1 text-xs text-[#8b949e] hover:text-[#e6edf3] disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
              >
                Next <ChevronRight size={13} />
              </button>
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selected && (
          <div className="w-full lg:w-80 lg:flex-shrink-0 bg-[#161b22] border border-[#21262d] rounded-xl p-5 space-y-4 self-start lg:sticky lg:top-0">
            <div>
              <TypeBadge type={selected.type} />
              <h3 className="text-sm font-semibold text-[#e6edf3] mt-2">{selected.title}</h3>
            </div>
            <p className="text-xs text-[#8b949e] leading-relaxed whitespace-pre-wrap">
              {selected.content}
            </p>
            <div className="space-y-2 text-xs text-[#8b949e]">
              <div className="flex items-center gap-2">
                <User size={11} />
                <span className="font-mono">{selected.project}</span>
              </div>
              <div className="flex items-center gap-2">
                <Calendar size={11} />
                <span>{new Date(selected.created_at).toLocaleString()}</span>
              </div>
              {selected.tags?.length > 0 && (
                <div className="flex items-center gap-2 flex-wrap">
                  <Tag size={11} />
                  {selected.tags.map(t => (
                    <span key={t} className="bg-[#21262d] px-2 py-0.5 rounded text-[10px]">{t}</span>
                  ))}
                </div>
              )}
            </div>
            <div className="text-[10px] font-mono text-[#484f58] break-all">{selected.id}</div>
          </div>
        )}
      </div>

      {/* Confirm delete dialog */}
      {confirmDelete && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-[#161b22] border border-[#f8514930] rounded-xl p-6 max-w-sm w-full">
            <div className="flex items-center gap-3 mb-3">
              <AlertCircle size={18} className="text-[#f85149]" />
              <h3 className="text-sm font-semibold text-[#e6edf3]">Delete observation?</h3>
            </div>
            <p className="text-xs text-[#8b949e] mb-5">
              This action cannot be undone. The observation will be permanently removed from the vault.
            </p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDelete(null)}
                className="flex-1 px-4 py-2 text-sm text-[#8b949e] border border-[#30363d] rounded-lg hover:bg-[#21262d] transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(confirmDelete)}
                disabled={deleteMutation.isPending}
                className="flex-1 px-4 py-2 text-sm text-white bg-[#da3633] hover:bg-[#f85149] rounded-lg transition-colors disabled:opacity-50"
              >
                {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function TypeBadge({ type }: { type: string }) {
  const map: Record<string, string> = {
    decision: 'bg-[#388bfd20] text-[#388bfd] border-[#388bfd30]',
    pattern:  'bg-[#3fb95020] text-[#3fb950] border-[#3fb95030]',
    bugfix:   'bg-[#f8514920] text-[#f85149] border-[#f8514930]',
    learning: 'bg-[#a371f720] text-[#a371f7] border-[#a371f730]',
    context:  'bg-[#8b949e20] text-[#8b949e] border-[#8b949e30]',
  }
  const cls = map[type] ?? 'bg-[#21262d] text-[#8b949e] border-[#30363d]'
  return (
    <span className={`inline-block text-[10px] font-mono px-2 py-0.5 rounded border ${cls}`}>
      {type}
    </span>
  )
}
