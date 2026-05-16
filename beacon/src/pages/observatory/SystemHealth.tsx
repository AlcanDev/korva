import { useSystemStatus, useRestartVault, IDE } from '@/api/observatory'
import {
  Activity, Database, Cloud, Shield, BookOpen, Wand2,
  Clock, Box, FileText, KeyRound, RefreshCw, Server,
} from 'lucide-react'
import { StatusCard, StatusRow } from './components/StatusCard'

export default function SystemHealth() {
  const { data, isLoading, error, refetch } = useSystemStatus()
  const restart = useRestartVault()

  if (isLoading) {
    return (
      <div className="p-6 text-[#8b949e] text-sm">Loading system status…</div>
    )
  }
  if (error || !data) {
    return (
      <div className="p-6">
        <p className="text-[#f85149] text-sm">Could not load system status: {String(error)}</p>
        <button
          onClick={() => refetch()}
          className="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d]"
        >
          <RefreshCw size={12} /> Retry
        </button>
      </div>
    )
  }

  const hiveStatus = data.hive.enabled
    ? data.hive.consecutive_errors > 0
      ? 'warning'
      : 'ok'
    : 'disabled'

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[#e6edf3]">System Health</h1>
          <p className="text-xs text-[#8b949e] mt-1">
            Korva instance overview · auto-refresh every 15s
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d]"
          >
            <RefreshCw size={12} /> Refresh
          </button>
          <button
            onClick={() => {
              if (confirm('Restart the Vault server? In-flight requests may be interrupted.')) {
                restart.mutate()
              }
            }}
            disabled={restart.isPending}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#388bfd20] border border-[#388bfd40] text-[#388bfd] hover:bg-[#388bfd30] disabled:opacity-50"
          >
            <Server size={12} /> {restart.isPending ? 'Restarting…' : 'Restart Vault'}
          </button>
        </div>
      </header>

      <section className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <StatusCard
          title="Vault"
          icon={<Database size={14} />}
          status={data.vault.running ? 'ok' : 'error'}
        >
          <StatusRow label="Port" value={data.vault.port} mono />
          <StatusRow label="PID" value={data.vault.pid} mono />
          <StatusRow label="Uptime" value={formatDuration(data.vault.uptime_sec)} />
          <StatusRow label="Version" value={data.vault.version || '—'} mono />
        </StatusCard>

        <StatusCard
          title="Hive (cloud)"
          icon={<Cloud size={14} />}
          status={hiveStatus}
        >
          <StatusRow label="Enabled" value={data.hive.enabled ? 'yes' : 'no'} />
          {data.hive.endpoint && <StatusRow label="Endpoint" value={data.hive.endpoint} mono />}
          <StatusRow label="Pending outbox" value={data.hive.pending_outbox} mono />
          <StatusRow label="Consecutive errors" value={data.hive.consecutive_errors} mono />
          {data.hive.last_sync_at && (
            <StatusRow label="Last sync" value={timeAgo(data.hive.last_sync_at)} />
          )}
        </StatusCard>

        <StatusCard
          title="Sentinel"
          icon={<Shield size={14} />}
          status={data.sentinel.enabled ? 'ok' : 'disabled'}
        >
          <StatusRow label="Enabled" value={data.sentinel.enabled ? 'yes' : 'no'} />
          <StatusRow label="Profile" value={data.sentinel.profile} mono />
          <StatusRow label="Built-in rules" value={data.sentinel.builtin_count} mono />
          <StatusRow label="Custom rules" value={data.sentinel.custom_count} mono />
          <StatusRow
            label="Hooks installed"
            value={(data.sentinel.hooks_installed ?? []).join(', ') || '—'}
            mono
          />
        </StatusCard>

        <StatusCard title="Lore" icon={<BookOpen size={14} />} status="info">
          <StatusRow
            label="Active scrolls"
            value={(data.lore.active_scrolls ?? []).join(', ') || '—'}
            mono
          />
          <StatusRow label="Available" value={data.lore.available_scrolls_count} mono />
        </StatusCard>

        <StatusCard title="Skills" icon={<Wand2 size={14} />} status="info">
          <StatusRow label="Installed" value={data.skills.installed_count} mono />
          {data.skills.last_sync_at && (
            <StatusRow label="Last sync" value={timeAgo(data.skills.last_sync_at)} />
          )}
        </StatusCard>

        <StatusCard
          title="License"
          icon={<KeyRound size={14} />}
          status={data.license.tier === 'community' ? 'info' : 'ok'}
        >
          <StatusRow label="Tier" value={data.license.tier} />
          {data.license.expiration_at && (
            <StatusRow label="Expires" value={fmtDate(data.license.expiration_at)} />
          )}
          {data.license.seats_total > 0 && (
            <StatusRow
              label="Seats"
              value={`${data.license.seats_used} / ${data.license.seats_total}`}
              mono
            />
          )}
        </StatusCard>

        <StatusCard title="Sessions" icon={<Clock size={14} />} status="info">
          <StatusRow label="Total" value={data.sessions.total} mono />
          <StatusRow label="Active (24h)" value={data.sessions.active_24h} mono />
        </StatusCard>

        <StatusCard title="Observations" icon={<Box size={14} />} status="info">
          <StatusRow label="Total" value={data.observations.total} mono />
          {Object.entries(data.observations.by_type)
            .sort(([, a], [, b]) => b - a)
            .slice(0, 4)
            .map(([type, count]) => (
              <StatusRow key={type} label={type} value={count} mono />
            ))}
        </StatusCard>

        <StatusCard title="Prompts" icon={<FileText size={14} />} status="info">
          <StatusRow label="Saved" value={data.prompts.total} mono />
        </StatusCard>
      </section>

      <section>
        <h2 className="text-sm font-semibold text-[#e6edf3] mb-3 flex items-center gap-2">
          <Activity size={14} /> Detected IDEs
        </h2>
        {data.ide.length === 0 ? (
          <p className="text-xs text-[#8b949e]">
            No IDEs detected on this machine. Korva will still work via the CLI; configure your editor for richer integration.
          </p>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {data.ide.map((ide) => (
              <IDECard key={ide.name} ide={ide} />
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function IDECard({ ide }: { ide: IDE }) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-3 flex items-start justify-between gap-3">
      <div className="min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <h4 className="text-sm font-medium text-[#e6edf3]">{ide.name}</h4>
          {ide.is_default && (
            <span className="text-[9px] uppercase tracking-wider px-1.5 py-0.5 rounded bg-[#388bfd20] text-[#388bfd]">
              Primary
            </span>
          )}
        </div>
        {ide.config_path && (
          <p className="text-[10px] text-[#8b949e] font-mono truncate">{ide.config_path}</p>
        )}
        {ide.version && <p className="text-[10px] text-[#8b949e]">v{ide.version}</p>}
      </div>
      <span
        className={`text-[10px] px-2 py-0.5 rounded-full border font-medium ${
          ide.has_korva_mcp
            ? 'bg-[#2ea04320] text-[#2ea043] border-[#2ea04340]'
            : 'bg-[#21262d] text-[#484f58] border-[#30363d]'
        }`}
      >
        {ide.has_korva_mcp ? 'MCP wired' : 'Not wired'}
      </span>
    </div>
  )
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const m = Math.floor(seconds / 60)
  if (m < 60) return `${m}m`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ${m % 60}m`
  const d = Math.floor(h / 24)
  return `${d}d ${h % 24}h`
}

function timeAgo(iso: string): string {
  const then = new Date(iso).getTime()
  const diff = Math.max(0, Date.now() - then) / 1000
  if (diff < 60) return `${Math.round(diff)}s ago`
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`
  return `${Math.round(diff / 86400)}d ago`
}

function fmtDate(iso: string): string {
  try {
    return new Date(iso).toISOString().slice(0, 10)
  } catch {
    return iso
  }
}
