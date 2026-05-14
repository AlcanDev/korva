import { useState } from 'react'
import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router'
import { I18nProvider } from '@/contexts/i18n'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { ToastProvider, CommandPaletteProvider } from '@/components/ui'
import GlobalCommands from '@/components/GlobalCommands'
import {
  LayoutDashboard, Database, Clock, BookOpen, Settings, Activity, Shield, GitBranch, Menu, X,
} from 'lucide-react'
import { useVaultHealth } from '@/api/vault'
import LandingPage from '@/pages/landing/LandingPage'
import Overview from '@/pages/overview/Overview'
import VaultExplorer from '@/pages/vault-explorer/VaultExplorer'
import Sessions from '@/pages/sessions/Sessions'
import LoreManager from '@/pages/lore-manager/LoreManager'
import SettingsPage from '@/pages/settings/Settings'
import Admin from '@/pages/admin/Admin'
import HarnessDashboard from '@/pages/harness/HarnessDashboard'
import HarnessProjectDetail from '@/pages/harness/HarnessProjectDetail'

export default function App() {
  return (
    <ErrorBoundary>
    <I18nProvider>
    <ToastProvider>
    <CommandPaletteProvider>
    <BrowserRouter>
      <GlobalCommands />
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/admin/*" element={<Admin />} />
        <Route path="/app" element={<Navigate to="/app/overview" replace />} />
        <Route path="/app/*" element={<AppShell />} />
      </Routes>
    </BrowserRouter>
    </CommandPaletteProvider>
    </ToastProvider>
    </I18nProvider>
    </ErrorBoundary>
  )
}

// AppShell wraps the /app/* routes with the sidebar layout. Mobile gets
// a hamburger that reveals the sidebar as a drawer; md+ keeps the
// classic side rail (≥ 768px). Mirrors the pattern in Admin.tsx so the
// two product surfaces feel consistent.
function AppShell() {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: 'var(--surface)' }}>
      {/* Mobile overlay — tapping the dim region closes the drawer. */}
      {sidebarOpen && (
        <button
          type="button"
          aria-label="Close menu"
          className="fixed inset-0 bg-black/60 z-20 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar — drawer on mobile (off-screen by default), fixed rail on md+. */}
      <div
        className={`fixed inset-y-0 left-0 z-30 w-[260px] sm:w-72 md:w-[220px] flex-shrink-0 transform transition-transform duration-200 md:relative md:translate-x-0 md:z-auto ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <Sidebar
          onCloseMobile={() => setSidebarOpen(false)}
          showCloseButton={sidebarOpen}
        />
      </div>

      {/* Main column */}
      <div className="flex-1 flex flex-col overflow-hidden min-w-0">
        {/* Mobile top bar — hamburger + brand. Hidden on md+. */}
        <div
          className="flex items-center gap-3 px-3 py-3 border-b md:hidden flex-shrink-0"
          style={{ background: '#161B22', borderColor: 'var(--border)' }}
        >
          <button
            type="button"
            onClick={() => setSidebarOpen(true)}
            aria-label="Open menu"
            className="h-11 w-11 flex items-center justify-center -ml-1 rounded-md transition-colors hover:bg-white/5"
            style={{ color: 'var(--text-muted)' }}
          >
            <Menu size={20} />
          </button>
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <div
              className="w-7 h-7 rounded-md flex items-center justify-center font-bold text-[13px]"
              style={{ background: 'var(--accent)', color: '#000', fontFamily: 'var(--font-display)' }}
            >
              K
            </div>
            <span className="font-semibold text-sm truncate" style={{ color: 'var(--text)' }}>
              Korva
            </span>
          </div>
        </div>

        <main className="flex-1 overflow-y-auto">
          <Routes>
            <Route path="overview"           element={<Overview />} />
            <Route path="vault"              element={<VaultExplorer />} />
            <Route path="sessions"           element={<Sessions />} />
            <Route path="lore"               element={<LoreManager />} />
            <Route path="harness"            element={<HarnessDashboard />} />
            <Route path="harness/:project"   element={<HarnessProjectDetail />} />
            <Route path="settings"           element={<SettingsPage />} />
            <Route path="*"                  element={<Navigate to="/app/overview" replace />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

interface SidebarProps {
  onCloseMobile?: () => void
  showCloseButton?: boolean
}

function Sidebar({ onCloseMobile, showCloseButton }: SidebarProps) {
  const { data: health, isError } = useVaultHealth()
  const vaultOnline = !isError && health?.status === 'ok'

  const navItems = [
    { to: '/app/overview',  icon: LayoutDashboard, label: 'Overview' },
    { to: '/app/vault',     icon: Database,        label: 'Vault' },
    { to: '/app/sessions',  icon: Clock,           label: 'Sessions' },
    { to: '/app/lore',      icon: BookOpen,        label: 'Lore' },
    { to: '/app/harness',   icon: GitBranch,       label: 'Harness' },
    { to: '/app/settings',  icon: Settings,        label: 'Settings' },
  ]

  return (
    <aside
      className="h-full flex flex-col"
      style={{ background: '#161B22', borderRight: '1px solid var(--border)' }}
    >
      <div
        style={{
          padding: '18px 16px',
          borderBottom: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 10,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div
            style={{
              width: 26, height: 26, borderRadius: 7, background: 'var(--accent)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 13, color: '#000',
            }}
          >
            K
          </div>
          <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 15, color: 'var(--text)' }}>
            Korva
          </span>
        </div>
        {showCloseButton && onCloseMobile && (
          <button
            type="button"
            onClick={onCloseMobile}
            aria-label="Close menu"
            className="md:hidden h-9 w-9 flex items-center justify-center rounded-md transition-colors hover:bg-white/5"
            style={{ color: 'var(--text-muted)' }}
          >
            <X size={16} />
          </button>
        )}
      </div>

      <nav style={{ flex: 1, padding: '12px 8px', display: 'flex', flexDirection: 'column', gap: 2 }}>
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            onClick={onCloseMobile}
            style={({ isActive }) => ({
              display: 'flex', alignItems: 'center', gap: 10,
              // 44px min touch target on mobile (Apple HIG / Material) — desktop uses tighter spacing.
              padding: '10px 12px', minHeight: 44,
              borderRadius: 7, textDecoration: 'none', fontSize: 13.5,
              fontFamily: 'var(--font-body)', transition: 'all 0.15s',
              background: isActive ? 'var(--border)' : 'transparent',
              color: isActive ? 'var(--text)' : 'var(--text-muted)',
            })}
          >
            <Icon size={15} />
            {label}
          </NavLink>
        ))}
      </nav>

      <div style={{ padding: '4px 8px 6px' }}>
        <NavLink
          to="/admin"
          onClick={onCloseMobile}
          style={{
            display: 'flex', alignItems: 'center', gap: 8,
            padding: '8px 12px', minHeight: 36,
            borderRadius: 7, textDecoration: 'none', fontSize: 12, color: 'var(--text-dim)',
            fontFamily: 'var(--font-body)',
          }}
          onMouseEnter={e => { (e.currentTarget as HTMLElement).style.color = '#f0883e' }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.color = 'var(--text-dim)' }}
        >
          <Shield size={12} />
          Admin
        </NavLink>
      </div>

      <div
        style={{
          padding: '12px 16px',
          borderTop: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}
      >
        <Activity size={12} style={{ color: vaultOnline ? 'var(--accent)' : 'var(--text-dim)' }} />
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 11,
            color: vaultOnline ? 'var(--accent)' : 'var(--text-dim)',
          }}
        >
          {vaultOnline ? 'Vault online' : 'Vault offline'}
        </span>
      </div>
    </aside>
  )
}
