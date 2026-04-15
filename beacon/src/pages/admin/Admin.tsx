import { useState } from 'react'
import { NavLink, Routes, Route, Navigate } from 'react-router'
import {
  LayoutDashboard, Database, BookOpen, Shield, LogOut
} from 'lucide-react'
import { useAdminStore } from '@/stores/admin'
import AdminLogin from './AdminLogin'
import AdminDashboard from './AdminDashboard'
import AdminVault from './AdminVault'
import AdminScrolls from './AdminScrolls'

export default function Admin() {
  const { isAuthenticated, logout } = useAdminStore()

  if (!isAuthenticated) {
    return <AdminLogin />
  }

  return (
    <div className="flex h-screen overflow-hidden bg-[#0d1117]">
      <AdminSidebar onLogout={logout} />
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Navigate to="dashboard" replace />} />
          <Route path="dashboard" element={<AdminDashboard />} />
          <Route path="vault" element={<AdminVault />} />
          <Route path="scrolls" element={<AdminScrolls />} />
        </Routes>
      </main>
    </div>
  )
}

function AdminSidebar({ onLogout }: { onLogout: () => void }) {
  const navItems = [
    { to: 'dashboard', icon: LayoutDashboard, label: 'Dashboard' },
    { to: 'vault', icon: Database, label: 'Vault Browser' },
    { to: 'scrolls', icon: BookOpen, label: 'Scrolls & Instructions' },
  ]

  return (
    <aside className="w-56 flex-shrink-0 border-r border-[#21262d] flex flex-col bg-[#161b22]">
      {/* Logo */}
      <div className="px-4 py-5 border-b border-[#21262d]">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded bg-[#f0883e30] border border-[#f0883e50] flex items-center justify-center">
            <Shield size={13} className="text-[#f0883e]" />
          </div>
          <div>
            <span className="font-semibold text-[#e6edf3] text-sm">Korva Admin</span>
            <div className="text-[10px] text-[#f0883e]">Private access</div>
          </div>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
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
