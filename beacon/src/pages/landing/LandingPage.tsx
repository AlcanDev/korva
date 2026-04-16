import { useState, useEffect } from 'react'

/* ─── Nav ──────────────────────────────────────────────── */
function Nav() {
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20)
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <header
      style={{
        position: 'fixed', top: 0, left: 0, right: 0, zIndex: 100,
        borderBottom: scrolled ? '1px solid var(--border)' : '1px solid transparent',
        background: scrolled ? 'rgba(6,6,8,0.92)' : 'transparent',
        backdropFilter: scrolled ? 'blur(12px)' : 'none',
        transition: 'all 0.3s ease',
        padding: '0 max(24px, calc((100vw - 1100px) / 2))',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        height: 60,
      }}
    >
      {/* Logo */}
      <a href="/" style={{ display: 'flex', alignItems: 'center', gap: 10, textDecoration: 'none' }}>
        <div style={{
          width: 28, height: 28, borderRadius: 7,
          background: 'var(--accent)', display: 'flex',
          alignItems: 'center', justifyContent: 'center',
          fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 14, color: '#000',
        }}>K</div>
        <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 16, color: 'var(--text)' }}>
          korva
        </span>
      </a>

      {/* Links */}
      <nav style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <a href="https://github.com/alcandev/korva" target="_blank" rel="noopener noreferrer"
          style={{
            display: 'flex', alignItems: 'center', gap: 6, padding: '7px 14px',
            color: 'var(--text-muted)', textDecoration: 'none',
            fontFamily: 'var(--font-body)', fontSize: 14, borderRadius: 7,
            transition: 'color 0.15s',
          }}
          onMouseEnter={e => (e.currentTarget.style.color = 'var(--text)')}
          onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-muted)')}
        >
          <GitHubIcon />
          GitHub
        </a>
        <a href="/app" className="btn-primary" style={{ padding: '8px 18px', fontSize: 13 }}>
          Dashboard →
        </a>
      </nav>
    </header>
  )
}

/* ─── Hero ──────────────────────────────────────────────── */
function Hero() {
  return (
    <section
      className="bg-mesh"
      style={{
        position: 'relative', minHeight: '100vh',
        display: 'flex', alignItems: 'center',
        padding: '100px max(24px, calc((100vw - 1100px) / 2)) 80px',
        overflow: 'hidden',
      }}
    >
      <div className="grid-overlay" />

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 64, alignItems: 'center', position: 'relative', width: '100%' }}>
        {/* Left: copy */}
        <div>
          <div className="animate-fade-up" style={{ marginBottom: 24 }}>
            <span className="badge badge-green">
              <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--accent)', flexShrink: 0 }} />
              v0.1.0 · MIT License · Open Source
            </span>
          </div>

          <h1
            className="animate-fade-up delay-100"
            style={{
              fontFamily: 'var(--font-display)', fontWeight: 800,
              fontSize: 'clamp(40px, 5vw, 64px)', lineHeight: 1.05,
              color: 'var(--text)', margin: '0 0 24px',
              letterSpacing: '-0.02em',
            }}
          >
            The OS for<br />
            AI-driven<br />
            <span style={{ color: 'var(--accent)' }}>Engineering Teams.</span>
          </h1>

          <p
            className="animate-fade-up delay-200"
            style={{
              fontFamily: 'var(--font-body)', fontWeight: 300,
              fontSize: 18, lineHeight: 1.7,
              color: 'var(--text-muted)', margin: '0 0 40px',
              maxWidth: 440,
            }}
          >
            Give your AI persistent memory, architecture guardrails,
            knowledge injection, and structured workflows.
            All local. Zero cloud. Free forever.
          </p>

          <div className="animate-fade-up delay-300" style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
            <a href="#install" className="btn-primary">
              Install in 30 seconds ↓
            </a>
            <a href="https://github.com/alcandev/korva" target="_blank" rel="noopener noreferrer" className="btn-secondary">
              <GitHubIcon /> View on GitHub
            </a>
          </div>

          {/* Editor support */}
          <p className="animate-fade-up delay-400" style={{
            marginTop: 32, fontSize: 12.5, color: 'var(--text-dim)',
            fontFamily: 'var(--font-mono)', letterSpacing: '0.03em',
          }}>
            Works with&nbsp;
            <span style={{ color: 'var(--text-muted)' }}>GitHub Copilot · Claude Code · Cursor</span>
          </p>
        </div>

        {/* Right: terminal mockup */}
        <div className="animate-fade-in delay-400">
          <HeroTerminal />
        </div>
      </div>
    </section>
  )
}

