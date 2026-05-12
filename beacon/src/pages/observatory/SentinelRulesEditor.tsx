import { useEffect, useState } from 'react'
import {
  useSentinelRules,
  useUpdateSentinelRules,
  useTestSentinelRule,
  CustomSentinelRule,
} from '@/api/observatory'
import { Shield, Plus, Trash2, Save, Play } from 'lucide-react'

const PROFILES = ['minimal', 'standard', 'strict', 'custom']
const SEVERITIES = ['error', 'warning', 'info']

export default function SentinelRulesEditor() {
  const { data, isLoading, error, refetch } = useSentinelRules()
  const update = useUpdateSentinelRules()

  const [profile, setProfile] = useState('standard')
  const [rules, setRules] = useState<CustomSentinelRule[]>([])

  useEffect(() => {
    if (data) {
      setProfile(data.profile || 'standard')
      setRules(data.custom ?? [])
    }
  }, [data])

  if (isLoading) return <div className="p-6 text-[#8b949e] text-sm">Loading sentinel rules…</div>
  if (error || !data) {
    return <div className="p-6 text-[#f85149] text-sm">Error: {String(error)}</div>
  }

  const dirty =
    profile !== (data.profile || 'standard') ||
    JSON.stringify(rules) !== JSON.stringify(data.custom ?? [])

  function addRule() {
    setRules([
      ...rules,
      {
        id: `CUSTOM-${String(rules.length + 1).padStart(3, '0')}`,
        description: '',
        severity: 'error',
        pattern: '',
        message: '',
      },
    ])
  }

  function updateRule(i: number, patch: Partial<CustomSentinelRule>) {
    setRules(rules.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  }

  function removeRule(i: number) {
    setRules(rules.filter((_, idx) => idx !== i))
  }

  function save() {
    update.mutate(
      { profile, custom_rules: rules },
      {
        onSuccess: () => refetch(),
      },
    )
  }

  return (
    <div className="p-6 max-w-7xl mx-auto space-y-6">
      <header className="flex items-center justify-between flex-wrap gap-2">
        <div>
          <h1 className="text-xl font-semibold text-[#e6edf3] flex items-center gap-2">
            <Shield size={18} /> Sentinel Rules
          </h1>
          <p className="text-xs text-[#8b949e] mt-1 font-mono truncate">{data.rules_path}</p>
        </div>
        <button
          onClick={save}
          disabled={!dirty || update.isPending}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-40"
        >
          <Save size={12} /> {update.isPending ? 'Saving…' : 'Save'}
        </button>
      </header>

      {update.error && (
        <div className="rounded-md border border-[#f85149] bg-[#f8514920] p-3 text-xs text-[#f85149]">
          {String(update.error)}
        </div>
      )}

      <section className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
        <h3 className="text-sm font-semibold text-[#e6edf3] mb-3">Profile</h3>
        <div className="flex gap-2 flex-wrap">
          {PROFILES.map((p) => (
            <button
              key={p}
              onClick={() => setProfile(p)}
              className={`px-3 py-1 rounded text-xs ${
                profile === p
                  ? 'bg-[#388bfd20] text-[#388bfd] border border-[#388bfd40]'
                  : 'bg-[#21262d] text-[#8b949e] hover:text-[#e6edf3] border border-[#30363d]'
              }`}
            >
              {p}
            </button>
          ))}
        </div>
        <p className="text-[10px] text-[#484f58] mt-2">
          Built-in profiles select pre-defined rule subsets. Custom rules below run in addition to the profile's rules.
        </p>
      </section>

      <section>
        <h3 className="text-sm font-semibold text-[#e6edf3] mb-3">Built-in rules ({data.builtin.length})</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
          {data.builtin.map((b) => (
            <div
              key={b.id}
              className="rounded border border-[#30363d] bg-[#161b22] p-3 flex items-start gap-3"
            >
              <code className="font-mono text-xs text-[#388bfd] flex-shrink-0">{b.id}</code>
              <div className="flex-1 min-w-0">
                <p className="text-xs text-[#e6edf3]">{b.description}</p>
                <span
                  className={`text-[9px] uppercase tracking-wider ${
                    b.severity === 'error' ? 'text-[#f85149]' : 'text-[#d29922]'
                  }`}
                >
                  {b.severity}
                </span>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-[#e6edf3]">Custom rules ({rules.length})</h3>
          <button
            onClick={addRule}
            className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs bg-[#21262d] border border-[#30363d] text-[#e6edf3] hover:bg-[#30363d]"
          >
            <Plus size={12} /> Add rule
          </button>
        </div>
        {rules.length === 0 ? (
          <p className="text-xs text-[#8b949e]">
            No custom rules yet. Use built-in rules above or add your own regex-based checks.
          </p>
        ) : (
          <div className="space-y-3">
            {rules.map((rule, i) => (
              <RuleCard
                key={i}
                rule={rule}
                onChange={(patch) => updateRule(i, patch)}
                onRemove={() => removeRule(i)}
              />
            ))}
          </div>
        )}
      </section>

      <Playground />
    </div>
  )
}

function RuleCard({
  rule,
  onChange,
  onRemove,
}: {
  rule: CustomSentinelRule
  onChange: (patch: Partial<CustomSentinelRule>) => void
  onRemove: () => void
}) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <Field label="ID">
          <input
            type="text"
            value={rule.id}
            onChange={(e) => onChange({ id: e.target.value.toUpperCase() })}
            placeholder="CUSTOM-001"
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
        <Field label="Severity">
          <select
            value={rule.severity ?? 'error'}
            onChange={(e) => onChange({ severity: e.target.value })}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
          >
            {SEVERITIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Description" full>
          <input
            type="text"
            value={rule.description ?? ''}
            onChange={(e) => onChange({ description: e.target.value })}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
        <Field label="Regex pattern" full>
          <input
            type="text"
            value={rule.pattern}
            onChange={(e) => onChange({ pattern: e.target.value })}
            placeholder='console\.(log|debug)'
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
        <Field label="paths_include (one per line)" full>
          <textarea
            value={(rule.paths_include ?? []).join('\n')}
            onChange={(e) =>
              onChange({ paths_include: e.target.value.split('\n').filter((s) => s.trim()) })
            }
            placeholder={`src/**/*.ts`}
            rows={2}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
        <Field label="paths_exclude (one per line)" full>
          <textarea
            value={(rule.paths_exclude ?? []).join('\n')}
            onChange={(e) =>
              onChange({ paths_exclude: e.target.value.split('\n').filter((s) => s.trim()) })
            }
            placeholder={`src/**/*.spec.ts`}
            rows={2}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
        <Field label="Message" full>
          <input
            type="text"
            value={rule.message ?? ''}
            onChange={(e) => onChange({ message: e.target.value })}
            placeholder="Shown when the rule matches"
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
      </div>
      <div className="flex justify-end mt-3">
        <button
          onClick={onRemove}
          className="inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] text-[#f85149] hover:bg-[#f8514920]"
        >
          <Trash2 size={11} /> Remove
        </button>
      </div>
    </div>
  )
}

function Playground() {
  const test = useTestSentinelRule()
  // The default rule + sample code purposefully avoid any literal that the
  // sentinel self-check would flag (e.g. console.log) — picking `debugger` as
  // the demo pattern keeps the CI Sentinel job green while still showing a
  // realistic regex-based rule.
  const [rule, setRule] = useState<CustomSentinelRule>({
    id: 'PLAYGROUND-1',
    pattern: 'debugger',
    severity: 'error',
    paths_include: ['src/**/*.ts'],
    message: 'no debugger statements in production code',
  })
  const [code, setCode] = useState(`const x = 1
debugger
const y = 2`)
  const [filePath, setFilePath] = useState('src/app.ts')

  return (
    <section className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <h3 className="text-sm font-semibold text-[#e6edf3] mb-3">Test playground</h3>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="space-y-2">
          <Field label="Pattern">
            <input
              type="text"
              value={rule.pattern}
              onChange={(e) => setRule({ ...rule, pattern: e.target.value })}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
            />
          </Field>
          <Field label="paths_include (CSV)">
            <input
              type="text"
              value={(rule.paths_include ?? []).join(', ')}
              onChange={(e) =>
                setRule({
                  ...rule,
                  paths_include: e.target.value.split(',').map((s) => s.trim()).filter(Boolean),
                })
              }
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
            />
          </Field>
          <Field label="File path under test">
            <input
              type="text"
              value={filePath}
              onChange={(e) => setFilePath(e.target.value)}
              className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
            />
          </Field>
        </div>
        <Field label="Code snippet">
          <textarea
            value={code}
            onChange={(e) => setCode(e.target.value)}
            rows={6}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd]"
          />
        </Field>
      </div>
      <div className="flex items-center gap-3 mt-3">
        <button
          onClick={() => test.mutate({ rule, code, file_path: filePath })}
          disabled={test.isPending}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#388bfd20] border border-[#388bfd40] text-[#388bfd] hover:bg-[#388bfd30] disabled:opacity-40"
        >
          <Play size={12} /> {test.isPending ? 'Testing…' : 'Run'}
        </button>
        {test.data && (
          <span className="text-xs text-[#8b949e]">
            {test.data.applies
              ? `${test.data.matches.length} match${test.data.matches.length === 1 ? '' : 'es'}`
              : 'rule does not apply to this file path'}
          </span>
        )}
      </div>
      {test.error && (
        <div className="mt-2 text-xs text-[#f85149]">Error: {String(test.error)}</div>
      )}
      {test.data?.matches && test.data.matches.length > 0 && (
        <div className="mt-3 rounded border border-[#30363d] bg-[#0d1117] p-2 space-y-1">
          {test.data.matches.map((m, i) => (
            <div key={i} className="text-xs font-mono text-[#c9d1d9]">
              <span className="text-[#f85149]">L{m.line}:{m.column}</span>{' '}
              <code className="bg-[#21262d] px-1 rounded">{m.matched_text}</code>{' '}
              <span className="text-[#8b949e]">— {m.message}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

function Field({ label, children, full }: { label: string; children: React.ReactNode; full?: boolean }) {
  return (
    <div className={full ? 'md:col-span-2' : ''}>
      <label className="text-[10px] uppercase tracking-wider text-[#8b949e] mb-1 block">{label}</label>
      {children}
    </div>
  )
}
