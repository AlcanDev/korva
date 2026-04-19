import _React, { useState } from 'react'
import { ShieldCheck, ShieldOff, RefreshCw, KeyRound } from 'lucide-react'
import { useLicenseStatus } from '@/api/license'

export default function AdminLicense() {
  const { data, isLoading, error, refetch } = useLicenseStatus()
  const [keyInput, setKeyInput] = useState('')
  const [activating, setActivating] = useState(false)
  const [msg, setMsg] = useState('')

  const handleActivate = async () => {
    if (!keyInput.trim()) return
    setActivating(true)
    setMsg('')
    try {
      const res = await fetch('/vault-api/admin/license/activate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ license_key: keyInput.trim() }),
      })
      if (res.ok) {
        setMsg('License activated.')
        setKeyInput('')
        refetch()
      } else {
        const d = await res.json()
        setMsg(d.error ?? 'Activation failed.')
      }
    } catch {
      setMsg('Network error.')
    } finally {
      setActivating(false)
    }
  }

  if (isLoading) {
    return <PageShell><p className="text-[#8b949e] text-sm">Loading license status…</p></PageShell>
  }

  const isTeams = data?.tier === 'teams'
  const graceWarning = data && !data.grace_ok

  return (
    <PageShell>
      {/* Status card */}
      <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-5 mb-6">
        <div className="flex items-center gap-3 mb-4">
          {isTeams
            ? <ShieldCheck size={20} className="text-[#3fb950]" />
            : <ShieldOff size={20} className="text-[#8b949e]" />
          }
          <div>
            <p className="text-[#e6edf3] font-semibold capitalize">{data?.tier ?? 'community'} tier</p>
            {data?.license_id && (
              <p className="text-[10px] text-[#8b949e] font-mono">{data.license_id}</p>
            )}
          </div>
          <button
            onClick={() => refetch()}
            className="ml-auto text-[#8b949e] hover:text-[#e6edf3] transition-colors"
          >
            <RefreshCw size={14} />
          </button>
        </div>

        {graceWarning && (
          <div className="mb-4 rounded border border-[#d29922] bg-[#d2992215] p-3 text-xs text-[#d29922]">
            Grace period lapsed — running as Community tier. Connect to the licensing server to restore Teams access.
          </div>
        )}

        <div className="grid grid-cols-2 gap-3 text-xs">
          {data?.seats && (
            <Stat label="Seats" value={String(data.seats)} />
          )}
          {data?.expires_at && (
            <Stat label="Expires" value={data.expires_at.slice(0, 10)} />
          )}
          {data?.last_heartbeat && (
            <Stat label="Last check" value={new Date(data.last_heartbeat).toLocaleString()} />
          )}
          {data?.grace_remaining_hours !== undefined && (
            <Stat label="Grace remaining" value={`${data.grace_remaining_hours}h`} />
          )}
        </div>

        {isTeams && data.features.length > 0 && (
          <div className="mt-4">
            <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-2">Active features</p>
            <div className="flex flex-wrap gap-1.5">
              {data.features.map(f => (
                <span key={f} className="px-2 py-0.5 rounded-full text-[10px] bg-[#388bfd20] text-[#388bfd] border border-[#388bfd30]">
                  {f}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Activate form */}
      {!isTeams && (
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-5">
          <div className="flex items-center gap-2 mb-4">
            <KeyRound size={15} className="text-[#f0883e]" />
            <p className="text-[#e6edf3] text-sm font-medium">Activate Korva for Teams</p>
          </div>
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              placeholder="KORVA-XXXX-XXXX-XXXX"
              value={keyInput}
              onChange={e => setKeyInput(e.target.value)}
              className="flex-1 bg-[#0d1117] border border-[#30363d] rounded-md px-3 py-1.5 text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd] min-w-0"
            />
            <button
              onClick={handleActivate}
              disabled={activating || !keyInput.trim()}
              className="px-4 py-1.5 rounded-md text-sm bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors sm:flex-shrink-0"
            >
              {activating ? 'Activating…' : 'Activate'}
            </button>
          </div>
          {msg && (
            <p className={`mt-2 text-xs ${msg.includes('activated') ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>
              {msg}
            </p>
          )}
          {error && (
            <p className="mt-2 text-xs text-[#f85149]">Could not load license status.</p>
          )}
        </div>
      )}
    </PageShell>
  )
}

function PageShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="p-4 sm:p-6 max-w-2xl">
      <h1 className="text-[#e6edf3] text-lg font-semibold mb-5">License</h1>
      {children}
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[#484f58] text-[10px] uppercase tracking-wider">{label}</p>
      <p className="text-[#e6edf3] text-xs mt-0.5">{value}</p>
    </div>
  )
}
