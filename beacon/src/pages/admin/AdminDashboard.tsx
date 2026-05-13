import { Brain, Clock, FolderGit2, Zap, Database, LayoutDashboard, ArrowRight, CheckCircle2, Circle, Terminal, Sparkles, DollarSign, TrendingUp } from 'lucide-react'
import { NavLink } from 'react-router'
import { useAdminStats, type SessionRow } from '@/api/admin'
import { useCostSummary } from '@/api/cost'
import { useI18n } from '@/contexts/i18n'
import { MetricCard, PageHero, Card, CardHeader, CardBody, EmptyState, ErrorBanner, Spinner, Badge } from '@/components/ui'
import { LineChart, DonutChart, BarChart, Sparkline, CHART_PALETTE } from '@/components/charts'

// Phase 8.7 — refreshed AdminDashboard hero.
//
// The first screen an operator sees after login. Designed to communicate at
// a glance: "Korva is healthy, here's what your team did, here's what it
// costs". Drops the legacy PageHeader + KpiCard pattern in favour of the
// Phase 7/8 design-system primitives (PageHero, MetricCard with sparkline,
// LineChart with hover, DonutChart by type, BarChart of top projects). The
// FirstRunChecklist, RecentSessions, and Teams blocks remain — they were
// already well-shaped.

const TYPE_COLOR: Record<string, string> = {
  decision: CHART_PALETTE.cyan,
  pattern: CHART_PALETTE.volt,
  bugfix: CHART_PALETTE.rose,
  learning: CHART_PALETTE.purple,
  context: CHART_PALETTE.amber,
  antipattern: CHART_PALETTE.rose,
  task: CHART_PALETTE.emerald,
  feature: CHART_PALETTE.coral,
  refactor: CHART_PALETTE.cyan,
  discovery: CHART_PALETTE.purple,
  incident: CHART_PALETTE.rose,
}

