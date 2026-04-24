import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router'
import { I18nProvider } from '@/contexts/i18n'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import {
  LayoutDashboard, Database, Clock, BookOpen, Settings, Activity, Shield
} from 'lucide-react'
import { useVaultHealth } from '@/api/vault'
import LandingPage from '@/pages/landing/LandingPage'
import Overview from '@/pages/overview/Overview'
import VaultExplorer from '@/pages/vault-explorer/VaultExplorer'
import Sessions from '@/pages/sessions/Sessions'
import LoreManager from '@/pages/lore-manager/LoreManager'
import SettingsPage from '@/pages/settings/Settings'
import Admin from '@/pages/admin/Admin'

export default function App() {
  return (
    <ErrorBoundary>
    <I18nProvider>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/admin/*" element={<Admin />} />
        <Route path="/app" element={<Navigate to="/app/overview" replace />} />
        <Route
          path="/app/*"
          element={
            <div style={{ display: 'flex', height: '100vh', overflow: 'hidden', background: 'var(--surface)' }}>
              <Sidebar />
              <main style={{ flex: 1, overflowY: 'auto' }}>
                <Routes>
                  <Route path="overview"  element={<Overview />} />
                  <Route path="vault"     element={<VaultExplorer />} />
                  <Route path="sessions"  element={<Sessions />} />
                  <Route path="lore"      element={<LoreManager />} />
                  <Route path="settings"  element={<SettingsPage />} />
                  <Route path="*"         element={<Navigate to="/app/overview" replace />} />
                </Routes>
              </main>
            </div>
          }
        />
      </Routes>
    </BrowserRouter>
    </I18nProvider>
    </ErrorBoundary>
  )
}

function Sidebar() {
  const { data: health, isError } = useVaultHealth()
  const vaultOnline = !isError && health?.status === 'ok'

  const navItems = [
    { to: '/app/overview',  icon: LayoutDashboard, label: 'Overview' },
    { to: '/app/vault',     icon: Database,        label: 'Vault' },
    { to: '/app/sessions',  icon: Clock,           label: 'Sessions' },
    { to: '/app/lore',      icon: BookOpen,        label: 'Lore' },
    { to: '/app/settings',  icon: Settings,        label: 'Settings' },
  ]

  return (
    <aside style={{
      width: 220, flexShrink: 0, display: 'flex', flexDirection: 'column',
      background: '#161B22', borderRight: '1px solid var(--border)',
    }}>
      <div style={{ padding: '18px 16px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 10 }}>
        <div style={{
          width: 26, height: 26, borderRadius: 7, background: 'var(--accent)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 13, color: '#000',
        }}>K</div>
        <span style={{ fontFamily: 'var(--font-display)', fontWeight: 700, fontSize: 15, color: 'var(--text)' }}>Korva</span>
      </div>

      <nav style={{ flex: 1, padding: '12px 8px', display: 'flex', flexDirection: 'column', gap: 2 }}>
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            style={({ isActive }) => ({
              display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px',
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
          style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 12px', borderRadius: 7, textDecoration: 'none', fontSize: 12, color: 'var(--text-dim)', fontFamily: 'var(--font-body)' }}
          onMouseEnter={e => { (e.currentTarget as HTMLElement).style.color = '#f0883e' }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.color = 'var(--text-dim)' }}
        >
          <Shield size={12} />
          Admin
        </NavLink>
      </div>

      <div style={{ padding: '12px 16px', borderTop: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 8 }}>
        <Activity size={12} style={{ color: vaultOnline ? 'var(--accent)' : 'var(--text-dim)' }} />
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: vaultOnline ? 'var(--accent)' : 'var(--text-dim)' }}>
          {vaultOnline ? 'Vault online' : 'Vault offline'}
        </span>
      </div>
    </aside>
  )
}
