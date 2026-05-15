import { Layers } from 'lucide-react'
import { useEditorAdoption, type EditorAdoptionPayload } from '@/api/editorAdoption'

// Phase 18.D — Editor adoption widget.
//
// Shows which AI editors are calling the vault, broken down by share
// of total interactions in the last N days (default 7). Empty
// editor (callers that didn't send X-Korva-Editor) is bucketed as
// "anonymous" so operators can see opt-in coverage at a glance.
//
// Renders nothing useful when total=0 (fresh deploy, no traffic yet)
// — we still mount the card so the dashboard layout doesn't jump
// once data lands.

interface EditorAdoptionWidgetProps {
  windowDays?: number
}

// Stable color palette per editor — picks the same hue every render
// so visual scanning works.
const editorColor: Record<string, string> = {
  claude: '#f0883e',
  cursor: '#1f6feb',
  windsurf: '#3fb950',
  continue: '#d29922',
  copilot: '#a371f7',
  aider: '#79c0ff',
  codex: '#56d364',
}
const anonymousColor = '#30363d'

export function EditorAdoptionWidget({ windowDays = 7 }: EditorAdoptionWidgetProps) {
  const { data, isLoading, isError } = useEditorAdoption(windowDays)

  return (
    <section
      aria-label="Editor adoption"
      className="border border-[#21262d] rounded-lg p-4 bg-[#161b22]"
    >
      <header className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 text-[#8b949e]">
          <Layers size={14} />
          <h2 className="text-xs font-medium uppercase tracking-wider">Editor adoption</h2>
        </div>
        <span className="text-[11px] text-[#6e7681] font-mono">last {windowDays}d</span>
      </header>

      {isLoading && <SkeletonRows />}
      {isError && (
        <p className="text-xs text-[#8b949e]">
          Could not load adoption data — check that the vault is reachable.
        </p>
      )}
      {!isLoading && !isError && data && <AdoptionList payload={data} />}
    </section>
  )
}

interface AdoptionListProps {
  payload: EditorAdoptionPayload
}

function AdoptionList({ payload }: AdoptionListProps) {
  if (payload.total === 0) {
    return (
      <p className="text-xs text-[#6e7681]">
        No traffic yet. Set <code className="text-[#58a6ff]">X-Korva-Editor</code> in your
        editor's MCP config to start tracking adoption.
      </p>
    )
  }

  // Phase 19.D — total spans both telemetry channels. Headline
  // wording stays "interactions" (the user-facing label) but the
  // per-row hover surfaces the http/mcp split when both are
  // non-zero, so an operator can spot "this editor only shows up
  // via MCP" at a glance.
  return (
    <>
      <p className="text-xs text-[#8b949e] mb-3 tabular-nums">
        <span className="text-[#e6edf3] font-semibold">{payload.total}</span> interactions
      </p>
      <ul className="space-y-2">
        {payload.rows.map((row) => {
          const pct = (row.count / payload.total) * 100
          const label = row.editor || 'anonymous'
          const color = row.editor ? editorColor[row.editor] ?? '#8b949e' : anonymousColor
          const breakdown = formatChannelBreakdown(row.by_channel)
          return (
            <li key={label} className="text-xs">
              <div className="flex items-center justify-between mb-1">
                <span
                  className="text-[#e6edf3] capitalize"
                  title={breakdown}
                  aria-label={`${label}, ${row.count} interactions (${breakdown})`}
                >
                  {label}
                </span>
                <span className="text-[#8b949e] tabular-nums" title={breakdown}>
                  {row.count} · {pct.toFixed(1)}%
                </span>
              </div>
              <div className="h-1.5 bg-[#0d1117] rounded-full overflow-hidden">
                <div
                  className="h-full rounded-full transition-all"
                  style={{ width: `${pct}%`, background: color }}
                  aria-hidden
                />
              </div>
            </li>
          )
        })}
      </ul>
    </>
  )
}

// formatChannelBreakdown returns a compact label like "http 5 · mcp 10"
// for the tooltip. When the row has only one channel populated we
// shorten further to "mcp only" / "http only" so the badge isn't
// noisy for the common single-channel case.
function formatChannelBreakdown(by: { http: number; mcp: number } | undefined): string {
  // Defensive: older payloads (before 19.D) might not carry the field.
  if (!by) return ''
  if (by.http === 0 && by.mcp === 0) return ''
  if (by.http === 0) return 'mcp only'
  if (by.mcp === 0) return 'http only'
  return `http ${by.http} · mcp ${by.mcp}`
}

function SkeletonRows() {
  return (
    <div className="space-y-2" aria-hidden>
      {[0, 1, 2].map((i) => (
        <div key={i} className="h-3 bg-[#21262d] rounded animate-pulse" />
      ))}
    </div>
  )
}