export default function AdminDashboard() {
  const { data: stats, isLoading, error } = useAdminStats()
  const cost = useCostSummary(30)
  const { t } = useI18n()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64 text-volt">
        <Spinner size={20} />
      </div>
    )
  }

  if (error) {
    const msg = error.message
    const hint = msg.includes('403') || msg.includes('401')
      ? 'Admin key rejected — check ~/.korva/admin.key and re-login.'
      : msg.includes('500') || msg.includes('502') || msg.includes('503') || msg.includes('Failed to fetch')
        ? 'Vault server unreachable — run korva-vault (or korva vault start) and reload.'
        : `Unexpected error: ${msg}`
    return (
      <div className="p-4 sm:p-6">
        <ErrorBanner title={t.dashboard.couldNotLoad} message={hint} />
      </div>
    )
  }

  if (!stats) return null

  const topProjects = Object.entries(stats.by_project)
    .map(([label, value]) => ({ label, value }))
    .sort((a, b) => b.value - a.value)
    .slice(0, 8)
  const typeDonut = Object.entries(stats.by_type)
    .map(([label, value]) => ({
      label,
      value,
      color: TYPE_COLOR[label] ?? CHART_PALETTE.indigo,
    }))
    .filter((d) => d.value > 0)
    .sort((a, b) => b.value - a.value)

  const dailyLine = stats.daily_activity && stats.daily_activity.length > 0
    ? {
        labels: stats.daily_activity.map((d) => d.date.slice(5)),
        series: [
          {
            name: t.dashboard.observations,
            color: CHART_PALETTE.cyan,
            data: stats.daily_activity.map((d) => d.count),
          },
        ],
        sparkData: stats.daily_activity.map((d) => d.count),
      }
    : null

  // Estimated context tokens — chars/4 approximation against stored content.
  const estTokens = Math.round((stats.total_content_len ?? 0) / 4)

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-5 animate-fade-up">
      <PageHero
        eyebrow="Mission control"
        icon={<LayoutDashboard size={22} />}
        title={t.dashboard.title}
        subtitle={t.dashboard.description}
        badge={{
          tone: 'success',
          label: (
            <span className="inline-flex items-center gap-1.5">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-volt animate-pulse" />
              {t.dashboard.hintLabel ?? 'live'}
            </span>
          ),
        }}
      />

      {/* First-run checklist — shown when the vault is empty */}
      {stats.total_observations === 0 && <FirstRunChecklist />}

      {/* Hero metric strip — observations / sessions / projects / tokens + cost */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <MetricCard
          label={t.dashboard.observations}
          value={Intl.NumberFormat().format(stats.total_observations)}
          tone="cyan"
          icon={<Brain size={14} />}
          sparkline={dailyLine?.sparkData ? <Sparkline data={dailyLine.sparkData} color="var(--color-cyan-400)" /> : null}
        />
        <MetricCard
          label={t.dashboard.sessions}
          value={Intl.NumberFormat().format(stats.total_sessions)}
          tone="volt"
          icon={<Clock size={14} />}
        />
        <MetricCard
          label={t.dashboard.activeProjects}
          value={Intl.NumberFormat().format(Object.keys(stats.by_project).length)}
          tone="coral"
          icon={<FolderGit2 size={14} />}
        />
        <MetricCard
          label={t.dashboard.contextTokens}
          value={Intl.NumberFormat().format(estTokens)}
          tone="purple"
          icon={<Zap size={14} />}
          hint={t.dashboard.contextTokensHint}
        />
      </div>

      {/* Cost mini-strip — only shows when we have any spend */}
      {cost.data && cost.data.total_usd > 0 && (
        <Card variant="glass" tone="volt">
          <CardBody className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <CostMicro
              icon={<DollarSign size={14} />}
              label="Spent (30d)"
              value={`$${cost.data.total_usd.toFixed(2)}`}
              tone="volt"
            />
            <CostMicro
              icon={<TrendingUp size={14} />}
              label="Savings"
              value={`$${cost.data.savings_usd.toFixed(2)}`}
              tone="coral"
            />
            <CostMicro
              icon={<Sparkles size={14} />}
              label="Cache hit"
              value={`${(cost.data.cache_hit_pct * 100).toFixed(0)}%`}
              tone="purple"
            />
            <NavLink
              to="/admin/observatory/cost"
              className="flex items-center justify-end gap-1.5 text-xs text-cyan-300 hover:text-cyan-200 transition-colors"
            >
              Open Cost & ROI <ArrowRight size={12} />
            </NavLink>
          </CardBody>
        </Card>
      )}

      {/* Activity line + by-type donut, side-by-side on desktop */}
      <div className="grid grid-cols-1 xl:grid-cols-[2fr_1fr] gap-4">
        <Card>
          <CardHeader title={t.dashboard.activityTitle} subtitle={`${stats.daily_activity?.reduce((s, d) => s + d.count, 0) ?? 0} ${t.dashboard.observations.toLowerCase()}`} icon={<TrendingUp size={14} />} />
          <CardBody>
            {dailyLine ? (
              <LineChart xLabels={dailyLine.labels} series={dailyLine.series} height={220} />
            ) : (
              <EmptyState
                tone="cyan"
                icon={<TrendingUp size={22} />}
                title="No activity yet"
                description="Save your first observation from any AI editor — it'll show up here."
                compact
              />
            )}
          </CardBody>
        </Card>

        <Card>
          <CardHeader title={t.dashboard.byType} icon={<Database size={14} />} />
          <CardBody>
            {typeDonut.length === 0 ? (
              <EmptyState tone="cyan" icon={<Database size={22} />} title={t.dashboard.noData} compact />
            ) : (
              <DonutChart
                data={typeDonut}
                centerLabel={t.dashboard.observations}
                centerValue={stats.total_observations}
                stroke={18}
                size={140}
              />
            )}
          </CardBody>
        </Card>
      </div>

      {/* Top projects + recent sessions, side-by-side */}
      <div className="grid grid-cols-1 xl:grid-cols-[1fr_1.5fr] gap-4">
        <Card>
          <CardHeader title={t.dashboard.topProjects} icon={<FolderGit2 size={14} />} />
          <CardBody>
            <BarChart
              data={topProjects}
              maxRows={8}
              emptyMessage={t.dashboard.noData}
            />
          </CardBody>
        </Card>

        {stats.recent_sessions && stats.recent_sessions.length > 0 ? (
          <RecentSessions sessions={stats.recent_sessions} />
        ) : (
          <Card>
            <CardHeader title={t.dashboard.recentSessions} icon={<Clock size={14} />} />
            <CardBody>
              <EmptyState
                tone="volt"
                icon={<Clock size={22} />}
                title="No sessions yet"
                description="Sessions appear once an MCP client opens a conversation with your vault."
                compact
              />
            </CardBody>
          </Card>
        )}
      </div>

      {/* Teams breakdown */}
      {Object.keys(stats.by_team).length > 0 && (
        <Card>
          <CardHeader title={t.dashboard.teams} icon={<Zap size={14} />} />
          <CardBody>
            <div className="flex flex-wrap gap-2">
              {Object.entries(stats.by_team).map(([team, count]) => (
                <Badge key={team} tone="cyan" mono>
                  {team} <span className="text-ink-400 ml-1">({count})</span>
                </Badge>
              ))}
            </div>
          </CardBody>
        </Card>
      )}
    </div>
  )
}

