import { useState } from 'react'
import { BookMarked, Plus, Trash2, Tag, AlertCircle, Copy, Check } from 'lucide-react'
import { useAdminPrompts, useAdminSavePrompt, useAdminDeletePrompt, type Prompt } from '@/api/admin'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminPrompts() {
  const { data, isLoading, error } = useAdminPrompts()
  const savePrompt = useAdminSavePrompt()
  const deletePrompt = useAdminDeletePrompt()
  const { t } = useI18n()

  const [selected, setSelected] = useState<Prompt | null>(null)
  const [editName, setEditName] = useState('')
  const [editContent, setEditContent] = useState('')
  const [editTags, setEditTags] = useState('')
  const [isNew, setIsNew] = useState(false)
  const [msg, setMsg] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const prompts = data?.prompts ?? []

  const openNew = () => {
    setSelected({ id: '', name: '', content: '', tags: [], created_at: '', updated_at: '' })
    setEditName('')
    setEditContent('')
    setEditTags('')
    setIsNew(true)
    setMsg('')
  }

  const openExisting = (p: Prompt) => {
    setSelected(p)
    setEditName(p.name)
    setEditContent(p.content)
    setEditTags(p.tags.join(', '))
    setIsNew(false)
    setMsg('')
  }

  const handleSave = async () => {
    if (!editName.trim()) return
    setMsg('')
    try {
      const tags = editTags.split(',').map(t => t.trim()).filter(Boolean)
      await savePrompt.mutateAsync({ name: editName.trim(), content: editContent, tags })
      setMsg(t.prompts.savedOk)
      if (isNew) { setIsNew(false); setSelected(null) }
    } catch {
      setMsg(t.prompts.saveFailed)
    }
  }

  const handleDelete = async (name: string) => {
    await deletePrompt.mutateAsync(name)
    setConfirmDelete(null)
    if (selected?.name === name) setSelected(null)
  }

  const copyContent = () => {
    if (!editContent) return
    navigator.clipboard.writeText(editContent).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="p-4 sm:p-6 max-w-5xl space-y-5">
      <PageHeader
        icon={<BookMarked size={17} />}
        iconColor="#f0883e"
        title={t.prompts.title}
        description={t.prompts.description}
        hint={{ command: 'korva vault save-prompt', label: t.prompts.hintLabel }}
        actions={
          <button
            onClick={openNew}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
          >
            <Plus size={13} />
            {t.prompts.newPrompt}
          </button>
        }
      />

      {error && (
        <div className="rounded-lg border border-[#f85149] bg-[#f8514910] p-4">
          <p className="text-[#f85149] text-xs">{t.prompts.errorLoad}</p>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        {/* Prompt list */}
        <div className="md:col-span-2">
          <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
            <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
              <BookMarked size={13} className="text-[#8b949e]" />
              <span className="text-[#e6edf3] text-sm font-medium">{t.prompts.listHeader}</span>
              {data && (
                <span className="ml-auto text-[10px] text-[#484f58]">{data.total}</span>
              )}
            </div>

            {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}

            <div className="divide-y divide-[#21262d]">
              {prompts.map(p => (
                <div
                  key={p.id}
                  onClick={() => openExisting(p)}
                  className={`flex items-center justify-between px-4 py-2.5 cursor-pointer transition-colors ${
                    selected?.name === p.name ? 'bg-[#21262d]' : 'hover:bg-[#1c2128]'
                  }`}
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-[#e6edf3] text-xs truncate font-medium">{p.name}</p>
                    {p.tags.length > 0 && (
                      <div className="flex items-center gap-1 mt-0.5">
                        <Tag size={9} className="text-[#484f58]" />
                        <p className="text-[9px] text-[#484f58] truncate">{p.tags.join(', ')}</p>
                      </div>
                    )}
                  </div>
                  <button
                    onClick={e => { e.stopPropagation(); setConfirmDelete(p.name) }}
                    className="ml-2 text-[#484f58] hover:text-[#f85149] transition-colors flex-shrink-0"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              ))}

              {!isLoading && prompts.length === 0 && !error && (
                <div className="px-4 py-6 text-center">
                  <p className="text-xs text-[#484f58]">{t.prompts.noPrompts}</p>
                  <p className="text-[10px] text-[#30363d] mt-1">{t.prompts.noPromptsHint}</p>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Editor */}
        <div className="md:col-span-3">
          {selected ? (
            <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
              {/* Header: name + save */}
              <div className="flex items-center justify-between px-4 py-3 border-b border-[#21262d] gap-2">
                <input
                  value={editName}
                  onChange={e => setEditName(e.target.value)}
                  placeholder={t.prompts.namePlaceholder}
                  className="bg-transparent text-[#e6edf3] text-sm font-medium focus:outline-none flex-1 min-w-0"
                />
                <div className="flex items-center gap-2 flex-shrink-0">
                  <button
                    onClick={copyContent}
                    className="text-[#484f58] hover:text-[#8b949e] transition-colors"
                    title={t.prompts.copyContent}
                  >
                    {copied ? <Check size={13} className="text-[#3fb950]" /> : <Copy size={13} />}
                  </button>
                  <button
                    onClick={handleSave}
                    disabled={savePrompt.isPending || !editName.trim()}
                    className="px-3 py-1 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors"
                  >
                    {savePrompt.isPending ? t.common.saving : t.common.save}
                  </button>
                </div>
              </div>

              <div className="p-4 space-y-3">
                {/* Tags */}
                <div>
                  <label className="block text-[10px] text-[#484f58] uppercase tracking-wider mb-1">
                    {t.prompts.tagsLabel}
                  </label>
                  <input
                    value={editTags}
                    onChange={e => setEditTags(e.target.value)}
                    placeholder={t.prompts.tagsPlaceholder}
                    className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd] transition-colors"
                  />
                </div>

                {/* Content */}
                <div>
                  <div className="flex items-center justify-between mb-1">
                    <label className="text-[10px] text-[#484f58] uppercase tracking-wider">Content</label>
                    <span className="text-[10px] text-[#30363d]">
                      {t.prompts.characters(editContent.length)}
                    </span>
                  </div>
                  <textarea
                    value={editContent}
                    onChange={e => setEditContent(e.target.value)}
                    rows={16}
                    placeholder={t.prompts.contentPlaceholder}
                    className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono resize-none focus:outline-none focus:border-[#388bfd] transition-colors"
                  />
                </div>

                {msg && (
                  <p className={`text-xs ${msg === t.prompts.savedOk ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>
                    {msg}
                  </p>
                )}

                {/* Updated at */}
                {selected.updated_at && (
                  <p className="text-[10px] text-[#30363d]">
                    {new Date(selected.updated_at).toLocaleString()}
                  </p>
                )}
              </div>
            </div>
          ) : (
            <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-8 text-center">
              <BookMarked size={24} className="text-[#30363d] mx-auto mb-2" />
              <p className="text-[#484f58] text-xs">{t.prompts.emptyState}</p>
            </div>
          )}
        </div>
      </div>

      {/* Delete confirmation */}
      {confirmDelete && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div className="bg-[#161b22] border border-[#f8514930] rounded-xl p-6 max-w-sm w-full">
            <div className="flex items-center gap-3 mb-3">
              <AlertCircle size={18} className="text-[#f85149]" />
              <h3 className="text-sm font-semibold text-[#e6edf3]">{t.prompts.deleteConfirmTitle}</h3>
            </div>
            <p className="text-xs text-[#8b949e] mb-5">{t.prompts.deleteConfirmDesc}</p>
            <div className="flex gap-3">
              <button
                onClick={() => setConfirmDelete(null)}
                className="flex-1 px-4 py-2 text-sm text-[#8b949e] border border-[#30363d] rounded-lg hover:bg-[#21262d] transition-colors"
              >
                {t.common.cancel}
              </button>
              <button
                onClick={() => handleDelete(confirmDelete)}
                disabled={deletePrompt.isPending}
                className="flex-1 px-4 py-2 text-sm text-white bg-[#da3633] hover:bg-[#f85149] rounded-lg disabled:opacity-50 transition-colors"
              >
                {deletePrompt.isPending ? t.common.deleting : t.common.delete}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
