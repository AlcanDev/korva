import _React, { useState } from 'react'
import { NavLink, Routes, Route, Navigate } from 'react-router'
import {
  LayoutDashboard, Database, BookOpen, LogOut,
  Users, Wand2, Lock, ClipboardList, KeyRound, Menu, X, Activity, Clock
} from 'lucide-react'
import { KorvaLogo } from '@/components/KorvaLogo'
import { useAdminStore } from '@/stores/admin'
import { useLicenseStatus, isPaidTier } from '@/api/license'
import { useI18n, type EditorKey, EDITOR_INTEGRATION } from '@/contexts/i18n'
import AdminLogin from './AdminLogin'
import AdminDashboard from './AdminDashboard'
import AdminVault from './AdminVault'
import AdminScrolls from './AdminScrolls'
import AdminLicense from './AdminLicense'
import AdminTeams from './AdminTeams'
import AdminSkills from './AdminSkills'
import AdminScrollsPrivate from './AdminScrollsPrivate'
import AdminAudit from './AdminAudit'
import AdminCodeHealth from './AdminCodeHealth'
import AdminSessions from './AdminSessions'

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
            <KorvaLogo size={20} />
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
            <Route path="code-health" element={<AdminCodeHealth />} />
            <Route path="sessions" element={<AdminSessions />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

/** Shows upgrade prompt when not on Teams tier. */
function TeamsGate({ children }: { children: React.ReactNode }) {
  const { data, isLoading } = useLicenseStatus()
  const { t } = useI18n()

  if (isLoading) {
    return (
      <div className="p-6">
        <p className="text-[#8b949e] text-sm">{t.license.checkingLicense}</p>
      </div>
    )
  }

  if (!data || !isPaidTier(data.tier)) {
    return (
      <div className="p-4 sm:p-6 max-w-lg">
        <div className="rounded-lg border border-[#d29922] bg-[#d2992215] p-5">
          <div className="flex items-center gap-3 mb-3">
            <KeyRound size={18} className="text-[#d29922]" />
            <p className="text-[#e6edf3] font-medium text-sm">{t.license.teamsRequiredTitle}</p>
          </div>
          <p className="text-[#8b949e] text-xs mb-4">
            {t.license.teamsRequiredDesc}
          </p>
          <NavLink
            to="/admin/license"
            relative="route"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[#238636] text-white hover:bg-[#2ea043] transition-colors"
          >
            <KeyRound size={12} />
            {t.license.goToLicense}
          </NavLink>
        </div>
      </div>
    )
  }

  return <>{children}</>
}

const EDITOR_LABELS: Record<EditorKey, string> = Object.fromEntries(
  Object.entries(EDITOR_INTEGRATION).map(([k, v]) => [k, v.name])
) as Record<EditorKey, string>

function AdminSidebar({ onLogout, onNavigate }: { onLogout: () => void; onNavigate: () => void }) {
  const { data: lic } = useLicenseStatus()
  const { t, lang, setLang, editor, setEditor } = useI18n()
  const isTeams = isPaidTier(lic?.tier)

  const communityItems = [
    { to: 'dashboard',   icon: LayoutDashboard, label: t.nav.dashboard },
    { to: 'vault',       icon: Database,        label: t.nav.vaultBrowser,  subtitle: t.nav.vaultSubtitle },
    { to: 'sessions',    icon: Clock,           label: t.nav.sessions,      subtitle: t.nav.sessionsSubtitle },
    { to: 'scrolls',     icon: BookOpen,        label: t.nav.scrolls,       subtitle: t.nav.scrollsSubtitle },
    { to: 'code-health', icon: Activity,        label: t.nav.codeHealth,    subtitle: t.nav.codeHealthSubtitle },
    { to: 'license',     icon: KeyRound,        label: t.nav.license },
  ]

  const teamsItems = [
    { to: 'teams',           icon: Users,         label: t.nav.teams,          subtitle: t.nav.teamsSubtitle },
    { to: 'skills',          icon: Wand2,         label: t.nav.skills,         subtitle: t.nav.skillsSubtitle },
    { to: 'scrolls-private', icon: Lock,          label: t.nav.privateScrolls, subtitle: t.nav.privateScrollsSubtitle },
    { to: 'audit',           icon: ClipboardList, label: t.nav.auditLog },
  ]

  return (
    <aside className="w-56 h-full flex-shrink-0 border-r border-[#21262d] flex flex-col bg-[#161b22]">
      {/* Logo + language toggle */}
      <div className="px-4 py-5 border-b border-[#21262d] flex items-center justify-between">
        <div className="flex items-center gap-2 min-w-0">
          <KorvaLogo size={24} className="flex-shrink-0" />
          <div className="min-w-0">
            <span className="font-semibold text-[#e6edf3] text-sm block truncate">{t.nav.korvaAdmin}</span>
            <div className="text-[10px] text-[#f0883e]">{t.nav.privateAccess}</div>
          </div>
        </div>
        <div className="flex items-center gap-1 flex-shrink-0">
          {/* Close button — mobile only */}
          <button
            onClick={onNavigate}
            className="md:hidden text-[#8b949e] hover:text-[#e6edf3] transition-colors"
            aria-label={t.common.closeMenu}
          >
            <X size={16} />
          </button>
          {/* Language toggle */}
          <button
            onClick={() => setLang(lang === 'en' ? 'es' : 'en')}
            className="text-[10px] font-mono text-[#484f58] hover:text-[#8b949e] border border-[#30363d] hover:border-[#484f58] rounded px-1.5 py-0.5 transition-colors"
            title={lang === 'en' ? 'Cambiar a Español' : 'Switch to English'}
          >
            {lang === 'en' ? 'ES' : 'EN'}
          </button>
        </div>
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5 overflow-y-auto">
        {communityItems.map(({ to, icon: Icon, label, subtitle }) => (
          <SidebarLink key={to} to={to} icon={Icon} label={label} subtitle={subtitle} onClick={onNavigate} />
        ))}

        {/* Teams section — always visible, locked when Community */}
        <div className="px-3 pt-3 pb-1">
          <div className="flex items-center gap-1.5">
            <p className="text-[10px] text-[#484f58] uppercase tracking-wider">{t.nav.teamsSection}</p>
            {!isTeams && <Lock size={9} className="text-[#484f58]" />}
          </div>
        </div>
        {teamsItems.map(({ to, icon: Icon, label, subtitle }) => (
          isTeams
            ? <SidebarLink key={to} to={to} icon={Icon} label={label} subtitle={subtitle} onClick={onNavigate} />
            : <LockedSidebarLink key={to} to={to} icon={Icon} label={label} subtitle={subtitle} onClick={onNavigate} />
        ))}
      </nav>

      {/* Tier badge + version */}
      <div className="px-4 py-2 border-t border-[#21262d] flex items-center justify-between">
        <span className={`text-[10px] px-2 py-0.5 rounded-full border ${
          isTeams
            ? 'bg-[#388bfd20] text-[#388bfd] border-[#388bfd30]'
            : 'bg-[#21262d] text-[#484f58] border-[#30363d]'
        }`}>
          {isTeams ? 'Teams' : 'Community'}
        </span>
        <span className="text-[9px] text-[#30363d] font-mono">v1.0.0</span>
      </div>

      {/* Editor preference */}
      <div className="px-4 py-2 border-t border-[#21262d]">
        <p className="text-[9px] text-[#484f58] uppercase tracking-wider mb-1">{t.editor.label}</p>
        <select
          value={editor}
          onChange={e => setEditor(e.target.value as EditorKey)}
          className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-[10px] text-[#8b949e] focus:outline-none focus:border-[#388bfd] transition-colors"
        >
          {(Object.entries(EDITOR_LABELS) as [EditorKey, string][]).map(([key, label]) => (
            <option key={key} value={key}>{label}</option>
          ))}
        </select>
      </div>

      {/* Logout */}
      <div className="px-2 py-3 border-t border-[#21262d]">
        <button
          onClick={onLogout}
          className="flex items-center gap-2.5 w-full px-3 py-2 rounded-md text-sm text-[#8b949e] hover:text-[#f85149] hover:bg-[#f8514910] transition-colors"
        >
          <LogOut size={15} />
          {t.nav.signOut}
        </button>
      </div>
    </aside>
  )
}

function SidebarLink({ to, icon: Icon, label, subtitle, onClick }: {
  to: string
  icon: React.ElementType
  label: string
  subtitle?: string
  onClick: () => void
}) {
  return (
    <NavLink
      to={`/admin/${to}`}
      onClick={onClick}
      className={({ isActive }) =>
        `flex items-center gap-2.5 px-3 py-2 rounded-md transition-colors ${
          isActive
            ? 'bg-[#21262d] text-[#e6edf3]'
            : 'text-[#8b949e] hover:text-[#e6edf3] hover:bg-[#21262d]'
        }`
      }
    >
      <Icon size={15} className="flex-shrink-0" />
      <div className="flex-1 min-w-0">
        <span className="block text-sm truncate leading-tight">{label}</span>
        {subtitle && <span className="block text-[9px] text-[#484f58] truncate leading-tight mt-px">{subtitle}</span>}
      </div>
    </NavLink>
  )
}

/** Navigates to the page but shows lock icon — TeamsGate handles the upgrade prompt. */
function LockedSidebarLink({ to, icon: Icon, label, subtitle, onClick }: {
  to: string
  icon: React.ElementType
  label: string
  subtitle?: string
  onClick: () => void
}) {
  return (
    <NavLink
      to={`/admin/${to}`}
      onClick={onClick}
      className={({ isActive }) =>
        `flex items-center gap-2.5 px-3 py-2 rounded-md transition-colors ${
          isActive
            ? 'bg-[#21262d] text-[#8b949e]'
            : 'text-[#484f58] hover:text-[#8b949e] hover:bg-[#21262d]'
        }`
      }
    >
      <Icon size={15} className="flex-shrink-0" />
      <div className="flex-1 min-w-0">
        <span className="block text-sm truncate leading-tight">{label}</span>
        {subtitle && <span className="block text-[9px] truncate leading-tight mt-px opacity-60">{subtitle}</span>}
      </div>
      <Lock size={11} className="flex-shrink-0 opacity-60" />
    </NavLink>
  )
}
