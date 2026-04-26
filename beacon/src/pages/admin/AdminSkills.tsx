import _React, { useState } from 'react'
import { Wand2, Plus, Trash2, Save, History, ChevronDown, ChevronRight, Clock, User, Tag, Activity, CheckCircle2, AlertCircle } from 'lucide-react'
import { useSkills, useSaveSkill, useDeleteSkill, useSkillHistory, useSyncStatus } from '@/api/skills'
import type { Skill, SkillHistoryEntry, SyncStatusEntry } from '@/api/skills'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminSkills() {
  const [selected, setSelected] = useState<Skill | null>(null)
  const [isNew, setIsNew] = useState(false)
  const [activeTab, setActiveTab] = useState<'editor' | 'history'>('editor')
  const [globalTab, setGlobalTab] = useState<'skills' | 'sync'>('skills')
  const { t } = useI18n()

  const handleNew = () => {
    setSelected({ id: '', team_id: '', name: '', body: '', tags: '[]', version: 1, updated_by: '', scope: 'team', created_at: '', updated_at: '' })
    setIsNew(true)
    setActiveTab('editor')
  }

  const handleSelect = (sk: Skill) => {
    setSelected(sk)
    setIsNew(false)
    setActiveTab('editor')
  }

  const handleSaved = (updated: Skill) => {
    setIsNew(false)
    setSelected(updated)
  }

  return (
    <div className="p-4 sm:p-6 max-w-6xl">
      <PageHeader
        icon={<Wand2 size={17} />}
        iconColor="#388bfd"
        title={t.skills.title}
        description={t.skills.description}
        badge="Teams"
        badgeColor="blue"
        hint={{ command: 'korva skills sync', label: t.skills.hintLabel }}
        actions={
          globalTab === 'skills' ? (
            <button
              onClick={handleNew}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
            >
              <Plus size={13} />
              {t.skills.newSkill}
            </button>
          ) : undefined
        }
      />

      {/* Top-level nav: Skills | Sync Status */}
      <div className="flex gap-0 border-b border-[#21262d] mb-4">
        <button
          onClick={() => setGlobalTab('skills')}
          className={`flex items-center gap-1.5 px-4 py-2 text-xs transition-colors border-b-2 -mb-px ${
            globalTab === 'skills'
              ? 'border-[#388bfd] text-[#e6edf3]'
              : 'border-transparent text-[#8b949e] hover:text-[#c9d1d9]'
          }`}
        >
          <Wand2 size={12} />
          {t.skills.tabSkills}
        </button>
        <button
          onClick={() => setGlobalTab('sync')}
          className={`flex items-center gap-1.5 px-4 py-2 text-xs transition-colors border-b-2 -mb-px ${
            globalTab === 'sync'
              ? 'border-[#388bfd] text-[#e6edf3]'
              : 'border-transparent text-[#8b949e] hover:text-[#c9d1d9]'
          }`}
        >
          <Activity size={12} />
          {t.skills.tabSyncStatus}
        </button>
      </div>

      {globalTab === 'sync' ? (
        <SyncStatusPanel />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
          <div className="md:col-span-2">
            <SkillList selected={selected} onSelect={handleSelect} />
          </div>

          <div className="md:col-span-3 flex flex-col gap-0">
            {selected ? (
              <>
                {/* Tab bar */}
                <div className="flex border-b border-[#21262d] bg-[#161b22] rounded-t-lg overflow-hidden">
                  <button
                    onClick={() => setActiveTab('editor')}
                    className={`flex items-center gap-1.5 px-4 py-2.5 text-xs transition-colors border-b-2 ${
                      activeTab === 'editor'
                        ? 'border-[#388bfd] text-[#e6edf3]'
                        : 'border-transparent text-[#8b949e] hover:text-[#e6edf3]'
                    }`}
                  >
                    <Wand2 size={12} />
                    {t.skills.tabEditor}
                    {selected.version > 0 && (
                      <span className="ml-1 px-1.5 py-0.5 rounded text-[10px] bg-[#1f3d5c] text-[#58a6ff]">
                        v{selected.version}
                      </span>
                    )}
                  </button>
                  {!isNew && (
                    <button
                      onClick={() => setActiveTab('history')}
                      className={`flex items-center gap-1.5 px-4 py-2.5 text-xs transition-colors border-b-2 ${
                        activeTab === 'history'
                          ? 'border-[#388bfd] text-[#e6edf3]'
                          : 'border-transparent text-[#8b949e] hover:text-[#e6edf3]'
                      }`}
                    >
                      <History size={12} />
                      {t.skills.tabHistory}
                    </button>
                  )}
                </div>

                {activeTab === 'editor' ? (
                  <SkillEditor skill={selected} isNew={isNew} onSaved={handleSaved} />
                ) : (
                  <SkillHistoryPanel skillId={selected.id} />
                )}
              </>
            ) : (
              <EmptyState />
            )}
          </div>
        </div>
      )}
    </div>
  )
}

