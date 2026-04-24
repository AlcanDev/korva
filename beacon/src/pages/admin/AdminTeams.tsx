import _React, { useState } from 'react'
import { Users, UserPlus, Trash2, ChevronRight, Mail, Copy, Check, X, RefreshCw, Clock, Activity, ShieldOff } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { useI18n } from '@/contexts/i18n'
import {
  useTeams, useTeamMembers, useCreateTeam, useAddMember, useRemoveMember,
  useTeamInvites, useCreateInvite, useRevokeInvite,
  useTeamSessions, useRevokeTeamSession,
} from '@/api/teams'
import { useLicenseStatus } from '@/api/license'
import type { Team, Invite, MemberSession } from '@/api/teams'

type Tab = 'members' | 'invites' | 'sessions'

export default function AdminTeams() {
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null)
  const [tab, setTab] = useState<Tab>('members')
  const { t } = useI18n()

  const handleSelect = (team: Team) => {
    setSelectedTeam(team)
    setTab('members')
  }

  return (
    <div className="p-4 sm:p-6 max-w-5xl">
      <PageHeader
        icon={<Users size={17} />}
        iconColor="#f0883e"
        title={t.teams.title}
        description={t.teams.description}
        badge="Teams"
        badgeColor="orange"
        hint={{ command: 'korva auth redeem <token>', label: t.teams.hintLabel }}
      />
      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        <div className="md:col-span-2">
          <TeamList selected={selectedTeam} onSelect={handleSelect} />
        </div>
        <div className="md:col-span-3">
          {selectedTeam ? (
            <div className="space-y-3">
              <TabBar tab={tab} onChange={setTab} />
              {tab === 'members' && <TeamMembers team={selectedTeam} />}
              {tab === 'invites' && <TeamInvites team={selectedTeam} />}
              {tab === 'sessions' && <TeamSessions team={selectedTeam} />}
            </div>
          ) : (
            <Empty />
          )}
        </div>
      </div>
    </div>
  )
}

function TabBar({ tab, onChange }: { tab: Tab; onChange: (t: Tab) => void }) {
  const { t } = useI18n()
  const tabs: { id: Tab; label: string; icon: React.ElementType }[] = [
    { id: 'members', label: t.teams.tabMembers, icon: Users },
    { id: 'invites', label: t.teams.tabInvites, icon: Mail },
    { id: 'sessions', label: t.teams.tabSessions, icon: Activity },
  ]
  return (
    <div className="flex gap-1 bg-[#161b22] border border-[#21262d] rounded-lg p-1">
      {tabs.map(({ id, label, icon: Icon }) => (
        <button
          key={id}
          onClick={() => onChange(id)}
          className={`flex items-center gap-1.5 flex-1 justify-center py-1.5 rounded text-xs transition-colors ${
            tab === id ? 'bg-[#21262d] text-[#e6edf3]' : 'text-[#8b949e] hover:text-[#e6edf3]'
          }`}
        >
          <Icon size={12} /> {label}
        </button>
      ))}
    </div>
  )
}

// ── Team list (left panel) ────────────────────────────────────────────────────

