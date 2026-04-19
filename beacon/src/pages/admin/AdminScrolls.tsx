import { useState } from 'react'
import { BookOpen, Plus, Save, RefreshCw, CheckCircle, AlertTriangle } from 'lucide-react'

// Scroll management — this page lets you view and edit the AI instructions
// that get injected into projects via korva init --profile

interface ScrollEntry {
  id: string
  title: string
  active: boolean
  path: string
}

// Default scrolls matching the team profile
const DEFAULT_SCROLLS: ScrollEntry[] = [
  { id: 'nestjs-hexagonal', title: 'NestJS Hexagonal Architecture', active: true, path: 'scrolls/nestjs-hexagonal/SCROLL.md' },
  { id: 'nestjs-bff', title: 'NestJS BFF Pattern', active: true, path: 'scrolls/nestjs-bff/SCROLL.md' },
  { id: 'typescript', title: 'TypeScript Strict Mode', active: true, path: 'scrolls/typescript/SCROLL.md' },
  { id: 'testing-jest', title: 'Testing with Jest', active: true, path: 'scrolls/testing-jest/SCROLL.md' },
  { id: 'apigee-fif', title: 'Apigee OAuth2 (FIF)', active: true, path: 'scrolls/apigee-fif/SCROLL.md' },
  { id: 'architecture-fif', title: 'Architecture ADRs (FIF)', active: true, path: 'scrolls/architecture-fif/SCROLL.md' },
  { id: 'devops-fif', title: 'DevOps / CI-CD (FIF)', active: true, path: 'scrolls/devops-fif/SCROLL.md' },
  { id: 'forge-sdd', title: 'Forge SDD Workflow', active: true, path: 'scrolls/forge-sdd/SCROLL.md' },
  { id: 'nx-monorepo', title: 'Nx Monorepo', active: false, path: 'scrolls/nx-monorepo/SCROLL.md' },
  { id: 'qa-strategy', title: 'QA Strategy', active: false, path: 'scrolls/qa-strategy/SCROLL.md' },
  { id: 'security-fif', title: 'Security (FIF)', active: false, path: 'scrolls/security-fif/SCROLL.md' },
  { id: 'frontend-fif', title: 'Frontend (Angular/FIF)', active: false, path: 'scrolls/frontend-fif/SCROLL.md' },
  { id: 'dev-workflow', title: 'Dev Workflow', active: false, path: 'scrolls/dev-workflow/SCROLL.md' },
]

interface InstructionFile {
  name: string
  description: string
  key: 'copilot' | 'claude'
}

const INSTRUCTION_FILES: InstructionFile[] = [
  { name: 'copilot-extensions.md', description: 'Injected into .github/copilot-instructions.md', key: 'copilot' },
  { name: 'claude-extensions.md', description: 'Injected into CLAUDE.md', key: 'claude' },
]

