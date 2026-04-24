import { describe, it, expect, beforeEach } from 'vitest'
import { useAdminStore } from '@/stores/admin'

describe('admin store', () => {
  beforeEach(() => {
    useAdminStore.setState({ key: '', isAuthenticated: false })
  })

  it('starts unauthenticated', () => {
    const state = useAdminStore.getState()
    expect(state.isAuthenticated).toBe(false)
    expect(state.key).toBe('')
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
})