// ── Skill list ────────────────────────────────────────────────────────────────

function SkillList({ selected, onSelect }: { selected: Skill | null; onSelect: (sk: Skill) => void }) {
  const { data, isLoading } = useSkills()
  const deleteSkill = useDeleteSkill()
  const { t } = useI18n()

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
        <Wand2 size={14} className="text-[#8b949e]" />
        <span className="text-[#e6edf3] text-sm font-medium">
          {t.skills.tabSkills} {data && <span className="text-[#484f58] text-xs">({data.count})</span>}
        </span>
      </div>
      {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}
      <div className="divide-y divide-[#21262d] max-h-[calc(100vh-300px)] overflow-y-auto">
        {data?.skills.map(sk => (
          <div
            key={sk.id}
            onClick={() => onSelect(sk)}
            className={`flex items-center justify-between px-4 py-2.5 cursor-pointer transition-colors group ${
              selected?.id === sk.id ? 'bg-[#21262d]' : 'hover:bg-[#1c2128]'
            }`}
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <p className="text-[#e6edf3] text-xs truncate">{sk.name}</p>
                <VersionBadge version={sk.version} />
              </div>
              <div className="flex items-center gap-2 mt-0.5">
                {sk.team_id && (
                  <span className="text-[10px] text-[#484f58] truncate">{sk.team_id}</span>
                )}
                {sk.scope === 'org' && (
                  <span className="text-[10px] text-[#d29922] bg-[#2d2600] px-1 rounded">org</span>
                )}
                {sk.updated_by && (
                  <span className="text-[10px] text-[#484f58] truncate hidden group-hover:inline">
                    by {sk.updated_by}
                  </span>
                )}
              </div>
            </div>
            <button
              onClick={e => { e.stopPropagation(); deleteSkill.mutate(sk.id) }}
              className="ml-2 text-[#484f58] hover:text-[#f85149] transition-colors flex-shrink-0 opacity-0 group-hover:opacity-100"
            >
              <Trash2 size={12} />
            </button>
          </div>
        ))}
        {data?.skills.length === 0 && !isLoading && (
          <p className="px-4 py-6 text-xs text-[#484f58] text-center">
            {t.skills.noSkills}
          </p>
        )}
      </div>
    </div>
  )
}

// ── Editor ────────────────────────────────────────────────────────────────────

