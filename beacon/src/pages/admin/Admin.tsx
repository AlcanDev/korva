import _React, { useState } from 'react'
import { NavLink, Routes, Route, Navigate } from 'react-router'
import {
  LayoutDashboard, Database, BookOpen, Shield, LogOut,
  Users, Wand2, Lock, ClipboardList, KeyRound, Menu, X
} from 'lucide-react'
import { useAdminStore } from '@/stores/admin'
import { useLicenseStatus } from '@/api/license'
import AdminLogin from './AdminLogin'
import AdminDashboard from './AdminDashboard'
import AdminVault from './AdminVault'
import AdminScrolls from './AdminScrolls'
import AdminLicense from './AdminLicense'
import AdminTeams from './AdminTeams'
import AdminSkills from './AdminSkills'
import AdminScrollsPrivate from './AdminScrollsPrivate'
import AdminAudit from './AdminAudit'

export default function Admin() {
  const { isAuthenticated, logout } = useAdminStore()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  if (!isAuthenticated) {
    return <AdminLogin />
  }

  return (
    <div className="flex h-screen overflow-hidden bg-[#0d1117]">
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/60 z-20 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div
        className={`fixed inset-y-0 left-0 z-30 w-56 flex-shrink-0 transform transition-transform duration-200 md:relative md:translate-x-0 md:z-auto ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <AdminSidebar onLogout={logout} onNavigate={() => setSidebarOpen(false)} />
      </div>

      {/* Main content */}
      <div className="flex-1 flex flex-col overflow-hidden min-w-0">
        {/* Mobile top bar */}
        <div className="flex items-center gap-3 px-4 py-3 border-b border-[#21262d] bg-[#161b22] md:hidden flex-shrink-0">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-[#8b949e] hover:text-[#e6edf3] transition-colors"
            aria-label="Open menu"
          >
            <Menu size={20} />
          </button>
          <div className="flex items-center gap-2">
            <div className="w-5 h-5 rounded bg-[#f0883e30] border border-[#f0883e50] flex items-center justify-center">
              <Shield size={11} className="text-[#f0883e]" />
            </div>
            <span className="font-semibold text-[#e6edf3] text-sm">Korva Admin</span>
          </div>
        </div>

        <main className="flex-1 overflow-auto">
          <Routes>
            <Route path="/" element={<Navigate to="dashboard" replace />} />
            <Route path="dashboard" element={<AdminDashboard />} />
            <Route path="vault" element={<AdminVault />} />
            <Route path="scrolls" element={<AdminScrolls />} />
            <Route path="license" element={<AdminLicense />} />
            <Route path="teams" element={<TeamsGate><AdminTeams /></TeamsGate>} />
            <Route path="skills" element={<TeamsGate><AdminSkills /></TeamsGate>} />
            <Route path="scrolls-private" element={<TeamsGate><AdminScrollsPrivate /></TeamsGate>} />
            <Route path="audit" element={<TeamsGate><AdminAudit /></TeamsGate>} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

/** TeamsGate redirects to /admin/license when the license tier is community. */
function TeamsGate({ children }: { children: React.ReactNode }) {
  const { data, isLoading } = useLicenseStatus()

  if (isLoading) {
    return (
      <div className="p-6">
        <p className="text-[#8b949e] text-sm">Checking license…</p>
      </div>
    )
  }

  if (!data || data.tier !== 'teams') {
    return (
      <div className="p-4 sm:p-6 max-w-lg">
        <div className="rounded-lg border border-[#d29922] bg-[#d2992215] p-5">
          <div className="flex items-center gap-3 mb-3">
            <KeyRound size={18} className="text-[#d29922]" />
            <p className="text-[#e6edf3] font-medium text-sm">Korva for Teams required</p>
          </div>
          <p className="text-[#8b949e] text-xs mb-4">
            This section requires an active Korva for Teams license.
            Activate your license key to unlock Teams features.
          </p>
          <NavLink
            to="/admin/license"
            relative="route"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
          >
            <KeyRound size={12} />
            Go to License
          </NavLink>
        </div>
      </div>
    )
  }

  return <>{children}</>
}

function AdminSidebar({ onLogout, onNavigate }: { onLogout: () => void; onNavigate: () => void }) {
  const { data: lic } = useLicenseStatus()
  const isTeams = lic?.tier === 'teams'

  const communityItems = [
    { to: 'dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { to: 'vault', icon: Database, label: 'Vault Browser' },
    { to: 'scrolls', icon: BookOpen, label: 'Scrolls' },
    { to: 'license', icon: KeyRound, label: 'License' },
  ]

  const teamsItems = [
    { to: 'teams', icon: Users, label: 'Teams' },
    { to: 'skills', icon: Wand2, label: 'Skills' },
    { to: 'scrolls-private', icon: Lock, label: 'Private Scrolls' },
    { to: 'audit', icon: ClipboardList, label: 'Audit Log' },
  ]

  return (
    <aside className="w-56 h-full flex-shrink-0 border-r border-[#21262d] flex flex-col bg-[#161b22]">
      {/* Logo */}
      <div className="px-4 py-5 border-b border-[#21262d] flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded bg-[#f0883e30] border border-[#f0883e50] flex items-center justify-center">
            <Shield size={13} className="text-[#f0883e]" />
          </div>
          <div>
            <span className="font-semibold text-[#e6edf3] text-sm">Korva Admin</span>
            <div className="text-[10px] text-[#f0883e]">Private access</div>
          </div>
        </div>
        {/* Close button — mobile only */}
        <button
          onClick={onNavigate}
          className="md:hidden text-[#8b949e] hover:text-[#e6edf3] transition-colors"
          aria-label="Close menu"
        >
          <X size={16} />
        </button>
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5 overflow-y-auto">
        {communityItems.map(({ to, icon: Icon, label }) => (
          <SidebarLink key={to} to={to} icon={Icon} label={label} onClick={onNavigate} />
        ))}

        {isTeams && (
          <>
            <div className="px-3 pt-3 pb-1">
              <p className="text-[10px] text-[#484f58] uppercase tracking-wider">Teams</p>
            </div>
            {teamsItems.map(({ to, icon: Icon, label }) => (
              <SidebarLink key={to} to={to} icon={Icon} label={label} onClick={onNavigate} />
            ))}
          </>
        )}
      </nav>

      {/* Tier badge */}
      <div className="px-4 py-2 border-t border-[#21262d]">
        <span className={`text-[10px] px-2 py-0.5 rounded-full border ${
          isTeams
            ? 'bg-[#388bfd20] text-[#388bfd] border-[#388bfd30]'
            : 'bg-[#21262d] text-[#484f58] border-[#30363d]'
        }`}>
          {isTeams ? 'Teams' : 'Community'}
        </span>
      </div>

      {/* Logout */}
      <div className="px-2 py-3 border-t border-[#21262d]">
        <button
          onClick={onLogout}
          className="flex items-center gap-2.5 w-full px-3 py-2 rounded-md text-sm text-[#8b949e] hover:text-[#f85149] hover:bg-[#f8514910] transition-colors"
        >
          <LogOut size={15} />
          Sign out
        </button>
      </div>
    </aside>
  )
}

function SidebarLink({ to, icon: Icon, label, onClick }: { to: string; icon: React.ElementType; label: string; onClick: () => void }) {
  return (
    <NavLink
      to={`/admin/${to}`}
      onClick={onClick}
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
  )
}
