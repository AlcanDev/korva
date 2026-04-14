import { BrowserRouter, Routes, Route, NavLink } from 'react-router'
import {
  LayoutDashboard, Database, Clock, BookOpen, Settings, Activity
} from 'lucide-react'
import { useVaultHealth } from '@/api/vault'
import Overview from '@/pages/overview/Overview'
import VaultExplorer from '@/pages/vault-explorer/VaultExplorer'
import Sessions from '@/pages/sessions/Sessions'
import LoreManager from '@/pages/lore-manager/LoreManager'
import SettingsPage from '@/pages/settings/Settings'

export default function App() {
  return (
    <BrowserRouter>
      <div className="flex h-screen overflow-hidden bg-[#0d1117]">
        <Sidebar />
        <main className="flex-1 overflow-auto">
          <Routes>
            <Route path="/" element={<Overview />} />
            <Route path="/vault" element={<VaultExplorer />} />
            <Route path="/sessions" element={<Sessions />} />
            <Route path="/lore" element={<LoreManager />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}

function Sidebar() {
  const { data: health, isError } = useVaultHealth()
  const vaultOnline = !isError && health?.status === 'ok'

  const navItems = [
    { to: '/', icon: LayoutDashboard, label: 'Overview' },
    { to: '/vault', icon: Database, label: 'Vault' },
    { to: '/sessions', icon: Clock, label: 'Sessions' },
    { to: '/lore', icon: BookOpen, label: 'Lore' },
    { to: '/settings', icon: Settings, label: 'Settings' },
  ]

  return (
    <aside className="w-56 flex-shrink-0 border-r border-[#21262d] flex flex-col bg-[#161b22]">
      {/* Logo */}
      <div className="px-4 py-5 border-b border-[#21262d]">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded bg-[#238636] flex items-center justify-center text-white text-xs font-bold">
            K
          </div>
          <span className="font-semibold text-[#e6edf3]">Korva Beacon</span>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors ${
                isActive
                  ? 'bg-[#21262d] text-[#e6edf3]'
                  : 'text-[#8b949e] hover:text-[#e6edf3] hover:bg-[#21262d]'
              }`
            }
          >
            <Icon size={15} />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* Vault status */}
      <div className="px-4 py-3 border-t border-[#21262d]">
        <div className="flex items-center gap-2 text-xs">
          <Activity
            size={12}
            className={vaultOnline ? 'text-[#3fb950]' : 'text-[#8b949e]'}
          />
          <span className={vaultOnline ? 'text-[#3fb950]' : 'text-[#8b949e]'}>
            {vaultOnline ? 'Vault online' : 'Vault offline'}
          </span>
        </div>
      </div>
    </aside>
  )
}