function HeroTerminal() {
  return (
    <div className="terminal" style={{ maxWidth: 520 }}>
      <div className="terminal-bar">
        <div className="terminal-dot" style={{ background: '#FF5F57' }} />
        <div className="terminal-dot" style={{ background: '#FEBC2E' }} />
        <div className="terminal-dot" style={{ background: '#28C840' }} />
        <span style={{ marginLeft: 8, color: 'var(--text-dim)', fontSize: 11 }}>korva — zsh</span>
      </div>
      <div className="terminal-body">
        <div><span className="term-prompt">$</span> <span className="term-cmd">korva init</span></div>
        <div className="term-success">✓ Vault online · localhost:7437</div>
        <div className="term-success">✓ 16 curated scrolls loaded</div>
        <div className="term-success">✓ Sentinel pre-commit hook installed</div>
        <div style={{ marginTop: 12 }}>
          <span className="term-prompt">$</span>{' '}
          <span className="term-cmd">vault_context(<span className="term-string">"payments"</span>)</span>
        </div>
        <div className="term-output">Loading memories for project: payments-api</div>
        <div style={{ marginTop: 4 }}>
          <span className="term-success">✓</span>{' '}
          <span className="term-key">Decision #23</span>{' '}
          <span className="term-comment">— Idempotency keys: payment:{`{id}`}:{`{amount}`}</span>
        </div>
        <div>
          <span className="term-success">✓</span>{' '}
          <span className="term-key">Incident #07</span>{' '}
          <span className="term-comment">— Race condition · Redis lock 30s TTL · always</span>
        </div>
        <div>
          <span className="term-success">✓</span>{' '}
          <span className="term-key">Rule</span>{' '}
          <span className="term-comment">— Never floats for money · use Decimal.js</span>
        </div>
        <div style={{ marginTop: 12 }}>
          <span className="term-output">89 memories loaded · 3 critical · context ready</span>
        </div>
        <div style={{ marginTop: 12 }}>
          <span className="term-prompt">$</span> <span className="cursor" />
        </div>
      </div>
    </div>
  )
}

