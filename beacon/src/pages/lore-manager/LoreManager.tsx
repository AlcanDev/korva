import { BookOpen } from 'lucide-react'

const CURATED_SCROLLS = [
  { id: 'nestjs-hexagonal', team: 'backend', description: 'Hexagonal architecture with NestJS + Fastify' },
  { id: 'nestjs-bff', team: 'backend', description: 'BFF patterns — stateless, HttpService, Vault secrets' },
  { id: 'typescript', team: 'all', description: 'Strict TypeScript — branded types, Zod, Result<T,E>' },
  { id: 'testing-jest', team: 'all', description: 'Co-located specs, port mocking, coverage thresholds' },
  { id: 'nx-monorepo', team: 'all', description: 'Nx workspace @home-api — affected, libs, boundaries' },
  { id: 'gitlab-ci', team: 'devops', description: 'GitLab CI pipelines, Docker, HashiCorp Vault secrets' },
  { id: 'angular-wc', team: 'frontend', description: 'Angular 20 Elements + host-bridge' },
  { id: 'design-ui', team: 'frontend', description: 'Design design system — Sass variables, utility classes' },
  { id: 'forge-sdd', team: 'all', description: '5-phase SDD workflow for AI-assisted development' },
]

const TEAM_COLORS: Record<string, string> = {
  backend: '#1f6feb',
  frontend: '#9e6a03',
  devops: '#238636',
  all: '#6e7681',
}

export default function LoreManager() {
  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Lore</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">Knowledge Scrolls — architecture rules for your AI assistant</p>
      </header>

      <div className="space-y-2">
        {CURATED_SCROLLS.map((scroll) => (
          <div
            key={scroll.id}
            className="border border-[#21262d] rounded-lg px-4 py-3 bg-[#161b22] flex items-center gap-3"
          >
            <BookOpen size={14} className="text-[#6e7681] flex-shrink-0" />
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-[#e6edf3]">{scroll.id}</span>
                <span
                  className="text-xs px-1.5 py-0.5 rounded-full"
                  style={{
                    background: (TEAM_COLORS[scroll.team] ?? '#6e7681') + '22',
                    color: TEAM_COLORS[scroll.team] ?? '#6e7681',
                  }}
                >
                  {scroll.team}
                </span>
              </div>
              <p className="text-xs text-[#8b949e] mt-0.5">{scroll.description}</p>
            </div>
            <code className="text-xs text-[#6e7681] flex-shrink-0">
              korva lore add {scroll.id}
            </code>
          </div>
        ))}
      </div>

      <p className="text-xs text-[#6e7681] mt-4">
        Private team scrolls are installed via <code className="text-[#79c0ff]">korva init --profile</code>
      </p>
    </div>
  )
}
