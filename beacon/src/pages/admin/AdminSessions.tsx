import { useState, useMemo } from 'react'
import { Clock, FolderGit2, Bot, Layers, Search, X } from 'lucide-react'
import { useAdminSessionsWithStats, type SessionRow } from '@/api/admin'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminSessions() {
  const { data, isLoading, error } = useAdminSessionsWithStats()
  const [search, setSearch] = useState('')
  const [filterProject, setFilterProject] = useState('')
  const { t } = useI18n()

  const sessions = data?.sessions ?? []

  const projects = useMemo(() => {
    const set = new Set(sessions.map(s => s.project).filter(Boolean))
    return Array.from(set).sort()
  }, [sessions])

  const filtered = useMemo(() => {
    return sessions.filter(s => {
      if (filterProject && s.project !== filterProject) return false
      if (search) {
        const q = search.toLowerCase()
        return (
          s.project?.toLowerCase().includes(q) ||
          s.goal?.toLowerCase().includes(q) ||
          s.agent?.toLowerCase().includes(q)
        )
      }
      return true
    })
  }, [sessions, search, filterProject])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-[#388bfd]" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="m-6 bg-[#f8514912] border border-[#f8514930] rounded-xl p-5">
        <p className="text-[#f85149] text-sm font-medium">{t.sessions.couldNotLoad}</p>
        <p className="text-[#8b949e] text-xs mt-1">{error.message}</p>
      </div>
    )
  }

  return (
    <div className="p-4 sm:p-6 space-y-5">
      <PageHeader
        icon={<Clock size={17} />}
        iconColor="#3fb950"
        title={t.sessions.title}
        description={t.sessions.description}
        hint={{ command: 'korva status', label: t.sessions.hintLabel }}
      />

      {/* Filters */}
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="relative flex-1">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#484f58]" />
          <input
            type="text"
            placeholder={t.sessions.searchPlaceholder}
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full pl-8 pr-8 py-2 bg-[#161b22] border border-[#30363d] rounded-lg text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd] transition-colors"
          />
          {search && (
            <button onClick={() => setSearch('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-[#484f58] hover:text-[#8b949e]">
              <X size={13} />
            </button>
          )}
        </div>
        <select
          value={filterProject}
          onChange={e => setFilterProject(e.target.value)}
          className="sm:w-52 bg-[#161b22] border border-[#30363d] rounded-lg px-3 py-2 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd] transition-colors"
        >
          <option value="">{t.sessions.allProjects}</option>
          {projects.map(p => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
      </div>

      {/* Count */}
      <div className="text-xs text-[#484f58]">
        {t.sessions.count(filtered.length, data?.total ?? sessions.length)}
      </div>

      {/* Sessions list */}
      {filtered.length === 0 ? (
        <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-10 text-center">
          <p className="text-[#484f58] text-sm">{t.sessions.noSessions}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map(s => <SessionCard key={s.id} session={s} noGoalLabel={t.sessions.noGoal} />)}
        </div>
      )}
    </div>
  )
}

function SessionCard({ session: s, noGoalLabel }: { session: SessionRow; noGoalLabel: string }) {
  return (
    <div className="bg-[#161b22] border border-[#21262d] rounded-xl p-4 hover:border-[#30363d] transition-colors">
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0 space-y-1.5">
          {/* Project + agent */}
          <div className="flex items-center gap-2 flex-wrap">
            {s.project ? (
              <div className="flex items-center gap-1.5">
                <FolderGit2 size={12} className="text-[#388bfd] flex-shrink-0" />
                <span className="text-xs font-mono text-[#388bfd] font-medium">{s.project}</span>
              </div>
            ) : (
              <span className="text-xs text-[#484f58]">—</span>
            )}
            {s.agent && (
              <div className="flex items-center gap-1">
                <Bot size={11} className="text-[#8b949e]" />
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#21262d] text-[#8b949e]">{s.agent}</span>
              </div>
            )}
          </div>

          {/* Goal */}
          {s.goal ? (
            <p className="text-sm text-[#e6edf3] line-clamp-2 leading-snug">{s.goal}</p>
          ) : (
            <p className="text-xs text-[#484f58] italic">{noGoalLabel}</p>
          )}

          {/* Session ID */}
          <p className="text-[10px] font-mono text-[#30363d]">{s.id}</p>
        </div>

        {/* Stats */}
        <div className="flex flex-col items-end gap-2 shrink-0 text-right">
          <div className="flex items-center gap-1.5">
            <Layers size={11} className="text-[#8b949e]" />
            <span className="text-sm font-bold text-[#e6edf3] tabular-nums">{s.obs_count}</span>
            <span className="text-xs text-[#484f58]">obs</span>
          </div>
          {s.duration_min > 0 && (
            <div className="flex items-center gap-1">
              <Clock size={11} className="text-[#8b949e]" />
              <span className="text-xs text-[#8b949e]">{formatDuration(s.duration_min)}</span>
            </div>
          )}
          <div className="text-[10px] text-[#484f58]">{formatRelative(s.started_at)}</div>
        </div>
      </div>
    </div>
  )
}

function formatDuration(mins: number): string {
  if (mins < 60) return `${mins}m`
  const h = Math.floor(mins / 60)
  const m = mins % 60
  return m > 0 ? `${h}h ${m}m` : `${h}h`
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  if (days < 30) return `${days}d ago`
  return new Date(iso).toLocaleDateString()
}
