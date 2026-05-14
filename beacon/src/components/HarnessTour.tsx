import { useEffect, useState, type ReactNode } from 'react'
import { GitBranch, FileText, ShieldCheck, GitPullRequest, Sparkles, ArrowRight, ArrowLeft, X } from 'lucide-react'

// HarnessTour — Phase 15.C, the dashboard counterpart of the global
// OnboardingTour. Walks new operators through the harness pages they
// just landed on:
//   1. What they're looking at
//   2. What `spec_ready` means + where to drill in
//   3. The Korva harness CLI basics
//   4. The CI gate (Phase 15.A) so they understand merge enforcement
//   5. A "you're ready" CTA pointing at `korva harness init`
//
// Persistence: localStorage key `korva.harness.tour.completed`. Auto-
// arms on first visit to /app/harness. Mirrors OnboardingTour's
// centred-modal approach (no spotlight refs needed; runs in jsdom for
// unit tests).

const STORAGE_KEY = 'korva.harness.tour.completed'

export function hasCompletedHarnessTour(): boolean {
  if (typeof window === 'undefined') return true
  return window.localStorage.getItem(STORAGE_KEY) === '1'
}

export function markHarnessTourCompleted(): void {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(STORAGE_KEY, '1')
}

export function resetHarnessTour(): void {
  if (typeof window === 'undefined') return
  window.localStorage.removeItem(STORAGE_KEY)
}

interface TourStep {
  icon: ReactNode
  title: string
  body: ReactNode
  cta?: { label: string; href: string; external?: boolean }
}

const STEPS: TourStep[] = [
  {
    icon: <GitBranch size={26} />,
    title: 'Welcome to the Harness Dashboard',
    body: (
      <>
        Every project that runs <code className="font-mono text-[#58a6ff]">korva harness</code> against
        the Vault shows up here. Cards roll up the latest backlog state and the
        most-recent transition; click in to see the full <code className="font-mono text-[#58a6ff]">feature_list.json</code>{' '}
        and the timeline.
      </>
    ),
  },
  {
    icon: <FileText size={26} />,
    title: 'spec_ready means awaiting human approval',
    body: (
      <>
        SDD-flagged features pause at <span className="text-[#d29922] font-medium">spec_ready</span> until a
        human reviews <code className="font-mono text-[#58a6ff]">specs/&lt;feature&gt;/{'{requirements,design,tasks}'}.md</code>{' '}
        and runs <code className="font-mono text-[#58a6ff]">korva harness start &lt;id&gt;</code>.
        The dashboard surfaces this with an "Awaiting approval" hint on the project detail page.
      </>
    ),
  },
  {
    icon: <ShieldCheck size={26} />,
    title: 'Lint specs before approval',
    body: (
      <>
        <code className="font-mono text-[#58a6ff]">korva harness review &lt;id&gt;</code> runs the EARS linter +
        R↔task traceability + acceptance coverage check. The reviewer subagent
        gates on it via the <code className="font-mono text-[#58a6ff]">vault_harness_spec_review</code> MCP tool.
      </>
    ),
  },
  {
    icon: <GitPullRequest size={26} />,
    title: 'Wire the CI gate in two minutes',
    body: (
      <>
        <code className="font-mono text-[#58a6ff]">korva harness ci install --provider=github-actions</code>{' '}
        (or <code className="font-mono text-[#58a6ff]">gitlab-ci</code>) drops a workflow that runs{' '}
        <code className="font-mono text-[#58a6ff]">korva harness check</code> on every PR / MR and posts
        the backlog summary as a comment. No manual YAML editing.
      </>
    ),
  },
  {
    icon: <Sparkles size={26} />,
    title: "You're ready",
    body: (
      <>
        Bootstrap a harness in any repo with <code className="font-mono text-[#58a6ff]">korva harness init --sdd</code>.
        Once an agent calls the MCP tools with a <code className="font-mono text-[#58a6ff]">project</code> arg, this
        dashboard fills with live data.
      </>
    ),
    cta: { label: 'Read the docs', href: 'https://korva.dev/docs/harness', external: true },
  },
]

export interface HarnessTourProps {
  open: boolean
  onClose: (completed: boolean) => void
}

