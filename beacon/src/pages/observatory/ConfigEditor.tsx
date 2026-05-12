import { useEffect, useState } from 'react'
import { useConfig, useUpdateConfig, KorvaConfig } from '@/api/observatory'
import { Save, AlertTriangle, RefreshCw, Settings as SettingsIcon } from 'lucide-react'

type Scope = 'local' | 'global'

const TABS = ['general', 'vault', 'lore', 'sentinel', 'hive', 'license'] as const
type Tab = typeof TABS[number]

export default function ConfigEditor() {
  const [scope, setScope] = useState<Scope>('local')
  const [tab, setTab] = useState<Tab>('general')
  const { data, isLoading, error, refetch } = useConfig(scope)
  const update = useUpdateConfig()

  const [draft, setDraft] = useState<KorvaConfig | null>(null)

  useEffect(() => {
    if (data?.config) setDraft(structuredClone(data.config))
  }, [data])

  if (isLoading) return <div className="p-6 text-[#8b949e] text-sm">Loading config…</div>
  if (error || !data) {
    return <div className="p-6 text-[#f85149] text-sm">Could not load config: {String(error)}</div>
  }
  if (!draft) return null

  const dirty = JSON.stringify(draft) !== JSON.stringify(data.config)

  function handleSave() {
    if (!data) return
    update.mutate(
      { scope, expected_hash: data.hash, config: draft! },
      {
        onSuccess: () => refetch(),
      },
    )
  }

  return (
    <div className="p-6 max-w-5xl mx-auto space-y-4">
      <header className="flex items-center justify-between flex-wrap gap-2">
        <div>
          <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
            <SettingsIcon size={18} /> Configuration
          </h1>
          <p className="text-xs text-[#8b949e] mt-1 font-mono truncate">{data.path}</p>
        </div>
        <div className="flex items-center gap-2">
          <ScopeSwitch value={scope} onChange={setScope} />
          <button
            disabled={!dirty || update.isPending}
            onClick={handleSave}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40"
          >
            <Save size={12} /> {update.isPending ? 'Saving…' : 'Save'}
          </button>
          <button
            onClick={() => setDraft(structuredClone(data.config))}
            disabled={!dirty}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d] disabled:opacity-40"
          >
            <RefreshCw size={12} /> Reset
          </button>
        </div>
      </header>

      {update.data?.restart_required && update.data.restart_required.length > 0 && (
        <div className="rounded-md border border-[#d29922] bg-[#d2992220] p-3 flex items-start gap-2">
          <AlertTriangle size={14} className="text-[#d29922] mt-0.5" />
          <div className="text-xs">
            <p className="text-[#d29922] font-medium mb-0.5">Restart required</p>
            <p className="text-[#c9d1d9]">
              Changes to{' '}
              <code className="font-mono">{update.data.restart_required.join(', ')}</code>{' '}
              take effect after a Vault restart (System Health → Restart Vault).
            </p>
          </div>
        </div>
      )}

      {update.error && (
        <div className="rounded-md border border-[#f85149] bg-[#f8514920] p-3 text-xs text-[#f85149]">
          {String(update.error)}
        </div>
      )}

      <div className="flex border-b border-[#30363d]">
        {TABS.map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-xs uppercase tracking-wider transition-colors ${
              tab === t
                ? 'text-[#e6edf3] border-b-2 border-[#388bfd]'
                : 'text-[#8b949e] hover:text-[#e6edf3]'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
        {tab === 'general' && <GeneralFields cfg={draft} onChange={setDraft} />}
        {tab === 'vault' && <VaultFields cfg={draft} onChange={setDraft} />}
        {tab === 'lore' && <LoreFields cfg={draft} onChange={setDraft} />}
        {tab === 'sentinel' && <SentinelFields cfg={draft} onChange={setDraft} />}
        {tab === 'hive' && <HiveFields cfg={draft} onChange={setDraft} />}
        {tab === 'license' && <LicenseFields cfg={draft} onChange={setDraft} />}
      </div>
    </div>
  )
}

// ── Subcomponents per tab ─────────────────────────────────────────────────────

interface FieldsProps {
  cfg: KorvaConfig
  onChange: (cfg: KorvaConfig) => void
}

function GeneralFields({ cfg, onChange }: FieldsProps) {
  return (
    <div className="grid grid-cols-2 gap-4">
      <TextField label="Project" value={cfg.project} onChange={(v) => onChange({ ...cfg, project: v })} />
      <TextField label="Team" value={cfg.team} onChange={(v) => onChange({ ...cfg, team: v })} />
      <TextField label="Country (ISO)" value={cfg.country} onChange={(v) => onChange({ ...cfg, country: v.toUpperCase() })} />
      <SelectField
        label="Agent"
        value={cfg.agent}
        options={['copilot', 'claude', 'cursor']}
        onChange={(v) => onChange({ ...cfg, agent: v })}
      />
    </div>
  )
}

function VaultFields({ cfg, onChange }: FieldsProps) {
  const v = cfg.vault
  const set = (patch: Partial<typeof v>) => onChange({ ...cfg, vault: { ...v, ...patch } })
  return (
    <div className="grid grid-cols-2 gap-4">
      <NumberField label="Port" value={v.port} onChange={(x) => set({ port: x })} hint="1024–65535 · restart required" />
      <BoolField label="Auto-start with editor" value={v.auto_start} onChange={(x) => set({ auto_start: x })} />
      <TextField label="Sync repo (git URL)" value={v.sync_repo ?? ''} onChange={(x) => set({ sync_repo: x })} />
      <TextField label="Sync branch" value={v.sync_branch ?? ''} onChange={(x) => set({ sync_branch: x })} />
      <BoolField label="Auto-sync" value={v.auto_sync ?? false} onChange={(x) => set({ auto_sync: x })} />
      <NumberField
        label="Sync interval (min)"
        value={v.sync_interval_minutes ?? 0}
        onChange={(x) => set({ sync_interval_minutes: x })}
        hint="Teams tier"
      />
      <NumberField
        label="Retention (days)"
        value={v.retention_days ?? 0}
        onChange={(x) => set({ retention_days: x })}
        hint="Teams tier · 0 = disabled"
      />
      <TextField
        label="Webhook URL"
        value={v.webhook_url ?? ''}
        onChange={(x) => set({ webhook_url: x })}
        hint="Teams tier"
      />
      <ChipField
        label="Private patterns"
        value={v.private_patterns ?? []}
        onChange={(x) => set({ private_patterns: x })}
        full
      />
    </div>
  )
}

function LoreFields({ cfg, onChange }: FieldsProps) {
  const l = cfg.lore
  const set = (patch: Partial<typeof l>) => onChange({ ...cfg, lore: { ...l, ...patch } })
  return (
    <div className="grid grid-cols-2 gap-4">
      <ChipField
        label="Active scrolls"
        value={l.active_scrolls ?? []}
        onChange={(x) => set({ active_scrolls: x })}
        full
      />
      <SelectField
        label="Scroll priority"
        value={l.scroll_priority ?? 'private_first'}
        options={['private_first', 'public_first']}
        onChange={(x) => set({ scroll_priority: x })}
      />
    </div>
  )
}

function SentinelFields({ cfg, onChange }: FieldsProps) {
  const s = cfg.sentinel
  const set = (patch: Partial<typeof s>) => onChange({ ...cfg, sentinel: { ...s, ...patch } })
  return (
    <div className="grid grid-cols-2 gap-4">
      <BoolField label="Enabled" value={s.enabled} onChange={(x) => set({ enabled: x })} />
      <BoolField
        label="Block on violation"
        value={s.block_on_violation ?? true}
        onChange={(x) => set({ block_on_violation: x })}
      />
      <ChipField
        label="Hooks"
        value={s.hooks}
        onChange={(x) => set({ hooks: x })}
        hint="pre-commit · pre-push · commit-msg"
      />
      <TextField
        label="Custom rules path"
        value={s.rules_path ?? ''}
        onChange={(x) => set({ rules_path: x })}
        hint="e.g. .korva/sentinel-rules.yaml"
      />
    </div>
  )
}

function HiveFields({ cfg, onChange }: FieldsProps) {
  const h = cfg.hive
  const set = (patch: Partial<typeof h>) => onChange({ ...cfg, hive: { ...h, ...patch } })
  return (
    <div className="grid grid-cols-2 gap-4">
      <BoolField label="Enabled" value={h.enabled} onChange={(x) => set({ enabled: x })} />
      <TextField
        label="Endpoint"
        value={h.endpoint}
        onChange={(x) => set({ endpoint: x })}
        hint="restart required"
      />
      <NumberField
        label="Sync interval (min)"
        value={h.interval_minutes}
        onChange={(x) => set({ interval_minutes: x })}
      />
      <ChipField label="Allowed types" value={h.allowed_types ?? []} onChange={(x) => set({ allowed_types: x })} full />
      <ChipField
        label="Reject patterns"
        value={h.reject_patterns ?? []}
        onChange={(x) => set({ reject_patterns: x })}
        full
      />
    </div>
  )
}

function LicenseFields({ cfg, onChange }: FieldsProps) {
  return (
    <div>
      <TextField
        label="Activation URL"
        value={cfg.license.activation_url ?? ''}
        onChange={(v) =>
          onChange({ ...cfg, license: { ...cfg.license, activation_url: v } })
        }
      />
      <p className="text-[10px] text-[#484f58] mt-2">
        Other license fields (tier, seats, expiration) are read-only — managed by{' '}
        <code className="font-mono">korva license activate / deactivate</code>.
      </p>
    </div>
  )
}

// ── Field primitives ──────────────────────────────────────────────────────────

function ScopeSwitch({ value, onChange }: { value: Scope; onChange: (s: Scope) => void }) {
  return (
    <div className="inline-flex border border-[#30363d] rounded-md overflow-hidden text-xs">
      {(['local', 'global'] as const).map((s) => (
        <button
          key={s}
          onClick={() => onChange(s)}
          className={`px-3 py-1 ${
            value === s
              ? 'bg-[#388bfd20] text-[#388bfd]'
              : 'bg-[#21262d] text-[#8b949e] hover:text-[#e6edf3]'
          }`}
        >
          {s}
        </button>
      ))}
    </div>
  )
}

function TextField({
  label,
  value,
  onChange,
  hint,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  hint?: string
}) {
  return (
    <div>
      <Label text={label} hint={hint} />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
      />
    </div>
  )
}

function NumberField({
  label,
  value,
  onChange,
  hint,
}: {
  label: string
  value: number
  onChange: (v: number) => void
  hint?: string
}) {
  return (
    <div>
      <Label text={label} hint={hint} />
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(parseInt(e.target.value, 10) || 0)}
        className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
      />
    </div>
  )
}

