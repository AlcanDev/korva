import _React, { useState } from 'react'
import { Wand2, Plus, Trash2, Save } from 'lucide-react'
import { useSkills, useSaveSkill, useDeleteSkill } from '@/api/skills'
import type { Skill } from '@/api/skills'

export default function AdminSkills() {
  const [selected, setSelected] = useState<Skill | null>(null)
  const [isNew, setIsNew] = useState(false)

  const handleNew = () => {
    setSelected({ id: '', team_id: '', name: '', body: '', tags: '[]', created_at: '', updated_at: '' })
    setIsNew(true)
  }

  const handleSelect = (sk: Skill) => {
    setSelected(sk)
    setIsNew(false)
  }

  const handleSaved = () => {
    setIsNew(false)
    setSelected(null)
  }

  return (
    <div className="p-4 sm:p-6 max-w-5xl">
      <div className="flex items-center justify-between mb-5">
        <h1 className="text-[#e6edf3] text-lg font-semibold">Skills</h1>
        <button
          onClick={handleNew}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
        >
          <Plus size={13} />
          New skill
        </button>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        <div className="md:col-span-2">
          <SkillList selected={selected} onSelect={handleSelect} />
        </div>
        <div className="md:col-span-3">
          {selected
            ? <SkillEditor skill={selected} isNew={isNew} onSaved={handleSaved} />
            : <EmptyState />
          }
        </div>
      </div>
    </div>
  )
}

function SkillList({ selected, onSelect }: { selected: Skill | null; onSelect: (sk: Skill) => void }) {
  const { data, isLoading } = useSkills()
  const deleteSkill = useDeleteSkill()

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
        <Wand2 size={14} className="text-[#8b949e]" />
        <span className="text-[#e6edf3] text-sm font-medium">
          Skills {data && <span className="text-[#484f58] text-xs">({data.count})</span>}
        </span>
      </div>
      {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">Loading…</p>}
      <div className="divide-y divide-[#21262d] max-h-[calc(100vh-300px)] overflow-y-auto">
        {data?.skills.map(sk => (
          <div
            key={sk.id}
            onClick={() => onSelect(sk)}
            className={`flex items-center justify-between px-4 py-2.5 cursor-pointer transition-colors ${
              selected?.id === sk.id ? 'bg-[#21262d]' : 'hover:bg-[#1c2128]'
            }`}
          >
            <div className="min-w-0">
              <p className="text-[#e6edf3] text-xs truncate">{sk.name}</p>
              {sk.team_id && <p className="text-[10px] text-[#484f58] truncate">{sk.team_id}</p>}
            </div>
            <button
              onClick={e => { e.stopPropagation(); deleteSkill.mutate(sk.id) }}
              className="ml-2 text-[#484f58] hover:text-[#f85149] transition-colors flex-shrink-0"
            >
              <Trash2 size={12} />
            </button>
          </div>
        ))}
        {data?.skills.length === 0 && !isLoading && (
          <p className="px-4 py-3 text-xs text-[#484f58]">No skills yet.</p>
        )}
      </div>
    </div>
  )
}

function SkillEditor({ skill, isNew, onSaved }: { skill: Skill; isNew: boolean; onSaved: () => void }) {
  const saveSkill = useSaveSkill()
  const [name, setName] = useState(skill.name)
  const [body, setBody] = useState(skill.body)
  const [teamID, setTeamID] = useState(skill.team_id)
  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState('')

  const handleSave = async () => {
    if (!name.trim() || !body.trim()) return
    setSaving(true)
    try {
      await saveSkill.mutateAsync({ team_id: teamID, name: name.trim(), body })
      setMsg('Saved.')
      if (isNew) onSaved()
    } catch {
      setMsg('Save failed.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden flex flex-col h-full">
      <div className="px-4 py-3 border-b border-[#21262d] flex items-center justify-between">
        <p className="text-[#e6edf3] text-sm font-medium">{isNew ? 'New skill' : skill.name}</p>
        <button
          onClick={handleSave}
          disabled={saving}
          className="flex items-center gap-1.5 px-3 py-1 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors"
        >
          <Save size={12} />
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>
      <div className="p-4 space-y-3 flex-1">
        <div>
          <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">Name</label>
          <input
            value={name}
            onChange={e => setName(e.target.value)}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
          />
        </div>
        <div>
          <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">Team ID (optional)</label>
          <input
            value={teamID}
            onChange={e => setTeamID(e.target.value)}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
          />
        </div>
        <div className="flex-1">
          <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">Body (Markdown)</label>
          <textarea
            value={body}
            onChange={e => setBody(e.target.value)}
            rows={14}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono resize-none focus:outline-none focus:border-[#388bfd]"
          />
        </div>
        {msg && (
          <p className={`text-xs ${msg === 'Saved.' ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>{msg}</p>
        )}
      </div>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-8 text-center">
      <Wand2 size={24} className="text-[#30363d] mx-auto mb-2" />
      <p className="text-[#484f58] text-xs">Select a skill to edit or create a new one</p>
    </div>
  )
}
