import { useState } from 'react'
import { Shield, Eye, EyeOff, Loader2 } from 'lucide-react'
import { useAdminStore } from '@/stores/admin'
import { checkAdminKey } from '@/api/admin'

export default function AdminLogin() {
  const [key, setKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const { setKey: storeSetKey } = useAdminStore()

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    if (!key.trim()) return
    setLoading(true)
    setError('')
    try {
      const ok = await checkAdminKey(key.trim())
      if (ok) {
        storeSetKey(key.trim())
      } else {
        setError('Invalid admin key. Check ~/.korva/admin.key on your machine.')
      }
    } catch {
      setError('Could not connect to Vault. Make sure korva-vault is running.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#0d1117] flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-full bg-[#161b22] border border-[#30363d] mb-4">
            <Shield size={24} className="text-[#f0883e]" />
          </div>
          <h1 className="text-xl font-semibold text-[#e6edf3]">Korva Admin</h1>
          <p className="text-sm text-[#8b949e] mt-1">
            Private access — Team Lead only
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleLogin} className="bg-[#161b22] border border-[#30363d] rounded-xl p-6 space-y-4">
          <div>
            <label className="block text-sm text-[#8b949e] mb-2">
              Admin Key
            </label>
            <div className="relative">
              <input
                type={showKey ? 'text' : 'password'}
                value={key}
                onChange={e => setKey(e.target.value)}
                placeholder="Paste your admin key..."
                className="w-full bg-[#0d1117] border border-[#30363d] rounded-lg px-3 py-2.5 text-sm text-[#e6edf3] placeholder-[#484f58] focus:outline-none focus:border-[#388bfd] pr-10"
                autoFocus
              />
              <button
                type="button"
                onClick={() => setShowKey(v => !v)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-[#484f58] hover:text-[#8b949e]"
              >
                {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
            <p className="text-xs text-[#484f58] mt-1.5">
              Found in <code className="text-[#79c0ff]">~/.korva/admin.key</code> → copy the <code className="text-[#79c0ff]">"key"</code> field
            </p>
          </div>

          {error && (
            <p className="text-sm text-[#f85149] bg-[#f8514912] border border-[#f8514930] rounded-lg px-3 py-2">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading || !key.trim()}
            className="w-full bg-[#238636] hover:bg-[#2ea043] disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium px-4 py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            {loading ? <Loader2 size={14} className="animate-spin" /> : <Shield size={14} />}
            {loading ? 'Verifying...' : 'Access Admin Panel'}
          </button>
        </form>

        <p className="text-center text-xs text-[#484f58] mt-4">
          Session expires when you close this tab
        </p>
      </div>
    </div>
  )
}
