import { Clock, Bot, Database, CheckCircle, Circle } from 'lucide-react'
import { useSessions, type SessionRow } from '@/api/vault'

export default function Sessions() {
  const { data, isLoading, isError } = useSessions()

  if (isLoading) return <PageSkeleton />
  if (isError) return (
    <div className="p-6 max-w-5xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Sessions</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">History of AI-assisted development sessions</p>
      </header>
      <p className="text-sm text-[#8b949e]">Could not load sessions.</p>
    </div>
  )

  const sessions = data?.sessions ?? []

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Sessions</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">History of AI-assisted development sessions</p>
      </header>

      {sessions.length === 0 ? (
        <div className="border border-[#21262d] rounded-lg p-8 text-center">
          <Clock size={32} className="text-[#30363d] mx-auto mb-3" />
          <p className="text-sm text-[#8b949e]">No sessions yet</p>
          <p className="text-xs text-[#6e7681] mt-1">
            Start a session with <code className="text-[#79c0ff]">vault_session_start</code> from your AI assistant
          </p>
        </div>
      ) : (
        <>
          <p className="text-xs text-[#8b949e] mb-4">
            {data?.total ?? 0} session{data?.total !== 1 ? 's' : ''}
          </p>
          <div className="space-y-3">
            {sessions.map((s) => <SessionCard key={s.id} session={s} />)}
          </div>
        </>
      )}
    </div>
  )
}

function SessionCard({ session: s }: { session: SessionRow }) {
  const isActive = !s.ended_at
  const duration = s.duration_min > 0
    ? s.duration_min >= 60
      ? `${Math.floor(s.duration_min / 60)}h ${s.duration_min % 60}m`
      : `${s.duration_min}m`
    : null

  return (
    <div className="border border-[#21262d] rounded-lg p-4 bg-[#161b22] hover:border-[#30363d] transition-colors">
      <div className="flex items-start justify-between gap-3 mb-2">
        <div className="flex items-center gap-2 min-w-0">
          {isActive
            ? <Circle size={10} className="text-[#3fb950] flex-shrink-0 fill-[#3fb950]" />
            : <CheckCircle size={10} className="text-[#8b949e] flex-shrink-0" />
          }
          <h3 className="text-sm font-medium text-[#e6edf3] truncate">{s.goal || 'Untitled session'}</h3>
        </div>
        <span className={`text-xs px-2 py-0.5 rounded-full flex-shrink-0 ${
          isActive
            ? 'bg-[#3fb95022] text-[#3fb950]'
            : 'bg-[#21262d] text-[#8b949e]'
        }`}>
          {isActive ? 'active' : 'ended'}
        </span>
      </div>

      <div className="flex items-center gap-4 text-xs text-[#6e7681] flex-wrap">
        <span className="flex items-center gap-1.5">
          <Database size={10} />
          {s.project}
        </span>
        <span className="flex items-center gap-1.5">
          <Bot size={10} />
          {s.agent || 'unknown'}
        </span>
        <span className="flex items-center gap-1.5">
          <Database size={10} className="opacity-0" />
          {s.obs_count} observation{s.obs_count !== 1 ? 's' : ''}
        </span>
        {duration && (
          <span className="flex items-center gap-1.5">
            <Clock size={10} />
            {duration}
          </span>
        )}
        <span className="ml-auto tabular-nums">{formatDate(s.started_at)}</span>
      </div>
    </div>
  )
}

function PageSkeleton() {
  return (
    <div className="p-6 max-w-5xl">
      <div className="h-7 w-32 bg-[#21262d] rounded mb-5 animate-pulse" />
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-20 bg-[#161b22] border border-[#21262d] rounded-lg animate-pulse" />
        ))}
      </div>
    </div>
  )
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}