/* ─── Problem ────────────────────────────────────────────── */
function ProblemSection() {
  return (
    <section style={{
      background: 'var(--surface)',
      borderTop: '1px solid var(--border)',
      borderBottom: '1px solid var(--border)',
      padding: '80px max(24px, calc((100vw - 1100px) / 2))',
    }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 80, alignItems: 'center' }}>
        {/* Left */}
        <div>
          <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 16 }}>
            The Problem
          </p>
          <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 40px)', lineHeight: 1.15, margin: '0 0 20px', letterSpacing: '-0.02em' }}>
            Every AI session<br />starts from zero.
          </h2>
          <p style={{ fontFamily: 'var(--font-body)', fontSize: 16, color: 'var(--text-muted)', lineHeight: 1.7, maxWidth: 420, margin: 0 }}>
            Your AI doesn't know the race condition you fixed last October. It doesn't know you chose event sourcing in March. It doesn't know the team rule "never access the database from a controller."
          </p>
          <p style={{ fontFamily: 'var(--font-body)', fontSize: 16, color: 'var(--text-muted)', lineHeight: 1.7, maxWidth: 420, margin: '16px 0 0' }}>
            Every developer explains context for 15 minutes before every session. <strong style={{ color: 'var(--text)' }}>Every. Single. Day.</strong>
          </p>
        </div>

        {/* Right: before/after */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Without */}
          <div style={{ background: 'rgba(248,81,73,0.06)', border: '1px solid rgba(248,81,73,0.15)', borderRadius: 10, padding: '18px 20px' }}>
            <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--red)', letterSpacing: '0.08em', margin: '0 0 10px' }}>WITHOUT KORVA</p>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12.5, lineHeight: 1.7, color: 'var(--text-muted)' }}>
              <div><span style={{ color: 'var(--text-dim)' }}>Session #47:</span></div>
              <div>You: <span style={{ color: 'var(--text)' }}>"Remember: hexagonal architecture,</span></div>
              <div style={{ paddingLeft: 20 }}><span style={{ color: 'var(--text)' }}>Repository pattern, CQRS..."</span></div>
              <div>AI:&nbsp;&nbsp;<span style={{ color: 'var(--red)' }}>[violates everything again]</span></div>
              <div style={{ marginTop: 8, color: 'var(--text-dim)', fontSize: 11 }}>// Same mistake, 47 sessions.</div>
            </div>
          </div>

          {/* With */}
          <div style={{ background: 'var(--accent-dim)', border: '1px solid rgba(34,197,94,0.2)', borderRadius: 10, padding: '18px 20px' }}>
            <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--accent)', letterSpacing: '0.08em', margin: '0 0 10px' }}>WITH KORVA</p>
            <div style={{ fontFamily: 'var(--font-mono)', fontSize: 12.5, lineHeight: 1.7, color: 'var(--text-muted)' }}>
              <div><span className="term-success">✓</span> Architecture: CQRS + Events (Mar 2024)</div>
              <div><span className="term-success">✓</span> Rule: Repository interface only</div>
              <div><span className="term-success">✓</span> Incident: Direct DB = prod outage</div>
              <div style={{ marginTop: 8, color: 'var(--text)' }}>AI generates perfect code.</div>
              <div style={{ color: 'var(--accent)', fontSize: 11 }}>First try. No explanation needed.</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

/* ─── Four Pillars ──────────────────────────────────────── */
const pillars = [
  {
    icon: '🧠',
    name: 'Vault',
    tagline: 'Persistent AI Memory',
    desc: 'Save decisions, incidents, and patterns permanently. The AI learns from your team\'s history and compounds knowledge over time.',
    detail: 'vault_save · vault_context · vault_search · vault_timeline',
    color: '#22C55E',
  },
  {
    icon: '🛡️',
    name: 'Sentinel',
    tagline: 'Architecture Guardrails',
    desc: 'Pre-commit hooks that catch violations before they reach your codebase. Hardcoded secrets, wrong layer access, timing attacks — all blocked.',
    detail: 'SEC-001..006 · ARC-001..003 · NAM-001 · TEST-001 · DEPS-001',
    color: '#388BFD',
  },
  {
    icon: '📜',
    name: 'Lore',
    tagline: 'Knowledge Injection',
    desc: 'Open payments.ts and your AI already knows PCI-DSS rules, idempotency requirements, and your team\'s past race condition fix.',
    detail: '20+ curated scrolls · auto-load by file context · team profiles',
    color: '#D29922',
  },
  {
    icon: '⚙️',
    name: 'Forge',
    tagline: 'Structured Workflow',
    desc: '5-phase spec-driven development prevents AI from diving straight into code. Exploration → Spec → Architecture → Implementation → Verification.',
    detail: 'Phase gates · vault-aware design · Sentinel validation',
    color: '#A371F7',
  },
]

