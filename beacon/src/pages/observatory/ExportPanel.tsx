import { useState } from 'react'
import { Download, Check, AlertCircle, FileText } from 'lucide-react'
import { useExportObsidian } from '@/api/export'
import { useProjects } from '@/api/projects'

// Fase 6.2 — UI sobre `/admin/export/obsidian`. El operador elige una carpeta
// de salida (absoluta o relativa al cwd del servidor), opcionalmente acota por
// proyecto/tipo, y dispara la exportación. El backend escribe el árbol de
// archivos y devuelve un resumen con los contadores que usamos para confirmar
// éxito.

const OBSERVATION_TYPES = [
  'decision',
  'pattern',
  'bugfix',
  'learning',
  'context',
  'antipattern',
  'task',
  'feature',
  'refactor',
  'discovery',
  'incident',
] as const

export default function ExportPanel() {
  const [out, setOut] = useState('')
  const [project, setProject] = useState('')
  const [obsType, setObsType] = useState('')

  const projects = useProjects()
  const exporter = useExportObsidian()

  const ready = out.trim() !== ''

  function submit() {
    if (!ready || exporter.isPending) return
    exporter.mutate({
      out: out.trim(),
      project: project || undefined,
      type: obsType || undefined,
    })
  }

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-4">
      <header>
        <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
          <Download size={18} /> Obsidian export
        </h1>
        <p className="text-xs text-[#8b949e] mt-1">
          Render the vault as Obsidian-flavored markdown. Each observation becomes a
          note with YAML frontmatter and <code className="font-mono">[[wikilinks]]</code>{' '}
          to related observations. Re-running over the same directory is safe — notes
          are rewritten from the live store state.
        </p>
      </header>

      <div className="rounded-lg border border-[#30363d] bg-[#0d1117] p-4 space-y-4">
        <div>
          <label
            htmlFor="export-out"
            className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1"
          >
            Output directory *
          </label>
          <input
            id="export-out"
            type="text"
            value={out}
            onChange={e => setOut(e.target.value)}
            placeholder="/Users/me/vaults/korva-vault"
            className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3] font-mono"
          />
          <p className="text-[10px] text-[#484f58] mt-1">
            Path is resolved relative to the vault process. Create it under your
            Obsidian vaults folder so File → Open vault picks it up immediately.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="export-project"
              className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1"
            >
              Project (optional)
            </label>
            <select
              id="export-project"
              value={project}
              onChange={e => setProject(e.target.value)}
              className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3]"
            >
              <option value="">All projects</option>
              {(projects.data?.projects ?? []).map(p => (
                <option key={p.name} value={p.name}>
                  {p.name} ({p.observation_count})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label
              htmlFor="export-type"
              className="block text-[10px] uppercase tracking-wider text-[#8b949e] mb-1"
            >
              Type (optional)
            </label>
            <select
              id="export-type"
              value={obsType}
              onChange={e => setObsType(e.target.value)}
              className="w-full bg-[#161b22] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3]"
            >
              <option value="">All types</option>
              {OBSERVATION_TYPES.map(t => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={submit}
            disabled={!ready || exporter.isPending}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40"
          >
            <Download size={12} /> {exporter.isPending ? 'Exporting…' : 'Run export'}
          </button>
          {!ready && (
            <span className="text-[10px] text-[#484f58] inline-flex items-center gap-1.5">
              <AlertCircle size={11} /> An output directory is required
            </span>
          )}
        </div>
      </div>

      {exporter.error && (
        <div className="rounded-lg border border-[#f8514940] bg-[#f8514910] p-3">
          <p className="text-xs text-[#f85149]">
            <AlertCircle size={12} className="inline mr-1" /> {String(exporter.error)}
          </p>
        </div>
      )}

      {exporter.data && <ExportResultCard result={exporter.data} />}
    </div>
  )
}

function ExportResultCard({
  result,
}: {
  result: {
    out_dir: string
    file_count: number
    project_count: number
    by_type: Record<string, number>
    generated_at: string
  }
}) {
  const byType = Object.entries(result.by_type).sort((a, b) => b[1] - a[1])
  return (
    <div className="rounded-lg border border-[#2ea04340] bg-[#2ea04310] p-4 space-y-3">
      <h3 className="text-sm font-semibold text-[#2ea043] flex items-center gap-1.5">
        <Check size={14} /> Export written
      </h3>
      <dl className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
        <div>
          <dt className="text-[10px] uppercase tracking-wider text-[#8b949e]">Files</dt>
          <dd className="text-[#e6edf3] font-mono">{result.file_count}</dd>
        </div>
        <div>
          <dt className="text-[10px] uppercase tracking-wider text-[#8b949e]">
            Projects
          </dt>
          <dd className="text-[#e6edf3] font-mono">{result.project_count}</dd>
        </div>
        <div>
          <dt className="text-[10px] uppercase tracking-wider text-[#8b949e]">
            Generated
          </dt>
          <dd className="text-[#e6edf3] font-mono">
            {new Date(result.generated_at).toISOString().replace('T', ' ').slice(0, 19)}
          </dd>
        </div>
      </dl>
      <div>
        <p className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1">
          Output path
        </p>
        <code className="block font-mono text-[11px] text-[#c9d1d9] bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5">
          {result.out_dir}
        </code>
      </div>
      {byType.length > 0 && (
        <div>
          <p className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1.5">
            By type
          </p>
          <div className="flex flex-wrap gap-1.5">
            {byType.map(([t, n]) => (
              <span
                key={t}
                className="text-[10px] bg-[#21262d] border border-[#30363d] rounded px-1.5 py-0.5 text-[#c9d1d9]"
              >
                <FileText size={9} className="inline mr-1" />
                {t}: <span className="font-mono">{n}</span>
              </span>
            ))}
          </div>
        </div>
      )}
      <p className="text-[11px] text-[#8b949e]">
        Open in Obsidian via <em>File → Open vault</em> and pick the output folder.
      </p>
    </div>
  )
}
