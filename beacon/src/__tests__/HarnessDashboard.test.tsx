import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import HarnessDashboard, { StatusPill, formatRelative } from '@/pages/harness/HarnessDashboard'
import { useAdminStore } from '@/stores/admin'

// renderWithProviders wraps the SUT in the providers it expects in
// production: a fresh QueryClient (so each test starts with a cold
// cache) and a MemoryRouter (since the dashboard renders <Link> nodes).
function renderWithProviders(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

// stubFetch installs a global fetch double for the duration of one
// test. Returns a helper to assert the requested URL.
function stubFetch(handler: (url: string) => Response | Promise<Response>) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : (input as URL).toString()
    return handler(url)
  }) as unknown as typeof fetch
}

beforeEach(() => {
  // Pretend the user has an admin key so adminFetch wires the header
  // through; the tests that care about wire shape will assert on the
  // fetch call args directly.
  useAdminStore.setState({
    key: 'k',
    sessionToken: '',
    authMode: 'key',
    isAuthenticated: true,
  })
})

describe('HarnessDashboard', () => {
  it('shows the empty-state message when the API returns zero projects', async () => {
    stubFetch(() =>
      new Response(JSON.stringify({ projects: [], count: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    renderWithProviders(<HarnessDashboard />)
    await waitFor(() =>
      expect(screen.getByText(/No harness state yet/i)).toBeInTheDocument(),
    )
    // The empty state should suggest the next CLI step.
    expect(screen.getByText(/korva harness init/)).toBeInTheDocument()
  })

  it('renders one card per project with status pill', async () => {
    stubFetch(() =>
      new Response(
        JSON.stringify({
          projects: [
            {
              team_id: 't',
              project: 'auth_layer',
              root: '/repos/auth',
              updated_at: new Date().toISOString(),
              last_transition_at: new Date().toISOString(),
              last_transition_to: 'in_progress',
            },
            {
              team_id: 't',
              project: 'billing',
              root: '/repos/billing',
              updated_at: new Date().toISOString(),
              last_transition_at: new Date().toISOString(),
              last_transition_to: 'done',
            },
          ],
          count: 2,
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    renderWithProviders(<HarnessDashboard />)
    await waitFor(() => expect(screen.getByText('auth_layer')).toBeInTheDocument())
    expect(screen.getByText('billing')).toBeInTheDocument()
    // Status pill labels are humanized (in_progress → "in progress").
    expect(screen.getByText('in progress')).toBeInTheDocument()
    expect(screen.getByText('done')).toBeInTheDocument()
  })

  it('renders a card link that points at the detail route with root in querystring', async () => {
    stubFetch(() =>
      new Response(
        JSON.stringify({
          projects: [
            {
              team_id: 't',
              project: 'auth/v2',
              root: '/repos/p one',
              updated_at: new Date().toISOString(),
            },
          ],
          count: 1,
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    renderWithProviders(<HarnessDashboard />)
    const link = await screen.findByRole('link', { name: /auth\/v2/i })
    // Both project (with slash) and root (with space) must be URL-encoded.
    expect(link).toHaveAttribute(
      'href',
      '/app/harness/auth%2Fv2?root=%2Frepos%2Fp%20one',
    )
  })

  it('renders an error banner when the vault returns a non-2xx', async () => {
    stubFetch(() => new Response('boom', { status: 500 }))
    renderWithProviders(<HarnessDashboard />)
    await waitFor(() =>
      expect(screen.getByText(/Could not load harness state/i)).toBeInTheDocument(),
    )
    // The error banner must be marked role=alert for assistive tech.
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('refresh button is keyboard-accessible (has aria-label)', async () => {
    stubFetch(() =>
      new Response(JSON.stringify({ projects: [], count: 0 }), { status: 200 }),
    )
    renderWithProviders(<HarnessDashboard />)
    const btn = await screen.findByRole('button', { name: /refresh harness projects/i })
    expect(btn).toBeInTheDocument()
  })
})

describe('StatusPill', () => {
  it('humanizes underscored status names', () => {
    render(<StatusPill status="in_progress" />)
    expect(screen.getByText('in progress')).toBeInTheDocument()
    render(<StatusPill status="spec_ready" />)
    expect(screen.getByText('spec ready')).toBeInTheDocument()
  })
  it('falls back to the raw status when unknown', () => {
    render(<StatusPill status="something_new" />)
    expect(screen.getByText('something_new')).toBeInTheDocument()
  })
})

describe('formatRelative', () => {
  // Use stable now so the tests aren't flaky across midnights.
  const now = Date.parse('2026-05-14T12:00:00Z')

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })

  it('renders seconds for very recent timestamps', () => {
    expect(formatRelative(new Date(now - 5_000).toISOString())).toBe('5s ago')
  })
  it('renders minutes', () => {
    expect(formatRelative(new Date(now - 5 * 60_000).toISOString())).toBe('5m ago')
  })
  it('renders hours', () => {
    expect(formatRelative(new Date(now - 3 * 3600_000).toISOString())).toBe('3h ago')
  })
  it('renders days for less than a week', () => {
    expect(formatRelative(new Date(now - 3 * 86_400_000).toISOString())).toBe('3d ago')
  })
  it('falls back to a date for older timestamps', () => {
    const got = formatRelative(new Date(now - 40 * 86_400_000).toISOString())
    expect(got).toMatch(/[A-Z][a-z]{2}\s+\d+/) // e.g. "Apr 4"
  })
  it('handles invalid input gracefully', () => {
    expect(formatRelative('')).toBe('—')
    expect(formatRelative('not-a-date')).toBe('—')
  })
  it('says "just now" for future timestamps (clock skew)', () => {
    expect(formatRelative(new Date(now + 60_000).toISOString())).toBe('just now')
  })
})
