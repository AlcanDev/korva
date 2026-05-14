import { describe, it, expect, beforeEach, vi } from 'vitest'
import { consumeOIDCSessionFromURL } from '@/auth/oidc'
import { useAdminStore } from '@/stores/admin'

// Phase 15.D — verify the SPA's side of the OIDC handshake:
//   - returns false when no #session= fragment is present
//   - returns false on an empty token (#session=)
//   - returns true + seeds the admin store + strips the fragment
//     when given a non-empty token
//   - preserves the path + search params during the cleanup

function setURL(href: string) {
	// jsdom lets us reset location via history.replaceState as long as the
	// origin is the same. Tests run on `about:blank` by default, so we
	// pre-load a real URL first.
	window.history.replaceState(null, '', href)
}

describe('consumeOIDCSessionFromURL', () => {
	beforeEach(() => {
		// Reset the admin store + URL to a clean slate.
		useAdminStore.setState({
			key: '',
			sessionToken: '',
			authMode: 'key',
			isAuthenticated: false,
		})
		setURL('/app/overview')
		sessionStorage.clear()
	})

	it('returns false when no fragment is set', () => {
		expect(consumeOIDCSessionFromURL()).toBe(false)
		expect(useAdminStore.getState().sessionToken).toBe('')
	})

	it('returns false on an empty token', () => {
		setURL('/app/overview#session=')
		expect(consumeOIDCSessionFromURL()).toBe(false)
		expect(useAdminStore.getState().sessionToken).toBe('')
	})

	it('seeds the admin store and strips the fragment when token is present', () => {
		setURL('/app/overview#session=abc123token')
		expect(consumeOIDCSessionFromURL()).toBe(true)
		const s = useAdminStore.getState()
		expect(s.sessionToken).toBe('abc123token')
		expect(s.authMode).toBe('session')
		expect(s.isAuthenticated).toBe(true)
		expect(window.location.hash).toBe('')
		expect(window.location.pathname).toBe('/app/overview')
	})

	it('preserves path + query string when stripping the fragment', () => {
		setURL('/app/overview?from=login&team=ops#session=zzz')
		expect(consumeOIDCSessionFromURL()).toBe(true)
		expect(window.location.pathname).toBe('/app/overview')
		expect(window.location.search).toBe('?from=login&team=ops')
		expect(window.location.hash).toBe('')
	})

	it('ignores fragments that are not #session=', () => {
		setURL('/app/overview#some-anchor')
		expect(consumeOIDCSessionFromURL()).toBe(false)
		expect(window.location.hash).toBe('#some-anchor')
	})

	it('is no-op when window is undefined (SSR safety)', () => {
		// Stash window so we can reinstate it after the test.
		const originalWindow = globalThis.window
		// @ts-expect-error — simulating SSR by undefining window.
		delete (globalThis as { window?: Window }).window
		try {
			expect(consumeOIDCSessionFromURL()).toBe(false)
		} finally {
			globalThis.window = originalWindow
		}
	})
})

describe('main.tsx OIDC bootstrap (integration shape)', () => {
	// We don't import main.tsx (it would call createRoot and try to
	// mount), but we can simulate its call site and assert the
	// post-call invariants are stable.
	beforeEach(() => {
		useAdminStore.setState({
			key: '',
			sessionToken: '',
			authMode: 'key',
			isAuthenticated: false,
		})
	})

	it('calling consumeOIDCSessionFromURL twice is idempotent', () => {
		setURL('/app/overview#session=once')
		expect(consumeOIDCSessionFromURL()).toBe(true)
		// Second call: fragment is gone, no-op.
		expect(consumeOIDCSessionFromURL()).toBe(false)
		expect(useAdminStore.getState().sessionToken).toBe('once')
	})

	it('uses replaceState (not pushState) so back button is preserved', () => {
		setURL('/app/overview#session=keep-back-button')
		const spy = vi.spyOn(window.history, 'replaceState')
		consumeOIDCSessionFromURL()
		expect(spy).toHaveBeenCalled()
		spy.mockRestore()
	})
})
