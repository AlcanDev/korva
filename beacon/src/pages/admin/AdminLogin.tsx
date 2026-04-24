import { useState } from 'react'
import { Eye, EyeOff, Loader2 } from 'lucide-react'
import { useAdminStore } from '@/stores/admin'
import { checkAdminKey } from '@/api/admin'
import { useI18n } from '@/contexts/i18n'
import { KorvaLogo } from '@/components/KorvaLogo'

const MAX_ATTEMPTS = 5
const LOCKOUT_MS = 30_000

export default function AdminLogin() {
  const [key, setKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [attempts, setAttempts] = useState(0)
  const [lockoutUntil, setLockoutUntil] = useState<number | null>(null)
  const { setKey: storeSetKey } = useAdminStore()
  const { t } = useI18n()

  const now = Date.now()
  const isLockedOut = lockoutUntil !== null && now < lockoutUntil
  const secondsLeft = isLockedOut ? Math.ceil((lockoutUntil! - now) / 1000) : 0

  async function handleLogin(e: React.FormEvent) {
    e.preventDefault()
    if (!key.trim() || isLockedOut) return
    setLoading(true)
    setError('')
    try {
      const ok = await checkAdminKey(key.trim())
      if (ok) {
        storeSetKey(key.trim())
      } else {
        const next = attempts + 1
        if (next >= MAX_ATTEMPTS) {
          setLockoutUntil(Date.now() + LOCKOUT_MS)
          setAttempts(0)
        } else {
          setAttempts(next)
        }
        setError(t.auth.errorKey)
      }
    } catch {
      setError(t.auth.errorVault)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#0d1117] flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center mb-4">
            <KorvaLogo size={56} />
          </div>
          <h1 className="text-xl font-semibold text-[#e6edf3]">{t.auth.title}</h1>
          <p className="text-sm text-[#8b949e] mt-1">{t.auth.subtitle}</p>
        </div>

        {/* Form */}
        <form onSubmit={handleLogin} className="bg-[#161b22] border border-[#30363d] rounded-xl p-6 space-y-4">
          <div>
            <label className="block text-sm text-[#8b949e] mb-2">
              {t.auth.keyLabel}
            </label>
            <div className="relative">
              <input
                type={showKey ? 'text' : 'password'}
                value={key}
                onChange={e => setKey(e.target.value)}
                placeholder={t.auth.keyPlaceholder}
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
            <p className="text-xs text-[#484f58] mt-1.5">{t.auth.keyHint}</p>
            {key.trim().length > 0 && key.trim().length < 16 && (
              <p className="text-xs text-[#d29922] mt-1">{t.auth.keyTooShort}</p>
            )}
          </div>

          {isLockedOut && (
            <p className="text-sm text-[#d29922] bg-[#d2992212] border border-[#d2992230] rounded-lg px-3 py-2">
              {t.auth.rateLimited(secondsLeft)}
            </p>
          )}

          {!isLockedOut && error && (
            <p className="text-sm text-[#f85149] bg-[#f8514912] border border-[#f8514930] rounded-lg px-3 py-2">
              {error}
            </p>
          )}

          <button
            type="submit"
            disabled={loading || !key.trim() || isLockedOut}
            className="w-full bg-[#238636] hover:bg-[#2ea043] disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium px-4 py-2.5 rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            {loading ? <Loader2 size={14} className="animate-spin" /> : <KorvaLogo size={14} />}
            {loading ? t.auth.verifying : t.auth.submit}
          </button>
        </form>

        <p className="text-center text-xs text-[#484f58] mt-4">
          {t.auth.sessionNote}
        </p>
      </div>
    </div>
  )
}