function CostMicro({ icon, label, value, tone }: { icon: React.ReactNode; label: string; value: string; tone: 'volt' | 'coral' | 'purple' }) {
  const color =
    tone === 'volt' ? 'text-volt' : tone === 'coral' ? 'text-coral' : 'text-purple-400'
  return (
    <div className="flex items-start gap-3">
      <span className={`mt-1 ${color}`} aria-hidden>{icon}</span>
      <div>
        <p className="text-[10px] uppercase tracking-wider text-ink-400">{label}</p>
        <p className={`font-mono font-700 text-xl ${color}`}>{value}</p>
      </div>
    </div>
  )
}

// ── Recent Sessions ───────────────────────────────────────────────────────────

function RecentSessions({ sessions }: { sessions: SessionRow[] }) {
  const { t } = useI18n()
  return (
    <Card>
      <CardHeader
        title={t.dashboard.recentSessions}
        icon={<Clock size={14} />}
        actions={
          <NavLink
            to="/admin/sessions"
            className="flex items-center gap-1 text-xs text-cyan-300 hover:text-cyan-200 transition-colors"
          >
            {t.dashboard.viewAll} <ArrowRight size={11} />
          </NavLink>
        }
      />
      <CardBody className="!p-0">
        <ul className="divide-y divide-white/5">
          {sessions.map((s) => (
            <li key={s.id} className="flex items-center gap-3 px-4 py-3 hover:bg-white/3 transition-colors">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-xs font-mono text-cyan-300 truncate max-w-[160px]">{s.project || '—'}</span>
                  {s.agent && (
                    <Badge tone="neutral" mono>
                      {s.agent}
                    </Badge>
                  )}
                </div>
                {s.goal && (
                  <p className="text-xs text-ink-400 truncate mt-0.5">{s.goal}</p>
                )}
              </div>
              <div className="flex items-center gap-3 shrink-0 text-right">
                <div className="text-xs text-ink-500">
                  <span className="text-ink-100 font-medium">{s.obs_count}</span> obs
                </div>
                {s.duration_min > 0 && (
                  <div className="text-xs text-ink-500">
                    {s.duration_min < 60 ? `${s.duration_min}m` : `${Math.round(s.duration_min / 60)}h`}
                  </div>
                )}
                <div className="text-[10px] font-mono text-ink-500 tabular-nums">
                  {formatRelative(s.started_at)}
                </div>
              </div>
            </li>
          ))}
        </ul>
      </CardBody>
    </Card>
  )
}

// ── First-run checklist ───────────────────────────────────────────────────────

const CHECKLIST_STEPS = [
  {
    id: 'vault',
    label: 'Vault is running',
    hint: 'You\'re here — vault is up.',
    done: true,
    code: null,
  },
  {
    id: 'mcp',
    label: 'Connect an MCP client',
    hint: 'Add the vault MCP server to Claude Code, Cursor, or Copilot.',
    done: false,
    code: 'korva setup mcp',
  },
  {
    id: 'init',
    label: 'Initialize a project',
    hint: 'Run in your project root to create korva.config.json.',
    done: false,
    code: 'korva init',
  },
  {
    id: 'scroll',
    label: 'Add a Scroll (optional)',
    hint: 'Scrolls inject knowledge into every AI context window.',
    done: false,
    code: 'korva scrolls add forge-sdd',
  },
  {
    id: 'save',
    label: 'Save your first observation',
    hint: 'Ask your AI assistant to call vault_save — it will appear here.',
    done: false,
    code: null,
  },
]

function FirstRunChecklist() {
  return (
    <div className="rounded-xl border border-[#388bfd30] bg-[#388bfd08] p-5">
      <div className="flex items-center gap-2 mb-4">
        <Sparkles size={15} className="text-[#388bfd]" />
        <h3 className="text-sm font-medium text-[#e6edf3]">Getting started</h3>
        <span className="ml-auto text-[10px] text-[#484f58]">No observations yet</span>
      </div>
      <div className="space-y-3">
        {CHECKLIST_STEPS.map((step) => (
          <div key={step.id} className="flex items-start gap-3">
            <div className="mt-0.5 flex-shrink-0">
              {step.done
                ? <CheckCircle2 size={15} className="text-[#3fb950]" />
                : <Circle size={15} className="text-[#30363d]" />}
            </div>
            <div className="flex-1 min-w-0">
              <p className={`text-sm ${step.done ? 'text-[#484f58] line-through' : 'text-[#e6edf3]'}`}>
                {step.label}
              </p>
              <p className="text-[11px] text-[#8b949e] mt-0.5">{step.hint}</p>
              {step.code && !step.done && (
                <div className="mt-1.5 flex items-center gap-1.5 bg-[#0d1117] rounded-md px-2.5 py-1.5 w-fit border border-[#21262d]">
                  <Terminal size={11} className="text-[#484f58] flex-shrink-0" />
                  <code className="text-xs font-mono text-[#58a6ff]">{step.code}</code>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}
