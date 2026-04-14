import { Settings } from 'lucide-react'
import { useVaultStats } from '@/api/vault'

export default function SettingsPage() {
  const { data: stats } = useVaultStats()

  return (
    <div className="p-6 max-w-3xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Settings</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">Vault configuration and connection</p>
      </header>

      {/* Connection */}
      <section className="border border-[#21262d] rounded-lg p-4 mb-4 bg-[#161b22]">
        <h2 className="text-sm font-semibold text-[#e6edf3] mb-3 flex items-center gap-2">
          <Settings size={14} /> Vault Connection
        </h2>
        <div className="space-y-2 text-sm">
          <Row label="API URL" value="http://127.0.0.1:7437" />
          <Row label="Dashboard proxy" value="/vault-api → :7437" />
          {stats && <Row label="Total observations" value={String(stats.total_observations)} />}
        </div>
      </section>

      {/* CLI */}
      <section className="border border-[#21262d] rounded-lg p-4 bg-[#161b22]">
        <h2 className="text-sm font-semibold text-[#e6edf3] mb-3">Useful Commands</h2>
        <div className="space-y-2">
          {[
            ['korva status', 'Show vault and agent status'],
            ['korva doctor', 'Run health checks'],
            ['korva sync --profile', 'Update team profile'],
            ['korva lore list', 'List available scrolls'],
            ['korva sentinel install', 'Install pre-commit hooks'],
          ].map(([cmd, desc]) => (
            <div key={cmd} className="flex items-center gap-3 text-sm">
              <code className="text-[#79c0ff] w-44 flex-shrink-0">{cmd}</code>
              <span className="text-[#8b949e]">{desc}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  )
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-3">
      <span className="text-[#8b949e] w-36 flex-shrink-0">{label}</span>
      <code className="text-[#e6edf3] text-xs bg-[#21262d] px-2 py-0.5 rounded">{value}</code>
    </div>
  )
}