function BoolField({
  label,
  value,
  onChange,
}: {
  label: string
  value: boolean
  onChange: (v: boolean) => void
}) {
  return (
    <div>
      <Label text={label} />
      <label className="inline-flex items-center gap-2 cursor-pointer">
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
          className="accent-[#388bfd]"
        />
        <span className="text-xs text-[#c9d1d9]">{value ? 'on' : 'off'}</span>
      </label>
    </div>
  )
}

function SelectField({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: string[]
  onChange: (v: string) => void
}) {
  return (
    <div>
      <Label text={label} />
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    </div>
  )
}

function ChipField({
  label,
  value,
  onChange,
  full,
  hint,
}: {
  label: string
  value: string[]
  onChange: (v: string[]) => void
  full?: boolean
  hint?: string
}) {
  const [input, setInput] = useState('')
  return (
    <div className={full ? 'col-span-2' : ''}>
      <Label text={label} hint={hint} />
      <div className="flex flex-wrap gap-1.5 p-1.5 bg-[#0d1117] border border-[#30363d] rounded min-h-[32px]">
        {value.map((chip, i) => (
          <span
            key={i}
            className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-[#21262d] text-xs text-[#e6edf3]"
          >
            <code className="font-mono">{chip}</code>
            <button
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              className="text-[#484f58] hover:text-[#f85149]"
              aria-label="remove"
            >
              ×
            </button>
          </span>
        ))}
        <input
          type="text"
          value={input}
          placeholder="add…"
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && input.trim()) {
              e.preventDefault()
              onChange([...value, input.trim()])
              setInput('')
            }
          }}
          className="flex-1 min-w-[80px] bg-transparent text-xs text-[#e6edf3] focus:outline-none"
        />
      </div>
    </div>
  )
}

function Label({ text, hint }: { text: string; hint?: string }) {
  return (
    <div className="mb-1">
      <label className="text-[10px] uppercase tracking-wider text-[#8b949e]">{text}</label>
      {hint && <span className="text-[9px] text-[#484f58] ml-2">{hint}</span>}
    </div>
  )
}