function TeamList({ selected, onSelect }: { selected: Team | null; onSelect: (team: Team) => void }) {
  const { data, isLoading } = useTeams()
  const createTeam = useCreateTeam()
  const [newName, setNewName] = useState('')
  const [newOwner, setNewOwner] = useState('')
  const [adding, setAdding] = useState(false)
  const { t } = useI18n()

  const handleCreate = async () => {
    if (!newName.trim()) return
    await createTeam.mutateAsync({ name: newName.trim(), owner: newOwner.trim() })
    setNewName('')
    setNewOwner('')
    setAdding(false)
  }

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#21262d]">
        <div className="flex items-center gap-2">
          <Users size={14} className="text-[#8b949e]" />
          <span className="text-[#e6edf3] text-sm font-medium">{t.teams.listHeader}</span>
          {data && <span className="text-[#484f58] text-xs">({data.teams.length})</span>}
        </div>
        <button onClick={() => setAdding(v => !v)} className="text-[#8b949e] hover:text-[#388bfd] transition-colors" title={t.teams.newTeam}>
          <UserPlus size={14} />
        </button>
      </div>

      {adding && (
        <div className="p-3 border-b border-[#21262d] space-y-2">
          <input
            placeholder={t.teams.teamNamePlaceholder}
            value={newName}
            onChange={e => setNewName(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
            autoFocus
          />
          <input
            placeholder={t.teams.ownerPlaceholder}
            value={newOwner}
            onChange={e => setNewOwner(e.target.value)}
            className="w-full bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
          />
          <div className="flex gap-2">
            <button
              onClick={handleCreate}
              disabled={!newName.trim() || createTeam.isPending}
              className="flex-1 py-1 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors"
            >
              {createTeam.isPending ? t.common.creating : t.common.create}
            </button>
            <button onClick={() => setAdding(false)} className="px-2 py-1 rounded text-xs text-[#8b949e] hover:text-[#e6edf3] transition-colors">
              {t.common.cancel}
            </button>
          </div>
        </div>
      )}

      {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}
      {data?.teams.map(team => (
        <button
          key={team.id}
          onClick={() => onSelect(team)}
          className={`w-full flex items-center justify-between px-4 py-2.5 text-left border-b border-[#21262d] last:border-0 transition-colors ${
            selected?.id === team.id ? 'bg-[#21262d]' : 'hover:bg-[#1c2128]'
          }`}
        >
          <div>
            <p className="text-[#e6edf3] text-xs font-medium">{team.name}</p>
            {team.owner && <p className="text-[10px] text-[#8b949e]">{team.owner}</p>}
          </div>
          <ChevronRight size={12} className="text-[#484f58]" />
        </button>
      ))}
      {data?.teams.length === 0 && !isLoading && !adding && (
        <p className="px-4 py-3 text-xs text-[#484f58]">{t.teams.noTeams}</p>
      )}
    </div>
  )
}

// ── Seat counter ──────────────────────────────────────────────────────────────

function SeatBadge({ teamId }: { teamId: string }) {
  const { data: members } = useTeamMembers(teamId)
  const { data: lic } = useLicenseStatus()
  const { t } = useI18n()
  const count = members?.members.length ?? 0
  const seats = lic?.seats

  if (!seats) return null

  const pct = Math.round((count / seats) * 100)
  const over = count >= seats
  return (
    <div className="flex items-center gap-2">
      <span className={`text-[10px] ${over ? 'text-[#f85149]' : 'text-[#8b949e]'}`}>
        {t.teams.seatsDisplay(count, seats)}
      </span>
      <div className="w-20 h-1 bg-[#21262d] rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${over ? 'bg-[#f85149]' : pct > 80 ? 'bg-[#d29922]' : 'bg-[#3fb950]'}`}
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
    </div>
  )
}

// ── Members tab ───────────────────────────────────────────────────────────────

function TeamMembers({ team }: { team: Team }) {
  const { data, isLoading } = useTeamMembers(team.id)
  const addMember = useAddMember(team.id)
  const removeMember = useRemoveMember(team.id)
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState('member')
  const [err, setErr] = useState('')
  const { t } = useI18n()

  const handleAdd = async () => {
    if (!newEmail.trim()) return
    setErr('')
    try {
      await addMember.mutateAsync({ email: newEmail.trim(), role: newRole })
      setNewEmail('')
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      setErr(msg.includes('402') ? t.teams.seatLimitReached : t.teams.addMemberFailed)
    }
  }

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="px-4 py-3 border-b border-[#21262d] flex items-center justify-between">
        <div>
          <p className="text-[#e6edf3] text-sm font-medium">{team.name}</p>
          {team.license_id && <p className="text-[10px] text-[#8b949e] font-mono mt-0.5">{team.license_id}</p>}
        </div>
        <SeatBadge teamId={team.id} />
      </div>

      <div className="p-4 border-b border-[#21262d]">
        <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-2">{t.teams.addMember}</p>
        <div className="flex gap-2">
          <input
            placeholder={t.teams.emailPlaceholder}
            value={newEmail}
            onChange={e => setNewEmail(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleAdd()}
            className="flex-1 bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
          />
          <select
            value={newRole}
            onChange={e => setNewRole(e.target.value)}
            className="bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] focus:outline-none"
          >
            <option value="member">{t.teams.roleMember}</option>
            <option value="admin">{t.teams.roleAdmin}</option>
          </select>
          <button
            onClick={handleAdd}
            disabled={addMember.isPending || !newEmail.trim()}
            className="px-3 py-1 rounded text-xs bg-[#21262d] text-[#e6edf3] hover:bg-[#30363d] disabled:opacity-50 transition-colors"
          >
            {addMember.isPending ? '…' : t.common.add}
          </button>
        </div>
        {err && <p className="mt-1.5 text-[10px] text-[#f85149]">{err}</p>}
        <p className="mt-1.5 text-[10px] text-[#484f58]">{t.teams.addAfterNote}</p>
      </div>

      <div className="divide-y divide-[#21262d] max-h-80 overflow-y-auto">
        {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}
        {data?.members.map(m => (
          <div key={m.id} className="flex items-center justify-between px-4 py-2.5">
            <div>
              <p className="text-[#e6edf3] text-xs">{m.email}</p>
              <span className={`inline-block text-[10px] px-1.5 py-0.5 rounded mt-0.5 ${
                m.role === 'admin' ? 'bg-[#a371f720] text-[#a371f7]' : 'bg-[#21262d] text-[#8b949e]'
              }`}>{m.role}</span>
            </div>
            <button onClick={() => removeMember.mutate(m.id)} className="text-[#484f58] hover:text-[#f85149] transition-colors" title={t.common.delete}>
              <Trash2 size={13} />
            </button>
          </div>
        ))}
        {data?.members.length === 0 && !isLoading && (
          <p className="px-4 py-3 text-xs text-[#484f58]">{t.teams.noMembers}</p>
        )}
      </div>
    </div>
  )
}

// ── Invites tab ───────────────────────────────────────────────────────────────

function TeamInvites({ team }: { team: Team }) {
  const { data, isLoading, refetch } = useTeamInvites(team.id)
  const createInvite = useCreateInvite(team.id)
  const revokeInvite = useRevokeInvite(team.id)
  const [email, setEmail] = useState('')
  const [newToken, setNewToken] = useState<{ token: string; email: string } | null>(null)
  const [err, setErr] = useState('')
  const { t } = useI18n()

  const handleCreate = async () => {
    if (!email.trim()) return
    setErr('')
    setNewToken(null)
    try {
      const result = await createInvite.mutateAsync(email.trim())
      if (result.token) {
        setNewToken({ token: result.token, email: email.trim() })
        setEmail('')
      }
    } catch {
      setErr(t.teams.inviteFailed)
    }
  }

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#21262d]">
        <div className="flex items-center gap-2">
          <Mail size={14} className="text-[#8b949e]" />
          <span className="text-[#e6edf3] text-sm font-medium">{t.teams.tabInvites}</span>
          {data && <span className="text-[#484f58] text-xs">({data.count})</span>}
        </div>
        <button onClick={() => refetch()} className="text-[#484f58] hover:text-[#8b949e] transition-colors"><RefreshCw size={12} /></button>
      </div>

      {newToken && (
        <div className="mx-4 mt-4 rounded-lg border border-[#3fb95040] bg-[#3fb95010] p-4">
          <p className="text-[#3fb950] text-xs font-medium mb-1">{t.teams.inviteTokenLabel(newToken.email)}</p>
          <p className="text-[10px] text-[#8b949e] mb-2">{t.teams.inviteTokenNote}</p>
          <div className="flex items-center gap-2">
            <code className="flex-1 bg-[#0d1117] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono break-all select-all">{newToken.token}</code>
            <CopyButton text={newToken.token} />
          </div>
          <p className="mt-2 text-[10px] text-[#484f58]">{t.teams.inviteCommand(newToken.token)}</p>
          <button onClick={() => setNewToken(null)} className="mt-2 text-[10px] text-[#484f58] hover:text-[#8b949e] transition-colors">{t.common.dismiss}</button>
        </div>
      )}

      <div className="p-4 border-b border-[#21262d]">
        <p className="text-[10px] text-[#8b949e] uppercase tracking-wider mb-2">{t.teams.newInvite}</p>
        <div className="flex gap-2">
          <input
            placeholder={t.teams.inviteEmailPlaceholder}
            value={email}
            onChange={e => setEmail(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            className="flex-1 bg-[#0d1117] border border-[#30363d] rounded px-2 py-1 text-xs text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd]"
          />
          <button
            onClick={handleCreate}
            disabled={createInvite.isPending || !email.trim()}
            className="px-3 py-1 rounded text-xs bg-[#238636] text-white hover:bg-[#2ea043] disabled:opacity-50 transition-colors"
          >
            {createInvite.isPending ? '…' : t.common.generate}
          </button>
        </div>
        {err && <p className="mt-1.5 text-[10px] text-[#f85149]">{err}</p>}
      </div>

      <div className="divide-y divide-[#21262d] max-h-72 overflow-y-auto">
        {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}
        {data?.invites.map(inv => <InviteRow key={inv.id} invite={inv} onRevoke={() => revokeInvite.mutate(inv.id)} />)}
        {data?.invites.length === 0 && !isLoading && <p className="px-4 py-3 text-xs text-[#484f58]">{t.teams.noInvites}</p>}
      </div>
    </div>
  )
}

function InviteRow({ invite, onRevoke }: { invite: Invite; onRevoke: () => void }) {
  const { t } = useI18n()
  const colors = { pending: 'text-[#d29922] bg-[#d2992215]', used: 'text-[#3fb950] bg-[#3fb95015]', expired: 'text-[#484f58] bg-[#21262d]' }
  const statusLabel = invite.status === 'pending' ? t.teams.statusPending
    : invite.status === 'used' ? t.teams.statusUsed
    : t.teams.statusExpired
  return (
    <div className="flex items-center justify-between px-4 py-2.5">
      <div className="min-w-0">
        <p className="text-[#e6edf3] text-xs truncate">{invite.email}</p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className={`text-[10px] px-1.5 py-0.5 rounded ${colors[invite.status]}`}>{statusLabel}</span>
          <span className="text-[10px] text-[#484f58] flex items-center gap-1"><Clock size={9} />{new Date(invite.expires_at).toLocaleDateString()}</span>
        </div>
      </div>
      {invite.status === 'pending' && (
        <button onClick={onRevoke} className="ml-2 text-[#484f58] hover:text-[#f85149] transition-colors flex-shrink-0" title={t.common.revoke}>
          <X size={13} />
        </button>
      )}
    </div>
  )
}

// ── Sessions tab ──────────────────────────────────────────────────────────────

function TeamSessions({ team }: { team: Team }) {
  const { data, isLoading, refetch } = useTeamSessions(team.id)
  const revoke = useRevokeTeamSession(team.id)
  const { t } = useI18n()

  const active = data?.sessions.filter(s => s.status === 'active') ?? []
  const expired = data?.sessions.filter(s => s.status === 'expired') ?? []

  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#21262d]">
        <div className="flex items-center gap-2">
          <Activity size={14} className="text-[#8b949e]" />
          <span className="text-[#e6edf3] text-sm font-medium">{t.teams.activeSessions}</span>
          {data && (
            <span className={`text-[10px] px-1.5 py-0.5 rounded ${active.length > 0 ? 'bg-[#3fb95020] text-[#3fb950]' : 'bg-[#21262d] text-[#484f58]'}`}>
              {t.teams.activeCount(active.length)}
            </span>
          )}
        </div>
        <button onClick={() => refetch()} className="text-[#484f58] hover:text-[#8b949e] transition-colors"><RefreshCw size={12} /></button>
      </div>

      {isLoading && <p className="px-4 py-3 text-xs text-[#8b949e]">{t.common.loading}</p>}

      <div className="divide-y divide-[#21262d] max-h-96 overflow-y-auto">
        {active.map(s => <SessionRow key={s.id} session={s} onRevoke={() => revoke.mutate(s.id)} />)}
        {active.length === 0 && !isLoading && (
          <div className="px-4 py-5 text-center">
            <Activity size={18} className="text-[#30363d] mx-auto mb-1" />
            <p className="text-xs text-[#484f58]">{t.teams.noSessions}</p>
          </div>
        )}

        {expired.length > 0 && (
          <>
            <div className="px-4 py-2 bg-[#0d1117]">
              <p className="text-[10px] text-[#484f58] uppercase tracking-wider">{t.teams.expiredSessions(expired.length)}</p>
            </div>
            {expired.map(s => <SessionRow key={s.id} session={s} />)}
          </>
        )}
      </div>
    </div>
  )
}

function SessionRow({ session, onRevoke }: { session: MemberSession; onRevoke?: () => void }) {
  const { t } = useI18n()
  const isActive = session.status === 'active'
  const lastSeen = new Date(session.last_seen)
  const minutesAgo = Math.floor((Date.now() - lastSeen.getTime()) / 60000)
  const lastSeenLabel = minutesAgo < 2 ? t.teams.lastSeenJustNow : minutesAgo < 60 ? t.teams.lastSeenMinutesAgo(minutesAgo) : lastSeen.toLocaleDateString()

  return (
    <div className="flex items-center justify-between px-4 py-2.5">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${isActive ? 'bg-[#3fb950]' : 'bg-[#484f58]'}`} />
          <p className="text-[#e6edf3] text-xs truncate">{session.email}</p>
        </div>
        <div className="flex items-center gap-3 mt-0.5 ml-3.5">
          <span className="text-[10px] text-[#484f58]">{t.teams.lastSeenLabel(lastSeenLabel)}</span>
          <span className="text-[10px] text-[#484f58]">{t.teams.expiresLabel2(new Date(session.expires_at).toLocaleDateString())}</span>
        </div>
      </div>
      {isActive && onRevoke && (
        <button
          onClick={onRevoke}
          className="ml-2 flex items-center gap-1 text-[10px] text-[#484f58] hover:text-[#f85149] transition-colors flex-shrink-0"
          title={t.teams.forceLogout}
        >
          <ShieldOff size={12} />
        </button>
      )}
    </div>
  )
}

// ── Shared helpers ────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const handleCopy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <button onClick={handleCopy} className="flex-shrink-0 p-1.5 rounded text-[#8b949e] hover:text-[#e6edf3] hover:bg-[#21262d] transition-colors" title="Copy">
      {copied ? <Check size={13} className="text-[#3fb950]" /> : <Copy size={13} />}
    </button>
  )
}

function Empty() {
  const { t } = useI18n()
  return (
    <div className="rounded-lg border border-[#21262d] bg-[#161b22] p-8 text-center">
      <Users size={24} className="text-[#30363d] mx-auto mb-2" />
      <p className="text-[#484f58] text-xs">{t.teams.emptyState}</p>
    </div>
  )
}
