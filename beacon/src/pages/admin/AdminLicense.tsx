import _React, { useState } from 'react'
import { ShieldCheck, ShieldOff, RefreshCw, KeyRound, ShieldX, AlertTriangle } from 'lucide-react'
import { useLicenseStatus, useLicenseActivate, useLicenseDeactivate } from '@/api/license'
import { PageHeader, InfoCallout } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

export default function AdminLicense() {
  const { data, isLoading, error, refetch } = useLicenseStatus()
  const activate = useLicenseActivate()
  const deactivate = useLicenseDeactivate()
  const [keyInput, setKeyInput] = useState('')
  const [msg, setMsg] = useState('')
  const [confirmDeactivate, setConfirmDeactivate] = useState(false)
  const { t } = useI18n()

  const handleActivate = async () => {
    if (!keyInput.trim()) return
    setMsg('')
    try {
      await activate.mutateAsync(keyInput.trim())
      setMsg(t.license.activateSuccess)
      setKeyInput('')
    } catch {
      setMsg(t.license.activateFailed)
    }
  }

  const handleDeactivate = async () => {
    setMsg('')
    try {
      await deactivate.mutateAsync()
      setMsg(t.license.deactivateSuccess)
      setConfirmDeactivate(false)
    } catch {
      setMsg(t.license.deactivateFailed)
      setConfirmDeactivate(false)
    }
  }

  if (isLoading) {
    return <PageShell t={t}><p className="text-[#8b949e] text-sm">{t.license.loading}</p></PageShell>
  }

  const isTeams = data?.tier === 'teams'
  const graceWarning = data && !data.grace_ok

  return (
    <PageShell t={t}>
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
            {t.license.graceWarning}
          </div>
        )}

        <div className="grid grid-cols-2 gap-3 text-xs">
          {data?.seats && (
            <Stat label={t.license.seatsLabel} value={String(data.seats)} />
          )}
          {data?.expires_at && (
            <Stat label={t.license.expiresLabel} value={data.expires_at.slice(0, 10)} />
          )}
          {data?.last_heartbeat && (
            <Stat label={t.license.lastCheckLabel} value={new Date(data.last_heartbeat).toLocaleString()} />
          )}
          {data?.grace_remaining_hours !== undefined && (
            <Stat label={t.license.graceRemainingLabel} value={`${data.grace_remaining_hours}h`} />
          )}
        </div>

        {isTeams && data.features.length > 0 && (
          <div className="mt-4">
            <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-2">{t.license.activeFeaturesLabel}</p>
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

      {/* Activate form — community tier only */}
      {!isTeams && (
        <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-5">
          <div className="flex items-center gap-2 mb-4">
            <KeyRound size={15} className="text-[#f0883e]" />
            <p className="text-[#e6edf3] text-sm font-medium">{t.license.activateTitle}</p>
          </div>
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              placeholder={t.license.activatePlaceholder}
              value={keyInput}
              onChange={e => setKeyInput(e.target.value)}
              className="flex-1 bg-[#0d1117] border border-[#30363d] rounded-md px-3 py-1.5 text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd] min-w-0"
            />
            <button
              onClick={handleActivate}
              disabled={activate.isPending || !keyInput.trim()}
              className="px-4 py-1.5 rounded-md text-sm bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors sm:flex-shrink-0"
            >
              {activate.isPending ? t.license.activating : t.license.activate}
            </button>
          </div>
          {msg && (
            <p className={`mt-2 text-xs ${msg === t.license.activateSuccess ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>
              {msg}
            </p>
          )}
          {error && (
            <p className="mt-2 text-xs text-[#f85149]">{t.license.loadError}</p>
          )}
        </div>
      )}

      {/* Deactivate — teams tier only */}
      {isTeams && (
        <div className="rounded-lg border border-[#f8514920] bg-[#f8514906] p-5 mt-2">
          <div className="flex items-start justify-between gap-4">
            <div className="flex items-start gap-2">
              <ShieldX size={15} className="text-[#f85149] mt-0.5 flex-shrink-0" />
              <div>
                <p className="text-[#e6edf3] text-sm font-medium">{t.license.deactivateTitle}</p>
                <p className="text-[10px] text-[#8b949e] mt-0.5 max-w-sm">{t.license.deactivateDesc}</p>
              </div>
            </div>
            <div className="flex-shrink-0">
              {!confirmDeactivate ? (
                <button
                  onClick={() => setConfirmDeactivate(true)}
                  className="px-3 py-1.5 rounded-md text-xs border border-[#f8514930] text-[#f85149] hover:bg-[#f8514915] transition-colors"
                >
                  {t.license.deactivate}
                </button>
              ) : (
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setConfirmDeactivate(false)}
                    className="px-3 py-1.5 rounded-md text-xs border border-[#30363d] text-[#8b949e] hover:bg-[#21262d] transition-colors"
                  >
                    {t.common.cancel}
                  </button>
                  <button
                    onClick={handleDeactivate}
                    disabled={deactivate.isPending}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#f85149] text-white hover:bg-[#da3633] disabled:opacity-50 transition-colors"
                  >
                    <AlertTriangle size={11} />
                    {deactivate.isPending ? t.common.saving : t.license.confirmDeactivate}
                  </button>
                </div>
              )}
            </div>
          </div>
          {msg && (
            <p className={`mt-3 text-xs ${msg === t.license.deactivateSuccess ? 'text-[#3fb950]' : 'text-[#f85149]'}`}>
              {msg}
            </p>
          )}
        </div>
      )}
    </PageShell>
  )
}

function PageShell({ children, t }: { children: React.ReactNode; t: ReturnType<typeof useI18n>['t'] }) {
  return (
    <div className="p-4 sm:p-6 max-w-2xl">
      <PageHeader
        icon={<KeyRound size={17} />}
        iconColor="#d29922"
        title={t.license.title}
        description={t.license.description}
      />
      <InfoCallout title={t.license.comparisonTitle} variant="tip" collapsible id="license-comparison">
        <div className="grid grid-cols-2 gap-3 mt-1">
          <div>
            <p className="font-medium text-[#8b949e] mb-0.5">{t.license.communityFree}</p>
            <ul className="space-y-0.5 text-[#484f58]">
              <li>{t.license.communityFeature1}</li>
              <li>{t.license.communityFeature2}</li>
              <li>{t.license.communityFeature3}</li>
            </ul>
          </div>
          <div>
            <p className="font-medium text-[#3fb950] mb-0.5">Teams</p>
            <ul className="space-y-0.5">
              <li>{t.license.teamsFeature1}</li>
              <li>{t.license.teamsFeature2}</li>
              <li>{t.license.teamsFeature3}</li>
            </ul>
          </div>
        </div>
      </InfoCallout>
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
