import { useMemo } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { ArrowLeft, AlertCircle, Clock } from 'lucide-react'
import {
  useHarnessProject,
  useHarnessTransitions,
  safeParseFeatureList,
  countByStatus,
  type FeatureListFeature,
  type HarnessTransition,
} from '@/api/harness'
import { StatusPill, formatRelative } from './HarnessDashboard'

// HarnessProjectDetail — Phase 14.3.
// Drill-in view for one (project, root) pair. Renders the full
// feature_list (parsed from the snapshot payload) plus the transition
// timeline. Both queries return only what the caller's team owns;
// cross-team access surfaces as a 404 banner.

export default function HarnessProjectDetail() {
  const { project } = useParams<{ project: string }>()
  const [search] = useSearchParams()
  const root = search.get('root') ?? ''

  const snapshotQ = useHarnessProject(project, root)
  const transitionsQ = useHarnessTransitions(project)

  const parsed = useMemo(
    () => (snapshotQ.data ? safeParseFeatureList(snapshotQ.data.payload) : null),
    [snapshotQ.data],
  )
  const counts = useMemo(
    () => (parsed ? countByStatus(parsed.features) : null),
    [parsed],
  )

  return (
    <div className="p-6 max-w-6xl">
      <header className="mb-6">
        <Link
          to="/app/harness"
          className="text-xs text-[#8b949e] hover:text-[#e6edf3] inline-flex items-center gap-1 mb-2 transition-colors"
        >
          <ArrowLeft size={12} /> All projects
        </Link>
        <h1 className="text-xl font-semibold text-[#e6edf3]" title={project}>
          {project}
        </h1>
        <p className="text-xs text-[#6e7681] font-mono mt-0.5" title={root}>
          {root}
        </p>
        {parsed?.rules.require_approved_spec_to_implement && (
          <div className="inline-flex items-center gap-1.5 mt-2 text-[11px] text-[#d29922] bg-[#9e6a0320] px-2 py-0.5 rounded">
            SDD mode
          </div>
        )}
      </header>

      {snapshotQ.isLoading && <SkeletonBlock />}

      {snapshotQ.error && (
        <ErrorBanner
          message={
            String(snapshotQ.error).includes('404')
              ? 'Snapshot not found.'
              : 'Could not load snapshot from the vault.'
          }
          detail={String(snapshotQ.error)}
        />
      )}

      {parsed && (
        <>
          {counts && <CountsRow counts={counts} sdd={Boolean(parsed.rules.require_approved_spec_to_implement)} />}
          <FeaturesTable features={parsed.features} />
          <Timeline
            transitions={transitionsQ.data?.transitions ?? []}
            isLoading={transitionsQ.isLoading}
          />
        </>
      )}
    </div>
  )
}

// ── counts ────────────────────────────────────────────────────────────────

function CountsRow({
  counts,
  sdd,
}: {
  counts: ReturnType<typeof countByStatus>
  sdd: boolean
}) {
  const cells = [
    { key: 'pending',     label: 'Pending',     value: counts.pending,     color: '#8b949e' },
    sdd ? { key: 'spec_ready', label: 'Spec ready', value: counts.spec_ready, color: '#d29922' } : null,
    { key: 'in_progress', label: 'In progress', value: counts.in_progress, color: '#58a6ff' },
    { key: 'done',        label: 'Done',        value: counts.done,        color: '#3fb950' },
    { key: 'blocked',     label: 'Blocked',     value: counts.blocked,     color: '#f85149' },
    { key: 'total',       label: 'Total',       value: counts.total,       color: '#e6edf3' },
  ].filter((c): c is { key: string; label: string; value: number; color: string } => c !== null)

  return (
    <section
      aria-label="Backlog counts"
      className="grid gap-3 mb-6"
      style={{ gridTemplateColumns: `repeat(${cells.length}, minmax(0, 1fr))` }}
    >
      {cells.map(c => (
        <div
          key={c.key}
          className="border border-[#21262d] rounded-lg p-3 text-center"
        >
          <div className="text-2xl font-semibold" style={{ color: c.color }}>
            {c.value}
          </div>
          <div className="text-[11px] text-[#8b949e] uppercase tracking-wider mt-1">
            {c.label}
          </div>
        </div>
      ))}
    </section>
  )
}