function PillarsSection() {
  return (
    <section style={{ padding: '96px max(24px, calc((100vw - 1100px) / 2))' }}>
      <div style={{ textAlign: 'center', marginBottom: 56 }}>
        <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
          Four Integrated Components
        </p>
        <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 44px)', lineHeight: 1.1, letterSpacing: '-0.02em', margin: '0 auto', maxWidth: 600 }}>
          Everything your AI needs to work like a senior engineer.
        </h2>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 20 }}>
        {pillars.map((p, i) => (
          <div key={p.name} className="feature-card animate-fade-up" style={{ animationDelay: `${i * 0.1}s` }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, marginBottom: 16 }}>
              <span style={{ fontSize: 28, lineHeight: 1 }}>{p.icon}</span>
              <div>
                <h3 style={{
                  fontFamily: 'var(--font-display)', fontWeight: 700,
                  fontSize: 20, color: p.color, margin: '0 0 2px',
                }}>{p.name}</h3>
                <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--text-muted)', margin: 0 }}>{p.tagline}</p>
              </div>
            </div>
            <p style={{ fontFamily: 'var(--font-body)', fontSize: 14.5, color: 'var(--text-muted)', lineHeight: 1.65, margin: '0 0 16px' }}>
              {p.desc}
            </p>
            <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', margin: 0 }}>
              {p.detail}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}

/* ─── Code Demo ──────────────────────────────────────────── */
function CodeDemoSection() {
  return (
    <section style={{
      background: 'var(--surface)',
      borderTop: '1px solid var(--border)',
      borderBottom: '1px solid var(--border)',
      padding: '96px max(24px, calc((100vw - 1100px) / 2))',
    }}>
      <div style={{ textAlign: 'center', marginBottom: 56 }}>
        <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
          Memory That Compounds
        </p>
        <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 44px)', lineHeight: 1.1, letterSpacing: '-0.02em', margin: 0 }}>
          Save once. Benefit forever.
        </h2>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 24 }}>
        {/* Save */}
        <div>
          <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', letterSpacing: '0.06em', margin: '0 0 10px' }}>
            FRIDAY 11PM — CRITICAL INCIDENT
          </p>
          <div className="terminal">
            <div className="terminal-bar">
              <div className="terminal-dot" style={{ background: '#FF5F57' }} />
              <div className="terminal-dot" style={{ background: '#FEBC2E' }} />
              <div className="terminal-dot" style={{ background: '#28C840' }} />
            </div>
            <div className="terminal-body" style={{ fontSize: 12 }}>
              <div><span className="term-prompt">$</span> <span className="term-cmd">vault_save({'{'}</span></div>
              <div style={{ paddingLeft: 20 }}><span className="term-key">type</span>: <span className="term-string">"incident"</span>,</div>
              <div style={{ paddingLeft: 20 }}><span className="term-key">title</span>: <span className="term-string">"Race condition in payment processor"</span>,</div>
              <div style={{ paddingLeft: 20 }}><span className="term-key">content</span>: <span className="term-string">"Two concurrent requests can double-charge.</span></div>
              <div style={{ paddingLeft: 36 }}><span className="term-string">Fix: Redis distributed lock on payment_id.</span></div>
              <div style={{ paddingLeft: 36 }}><span className="term-string">LOCK:payment:{'{'}'id'{'}'}:30s TTL — always."</span></div>
              <div><span className="term-cmd">{'}'}</span>)</div>
              <div style={{ marginTop: 8 }}><span className="term-success">✓ Observation saved · ID: 01HX8K...</span></div>
            </div>
          </div>
        </div>

        {/* 9 months later */}
        <div>
          <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', letterSpacing: '0.06em', margin: '0 0 10px' }}>
            9 MONTHS LATER — NEW DEVELOPER
          </p>
          <div className="terminal">
            <div className="terminal-bar">
              <div className="terminal-dot" style={{ background: '#FF5F57' }} />
              <div className="terminal-dot" style={{ background: '#FEBC2E' }} />
              <div className="terminal-dot" style={{ background: '#28C840' }} />
            </div>
            <div className="terminal-body" style={{ fontSize: 12 }}>
              <div><span className="term-comment">// Opens: src/payments/processor.ts</span></div>
              <div style={{ marginTop: 8 }}><span className="term-info">📜 Korva Vault loading memories...</span></div>
              <div style={{ marginTop: 8 }}>
                <span className="term-warn">⚠</span>{' '}
                <span style={{ color: 'var(--text)' }}>A past incident shows race conditions here.</span>
              </div>
              <div style={{ paddingLeft: 20 }}>
                <span className="term-output">Use Redis distributed lock on payment_id</span>
              </div>
              <div style={{ paddingLeft: 20 }}>
                <span className="term-output">or you risk double-charging customers.</span>
              </div>
              <div style={{ paddingLeft: 20 }}>
                <span className="term-output">LOCK:payment:{`{id}`} · 30s TTL · always.</span>
              </div>
              <div style={{ marginTop: 12, color: 'var(--accent)', fontSize: 11 }}>
                → Saved: 3-day debugging session, ~$12k incident cost
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

