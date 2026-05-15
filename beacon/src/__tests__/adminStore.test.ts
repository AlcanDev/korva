import { describe, it, expect, beforeEach } from 'vitest'
import { useAdminStore } from '@/stores/admin'

describe('admin store', () => {
  beforeEach(() => {
    useAdminStore.setState({
      key: '',
      sessionToken: '',
      authMode: 'key',
      isAuthenticated: false,
    })
    sessionStorage.clear()
  })

  it('starts unauthenticated', () => {
    const state = useAdminStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.key).toBe('')
    expect(state.sessionToken).toBe('')
    expect(state.authMode).toBe('key')
  })

  it('setKey authenticates with a non-empty key', () => {
    useAdminStore.getState().setKey('my-secret-key-123')
    const state = useAdminStore.getState()
    expect(state.isAuthenticated).toBe(true)
    expect(state.key).toBe('my-secret-key-123')
  })

  it('setKey with empty string stays unauthenticated', () => {
    useAdminStore.getState().setKey('')
    const state = useAdminStore.getState()
    expect(state.isAuthenticated).toBe(false)
  })

  it('logout clears key and authentication', () => {
    useAdminStore.getState().setKey('valid-key')
    useAdminStore.getState().logout()
    const state = useAdminStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.key).toBe('')
  })

  it('getState().key is used for X-Admin-Key header', () => {
    useAdminStore.getState().setKey('test-api-key')
    expect(useAdminStore.getState().key).toBe('test-api-key')
  })

  // Phase 20.A — pin the session-token branch (OIDC / invite flow).
  // Without this the dual-auth contract drifted silently.

  it('setSessionToken authenticates and switches authMode to "session"', () => {
    useAdminStore.getState().setSessionToken('opaque-token-xyz')
    const s = useAdminStore.getState()
    expect(s.isAuthenticated).toBe(true)
    expect(s.sessionToken).toBe('opaque-token-xyz')
    expect(s.authMode).toBe('session')
  })

  it('setSessionToken clears any previously-set admin key', () => {
    // Switching auth modes must not leave the old credential
    // hanging — adminFetch picks the wrong header otherwise.
    useAdminStore.getState().setKey('admin-secret')
    useAdminStore.getState().setSessionToken('new-session-token')
    const s = useAdminStore.getState()
    expect(s.key).toBe('')
    expect(s.authMode).toBe('session')
    expect(s.sessionToken).toBe('new-session-token')
  })

  it('setKey clears any previously-set session token', () => {
    useAdminStore.getState().setSessionToken('old-token')
    useAdminStore.getState().setKey('new-admin-key')
    const s = useAdminStore.getState()
    expect(s.sessionToken).toBe('')
    expect(s.authMode).toBe('key')
    expect(s.key).toBe('new-admin-key')
  })

  it('setSessionToken with empty string is unauthenticated', () => {
    useAdminStore.getState().setSessionToken('')
    const s = useAdminStore.getState()
    expect(s.isAuthenticated).toBe(false)
  })

  it('logout clears BOTH credentials regardless of which authMode was active', () => {
    useAdminStore.getState().setSessionToken('s')
    useAdminStore.getState().logout()
    let s = useAdminStore.getState()
    expect(s.isAuthenticated).toBe(false)
    expect(s.sessionToken).toBe('')
    expect(s.key).toBe('')
    expect(s.authMode).toBe('key')

    useAdminStore.getState().setKey('k')
    useAdminStore.getState().logout()
    s = useAdminStore.getState()
    expect(s.isAuthenticated).toBe(false)
    expect(s.key).toBe('')
    expect(s.sessionToken).toBe('')
  })

  it('persists state to sessionStorage (cleared when tab closes)', () => {
    useAdminStore.getState().setKey('persisted-key')
    // The persist middleware writes to sessionStorage under key
    // 'korva-admin'. We don't assert the exact JSON shape (that's
    // an internal contract) but we DO assert that something was
    // written, so a future change that breaks persistence
    // surfaces via this test.
    const raw = sessionStorage.getItem('korva-admin')
    expect(raw).not.toBeNull()
    const parsed = JSON.parse(raw!)
    expect(parsed.state.key).toBe('persisted-key')
    expect(parsed.state.isAuthenticated).toBe(true)
  })
})