export default function AdminScrolls() {
  const [scrolls, setScrolls] = useState<ScrollEntry[]>(DEFAULT_SCROLLS)
  const [activeTab, setActiveTab] = useState<'scrolls' | 'instructions'>('scrolls')
  const [editingInstruction, setEditingInstruction] = useState<'copilot' | 'claude' | null>(null)
  const [instructionText, setInstructionText] = useState('')
  const [saved, setSaved] = useState(false)

  function toggleScroll(id: string) {
    setScrolls(prev => prev.map(s => s.id === id ? { ...s, active: !s.active } : s))
  }

  function handleSaveScrolls() {
    // In production: PATCH /admin/scrolls with the active list
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  const activeCount = scrolls.filter(s => s.active).length

  return (
    <div className="p-4 sm:p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-[#e6edf3]">Scrolls & Instructions</h2>
          <p className="text-sm text-[#8b949e] mt-0.5">
            Manage the knowledge loaded into your AI assistants
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-[#21262d]">
        {(['scrolls', 'instructions'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium capitalize transition-colors border-b-2 -mb-px ${
              activeTab === tab
                ? 'border-[#388bfd] text-[#e6edf3]'
                : 'border-transparent text-[#8b949e] hover:text-[#e6edf3]'
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {activeTab === 'scrolls' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <p className="text-xs text-[#8b949e]">
              {activeCount} of {scrolls.length} scrolls active — loaded by AI on each session
            </p>
            <button
              onClick={handleSaveScrolls}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                saved
                  ? 'bg-[#3fb95020] text-[#3fb950] border border-[#3fb95030]'
                  : 'bg-[#238636] hover:bg-[#2ea043] text-white'
              }`}
            >
              {saved ? <CheckCircle size={13} /> : <Save size={13} />}
              {saved ? 'Saved!' : 'Save changes'}
            </button>
          </div>

          <div className="grid gap-2">
            {scrolls.map(scroll => (
              <div
                key={scroll.id}
                className={`flex items-center justify-between bg-[#161b22] border rounded-lg px-4 py-3 transition-all ${
                  scroll.active ? 'border-[#21262d]' : 'border-[#21262d] opacity-60'
                }`}
              >
                <div className="flex items-center gap-3">
                  <BookOpen size={14} className={scroll.active ? 'text-[#388bfd]' : 'text-[#484f58]'} />
                  <div>
                    <p className="text-sm text-[#e6edf3]">{scroll.title}</p>
                    <p className="text-xs text-[#484f58] font-mono">{scroll.path}</p>
                  </div>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={scroll.active}
                    onChange={() => toggleScroll(scroll.id)}
                    className="sr-only peer"
                  />
                  <div className="w-9 h-5 bg-[#21262d] peer-checked:bg-[#238636] rounded-full transition-colors after:content-[''] after:absolute after:top-0.5 after:left-0.5 after:bg-white after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:after:translate-x-4" />
                </label>
              </div>
            ))}
          </div>

          <div className="bg-[#388bfd12] border border-[#388bfd30] rounded-lg p-4 flex gap-3">
            <AlertTriangle size={14} className="text-[#388bfd] flex-shrink-0 mt-0.5" />
            <p className="text-xs text-[#8b949e]">
              Changes here update the active scroll list in <code className="text-[#79c0ff]">~/.korva/config.json</code>.
              Run <code className="text-[#79c0ff]">korva sync --profile</code> to push changes to the team.
            </p>
          </div>
        </div>
      )}

      {activeTab === 'instructions' && (
        <div className="space-y-4">
          <p className="text-xs text-[#8b949e]">
            These files get injected into project AI instruction files via <code className="text-[#79c0ff]">korva init --profile</code>
          </p>

          {editingInstruction ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-medium text-[#e6edf3]">
                  Editing: {editingInstruction === 'copilot' ? 'copilot-extensions.md' : 'claude-extensions.md'}
                </h3>
                <div className="flex gap-2">
                  <button
                    onClick={() => setEditingInstruction(null)}
                    className="px-3 py-1.5 text-xs text-[#8b949e] border border-[#30363d] rounded-lg hover:bg-[#21262d]"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => {
                      setSaved(true)
                      setEditingInstruction(null)
                      setTimeout(() => setSaved(false), 2000)
                    }}
                    className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#238636] hover:bg-[#2ea043] text-white rounded-lg"
                  >
                    <Save size={12} /> Save
                  </button>
                </div>
              </div>
              <textarea
                value={instructionText}
                onChange={e => setInstructionText(e.target.value)}
                className="w-full h-80 bg-[#0d1117] border border-[#30363d] rounded-lg p-4 text-sm text-[#e6edf3] font-mono focus:outline-none focus:border-[#388bfd] resize-none"
                placeholder="# Team Instructions&#10;&#10;## Architecture&#10;..."
              />
            </div>
          ) : (
            <div className="space-y-3">
              {INSTRUCTION_FILES.map(f => (
                <div key={f.key} className="bg-[#161b22] border border-[#21262d] rounded-xl p-5">
                  <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
                    <div>
                      <p className="text-sm font-medium text-[#e6edf3] font-mono">{f.name}</p>
                      <p className="text-xs text-[#8b949e] mt-0.5">{f.description}</p>
                    </div>
                    <div className="flex gap-2 flex-shrink-0">
                      <button
                        onClick={() => {
                          setInstructionText(`# ${f.name} — Team Instructions\n\nAdd your team-specific AI instructions here.\nThese will be injected into every developer's project.\n`)
                          setEditingInstruction(f.key)
                        }}
                        className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-[#8b949e] border border-[#30363d] rounded-lg hover:bg-[#21262d] hover:text-[#e6edf3] transition-colors"
                      >
                        <RefreshCw size={12} /> Edit
                      </button>
                      <button
                        onClick={() => {
                          setInstructionText('')
                          setEditingInstruction(f.key)
                        }}
                        className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-[#21262d] hover:bg-[#30363d] text-[#e6edf3] rounded-lg transition-colors"
                      >
                        <Plus size={12} /> New
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