/* ─── Sentinel Demo ──────────────────────────────────────── */
function SentinelSection() {
  return (
    <section style={{ padding: '96px max(24px, calc((100vw - 1100px) / 2))' }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 72, alignItems: 'center' }}>
        <div>
          <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--blue)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
            Sentinel Guardrails
          </p>
          <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 40px)', lineHeight: 1.15, letterSpacing: '-0.02em', margin: '0 0 20px' }}>
            Architecture rules enforced at commit time.
          </h2>
          <p style={{ fontFamily: 'var(--font-body)', fontSize: 16, color: 'var(--text-muted)', lineHeight: 1.7, maxWidth: 420, margin: '0 0 28px' }}>
            10 built-in rules covering hardcoded secrets, timing attacks, wrong-layer access, naming conventions, and dependency vulnerabilities.
            All configurable. All extensible with custom YAML rules.
          </p>
          <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: 10 }}>
            {['SEC-001 Hardcoded secrets', 'SEC-003 Timing attack patterns', 'ARC-001 Domain layer isolation', 'ARC-002 Controller coupling', 'NAM-001 Naming conventions'].map(rule => (
              <li key={rule} style={{ display: 'flex', alignItems: 'center', gap: 10, fontFamily: 'var(--font-mono)', fontSize: 13, color: 'var(--text-muted)' }}>
                <span style={{ color: 'var(--blue)', fontSize: 10 }}>■</span> {rule}
              </li>
            ))}
          </ul>
        </div>

        <div className="terminal">
          <div className="terminal-bar">
            <div className="terminal-dot" style={{ background: '#FF5F57' }} />
            <div className="terminal-dot" style={{ background: '#FEBC2E' }} />
            <div className="terminal-dot" style={{ background: '#28C840' }} />
            <span style={{ marginLeft: 8, color: 'var(--text-dim)', fontSize: 11 }}>git commit</span>
          </div>
          <div className="terminal-body">
            <div><span className="term-cmd">$ git commit -m "feat: user auth endpoint"</span></div>
            <div style={{ marginTop: 8 }}><span className="term-output">Running Korva Sentinel...</span></div>
            <div style={{ marginTop: 6 }}>
              <span className="term-success">✓ NAM-001</span><span className="term-output">  Naming conventions</span>
            </div>
            <div>
              <span className="term-success">✓ TEST-001</span><span className="term-output"> No debug logs in production</span>
            </div>
            <div>
              <span style={{ color: 'var(--red)' }}>✗ SEC-001</span><span className="term-output">  Hardcoded secret detected</span>
            </div>
            <div>
              <span style={{ color: 'var(--red)' }}>✗ SEC-003</span><span className="term-output">  Timing attack vulnerability</span>
            </div>
            <div>
              <span style={{ color: 'var(--red)' }}>✗ ARC-002</span><span className="term-output">  HTTP handler in domain layer</span>
            </div>
            <div style={{ marginTop: 10, fontFamily: 'var(--font-mono)', fontSize: 12 }}>
              <div style={{ color: 'var(--text-muted)' }}>src/auth/AuthService.ts:14</div>
              <div style={{ paddingLeft: 2 }}>
                <span style={{ color: 'var(--text)' }}>const secret = </span>
                <span style={{ color: 'var(--red)' }}>"sk_live_4xK9mP..."</span>
              </div>
              <div style={{ color: 'var(--orange)' }}>Use process.env.JWT_SECRET</div>
            </div>
            <div style={{ marginTop: 10 }}>
              <span style={{ color: 'var(--red)', fontWeight: 600 }}>3 critical issues. Commit blocked.</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}

