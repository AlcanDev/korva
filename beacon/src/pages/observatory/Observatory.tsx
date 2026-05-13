import { NavLink, Routes, Route, Navigate } from 'react-router'
import { Activity, Coins, Clock, Settings, Shield, GitMerge, FolderTree, Download } from 'lucide-react'
import SystemHealth from './SystemHealth'
import TokenAnalytics from './TokenAnalytics'
import ActivityTimeline from './ActivityTimeline'
import ConfigEditor from './ConfigEditor'
import SentinelRulesEditor from './SentinelRulesEditor'
import ConflictsPanel from './ConflictsPanel'
import ProjectsPanel from './ProjectsPanel'
import ExportPanel from './ExportPanel'

// Base path is hard-coded to match the parent route in Admin.tsx
// (`<Route path="observatory/*" element={<Observatory />} />`). Using absolute
// hrefs sidesteps React Router 7's splat-relative resolution, which would
// otherwise append the new segment to the current URL (e.g. clicking "Tokens"
// from /admin/observatory/health would navigate to
// /admin/observatory/health/tokens instead of /admin/observatory/tokens).
export const OBSERVATORY_BASE = '/admin/observatory'

const SUBNAV = [
  { path: 'health', label: 'System Health', icon: Activity },
  { path: 'tokens', label: 'Tokens', icon: Coins },
  { path: 'activity', label: 'Activity', icon: Clock },
  { path: 'conflicts', label: 'Conflicts', icon: GitMerge },
  { path: 'projects', label: 'Projects', icon: FolderTree },
  { path: 'export', label: 'Export', icon: Download },
  { path: 'config', label: 'Configuration', icon: Settings },
  { path: 'sentinel', label: 'Sentinel Rules', icon: Shield },
] as const

export default function Observatory() {
  return (
    <div className="flex flex-col h-full">
      <nav
        aria-label="Observatory sections"
        className="border-b border-[#21262d] bg-[#161b22] px-4 flex gap-1 overflow-x-auto flex-shrink-0"
      >
        {SUBNAV.map(({ path, label, icon: Icon }) => (
          <NavLink
            key={path}
            to={`${OBSERVATORY_BASE}/${path}`}
            end
            className={({ isActive }) =>
              `inline-flex items-center gap-2 px-3 py-2.5 text-xs whitespace-nowrap border-b-2 transition-colors ${
                isActive
                  ? 'text-[#e6edf3] border-[#388bfd]'
                  : 'text-[#8b949e] border-transparent hover:text-[#e6edf3]'
              }`
            }
          >
            <Icon size={13} />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="flex-1 overflow-auto">
        <Routes>
          <Route index element={<Navigate to={`${OBSERVATORY_BASE}/health`} replace />} />
          <Route path="health" element={<SystemHealth />} />
          <Route path="tokens" element={<TokenAnalytics />} />
          <Route path="activity" element={<ActivityTimeline />} />
          <Route path="conflicts" element={<ConflictsPanel />} />
          <Route path="projects" element={<ProjectsPanel />} />
          <Route path="export" element={<ExportPanel />} />
          <Route path="config" element={<ConfigEditor />} />
          <Route path="sentinel" element={<SentinelRulesEditor />} />
          {/* Anything else under /admin/observatory falls back to health so a
             stale bookmark or a fat-fingered URL never lands on a blank screen. */}
          <Route path="*" element={<Navigate to={`${OBSERVATORY_BASE}/health`} replace />} />
        </Routes>
      </div>
    </div>
  )
}
