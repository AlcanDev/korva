import { useState } from 'react'
import { Terminal, ChevronDown, ChevronRight, Copy, Check } from 'lucide-react'
import { useI18nOptional, EDITOR_INTEGRATION } from '@/contexts/i18n'

interface CliHint {
  command: string
  label?: string
}

interface PageHeaderProps {
  icon: React.ReactNode
  iconColor?: string
  title: string
  description: string
  badge?: string
  badgeColor?: 'blue' | 'green' | 'orange' | 'purple'
  hint?: CliHint
  actions?: React.ReactNode
}

export function PageHeader({
  icon,
  iconColor = '#8b949e',
  title,
  description,
  badge,
  badgeColor = 'blue',
  hint,
  actions,
}: PageHeaderProps) {
  const i18n = useI18nOptional()
  const editorInfo = i18n ? EDITOR_INTEGRATION[i18n.editor] : null
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    if (!hint) return
    navigator.clipboard.writeText(hint.command).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const badgeStyles = {
    blue:   'bg-[#388bfd20] text-[#388bfd] border-[#388bfd30]',
    green:  'bg-[#3fb95020] text-[#3fb950] border-[#3fb95030]',
    orange: 'bg-[#f0883e20] text-[#f0883e] border-[#f0883e30]',
    purple: 'bg-[#a371f720] text-[#a371f7] border-[#a371f730]',
  }

  return (
    <div className="mb-6">
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-start gap-3">
          <div
            className="w-9 h-9 rounded-lg flex items-center justify-center flex-shrink-0 border"
            style={{ background: `${iconColor}15`, borderColor: `${iconColor}30`, color: iconColor }}
          >
            {icon}
          </div>
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-[#e6edf3] text-lg font-semibold leading-tight">{title}</h1>
              {badge && (
                <span className={`px-2 py-0.5 rounded-full text-[10px] border font-medium ${badgeStyles[badgeColor]}`}>
                  {badge}
                </span>
              )}
            </div>
            <p className="text-[#8b949e] text-xs mt-0.5 leading-relaxed max-w-xl">{description}</p>
            {hint && (
              <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                <Terminal size={10} className="text-[#484f58]" />
                <div className="flex items-center gap-1 group">
                  <code className="text-[10px] font-mono text-[#8b949e] bg-[#21262d] border border-[#30363d] px-1.5 py-0.5 rounded">
                    {hint.command}
                  </code>
                  <button
                    onClick={handleCopy}
                    className="opacity-0 group-hover:opacity-100 text-[#484f58] hover:text-[#8b949e] transition-all"
                    title="Copy command"
                  >
                    {copied ? <Check size={9} className="text-[#3fb950]" /> : <Copy size={9} />}
                  </button>
                </div>
                {hint.label && (
                  <span className="text-[10px] text-[#484f58]">{hint.label}</span>
                )}
                {editorInfo && (
                  <span className="text-[10px] text-[#484f58] bg-[#21262d] border border-[#30363d] px-1.5 py-0.5 rounded ml-1">
                    {editorInfo.name}
                  </span>
                )}
              </div>
            )}
          </div>
        </div>
        {actions && (
          <div className="flex items-center gap-2 flex-shrink-0">{actions}</div>
        )}
      </div>
    </div>
  )
}

const CALLOUT_STORAGE_PREFIX = 'korva-callout-'

interface InfoCalloutProps {
  icon?: React.ReactNode
  title?: string
  children: React.ReactNode
  variant?: 'info' | 'warning' | 'tip'
  collapsible?: boolean
  id?: string
  defaultOpen?: boolean
}

export function InfoCallout({
  icon,
  title,
  children,
  variant = 'info',
  collapsible = false,
  id,
  defaultOpen = true,
}: InfoCalloutProps) {
  const styles = {
    info:    { border: '#388bfd30', bg: '#388bfd0a', text: '#388bfd' },
    warning: { border: '#d2992230', bg: '#d299220a', text: '#d29922' },
    tip:     { border: '#3fb95030', bg: '#3fb9500a', text: '#3fb950' },
  }
  const s = styles[variant]

  const [open, setOpen] = useState<boolean>(() => {
    if (!collapsible || !id) return true
    try {
      const stored = localStorage.getItem(`${CALLOUT_STORAGE_PREFIX}${id}`)
      return stored === null ? defaultOpen : stored === 'true'
    } catch { return defaultOpen }
  })

  const toggle = () => {
    if (!collapsible || !id) return
    const next = !open
    setOpen(next)
    try { localStorage.setItem(`${CALLOUT_STORAGE_PREFIX}${id}`, String(next)) } catch { /* ignore */ }
  }

  return (
    <div
      className="rounded-lg mb-4"
      style={{ border: `1px solid ${s.border}`, background: s.bg }}
    >
      {(icon || title || collapsible) && (
        <div
          className={`flex items-center justify-between gap-2 px-4 py-3 ${collapsible ? 'cursor-pointer select-none' : ''}`}
          onClick={collapsible ? toggle : undefined}
        >
          <div className="flex items-center gap-2">
            {icon && <span style={{ color: s.text }}>{icon}</span>}
            {title && <span className="text-xs font-medium" style={{ color: s.text }}>{title}</span>}
          </div>
          {collapsible && (
            <span style={{ color: s.text }}>
              {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            </span>
          )}
        </div>
      )}
      {open && (
        <div className={`text-xs text-[#8b949e] space-y-1 leading-relaxed px-4 pb-3 ${(icon || title || collapsible) ? '' : 'pt-3'}`}>
          {children}
        </div>
      )}
    </div>
  )
}