/* ─── Privacy / 3 Kingdoms ──────────────────────────────── */
function PrivacySection() {
  return (
    <section style={{
      background: 'var(--surface)',
      borderTop: '1px solid var(--border)',
      borderBottom: '1px solid var(--border)',
      padding: '96px max(24px, calc((100vw - 1100px) / 2))',
    }}>
      <div style={{ textAlign: 'center', marginBottom: 56 }}>
        <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
          Privacy by Architecture
        </p>
        <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 44px)', lineHeight: 1.1, letterSpacing: '-0.02em', margin: '0 auto 16px', maxWidth: 600 }}>
          3 Kingdoms. Zero data leaves your machine.
        </h2>
        <p style={{ fontFamily: 'var(--font-body)', fontSize: 16, color: 'var(--text-muted)', maxWidth: 520, margin: '0 auto' }}>
          Your code, decisions, and secrets never reach our servers. The vault runs on localhost. MCP uses stdin/stdout. Privacy is structural, not a policy.
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 24 }}>
        {[
          {
            num: '1',
            title: 'Public Repo',
            sub: 'github.com/alcandev/korva',
            color: '#388BFD',
            items: ['Core engine · CLI', 'Vault · Sentinel', '20+ Lore scrolls', 'MIT license'],
            note: 'Zero knowledge of your team\'s data.',
          },
          {
            num: '2',
            title: 'Team Profile',
            sub: 'github.com/YOUR-ORG/korva-profile',
            color: '#D29922',
            items: ['Private scrolls', 'Custom Sentinel rules', 'Team AI instructions', 'Your architecture IP'],
            note: 'Clones to your machine. Never merges to public.',
          },
          {
            num: '3',
            title: 'Your Machine',
            sub: '~/.korva/ · localhost:7437',
            color: '#22C55E',
            items: ['vault.db (SQLite)', 'admin.key (0600)', 'Runtime decisions', 'All vault memories'],
            note: 'Stays here. Forever. Unless you choose otherwise.',
          },
        ].map((k) => (
          <div key={k.num} className="kingdom-node">
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
              <div style={{
                width: 32, height: 32, borderRadius: 8,
                background: `rgba(${k.color === '#388BFD' ? '56,139,253' : k.color === '#D29922' ? '210,153,34' : '34,197,94'},0.12)`,
                border: `1px solid ${k.color}40`,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 14, color: k.color,
              }}>{k.num}</div>
              <div>
                <h3 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 16, color: 'var(--text)', margin: 0 }}>{k.title}</h3>
                <p style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--text-dim)', margin: 0 }}>{k.sub}</p>
              </div>
            </div>
            <ul style={{ listStyle: 'none', padding: 0, margin: '0 0 16px', display: 'flex', flexDirection: 'column', gap: 6 }}>
              {k.items.map(item => (
                <li key={item} style={{ display: 'flex', alignItems: 'center', gap: 8, fontFamily: 'var(--font-mono)', fontSize: 12, color: 'var(--text-muted)' }}>
                  <span style={{ color: k.color, fontSize: 8 }}>◆</span> {item}
                </li>
              ))}
            </ul>
            <p style={{ fontFamily: 'var(--font-body)', fontSize: 12.5, color: 'var(--text-dim)', margin: 0, fontStyle: 'italic', lineHeight: 1.5 }}>
              {k.note}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}