function SkillEditor({
  skill,
  isNew,
  onSaved,
}: {
  skill: Skill
  isNew: boolean
  onSaved: (updated: Skill) => void
}) {
  const saveSkill = useSaveSkill()
  const [name, setName] = useState(skill.name)
  const [body, setBody] = useState(skill.body)
  const [teamID, setTeamID] = useState(skill.team_id)
  const [scope, setScope] = useState<'team' | 'org'>(
    (skill.scope as 'team' | 'org') || 'team'
  )
  const [summary, setSummary] = useState('')
  const [showSummary, setShowSummary] = useState(false)
  const [msg, setMsg] = useState('')
  const [msgType, setMsgType] = useState<'ok' | 'err'>('ok')
  const { t } = useI18n()

  const isDirty = name !== skill.name || body !== skill.body || scope !== skill.scope

  const handleSave = async () => {
    if (!name.trim() || !body.trim()) return
    try {
      const result = await saveSkill.mutateAsync({
        team_id: teamID,
        name: name.trim(),
        body,
        scope,
        summary: summary.trim() || undefined,
      })
      setMsg(t.skills.savedVersion(result.version))
      setMsgType('ok')
      setSummary('')
      setShowSummary(false)
      onSaved({ ...skill, name: name.trim(), body, scope, version: result.version, id: result.id })
    } catch {
      setMsg(t.skills.saveFailed)
      setMsgType('err')
    }
  }

  return (
    <div className="rounded-b-lg border border-t-0 border-[#21262d] bg-[#161b22] overflow-hidden flex flex-col">
      {/* Metadata bar */}
      {!isNew && (
        <div className="px-4 py-2 border-b border-[#21262d] flex items-center gap-4 text-[10px] text-[#8b949e]">
          {skill.updated_by && (
            <span className="flex items-center gap-1">
              <User size={10} />
              {skill.updated_by}
            </span>
          )}
          {skill.updated_at && (
            <span className="flex items-center gap-1">
              <Clock size={10} />
              {fmtDate(skill.updated_at)}
            </span>
          )}
          <span className="flex items-center gap-1">
            <Tag size={10} />
            {scope}
          </span>
        </div>
      )}

      <div className="p-4 space-y-3 flex-1">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">{t.skills.nameLabel}</label>
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
            />
          </div>
          <div>
            <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">{t.skills.scopeLabel}</label>
            <select
              value={scope}
              onChange={e => setScope(e.target.value as 'team' | 'org')}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
            >
              <option value="team">{t.skills.scopeTeam}</option>
              <option value="org">{t.skills.scopeOrg}</option>
            </select>
          </div>
        </div>

        <div>
          <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">{t.skills.teamIdLabel}</label>
          <input
            value={teamID}
            onChange={e => setTeamID(e.target.value)}
            placeholder={t.skills.teamIdPlaceholder}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
          />
        </div>

        <div className="flex-1">
          <label className="text-[10px] text-[#8b949e] uppercase tracking-wider block mb-1">{t.skills.bodyLabel}</label>
          <textarea
            value={body}
            onChange={e => setBody(e.target.value)}
            rows={13}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono resize-none focus:outline-none focus:border-[#388bfd] leading-relaxed"
          />
        </div>

        {/* Change summary (collapsible) */}
        {!isNew && (
          <div>
            <button
              onClick={() => setShowSummary(v => !v)}
              className="flex items-center gap-1 text-[10px] text-[#8b949e] hover:text-[#e6edf3] transition-colors"
            >
              {showSummary ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
              {t.skills.summaryLabel}
            </button>
            {showSummary && (
              <input
                value={summary}
                onChange={e => setSummary(e.target.value)}
                placeholder={t.skills.summaryPlaceholder}
                className="mt-1.5 w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
              />
            )}
          </div>
        )}

        <div className="flex items-center justify-between pt-1">
          {msg ? (
            <p className={`text-xs ${msgType === 'ok' ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>{msg}</p>
          ) : (
            <span />
          )}
          <button
            onClick={handleSave}
            disabled={saveSkill.isPending || (!isDirty && !isNew)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          >
            <Save size={12} />
            {saveSkill.isPending ? t.common.saving : isDirty || isNew ? t.common.save : t.common.saved}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── History panel ─────────────────────────────────────────────────────────────

function SkillHistoryPanel({ skillId }: { skillId: string }) {
  const { data, isLoading } = useSkillHistory(skillId)
  const [expanded, setExpanded] = useState<string | null>(null)
  const { t } = useI18n()

  return (
    <div className="rounded-b-lg border border-t-0 border-[#21262d] bg-[#161b22] flex-1">
      <div className="px-4 py-3 border-b border-[#21262d]">
        <p className="text-[#8b949e] text-xs">
          {t.skills.versionHistory} {data && <span className="text-[#484f58]">{t.skills.versionsCount(data.count)}</span>}
        </p>
      </div>

      {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}

      <div className="divide-y divide-[#21262d] max-h-[calc(100vh-280px)] overflow-y-auto">
        {data?.history.map((h, i) => (
          <HistoryRow
            key={h.id}
            entry={h}
            isLatest={i === 0}
            isExpanded={expanded === h.id}
            onToggle={() => setExpanded(expanded === h.id ? null : h.id)}
          />
        ))}
        {data?.history.length === 0 && !isLoading && (
          <p className="px-4 py-4 text-xs text-[#484f58] text-center">{t.skills.noHistory}</p>
        )}
      </div>
    </div>
  )
}

function HistoryRow({
  entry,
  isLatest,
  isExpanded,
  onToggle,
}: {
  entry: SkillHistoryEntry
  isLatest: boolean
  isExpanded: boolean
  onToggle: () => void
}) {
  const { t } = useI18n()
  return (
    <div className="px-4 py-3">
      <div
        className="flex items-start gap-3 cursor-pointer"
        onClick={onToggle}
      >
        <div className="flex flex-col items-center mt-0.5 flex-shrink-0">
          <div className={`w-2 h-2 rounded-full ${isLatest ? 'bg-[#3fb950]' : 'bg-[#30363d]'}`} />
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <VersionBadge version={entry.version} highlight={isLatest} />
            {entry.summary ? (
              <span className="text-xs text-[#e6edf3] truncate">{entry.summary}</span>
            ) : (
              <span className="text-xs text-[#484f58] italic">{t.skills.noSummary}</span>
            )}
          </div>
          <div className="flex items-center gap-3 mt-1 text-[10px] text-[#8b949e]">
            {entry.changed_by && (
              <span className="flex items-center gap-1">
                <User size={9} />
                {entry.changed_by}
              </span>
            )}
            <span className="flex items-center gap-1">
              <Clock size={9} />
              {fmtDate(entry.changed_at)}
            </span>
          </div>
        </div>

        <span className="text-[#484f58] flex-shrink-0 mt-0.5">
          {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </span>
      </div>

      {isExpanded && (
        <div className="mt-3 ml-5">
          <pre className="text-[10px] text-[#8b949e] bg-[#0d1117] border border-[#21262d] rounded p-3 overflow-x-auto whitespace-pre-wrap leading-relaxed max-h-48 overflow-y-auto font-mono">
            {entry.body || t.skills.emptyBody}
          </pre>
        </div>
      )}
    </div>
  )
}

// ── Shared helpers ────────────────────────────────────────────────────────────

function VersionBadge({ version, highlight }: { version: number; highlight?: boolean }) {
  if (!version) return null
  return (
    <span
      className={`px-1.5 py-0.5 rounded text-[10px] font-mono flex-shrink-0 ${
        highlight
          ? 'bg-[#1f3d5c] text-[#58a6ff]'
          : 'bg-[#21262d] text-[#8b949e]'
      }`}
    >
      v{version}
    </span>
  )
}

function EmptyState() {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-10 text-center">
      <Wand2 size={28} className="text-[#30363d] mx-auto mb-3" />
      <p className="text-[#e6edf3] text-sm font-medium mb-1">{t.skills.emptyState}</p>
      <p className="text-[#484f58] text-xs">{t.skills.emptyStateHint}</p>
    </div>
  )
}

// ── Sync Status panel ─────────────────────────────────────────────────────────

function SyncStatusPanel() {
  const { data, isLoading } = useSyncStatus()
  const { t } = useI18n()

  const upToDate = data?.entries.filter(e => e.is_up_to_date).length ?? 0
  const behind = (data?.count ?? 0) - upToDate

  return (
    <div className="space-y-4">
      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-3">
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] px-4 py-3">
          <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-1">{t.skills.syncTotalSynced}</p>
          <p className="text-xl font-semibold text-[#e6edf3]">{data?.count ?? '—'}</p>
        </div>
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] px-4 py-3">
          <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-1">{t.skills.syncUpToDate}</p>
          <p className="text-xl font-semibold text-[#3fb950]">{data ? upToDate : '—'}</p>
        </div>
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] px-4 py-3">
          <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-1">{t.skills.syncBehind}</p>
          <p className={`text-xl font-semibold ${behind > 0 ? 'text-[#f85149]' : 'text-[#e6edf3]'}`}>
            {data ? behind : '—'}
          </p>
        </div>
      </div>

      {/* Developer table */}
      <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
        <div className="flex items-center gap-2 px-4 py-3 border-b border-[#21262d]">
          <Activity size={14} className="text-[#8b949e]" />
          <span className="text-[#e6edf3] text-sm font-medium">{t.skills.syncStatusHeader}</span>
          {data?.latest_skill_at && (
            <span className="ml-auto text-[10px] text-[#484f58]">
              {t.skills.syncLatestSkill(fmtDate(data.latest_skill_at))}
            </span>
          )}
        </div>

        {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}

        {!isLoading && data?.count === 0 && (
          <div className="px-4 py-8 text-center">
            <Activity size={24} className="text-[#30363d] mx-auto mb-2" />
            <p className="text-[#484f58] text-xs">{t.skills.syncNoReports}</p>
            <p className="text-[#484f58] text-[10px] mt-1">{t.skills.syncNoReportsHint}</p>
          </div>
        )}

        {(data?.entries?.length ?? 0) > 0 && (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-[#21262d]">
                <th className="text-left px-4 py-2 text-[10px] text-[#8b949e] uppercase tracking-wider font-medium">{t.skills.syncColDeveloper}</th>
                <th className="text-left px-4 py-2 text-[10px] text-[#8b949e] uppercase tracking-wider font-medium">{t.skills.syncColLastSync}</th>
                <th className="text-right px-4 py-2 text-[10px] text-[#8b949e] uppercase tracking-wider font-medium">{t.skills.syncColSkills}</th>
                <th className="text-left px-4 py-2 text-[10px] text-[#8b949e] uppercase tracking-wider font-medium">{t.skills.syncColTarget}</th>
                <th className="text-right px-4 py-2 text-[10px] text-[#8b949e] uppercase tracking-wider font-medium">{t.skills.syncColStatus}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[#21262d]">
              {data?.entries.map((entry, i) => (
                <SyncStatusRow key={i} entry={entry} />
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

function SyncStatusRow({ entry }: { entry: SyncStatusEntry }) {
  const { t } = useI18n()
  const targetShort = entry.target
    ? entry.target.replace(/^\/Users\/[^/]+/, '~').replace(/^\/home\/[^/]+/, '~')
    : '—'

  return (
    <tr className="hover:bg-[#1c2128] transition-colors">
      <td className="px-4 py-2.5">
        <div className="flex items-center gap-1.5">
          <User size={11} className="text-[#484f58] flex-shrink-0" />
          <span className="text-[#e6edf3] truncate max-w-[160px]">{entry.user_email || '—'}</span>
        </div>
      </td>
      <td className="px-4 py-2.5">
        <div className="flex items-center gap-1 text-[#8b949e]">
          <Clock size={10} className="flex-shrink-0" />
          {entry.last_sync ? fmtDate(entry.last_sync) : '—'}
        </div>
      </td>
      <td className="px-4 py-2.5 text-right text-[#8b949e]">{entry.skills_count}</td>
      <td className="px-4 py-2.5">
        <span className="text-[#484f58] font-mono text-[10px]" title={entry.target}>{targetShort}</span>
      </td>
      <td className="px-4 py-2.5 text-right">
        {entry.is_up_to_date ? (
          <span className="inline-flex items-center gap-1 text-[#3fb950] text-[10px]">
            <CheckCircle2 size={11} />
            {t.skills.syncStatusUpToDate}
          </span>
        ) : (
          <span className="inline-flex items-center gap-1 text-[#f0883e] text-[10px]">
            <AlertCircle size={11} />
            {t.skills.syncStatusBehind}
          </span>
        )}
      </td>
    </tr>
  )
}

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return iso
  }
}
