import { useState } from 'react'
import { FolderTree, GitMerge, Trash2, Check, RefreshCw, AlertCircle } from 'lucide-react'
import {
  useProjects,
  useProjectSuggestions,
  useConsolidateProjects,
  usePruneProjects,
  type ConsolidationProposal,
} from '@/api/projects'

// Fase 6.1 — UI sobre `/admin/projects*`. Tres acciones del operador:
//
//   1. Listar todos los proyectos con sus contadores
//   2. Sugerir consolidaciones (proyectos con nombres-variante que
//      normalizan a la misma forma canónica)
//   3. Podar proyectos vacíos (sesiones sin observaciones asociadas)
//
// Cada mutación invalida la lista vía TanStack Query, así el panel se
// refresca sin recargas manuales.

type Tab = 'list' | 'suggestions' | 'prune'

export default function ProjectsPanel() {
  const [tab, setTab] = useState<Tab>('list')
  return (
    <div className="p-6 max-w-7xl mx-auto space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
          <FolderTree size={18} /> Projects
        </h1>
        <p className="text-xs text-[#8b949e] mt-1">
          Inspect, consolidate, and prune the projects tracked by the vault.
          Variant names like <code className="font-mono">alpha</code> /{' '}
          <code className="font-mono">Alpha</code> and orphan sessions from abandoned MCP
          runs are exactly what these tools clean up.
        </p>
      </header>

      <nav className="flex border-b border-[#21262d]">
        <TabButton active={tab === 'list'} onClick={() => setTab('list')}>
          Inventory
        </TabButton>
        <TabButton active={tab === 'suggestions'} onClick={() => setTab('suggestions')}>
          Consolidate
        </TabButton>
        <TabButton active={tab === 'prune'} onClick={() => setTab('prune')}>
          Prune empty
        </TabButton>
      </nav>

      {tab === 'list' && <ProjectsList />}
      {tab === 'suggestions' && <ConsolidateView />}
      {tab === 'prune' && <PruneView />}
    </div>
  )
}

// ── Inventory ────────────────────────────────────────────────────────────────