/* ─── Install ────────────────────────────────────────────── */
function InstallSection() {
  const [activeTab, setActiveTab] = useState<'macos' | 'windows' | 'docker'>('macos')
  const [copied, setCopied] = useState(false)

  const commands = {
    macos: 'curl -fsSL https://korva.dev/install.sh | bash',
    windows: 'irm https://korva.dev/install.ps1 | iex',
    docker: 'docker run -p 7437:7437 -v ~/.korva:/data ghcr.io/alcandev/korva-vault',
  }

  const copy = () => {
    navigator.clipboard.writeText(commands[activeTab])
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <section id="install" style={{
      padding: '96px max(24px, calc((100vw - 1100px) / 2))',
      textAlign: 'center',
    }}>
      <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
        Get Started
      </p>
      <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 44px)', lineHeight: 1.1, letterSpacing: '-0.02em', margin: '0 auto 12px', maxWidth: 500 }}>
        Install in 30 seconds.
      </h2>
      <p style={{ fontFamily: 'var(--font-body)', color: 'var(--text-muted)', fontSize: 16, maxWidth: 440, margin: '0 auto 48px' }}>
        Works on macOS, Linux, and Windows. No Docker required. No cloud account.
      </p>

      {/* Tabs */}
      <div style={{ display: 'flex', justifyContent: 'center', gap: 4, marginBottom: 16 }}>
        {(['macos', 'windows', 'docker'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            style={{
              padding: '7px 16px', borderRadius: 7, border: '1px solid',
              borderColor: activeTab === tab ? 'rgba(34,197,94,0.4)' : 'var(--border)',
              background: activeTab === tab ? 'var(--accent-dim)' : 'transparent',
              color: activeTab === tab ? 'var(--accent)' : 'var(--text-muted)',
              fontFamily: 'var(--font-mono)', fontSize: 12,
              cursor: 'pointer', transition: 'all 0.15s',
            }}
          >
            {tab === 'macos' ? 'macOS / Linux' : tab === 'windows' ? 'Windows' : 'Docker'}
          </button>
        ))}
      </div>

      {/* Command */}
      <div style={{ maxWidth: 640, margin: '0 auto 48px' }}>
        <div className="install-box">
          <span style={{ color: 'var(--text)' }}>
            <span style={{ color: 'var(--accent)' }}>$ </span>
            {commands[activeTab]}
          </span>
          <button className={`copy-btn${copied ? ' copied' : ''}`} onClick={copy}>
            {copied ? '✓ copied' : 'copy'}
          </button>
        </div>
      </div>

      {/* Steps */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 24, maxWidth: 800, margin: '0 auto' }}>
        {[
          { step: '01', title: 'Install', desc: 'One command. Installs the CLI, vault binary, and Sentinel hooks.' },
          { step: '02', title: 'Connect editors', desc: 'korva setup --all. Auto-configures Copilot, Claude Code, and Cursor.' },
          { step: '03', title: 'Start building', desc: 'vault_save, vault_context, and vault_search are live in your AI session.' },
        ].map(s => (
          <div key={s.step} style={{ textAlign: 'left' }}>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 28, fontWeight: 600, color: 'var(--border)', display: 'block', marginBottom: 12 }}>{s.step}</span>
            <h3 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 17, margin: '0 0 8px', color: 'var(--text)' }}>{s.title}</h3>
            <p style={{ fontFamily: 'var(--font-body)', fontSize: 13.5, color: 'var(--text-muted)', margin: 0, lineHeight: 1.6 }}>{s.desc}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

