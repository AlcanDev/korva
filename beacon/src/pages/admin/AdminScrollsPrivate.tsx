import _React, { useState } from 'react'
import { Lock, Plus, Trash2, FileText } from 'lucide-react'
import { useAdminStore } from '@/stores/admin'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'

const BASE = '/vault-api'

interface PrivateScroll {
  id: string
  name: string
  content: string
  created_at: string
  updated_at: string
}

async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const key = useAdminStore.getState().key
  const res = await fetch(BASE + path, {
    ...options,
    headers: { 'Content-Type': 'application/json', 'X-Admin-Key': key, ...options.headers },
  })
  if (!res.ok) throw new Error(`${res.status}`)
  return res.json() as Promise<T>
}

function usePrivateScrolls() {
  return useQuery({
    queryKey: ['admin', 'scrolls', 'private'],
    queryFn: () => adminFetch<{ scrolls: PrivateScroll[] }>('/admin/scrolls/private'),
    retry: false,
  })
}

function useSavePrivateScroll() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; content: string }) =>
      adminFetch<{ id: string }>('/admin/scrolls/private', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'scrolls', 'private'] }),
  })
}

function useDeletePrivateScroll() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      adminFetch<void>(`/admin/scrolls/private/${id}`, { method: 'DELETE' }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['admin', 'scrolls', 'private'] }),
  })
}

export default function AdminScrollsPrivate() {
  const { data, isLoading, error } = usePrivateScrolls()
  const saveScroll = useSavePrivateScroll()
  const deleteScroll = useDeletePrivateScroll()
  const [selected, setSelected] = useState<PrivateScroll | null>(null)
  const [editName, setEditName] = useState('')
  const [editContent, setEditContent] = useState('')
  const [isNew, setIsNew] = useState(false)
  const [msg, setMsg] = useState('')

  const openNew = () => {
    setSelected({ id: '', name: '', content: '', created_at: '', updated_at: '' })
    setEditName('')
    setEditContent('')
    setIsNew(true)
    setMsg('')
  }

  const openExisting = (s: PrivateScroll) => {
    setSelected(s)
    setEditName(s.name)
    setEditContent(s.content)
    setIsNew(false)
    setMsg('')
  }

  const handleSave = async () => {
    if (!editName.trim()) return
    try {
      await saveScroll.mutateAsync({ name: editName.trim(), content: editContent })
      setMsg('Saved.')
      if (isNew) { setIsNew(false); setSelected(null) }
    } catch {
      setMsg('Save failed.')
    }
  }

  return (
    <div className="p-4 sm:p-6 max-w-5xl">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[#e6edf3] text-lg font-semibold">Private Scrolls</h1>
        <button
          onClick={openNew}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
        >
          <Plus size={13} />
          New scroll
        </button>
      </div>

      {error && (
        <div className="rounded-lg border border-[#f85149] bg-[#f8514910] p-4 mb-4">
          <p className="text-[#f85149] text-xs">
            {String(error).includes('402')
              ? 'Private scrolls require Korva for Teams license.'
              : 'Failed to load scrolls.'}
          </p>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        {/* Scroll list */}
        <div className="md:col-span-2">
          <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
            <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
              <Lock size={13} className="text-[#8b949e]" />
              <span className="text-[#e6edf3] text-sm font-medium">Private</span>
            </div>
            {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">Loading…</p>}
            <div className="divide-y divide-[#21262d]">
              {data?.scrolls?.map(s => (
                <div
                  key={s.id}
                  onClick={() => openExisting(s)}
                  className={`flex items-center justify-between px-4 py-2.5 cursor-pointer transition-colors ${
                    selected?.id === s.id ? 'bg-[#21262d]' : 'hover:bg-[#1c2128]'
                  }`}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <FileText size={12} className="text-[#8b949e] flex-shrink-0" />
                    <p className="text-[#e6edf3] text-xs truncate">{s.name}</p>
                  </div>
                  <button
                    onClick={e => { e.stopPropagation(); deleteScroll.mutate(s.id) }}
                    className="ml-2 text-[#484f58] hover:text-[#f85149] transition-colors"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              ))}
              {(!data?.scrolls || data.scrolls.length === 0) && !isLoading && !error && (
                <p className="px-4 py-3 text-xs text-[#484f58]">No private scrolls.</p>
              )}
            </div>
          </div>
        </div>

        {/* Editor */}
        <div className="md:col-span-3">
          {selected ? (
            <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
              <div className="flex items-center justify-between px-4 py-3 border-b border-[#21262d]">
                <input
                  value={editName}
                  onChange={e => setEditName(e.target.value)}
                  placeholder="Scroll name"
                  className="bg-transparent text-[#e6edf3] text-sm font-medium focus:outline-none w-full"
                />
                <button
                  onClick={handleSave}
                  className="flex-shrink-0 ml-2 px-3 py-1 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
                >
                  Save
                </button>
              </div>
              <div className="p-4">
                <textarea
                  value={editContent}
                  onChange={e => setEditContent(e.target.value)}
                  rows={18}
                  placeholder="# Scroll content (Markdown)…"
                  className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono resize-none focus:outline-none focus:border-[#388bfd]"
                />
                {msg && (
                  <p className={`mt-2 text-xs ${msg === 'Saved.' ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>
                    {msg}
                  </p>
                )}
              </div>
            </div>
          ) : (
            <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-8 text-center">
              <Lock size={24} className="text-[#30363d] mx-auto mb-2" />
              <p className="text-[#484f58] text-xs">Select a scroll to edit or create a new one</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