export function HarnessTour({ open, onClose }: HarnessTourProps) {
  const [step, setStep] = useState(0)
  const total = STEPS.length
  const current = STEPS[step]

  // Reset to step 0 every time the tour opens — operators replaying
  // via ⌘K (or the sidebar tip when we add it) expect a fresh start.
  useEffect(() => {
    if (open) setStep(0)
  }, [open])

  // Escape closes the tour without marking completed (operator gets to
  // see it again next visit). Enter / Space advances.
  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onClose(false)
      } else if (e.key === 'Enter' || e.key === 'ArrowRight') {
        e.preventDefault()
        next()
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault()
        prev()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  function close(completed: boolean) {
    if (completed) markHarnessTourCompleted()
    onClose(completed)
  }
  function next() {
    if (step + 1 >= total) {
      close(true)
      return
    }
    setStep(s => s + 1)
  }
  function prev() {
    setStep(s => Math.max(0, s - 1))
  }

  if (!open) return null

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Harness tour"
      className="fixed inset-0 z-50 flex items-center justify-center px-4"
    >
      {/* Click backdrop = dismiss without completing. */}
      <button
        type="button"
        aria-label="Close tour"
        className="absolute inset-0 bg-black/70 backdrop-blur-sm"
        onClick={() => close(false)}
      />
      <div
        className="relative w-full max-w-lg rounded-2xl shadow-2xl overflow-hidden border border-[#30363d]"
        style={{ background: '#161B22' }}
      >
        <button
          type="button"
          onClick={() => close(false)}
          aria-label="Close"
          className="absolute top-3 right-3 h-9 w-9 flex items-center justify-center rounded-md text-[#8b949e] hover:text-[#e6edf3] hover:bg-white/5 transition-colors"
        >
          <X size={16} />
        </button>

        <div className="px-6 sm:px-8 pt-7 pb-2 text-center">
          <div
            className="mx-auto w-14 h-14 rounded-2xl flex items-center justify-center mb-4 border border-[#f0883e40]"
            style={{ background: '#f0883e15', color: '#f0883e' }}
          >
            {current.icon}
          </div>
          <h2 className="font-semibold text-lg sm:text-xl text-[#e6edf3] mb-3">
            {current.title}
          </h2>
          <div className="text-sm text-[#8b949e] leading-relaxed">{current.body}</div>
        </div>

        {/* Step dots — also serve as visual progress indicator. */}
        <div className="flex justify-center gap-1.5 mt-6 px-6 sm:px-8" aria-hidden="true">
          {STEPS.map((_, i) => (
            <span
              key={i}
              className={`h-1.5 rounded-full transition-all ${
                i === step ? 'w-6 bg-[#f0883e]' : 'w-1.5 bg-[#30363d]'
              }`}
            />
          ))}
        </div>

        <div className="flex items-center justify-between gap-3 px-6 sm:px-8 py-5 mt-3 border-t border-[#21262d]">
          <button
            type="button"
            onClick={prev}
            disabled={step === 0}
            aria-label="Previous step"
            className="h-10 px-3 inline-flex items-center gap-1.5 text-xs text-[#8b949e] rounded-md transition-colors disabled:opacity-30 disabled:pointer-events-none hover:text-[#e6edf3] hover:bg-white/5"
          >
            <ArrowLeft size={14} /> Back
          </button>
          <span className="text-[11px] text-[#6e7681] font-mono">
            {step + 1} / {total}
          </span>
          {current.cta ? (
            <a
              href={current.cta.href}
              target={current.cta.external ? '_blank' : undefined}
              rel={current.cta.external ? 'noopener noreferrer' : undefined}
              onClick={() => close(true)}
              className="h-10 px-4 inline-flex items-center gap-1.5 text-xs font-medium rounded-md transition-colors"
              style={{ background: '#f0883e', color: '#000' }}
            >
              {current.cta.label} <ArrowRight size={14} />
            </a>
          ) : (
            <button
              type="button"
              onClick={next}
              aria-label={step + 1 === total ? 'Finish tour' : 'Next step'}
              className="h-10 px-4 inline-flex items-center gap-1.5 text-xs font-medium rounded-md transition-colors"
              style={{ background: '#f0883e', color: '#000' }}
            >
              {step + 1 === total ? 'Finish' : 'Next'} <ArrowRight size={14} />
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
