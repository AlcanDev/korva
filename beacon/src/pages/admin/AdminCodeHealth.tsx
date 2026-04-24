import _React from 'react'
import {
  Activity, CheckCircle2, XCircle, AlertCircle, Clock,
  TrendingUp, Bug, Lightbulb, GitBranch, RefreshCw,
} from 'lucide-react'
import { useCodeHealth } from '@/api/codeHealth'
import type { CodeHealthProject, RecentCheckpoint } from '@/api/codeHealth'
import { PageHeader, InfoCallout } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

const SDD_PHASES = ['explore', 'propose', 'spec', 'design', 'tasks', 'apply', 'verify', 'archive']

export default function AdminCodeHealth() {
  const { data, isLoading, error, refetch } = useCodeHealth()
  const { t } = useI18n()

  return (
    <div className="p-4 sm:p-6 max-w-5xl space-y-6">
      <PageHeader
        icon={<Activity size={17} />}
        iconColor="#3fb950"
        title={t.codeHealth.title}
        description={t.codeHealth.description}
        hint={{ command: 'korva quality check --project <name>', label: t.codeHealth.hintLabel }}
        actions={
          <button
            onClick={() => refetch()}
            className="text-[#8b949e] hover:text-[#e6edf3] transition-colors p-1 rounded hover:bg-[#21262d]"
            title={t.common.refresh}
          >
            <RefreshCw size={14} />
          </button>
        }
      />

      {!isLoading && !error && data?.project_count === 0 && (
        <InfoCallout icon={<Lightbulb size={12} />} title={t.codeHealth.noDataTitle} variant="tip" collapsible id="code-health-nodata">
          <p>{t.codeHealth.noDataBody}</p>
          <p className="mt-1">{t.codeHealth.noDataHint}</p>
        </InfoCallout>
      )}

      {error && (
        <div className="rounded-lg border border-[#f85149] bg-[#f8514910] p-4">
          <p className="text-[#f85149] text-xs">{t.codeHealth.errorLoad}</p>
        </div>
      )}

      {isLoading && (
        <div className="flex items-center justify-center h-32">
          <div className="animate-spin rounded-full h-7 w-7 border-t-2 border-[#3fb950]" />
        </div>
      )}

      {data && data.project_count > 0 && (
        <>
          {/* Project health cards */}
          <div>
            <h2 className="text-sm font-medium text-[#8b949e] uppercase tracking-wider mb-3">{t.codeHealth.projectsHeader}</h2>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
              {data.projects.map(p => (
                <ProjectCard key={p.project} project={p} />
              ))}
            </div>
          </div>

          {/* Recent checkpoints */}
          {data.recent.length > 0 && (
            <div>
              <h2 className="text-sm font-medium text-[#8b949e] uppercase tracking-wider mb-3">{t.codeHealth.recentHeader}</h2>
              <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-[#21262d]">
                      <th className="text-left px-4 py-2 text-[10px] text-[#484f58] uppercase tracking-wider font-medium">{t.codeHealth.colProject}</th>
                      <th className="text-left px-4 py-2 text-[10px] text-[#484f58] uppercase tracking-wider font-medium">{t.codeHealth.colPhase}</th>
                      <th className="text-center px-4 py-2 text-[10px] text-[#484f58] uppercase tracking-wider font-medium">{t.codeHealth.colScore}</th>
                      <th className="text-center px-4 py-2 text-[10px] text-[#484f58] uppercase tracking-wider font-medium">{t.codeHealth.colGate}</th>
                      <th className="text-right px-4 py-2 text-[10px] text-[#484f58] uppercase tracking-wider font-medium">{t.codeHealth.colDate}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#21262d]">
                    {data.recent.map(cp => (
                      <CheckpointRow key={cp.id} checkpoint={cp} />
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function ProjectCard({ project: p }: { project: CodeHealthProject }) {
  const { t } = useI18n()
  const phaseIdx = SDD_PHASES.indexOf(p.sdd_phase)
  const phaseProgress = phaseIdx >= 0 ? ((phaseIdx + 1) / SDD_PHASES.length) * 100 : 0

  const scoreColor =
    p.avg_score >= 80 ? '#3fb950' :
    p.avg_score >= 60 ? '#d29922' : '#f85149'

  const statusIcon = p.last_status === 'pass'
    ? <CheckCircle2 size={12} className="text-[#3fb950]" />
    : p.last_status === 'fail'
      ? <XCircle size={12} className="text-[#f85149]" />
      : <AlertCircle size={12} className="text-[#d29922]" />

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-4 hover:border-[#30363d] transition-colors">
      {/* Header */}
      <div className="flex items-start justify-between mb-3">
        <div>
          <p className="text-[#e6edf3] text-sm font-medium font-mono truncate max-w-[180px]">{p.project}</p>
          <div className="flex items-center gap-1.5 mt-0.5">
            {statusIcon}
            <span className="text-[10px] text-[#8b949e] capitalize">{p.last_status || t.codeHealth.noReview}</span>
            {p.last_phase && (
              <span className="text-[10px] text-[#484f58]">· {p.last_phase} phase</span>
            )}
          </div>
        </div>
        <div className="text-center">
          <p className="text-2xl font-bold leading-none" style={{ color: scoreColor }}>
            {p.avg_score > 0 ? Math.round(p.avg_score) : '—'}
          </p>
          <p className="text-[9px] text-[#484f58] uppercase tracking-wider mt-0.5">{t.codeHealth.avgScore}</p>
        </div>
      </div>

      {/* SDD phase progress */}
      {p.sdd_phase && (
        <div className="mb-3">
          <div className="flex items-center justify-between mb-1">
            <div className="flex items-center gap-1">
              <GitBranch size={10} className="text-[#484f58]" />
              <span className="text-[10px] text-[#8b949e] capitalize">SDD: {p.sdd_phase}</span>
            </div>
            <span className="text-[10px] text-[#484f58]">{phaseIdx + 1}/{SDD_PHASES.length}</span>
          </div>
          <div className="h-1 bg-[#21262d] rounded-full overflow-hidden">
            <div
              className="h-full rounded-full transition-all"
              style={{ width: `${phaseProgress}%`, background: scoreColor }}
            />
          </div>
        </div>
      )}

      {/* Stats row */}
      <div className="flex items-center gap-4 text-[10px] text-[#484f58]">
        <span className="flex items-center gap-1">
          <TrendingUp size={10} />
          {t.codeHealth.checkpoints(p.total_checkpoints)}
        </span>
        {p.bugfix_count > 0 && (
          <span className="flex items-center gap-1 text-[#f85149]">
            <Bug size={10} />
            {t.codeHealth.bugs(p.bugfix_count)}
          </span>
        )}
        {p.pattern_count > 0 && (
          <span className="flex items-center gap-1 text-[#3fb950]">
            <Lightbulb size={10} />
            {t.codeHealth.patterns(p.pattern_count)}
          </span>
        )}
        {p.last_checked_at && (
          <span className="flex items-center gap-1 ml-auto">
            <Clock size={10} />
            {fmtDate(p.last_checked_at)}
          </span>
        )}
      </div>
    </div>
  )
}

function CheckpointRow({ checkpoint: cp }: { checkpoint: RecentCheckpoint }) {
  const scoreColor =
    cp.score >= 80 ? 'text-[#3fb950]' :
    cp.score >= 60 ? 'text-[#d29922]' : 'text-[#f85149]'

  return (
    <tr className="hover:bg-[#1c2128] transition-colors">
      <td className="px-4 py-2.5">
        <span className="font-mono text-[#e6edf3] truncate max-w-[140px] block">{cp.project}</span>
      </td>
      <td className="px-4 py-2.5">
        <span className="capitalize text-[#8b949e]">{cp.phase}</span>
      </td>
      <td className="px-4 py-2.5 text-center">
        <span className={`font-bold ${scoreColor}`}>{cp.score}</span>
      </td>
      <td className="px-4 py-2.5 text-center">
        {cp.gate_passed
          ? <CheckCircle2 size={12} className="text-[#3fb950] mx-auto" />
          : <XCircle size={12} className="text-[#f85149] mx-auto" />
        }
      </td>
      <td className="px-4 py-2.5 text-right">
        <span className="text-[#484f58] flex items-center justify-end gap-1">
          <Clock size={9} />
          {fmtDate(cp.created_at)}
        </span>
      </td>
    </tr>
  )
}

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit',
    })
  } catch {
    return iso
  }
}
