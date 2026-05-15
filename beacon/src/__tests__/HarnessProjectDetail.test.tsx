import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import HarnessProjectDetail from '@/pages/harness/HarnessProjectDetail'
import { useAdminStore } from '@/stores/admin'

// renderAt mounts the detail component inside a MemoryRouter set to
// the given URL so the route param + querystring are available to the
// component under test.
function renderAt(url: string) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route path="/app/harness/:project" element={<HarnessProjectDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

// stubFetch is per-test global fetch double. The handler maps each
// requested path to a Response so a single test can wire both the
// snapshot endpoint and the transitions endpoint.
function stubFetch(handler: (url: string) => Response | Promise<Response>) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : (input as URL).toString()
    return handler(url)
  }) as unknown as typeof fetch
}

const samplePayload = JSON.stringify({
  project: 'auth_layer',
  description: 'Auth bootstrap',
  rules: {
    one_feature_at_a_time: true,
    require_tests_to_close: true,
    require_approved_spec_to_implement: true,
    valid_status: ['pending', 'spec_ready', 'in_progress', 'done', 'blocked'],
  },
  features: [
    { id: 1, name: 'login', title: 'Email login', status: 'in_progress', sdd: true },
    { id: 2, name: 'logout', title: 'Logout endpoint', status: 'pending', sdd: false },
  ],
})

beforeEach(() => {
  useAdminStore.setState({
    key: 'k',
    sessionToken: '',
    authMode: 'key',
    isAuthenticated: true,
  })
})

describe('HarnessProjectDetail', () => {
  it('renders the project title, root, counts and feature rows', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'auth_layer', root: '/repos/auth',
          payload: samplePayload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/auth_layer?root=/repos/auth')
    // Wait for an element that only appears AFTER the snapshot fetch
    // resolves; the heading renders from the URL param even before
    // data arrives.
    await waitFor(() => expect(screen.getByText('Email login')).toBeInTheDocument())
    expect(screen.getByText('Logout endpoint')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'auth_layer' })).toBeInTheDocument()
    // SDD mode badge is shown for SDD-rule harnesses.
    expect(screen.getByText(/SDD mode/)).toBeInTheDocument()
    // Counts row includes spec_ready when the harness is SDD.
    expect(screen.getByText('Spec ready')).toBeInTheDocument()
  })

  it('hides spec_ready in counts when harness rule is off', async () => {
    const noSDD = JSON.stringify({
      project: 'p', rules: {
        one_feature_at_a_time: true, require_tests_to_close: true,
        valid_status: ['pending', 'in_progress', 'done', 'blocked'],
      },
      features: [{ id: 1, name: 'x', title: 'X', status: 'pending', sdd: false }],
    })
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'p', root: '/r', payload: noSDD, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/p?root=/r')
    // Wait for the feature title (only rendered after snapshot loads).
    await waitFor(() => expect(screen.getByText('X')).toBeInTheDocument())
    expect(screen.queryByText('Spec ready')).not.toBeInTheDocument()
    expect(screen.queryByText(/SDD mode/)).not.toBeInTheDocument()
  })

  it('shows the timeline rows when transitions are returned', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(
          JSON.stringify({
            transitions: [
              {
                id: 'a', team_id: 't', project: 'p', root: '/r',
                feature_id: 1, from_status: 'pending', to_status: 'in_progress',
                owner: 'alice', occurred_at: new Date().toISOString(),
              },
              {
                id: 'b', team_id: 't', project: 'p', root: '/r',
                feature_id: 1, from_status: 'in_progress', to_status: 'done',
                owner: 'bob', occurred_at: new Date().toISOString(),
              },
            ],
            count: 2, limit: 200,
          }),
          { status: 200 },
        )
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'p', root: '/r', payload: samplePayload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/p?root=/r')
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  it('shows a 404 banner when the snapshot does not exist', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response('not found', { status: 404 })
    })
    renderAt('/app/harness/missing?root=/r')
    await waitFor(() =>
      expect(screen.getByText(/Snapshot not found/)).toBeInTheDocument(),
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('renders the timeline empty-state when no transitions logged', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'p', root: '/r', payload: samplePayload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/p?root=/r')
    await waitFor(() =>
      expect(screen.getByText(/No transitions logged yet/)).toBeInTheDocument(),
    )
  })

  it('renders a back link to the dashboard', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'p', root: '/r', payload: samplePayload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/p?root=/r')
    const back = await screen.findByRole('link', { name: /all projects/i })
    expect(back).toHaveAttribute('href', '/app/harness')
  })
})

