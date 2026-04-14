import { Clock } from 'lucide-react'

export default function Sessions() {
  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-5">
        <h1 className="text-xl font-semibold text-[#e6edf3]">Sessions</h1>
        <p className="text-sm text-[#8b949e] mt-0.5">History of AI-assisted development sessions</p>
      </header>
      <div className="border border-[#21262d] rounded-lg p-8 text-center">
        <Clock size={32} className="text-[#30363d] mx-auto mb-3" />
        <p className="text-sm text-[#8b949e]">Sessions view coming soon</p>
        <p className="text-xs text-[#6e7681] mt-1">
          Start a session with <code className="text-[#79c0ff]">vault_session_start</code> from your AI assistant
        </p>
      </div>
    </div>
  )
}
