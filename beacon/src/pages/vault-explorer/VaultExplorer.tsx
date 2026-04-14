import { useState } from 'react'
import { Search, Tag } from 'lucide-react'
import { useSearch, type Observation } from '@/api/vault'

const TYPE_COLORS: Record<string, string> = {
  decision: '#238636',
  pattern: '#1f6feb',
  bugfix: '#da3633',
  learning: '#9e6a03',
  context: '#6e7681',
  antipattern: '#a371f7',
  task: '#58a6ff',
}

export default function VaultExplorer() {
  const [query, setQuery] = useState('')
  const [project, setProject] = useState('')
  const [team, setTeam] = useState('')
  const [country, setCountry] = useState('')
  const [type, setType] = useState('')

  const { data, isLoading } = useSearch(query, { project, team, country, type })

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Vault Explorer</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">Search and browse stored knowledge</p>
      </header>

      {/* Search bar */}
      <div className="relative mb-4">
        <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#8b949e]" />
        <input
          type="text"
          placeholder="Search observations..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full bg-[#161b22] border border-[#30363d] rounded-lg pl-9 pr-3 py-2 text-sm text-[#e6edf3] placeholder-[#6e7681] focus:outline-none focus:border-[#1f6feb]"
        />
      </div>

      {/* Filters */}
      <div className="flex gap-2 mb-5 flex-wrap">
        {[
          { value: project, setter: setProject, placeholder: 'Project' },
          { value: team, setter: setTeam, placeholder: 'Team' },
          { value: country, setter: setCountry, placeholder: 'Country' },
        ].map(({ value, setter, placeholder }) => (
          <input
            key={placeholder}
            type="text"
            placeholder={placeholder}
            value={value}
            onChange={(e) => setter(e.target.value)}
            className="bg-[#161b22] border border-[#30363d] rounded px-3 py-1.5 text-sm text-[#e6edf3] placeholder-[#6e7681] focus:outline-none focus:border-[#1f6feb] w-32"
          />
        ))}
        <select
          value={type}
          onChange={(e) => setType(e.target.value)}
          className="bg-[#161b22] border border-[#30363d] rounded px-3 py-1.5 text-sm text-[#e6edf3] focus:outline-none focus:border-[#1f6feb]"
        >
          <option value="">All types</option>
          {['decision', 'pattern', 'bugfix', 'learning', 'context', 'antipattern', 'task'].map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
      </div>

      {/* Results */}
      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-24 bg-[#161b22] border border-[#21262d] rounded-lg animate-pulse" />
          ))}
        </div>
      ) : (
        <>
          <p className="text-xs text-[#8b949e] mb-3">
            {data?.count ?? 0} observation{data?.count !== 1 ? 's' : ''}
          </p>
          <div className="space-y-3">
            {data?.results?.map((obs) => (
              <ObservationCard key={obs.id} obs={obs} />
            ))}
          </div>
          {data?.count === 0 && (
            <div className="text-center py-12 text-[#8b949e] text-sm">
              No observations found
            </div>
          )}
        </>
      )}
    </div>
  )
}

function ObservationCard({ obs }: { obs: Observation }) {
  const color = TYPE_COLORS[obs.type] ?? '#6e7681'

  return (
    <div className="border border-[#21262d] rounded-lg p-4 bg-[#161b22] hover:border-[#30363d] transition-colors">
      <div className="flex items-start justify-between gap-2 mb-2">
        <h3 className="text-sm font-medium text-[#e6edf3] leading-snug">{obs.title}</h3>
        <span
          className="text-xs px-2 py-0.5 rounded-full flex-shrink-0"
          style={{ background: color + '22', color }}
        >
          {obs.type}
        </span>
      </div>

      <p className="text-xs text-[#8b949e] line-clamp-2 mb-3">{obs.content}</p>

      <div className="flex items-center gap-3 text-xs text-[#6e7681]">
        {obs.project && <span>{obs.project}</span>}
        {obs.team && <span>·</span>}
        {obs.team && <span>{obs.team}</span>}
        {obs.country && <span>·</span>}
        {obs.country && <span>{obs.country}</span>}
        <span className="ml-auto">{formatDate(obs.created_at)}</span>
      </div>

      {obs.tags.length > 0 && (
        <div className="flex items-center gap-1.5 mt-2.5 flex-wrap">
          <Tag size={10} className="text-[#6e7681]" />
          {obs.tags.map((tag) => (
            <span key={tag} className="text-xs bg-[#21262d] text-[#8b949e] px-1.5 py-0.5 rounded">
              {tag}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}