function ProjectsList() {
  const { data, isLoading, error, refetch, isFetching } = useProjects()
  if (isLoading) return <p className="text-xs text-[#8b949e]">Loading…</p>
  if (error) return <p className="text-xs text-[#f85149]">Error: {String(error)}</p>
  const projects = data?.projects ?? []
  return (
    <>
      <div className="flex items-center justify-between">
        <p className="text-[10px] text-[#484f58]">
          {data?.count ?? 0} project(s) tracked
        </p>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-[10px] text-[#8b949e] hover:text-[#e6edf3] disabled:opacity-40"
        >
          <RefreshCw size={11} className={isFetching ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>
      {projects.length === 0 ? (
        <EmptyState
          title="No projects yet"
          subtitle="The vault hasn't recorded any observations or sessions."
        />
      ) : (
        <div className="rounded-lg border border-[#30363d] overflow-hidden">
          <table className="w-full text-xs">
            <thead className="bg-[#161b22] text-[10px] uppercase tracking-wider text-[#8b949e]">
              <tr>
                <th className="text-left py-2 px-3">Project</th>
                <th className="text-right py-2 px-3">Observations</th>
                <th className="text-right py-2 px-3">Sessions</th>
              </tr>
            </thead>
            <tbody className="bg-[#0d1117]">
              {projects.map(p => (
                <tr key={p.name} className="border-t border-[#21262d] hover:bg-[#161b22]">
                  <td className="py-2 px-3 text-[#e6edf3] font-mono">{p.name}</td>
                  <td className="py-2 px-3 text-right text-[#c9d1d9] font-mono">
                    {p.observation_count}
                  </td>
                  <td className="py-2 px-3 text-right text-[#c9d1d9] font-mono">
                    {p.session_count}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

// ── Consolidate ──────────────────────────────────────────────────────────────

function ConsolidateView() {
  const { data, isLoading, error, refetch, isFetching } = useProjectSuggestions()
  const consolidate = useConsolidateProjects()

  if (isLoading) return <p className="text-xs text-[#8b949e]">Loading…</p>
  if (error) return <p className="text-xs text-[#f85149]">Error: {String(error)}</p>

  const proposals = data?.proposals ?? []
  return (
    <>
      <div className="flex items-center justify-between">
        <p className="text-[10px] text-[#484f58]">
          {data?.count ?? 0} merge candidate(s)
        </p>
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-[10px] text-[#8b949e] hover:text-[#e6edf3] disabled:opacity-40"
        >
          <RefreshCw size={11} className={isFetching ? 'animate-spin' : ''} /> Re-scan
        </button>
      </div>
      {proposals.length === 0 ? (
        <EmptyState
          title="No variants found"
          subtitle="Every project has a unique normalized name."
        />
      ) : (
        <div className="space-y-3">
          {proposals.map(p => (
            <ProposalCard
              key={p.canonical}
              proposal={p}
              onMerge={(canonical, sources) =>
                consolidate.mutate({ canonical, sources })
              }
              pending={consolidate.isPending}
            />
          ))}
          {consolidate.isSuccess && (
            <p className="text-xs text-[#2ea043] flex items-center gap-1.5">
              <Check size={12} /> Merged {consolidate.data?.observations_updated ?? 0}{' '}
              observation(s) into{' '}
              <code className="font-mono">{consolidate.data?.canonical}</code>
            </p>
          )}
          {consolidate.error && (
            <p className="text-xs text-[#f85149]">
              {String(consolidate.error)}
            </p>
          )}
        </div>
      )}
    </>
  )
}

function ProposalCard({
  proposal,
  onMerge,
  pending,
}: {
  proposal: ConsolidationProposal
  onMerge: (canonical: string, sources: string[]) => void
  pending: boolean
}) {
  // El backend ya pone la variante con más observaciones como canonical
  // (sugerencia heurística). Permitimos al operador anularla con un select.
  const [canonical, setCanonical] = useState(proposal.canonical)
  const sources = proposal.variants
    .map(v => v.name)
    .filter(name => name !== canonical)

  return (
    <div className="rounded-lg border border-[#30363d] bg-[#0d1117] p-4 space-y-3">
      <div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-3 items-end">
        <div>
          <label className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
            Canonical name
          </label>
          <select
            value={canonical}
            onChange={e => setCanonical(e.target.value)}
            className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3]"
          >
            {proposal.variants.map(v => (
              <option key={v.name} value={v.name}>
                {v.name} ({v.observation_count} obs)
              </option>
            ))}
          </select>
        </div>
        <button
          onClick={() => onMerge(canonical, sources)}
          disabled={pending || sources.length === 0}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40 whitespace-nowrap"
        >
          <GitMerge size={12} /> {pending ? 'Merging…' : 'Merge into canonical'}
        </button>
      </div>
      <div>
        <p className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
          Sources (will be folded into canonical)
        </p>
        <div className="flex flex-wrap gap-1.5">
          {sources.length === 0 ? (
            <span className="text-[10px] text-[#484f58]">
              (no other variants — change canonical to merge)
            </span>
          ) : (
            sources.map(s => (
              <code
                key={s}
                className="font-mono text-[10px] bg-[#21262d] border border-[#30363d] rounded px-1.5 py-0.5 text-[#c9d1d9]"
              >
                {s}
              </code>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

// ── Prune ────────────────────────────────────────────────────────────────────

function PruneView() {
  const prune = usePruneProjects()
  const [confirmApply, setConfirmApply] = useState(false)

  function runDryRun() {
    setConfirmApply(false)
    prune.mutate({ apply: false })
  }
  function runApply() {
    prune.mutate({ apply: true }, { onSuccess: () => setConfirmApply(false) })
  }

  const data = prune.data
  const empty = data?.empty ?? []

  return (
    <>
      <p className="text-[10px] text-[#484f58]">
        Empty projects own sessions but zero observations. Prune drops the orphan
        sessions; observations are never touched.
      </p>
      <div className="flex items-center gap-2 flex-wrap">
        <button
          onClick={runDryRun}
          disabled={prune.isPending}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d] disabled:opacity-40"
        >
          <RefreshCw size={12} className={prune.isPending && !confirmApply ? 'animate-spin' : ''} />{' '}
          {prune.isPending && !confirmApply ? 'Scanning…' : 'Dry-run scan'}
        </button>
        {data && empty.length > 0 && !data.dry_run && (
          <span className="text-xs text-[#2ea043] inline-flex items-center gap-1.5">
            <Check size={12} /> Removed {data.sessions_removed} session(s)
          </span>
        )}
        {data && empty.length > 0 && data.dry_run && (
          <>
            {!confirmApply ? (
              <button
                onClick={() => setConfirmApply(true)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#f8514940] text-[#f85149] hover:bg-[#f8514920]"
              >
                <Trash2 size={12} /> Apply…
              </button>
            ) : (
              <>
                <span className="text-xs text-[#d29922] inline-flex items-center gap-1.5">
                  <AlertCircle size={12} /> This deletes {empty.length} project's
                  sessions. Sure?
                </span>
                <button
                  onClick={runApply}
                  disabled={prune.isPending}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#da3633] text-white hover:bg-[#f85149] disabled:opacity-40"
                >
                  <Trash2 size={12} />{' '}
                  {prune.isPending && confirmApply ? 'Pruning…' : 'Confirm apply'}
                </button>
                <button
                  onClick={() => setConfirmApply(false)}
                  className="px-3 py-1.5 rounded-md text-xs text-[#8b949e] hover:text-[#e6edf3]"
                >
                  Cancel
                </button>
              </>
            )}
          </>
        )}
      </div>
      {prune.error && (
        <p className="text-xs text-[#f85149]">{String(prune.error)}</p>
      )}
      {data && empty.length === 0 && (
        <EmptyState
          title="No empty projects"
          subtitle="Every project with sessions also has at least one observation."
        />
      )}
      {data && empty.length > 0 && (
        <div className="rounded-lg border border-[#30363d] overflow-hidden">
          <table className="w-full text-xs">
            <thead className="bg-[#161b22] text-[10px] uppercase tracking-wider text-[#8b949e]">
              <tr>
                <th className="text-left py-2 px-3">Project</th>
                <th className="text-right py-2 px-3">Sessions</th>
                <th className="text-right py-2 px-3">Prompts</th>
              </tr>
            </thead>
            <tbody className="bg-[#0d1117]">
              {empty.map(e => (
                <tr key={e.project} className="border-t border-[#21262d]">
                  <td className="py-2 px-3 text-[#e6edf3] font-mono">{e.project}</td>
                  <td className="py-2 px-3 text-right text-[#c9d1d9] font-mono">
                    {e.session_count}
                  </td>
                  <td className="py-2 px-3 text-right text-[#c9d1d9] font-mono">
                    {e.prompt_count}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

// ── Reusable bits ────────────────────────────────────────────────────────────

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-xs uppercase tracking-wider transition-colors ${
        active
          ? 'text-[#e6edf3] border-b-2 border-[#388bfd]'
          : 'text-[#8b949e] hover:text-[#e6edf3]'
      }`}
    >
      {children}
    </button>
  )
}

function EmptyState({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-6 text-center">
      <p className="text-sm text-[#8b949e]">{title}</p>
      <p className="text-xs text-[#484f58] mt-1">{subtitle}</p>
    </div>
  )
}