// ── features table ────────────────────────────────────────────────────────

function FeaturesTable({ features }: { features: FeatureListFeature[] }) {
  if (features.length === 0) {
    return (
      <section aria-label="Features" className="mb-8">
        <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3">Features</h2>
        <div className="border border-dashed border-[#30363d] rounded-lg p-6 text-center text-sm text-[#8b949e]">
          No features yet.
        </div>
      </section>
    )
  }
  return (
    <section aria-label="Features" className="mb-8">
      <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3">Features</h2>
      <div className="border border-[#21262d] rounded-lg overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-[#0d1117]">
            <tr className="text-left text-[11px] uppercase tracking-wider text-[#6e7681]">
              <th scope="col" className="px-3 py-2 w-12">#</th>
              <th scope="col" className="px-3 py-2">Title</th>
              <th scope="col" className="px-3 py-2 w-28">Status</th>
              <th scope="col" className="px-3 py-2 w-20 text-right">SDD</th>
            </tr>
          </thead>
          <tbody>
            {features.map(f => (
              <tr key={f.id} className="border-t border-[#21262d]">
                <td className="px-3 py-2 text-[#6e7681] font-mono text-xs">#{f.id}</td>
                <td className="px-3 py-2">
                  <div className="text-[#e6edf3]">{f.title || f.name}</div>
                  <div className="text-[11px] text-[#6e7681] font-mono">{f.name}</div>
                </td>
                <td className="px-3 py-2">
                  <StatusPill status={f.status} />
                </td>
                <td className="px-3 py-2 text-right text-xs text-[#8b949e]">
                  {f.sdd ? '✓' : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}

// ── timeline ──────────────────────────────────────────────────────────────

function Timeline({
  transitions,
  isLoading,
}: {
  transitions: HarnessTransition[]
  isLoading: boolean
}) {
  return (
    <section aria-label="Transition timeline" className="mb-8">
      <h2 className="text-sm font-semibold text-[#8b949e] uppercase tracking-wider mb-3 flex items-center gap-2">
        <Clock size={12} /> Recent transitions
      </h2>
      {isLoading && (
        <div className="border border-[#21262d] rounded-lg p-6 text-sm text-[#8b949e] text-center">
          Loading…
        </div>
      )}
      {!isLoading && transitions.length === 0 && (
        <div className="border border-dashed border-[#30363d] rounded-lg p-6 text-sm text-[#8b949e] text-center">
          No transitions logged yet for this project.
        </div>
      )}
      {transitions.length > 0 && (
        <div className="border border-[#21262d] rounded-lg divide-y divide-[#21262d]">
          {transitions.map(t => (
            <div key={t.id} className="px-3 py-2 flex items-center gap-3 text-sm">
              <span className="text-xs text-[#6e7681] w-20 flex-shrink-0">
                {formatRelative(t.occurred_at)}
              </span>
              <span className="text-xs text-[#6e7681] font-mono w-12 flex-shrink-0">
                #{t.feature_id}
              </span>
              <div className="flex items-center gap-2 flex-1 min-w-0">
                <StatusPill status={t.from_status} />
                <span className="text-[#6e7681]">→</span>
                <StatusPill status={t.to_status} />
              </div>
              {t.owner && (
                <span className="text-xs text-[#8b949e] font-mono truncate" title={t.owner}>
                  {t.owner}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// ── shared visual atoms ───────────────────────────────────────────────────

function SkeletonBlock() {
  return (
    <div className="space-y-4" aria-hidden="true">
      <div className="grid grid-cols-5 gap-3">
        {[0, 1, 2, 3, 4].map(i => (
          <div key={i} className="border border-[#21262d] rounded-lg p-3 animate-pulse h-16" />
        ))}
      </div>
      <div className="border border-[#21262d] rounded-lg p-4 animate-pulse h-32" />
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
