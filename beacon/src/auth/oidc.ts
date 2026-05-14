import { useAdminStore } from '@/stores/admin'

// Phase 15.D — Beacon side of the OIDC handshake.
//
// The vault's /auth/oidc/callback redirects to /app/overview with
// `#session=<plaintext-token>`. We surface the token to the rest of
// the SPA via the admin store and immediately strip the fragment so
// it doesn't leak into:
//   - browser history (URLs are kept indefinitely)
//   - the Referer header (sent on every cross-origin link click)
//   - shared screenshots / copy-pasted URLs
//
// Returns true when a token was harvested, false otherwise.
// Exported as a function so unit tests can drive it; main.tsx invokes
// it once at module load before React mounts.
export function consumeOIDCSessionFromURL(): boolean {
  if (typeof window === 'undefined') return false
  const hash = window.location.hash
  if (!hash.startsWith('#session=')) return false

  const token = hash.slice('#session='.length)
  if (token.length === 0) return false

  useAdminStore.getState().setSessionToken(token)
  // history.replaceState avoids creating a new history entry — the
  // Back button still works the way the operator expects.
  if (typeof history !== 'undefined' && typeof history.replaceState === 'function') {
    history.replaceState(null, '', window.location.pathname + window.location.search)
  }
  return true
}