// ─────────────────────── Phase 19.B — review verdict column ─────────────────

describe('HarnessProjectDetail — review verdict surface', () => {
  function reviewedPayload(reviewVerdict: 'approve' | 'needs_fixes' | 'reject') {
    return JSON.stringify({
      project: 'auth_layer',
      rules: {
        one_feature_at_a_time: true, require_tests_to_close: true,
        require_approved_spec_to_implement: true,
        valid_status: ['pending', 'spec_ready', 'in_progress', 'done', 'blocked'],
      },
      features: [
        {
          id: 1, name: 'login', title: 'Email login', status: 'spec_ready', sdd: true,
          review: {
            verdict: reviewVerdict, reviewer: 'alice@acme.io',
            at: '2026-05-14T10:00:00Z', issue_count: 2, error_count: 0,
            note: 'tighten R3',
          },
        },
        // A second feature with no review — proves the column still
        // renders an "—" for the unrelated row.
        { id: 2, name: 'logout', title: 'Logout', status: 'pending', sdd: false },
      ],
    })
  }

  function stubReviewedFetch(payload: string) {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'auth_layer', root: '/r',
          payload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
  }

  it('hides the Review column when no feature carries a verdict', async () => {
    stubFetch(url => {
      if (url.includes('/transitions')) {
        return new Response(JSON.stringify({ transitions: [], count: 0, limit: 200 }), { status: 200 })
      }
      return new Response(
        JSON.stringify({
          team_id: 't', project: 'p', root: '/r',
          payload: samplePayload, updated_at: new Date().toISOString(),
        }),
        { status: 200 },
      )
    })
    renderAt('/app/harness/p?root=/r')
    await waitFor(() => expect(screen.getByText('Email login')).toBeInTheDocument())
    // The "Review" column header must NOT be present.
    expect(screen.queryByRole('columnheader', { name: /^review$/i })).toBeNull()
  })

  it('renders the Review column with the approve pill', async () => {
    stubReviewedFetch(reviewedPayload('approve'))
    renderAt('/app/harness/auth_layer?root=/r')
    await waitFor(() => expect(screen.getByText('Email login')).toBeInTheDocument())
    expect(screen.getByRole('columnheader', { name: /^review$/i })).toBeInTheDocument()
    // Pill text + the tooltip-ish aria-label.
    expect(screen.getByText('approve')).toBeInTheDocument()
    const pill = screen.getByLabelText(/Review verdict approve/i)
    expect(pill).toHaveAttribute('title', expect.stringContaining('alice@acme.io'))
    expect(pill.getAttribute('title')).toContain('tighten R3')
  })

  it('renders the needs_fixes pill with humanized label', async () => {
    stubReviewedFetch(reviewedPayload('needs_fixes'))
    renderAt('/app/harness/auth_layer?root=/r')
    await waitFor(() => expect(screen.getByText('needs fixes')).toBeInTheDocument())
  })

  it('renders the reject pill', async () => {
    stubReviewedFetch(reviewedPayload('reject'))
    renderAt('/app/harness/auth_layer?root=/r')
    await waitFor(() => expect(screen.getByText('reject')).toBeInTheDocument())
  })

  it('shows the em-dash placeholder for unreviewed rows in the same table', async () => {
    stubReviewedFetch(reviewedPayload('approve'))
    renderAt('/app/harness/auth_layer?root=/r')
    await waitFor(() => expect(screen.getByText('Email login')).toBeInTheDocument())
    // The second feature has no review; it should render an em-dash
    // with a no-review aria-label so screen-readers don't conflate
    // empty with "approve".
    expect(screen.getByLabelText(/no review recorded/i)).toBeInTheDocument()
  })
})
