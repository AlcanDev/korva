import { useState } from 'react'
import { BookMarked } from 'lucide-react'
import { InfoCallout } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'

// ── TermTooltip ───────────────────────────────────────────────────────────────
// Wraps any inline term with a dashed underline and a hover definition popup.

interface TermTooltipProps {
  term: string
  definition: string
  children: React.ReactNode
}

export function TermTooltip({ term, definition, children }: TermTooltipProps) {
  const [open, setOpen] = useState(false)

  return (
    <span className="relative inline-block">
      <span
        tabIndex={0}
        role="button"
        aria-label={`Definition: ${term}`}
        className="border-b border-dashed border-[#388bfd60] text-[#388bfd] cursor-help focus:outline-none focus-visible:ring-1 focus-visible:ring-[#388bfd]"
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
      >
        {children}
      </span>
      {open && (
        <span
          role="tooltip"
          className="absolute bottom-full left-0 mb-2 w-72 z-50 rounded-lg border border-[#388bfd30] bg-[#0d1117] shadow-2xl p-3 pointer-events-none block"
        >
          <span className="flex items-center gap-1.5 mb-1.5">
            <BookMarked size={10} className="text-[#388bfd] flex-shrink-0" />
            <span className="text-[10px] font-semibold text-[#388bfd] uppercase tracking-wider">{term}</span>
          </span>
          <span className="text-[11px] text-[#8b949e] leading-relaxed block">{definition}</span>
        </span>
      )}
    </span>
  )
}

// ── GlossaryCallout ──────────────────────────────────────────────────────────
// Collapsible InfoCallout listing all Korva-specific terms with definitions.
// Defaults closed so it doesn't clutter the page; opens on demand.

export function GlossaryCallout() {
  const { t } = useI18n()

  const terms = [
    { term: t.glossary.scrollTerm,      def: t.glossary.scrollDef },
    { term: t.glossary.skillTerm,       def: t.glossary.skillDef },
    { term: t.glossary.vaultTerm,       def: t.glossary.vaultDef },
    { term: t.glossary.observationTerm, def: t.glossary.observationDef },
    { term: t.glossary.hiveTerm,        def: t.glossary.hiveDef },
    { term: t.glossary.sentinelTerm,    def: t.glossary.sentinelDef },
    { term: t.glossary.sddTerm,         def: t.glossary.sddDef },
    { term: t.glossary.mcpTerm,         def: t.glossary.mcpDef },
  ]

  return (
    <InfoCallout
      icon={<BookMarked size={12} />}
      title={t.glossary.title}
      variant="info"
      collapsible
      id="korva-glossary"
      defaultOpen={false}
    >
      <div className="space-y-2.5 mt-1">
        {terms.map(({ term, def }) => (
          <div key={term} className="flex gap-2 leading-relaxed">
            <span className="text-[#e6edf3] font-semibold flex-shrink-0 min-w-[5rem] text-right">
              {term}
            </span>
            <span className="text-[#484f58] flex-shrink-0">—</span>
            <span className="text-[#8b949e]">{def}</span>
          </div>
        ))}
      </div>
    </InfoCallout>
  )
}