/* ─── Open Source ────────────────────────────────────────── */
function OpenSourceSection() {
  return (
    <section style={{
      background: 'var(--surface)',
      borderTop: '1px solid var(--border)',
      padding: '80px max(24px, calc((100vw - 1100px) / 2))',
    }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 72, alignItems: 'center' }}>
        <div>
          <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--accent)', letterSpacing: '0.1em', textTransform: 'uppercase', marginBottom: 14 }}>
            Open Source · MIT License
          </p>
          <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 'clamp(28px, 3vw, 40px)', lineHeight: 1.15, letterSpacing: '-0.02em', margin: '0 0 20px' }}>
            Built in the open.<br />Owned by the community.
          </h2>
          <p style={{ fontFamily: 'var(--font-body)', fontSize: 16, color: 'var(--text-muted)', lineHeight: 1.7, margin: '0 0 32px', maxWidth: 420 }}>
            No paid tier. No telemetry. No SaaS. Runs entirely on your machine.
            The source is here — verify it yourself.
          </p>
          <div style={{ display: 'flex', gap: 12 }}>
            <a href="https://github.com/alcandev/korva" target="_blank" rel="noopener noreferrer" className="btn-primary">
              <GitHubIcon /> Star on GitHub
            </a>
            <a href="https://github.com/alcandev/korva/blob/main/CONTRIBUTING.md" target="_blank" rel="noopener noreferrer" className="btn-secondary">
              Contribute
            </a>
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {[
            { title: 'Write a Lore scroll', desc: 'Add best practices for your stack — Next.js, Laravel, Rust, Django, Go...' },
            { title: 'Add a Sentinel rule', desc: 'Encode patterns your team enforces. Share them with the community.' },
            { title: 'Share your Team Profile', desc: 'Sanitize and share your korva-team-profile structure to help others set up.' },
          ].map(c => (
            <div key={c.title} style={{
              background: 'var(--surface-2)', border: '1px solid var(--border)',
              borderRadius: 10, padding: '18px 22px',
            }}>
              <h4 style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 15, margin: '0 0 6px', color: 'var(--text)' }}>{c.title}</h4>
              <p style={{ fontFamily: 'var(--font-body)', fontSize: 13.5, color: 'var(--text-muted)', margin: 0, lineHeight: 1.55 }}>{c.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

/* ─── Footer ─────────────────────────────────────────────── */
function Footer() {
  return (
    <footer style={{
      borderTop: '1px solid var(--border)',
      padding: '36px max(24px, calc((100vw - 1100px) / 2))',
      display: 'flex', alignItems: 'center', justifyContent: 'space-between',
      flexWrap: 'wrap', gap: 20,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{
          width: 22, height: 22, borderRadius: 6, background: 'var(--accent)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 11, color: '#000',
        }}>K</div>
        <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 14, color: 'var(--text-muted)' }}>korva</span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)' }}>· MIT License</span>
      </div>

      <nav style={{ display: 'flex', gap: 24 }}>
        {[
          { label: 'GitHub', href: 'https://github.com/alcandev/korva' },
          { label: 'Docs', href: 'https://github.com/alcandev/korva/blob/main/docs/USAGE.md' },
          { label: 'Roadmap', href: 'https://github.com/alcandev/korva/blob/main/ROADMAP.md' },
          { label: 'Contributing', href: 'https://github.com/alcandev/korva/blob/main/CONTRIBUTING.md' },
          { label: 'Dashboard', href: '/app' },
        ].map(link => (
          <a key={link.label} href={link.href}
            target={link.href.startsWith('http') ? '_blank' : undefined}
            rel={link.href.startsWith('http') ? 'noopener noreferrer' : undefined}
            style={{
              fontFamily: 'var(--font-body)', fontSize: 13, color: 'var(--text-dim)',
              textDecoration: 'none', transition: 'color 0.15s',
            }}
            onMouseEnter={e => (e.currentTarget.style.color = 'var(--text-muted)')}
            onMouseLeave={e => (e.currentTarget.style.color = 'var(--text-dim)')}
          >
            {link.label}
          </a>
        ))}
      </nav>

      <p style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', margin: 0 }}>
        Build with intent. Ship with confidence.
      </p>
    </footer>
  )
}

/* ─── GitHub icon ────────────────────────────────────────── */
function GitHubIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0C5.374 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.509 11.509 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576C20.566 21.797 24 17.3 24 12c0-6.627-5.373-12-12-12z" />
    </svg>
  )
}

/* ─── Root export ────────────────────────────────────────── */
export default function LandingPage() {
  return (
    <div style={{ background: 'var(--bg)', minHeight: '100vh' }}>
      <Nav />
      <Hero />
      <ProblemSection />
      <PillarsSection />
      <CodeDemoSection />
      <SentinelSection />
      <PrivacySection />
      <InstallSection />
      <OpenSourceSection />
      <Footer />
    </div>
  )
}
