import { ReactNode } from 'react'

interface StatusCardProps {
  title: string
  icon?: ReactNode
  status?: 'ok' | 'warning' | 'error' | 'disabled' | 'info'
  children: ReactNode
}

const statusBadgeStyles: Record<NonNullable<StatusCardProps['status']>, string> = {
  ok: 'bg-[#2ea04320] text-[#2ea043] border-[#2ea04340]',
  warning: 'bg-[#d2992220] text-[#d29922] border-[#d2992240]',
  error: 'bg-[#f8514920] text-[#f85149] border-[#f8514940]',
  disabled: 'bg-[#21262d] text-[#484f58] border-[#30363d]',
  info: 'bg-[#388bfd20] text-[#388bfd] border-[#388bfd40]',
}

const statusLabels: Record<NonNullable<StatusCardProps['status']>, string> = {
  ok: 'OK',
  warning: 'Warning',
  error: 'Error',
  disabled: 'Disabled',
  info: 'Info',
}

export function StatusCard({ title, icon, status, children }: StatusCardProps) {
  return (
    <div className="rounded-lg border border-[#30363d] bg-[#161b22] p-4">
      <header className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          {icon && <span className="text-[#8b949e]">{icon}</span>}
          <h3 className="text-sm font-semibold text-[#e6edf3]">{title}</h3>
        </div>
        {status && (
          <span
            className={`text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-full border font-medium ${statusBadgeStyles[status]}`}
          >
            {statusLabels[status]}
          </span>
        )}
      </header>
      <div className="text-sm text-[#c9d1d9] space-y-1.5">{children}</div>
    </div>
  )
}

interface StatusRowProps {
  label: string
  value: ReactNode
  mono?: boolean
}

export function StatusRow({ label, value, mono }: StatusRowProps) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-[#8b949e]">{label}</span>
      <span className={`text-xs ${mono ? 'font-mono' : ''} text-[#e6edf3] truncate max-w-[60%]`}>
        {value}
      </span>
    </div>
  )
}
