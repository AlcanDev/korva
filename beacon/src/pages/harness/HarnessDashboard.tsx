import { Link } from 'react-router'
import { GitBranch, RefreshCw, AlertCircle } from 'lucide-react'
import {
  useHarnessProjects,
  safeParseFeatureList,
  countByStatus,
  type HarnessProjectSummary,
  type FeatureListPayload,
} from '@/api/harness'

// HarnessDashboard — Phase 14.3.
// Lists every harness-managed project the caller's team owns, with a
// per-card status roll-up and a link to the per-project detail view.
// Renders empty / loading / error states explicitly so the operator
// always sees signal (not a silent blank screen).

export default function HarnessDashboard() {
  const { data, isLoading, error, refetch } = useHarnessProjects()
  const projects = data?.projects ?? []

  return (
    <div className="p-6 max-w-6xl">
      <header className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
            <GitBranch size={18} className="text-[#f0883e]" />
            Harness
          </h1>
          <p className="text-sm text-[#8b949e] mt-0.5">
            Per-project state from <code className="text-[#58a6ff]">feature_list.json</code>, mirrored from the harness CLI / MCP layer.
          </p>
        </div>
        <button
          type="button"
          onClick={() => refetch()}
          aria-label="Refresh harness projects"
          className="text-[#8b949e] hover:text-[#e6edf3] p-1.5 rounded transition-colors"
        >
          <RefreshCw size={14} />
        </button>
      </header>

      {isLoading && <Skeleton />}

      {error && (
        <ErrorBanner
          message="Could not load harness state from the vault."
          detail={String(error)}
        />
      )}

      {!isLoading && !error && projects.length === 0 && <EmptyState />}

      {projects.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {projects.map(p => (
            <ProjectCard key={`${p.project}::${p.root}`} project={p} />
          ))}
        </div>
      )}
    </div>
  )
}

// ── card ───────────────────────────────────────────────────────────────────

function ProjectCard({ project }: { project: HarnessProjectSummary }) {
  // The dashboard list endpoint returns the summary roll-up; for a
  // tighter "live counts" view per card we'd need to fetch the full
  // snapshot. To keep the dashboard cheap we render only the summary
  // fields here and let the detail page compute counts from the
  // payload.
  const linkTo = `/app/harness/${encodeURIComponent(project.project)}?root=${encodeURIComponent(project.root)}`
  return (
    <Link
      to={linkTo}
      className="block border border-[#21262d] hover:border-[#30363d] rounded-lg p-4 transition-colors"
    >
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="min-w-0">
          <div className="text-sm font-semibold text-[#e6edf3] truncate" title={project.project}>
            {project.project}
          </div>
          <div className="text-xs text-[#6e7681] truncate font-mono" title={project.root}>
            {project.root}
          </div>
        </div>
        {project.last_transition_to && (
          <StatusPill status={project.last_transition_to} />
        )}
      </div>
      <div className="flex items-center justify-between text-xs text-[#8b949e]">
        <span>Updated {formatRelative(project.updated_at)}</span>
        {project.last_transition_at && (
          <span>Last transition {formatRelative(project.last_transition_at)}</span>
        )}
      </div>
    </Link>
  )
}

// ── status pill ────────────────────────────────────────────────────────────

const STATUS_COLORS: Record<string, { bg: string; fg: string; label: string }> = {
  pending:     { bg: '#30363d40', fg: '#8b949e', label: 'pending' },
  spec_ready:  { bg: '#9e6a0340', fg: '#d29922', label: 'spec ready' },
  in_progress: { bg: '#1f6feb40', fg: '#58a6ff', label: 'in progress' },
  done:        { bg: '#23863640', fg: '#3fb950', label: 'done' },
  blocked:     { bg: '#da363340', fg: '#f85149', label: 'blocked' },
}

export function StatusPill({ status }: { status: string }) {
  const c = STATUS_COLORS[status] ?? { bg: '#30363d40', fg: '#8b949e', label: status }
  return (
    <span
      className="text-[11px] font-medium px-2 py-0.5 rounded-full border whitespace-nowrap"
      style={{ background: c.bg, color: c.fg, borderColor: c.fg + '40' }}
    >
      {c.label}
    </span>
  )
}

// ── states ─────────────────────────────────────────────────────────────────

function Skeleton() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      {[0, 1, 2, 3].map(i => (
        <div
          key={i}
          className="border border-[#21262d] rounded-lg p-4 animate-pulse"
          aria-hidden="true"
        >
          <div className="h-4 bg-[#21262d] rounded w-1/2 mb-2" />
          <div className="h-3 bg-[#21262d] rounded w-3/4 mb-3" />
          <div className="h-3 bg-[#21262d] rounded w-1/3" />
        </div>
      ))}
    </div>
  )
}

function EmptyState() {
  return (
    <div className="border border-dashed border-[#30363d] rounded-lg p-8 text-center">
      <GitBranch size={28} className="mx-auto text-[#6e7681] mb-3" />
      <h3 className="text-sm font-semibold text-[#e6edf3] mb-1">No harness state yet</h3>
      <p className="text-sm text-[#8b949e] max-w-md mx-auto leading-relaxed">
        Projects appear here when an agent calls the MCP harness tools with a{' '}
        <code className="text-[#58a6ff]">project</code> argument. Run{' '}
        <code className="text-[#58a6ff]">korva harness init</code> in a repo to bootstrap one.
      </p>
    </div>
  )
}

function ErrorBanner({ message, detail }: { message: string; detail: string }) {
  return (
    <div
      role="alert"
      className="border border-[#da363340] bg-[#da363315] rounded-lg p-3 flex items-start gap-3"
    >
      <AlertCircle size={16} className="text-[#f85149] flex-shrink-0 mt-0.5" />
      <div className="text-sm">
        <div className="text-[#f85149] font-medium">{message}</div>
        <div className="text-[#8b949e] mt-0.5 font-mono text-xs">HTTP {detail}</div>
      </div>
    </div>
  )
}

// ── helpers ────────────────────────────────────────────────────────────────

// formatRelative renders an ISO timestamp as a compact "5m ago" / "2h
// ago" / "3d ago" / fallback "MMM d" string. Intentionally synchronous
// — no need for Intl.RelativeTimeFormat ceremony here.
export function formatRelative(iso: string): string {
  if (!iso) return '—'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return '—'
  const diff = Date.now() - t
  if (diff < 0) return 'just now'
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.floor(hr / 24)
  if (day < 7) return `${day}d ago`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

// Re-export feature parsing helpers for the detail page.
export { safeParseFeatureList, countByStatus }
export type { FeatureListPayload }
