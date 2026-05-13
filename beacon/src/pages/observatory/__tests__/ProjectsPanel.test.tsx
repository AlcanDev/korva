import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ProjectsPanel from '../ProjectsPanel'

// Fase 6.1 — verifica el contrato del Projects panel: lista, sugerencias y
// el flujo de prune con doble confirmación. Mockea fetch + adminStore así
// queda hermético.

vi.mock('@/stores/admin', () => ({
  useAdminStore: Object.assign(
    () => ({
      key: 'test-key',
      sessionToken: '',
      authMode: 'key' as const,
      isAuthenticated: true,
    }),
    {
      getState: () => ({ key: 'test-key', sessionToken: '', authMode: 'key' as const }),
    },
  ),
}))

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

const projectsFixture = {
  projects: [
    { name: 'korva', observation_count: 12, session_count: 5 },
    { name: 'vault-mcp', observation_count: 3, session_count: 1 },
  ],
  count: 2,
}

const suggestionsFixture = {
  proposals: [
    {
      canonical: 'korva',
      variants: [
        { name: 'korva', observation_count: 12, session_count: 5 },
        { name: 'Korva', observation_count: 1, session_count: 0 },
      ],
    },
  ],
  count: 1,
}

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  fetchMock = vi.fn(async (input?: RequestInfo | URL | string | null, init?: RequestInit) => {
    const url = input == null ? '' : typeof input === 'string' ? input : String(input)
    const method = init?.method ?? 'GET'

    if (method === 'GET' && url.includes('/admin/projects/suggestions'))
      return jsonResponse(suggestionsFixture)
    if (method === 'GET' && url.includes('/admin/projects')) return jsonResponse(projectsFixture)
    if (method === 'POST' && url.includes('/admin/projects/consolidate')) {
      return jsonResponse({
        status: 'merged',
        canonical: 'korva',
        sources: ['Korva'],
        observations_updated: 1,
        sessions_updated: 0,
        prompts_updated: 0,
      })
    }
    if (method === 'POST' && url.includes('/admin/projects/prune')) {
      const body = init?.body ? JSON.parse(String(init.body)) : {}
      return jsonResponse({
        empty: [{ project: 'abandoned', session_count: 2, prompt_count: 0 }],
        sessions_removed: body.apply ? 2 : 0,
        prompts_removed: 0,
        dry_run: !body.apply,
      })
    }
    return jsonResponse({})
  })
  vi.stubGlobal('fetch', fetchMock)
})

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ProjectsPanel />
    </QueryClientProvider>,
  )
}

describe('ProjectsPanel', () => {
  it('renders the inventory tab with project counts', async () => {
    renderPanel()
    // After the Phase 7 refresh the project name appears in both the table
    // row AND the "top projects" bar chart, so we just assert presence.
    const matches = await screen.findAllByText('korva')
    expect(matches.length).toBeGreaterThan(0)
    expect(screen.getAllByText('vault-mcp').length).toBeGreaterThan(0)
    // The CardHeader title carries the project count.
    expect(screen.getByText('2 project(s) tracked')).toBeTruthy()
  })

  it('shows merge proposals on the Consolidate tab', async () => {
    renderPanel()
    // Tabs are accessible buttons with role=tab in the new design system.
    fireEvent.click(screen.getByRole('tab', { name: /consolidate/i }))
    expect(await screen.findByText(/1 merge candidate/i)).toBeTruthy()
    expect(screen.getByText('Korva')).toBeTruthy()
  })

  it('prune defaults to dry-run and requires confirmation before applying', async () => {
    renderPanel()
    fireEvent.click(screen.getByRole('tab', { name: /prune empty/i }))

    // First click runs dry-run (button stays as <button> in the refresh).
    fireEvent.click(screen.getByRole('button', { name: /dry-run scan/i }))
    expect(await screen.findByText('abandoned')).toBeTruthy()

    // "Apply…" button shows; clicking it asks for confirmation rather than
    // firing the mutation.
    fireEvent.click(screen.getByRole('button', { name: /^apply…$/i }))
    expect(
      screen.getByText(/This deletes 1 project's sessions/i),
    ).toBeTruthy()

    const applyCallsBefore = fetchMock.mock.calls.filter(call => {
      const url = String(call[0])
      const init = call[1] as RequestInit | undefined
      if (init?.method !== 'POST' || !url.includes('/admin/projects/prune')) return false
      const body = init.body ? JSON.parse(String(init.body)) : {}
      return body.apply === true
    })
    expect(applyCallsBefore.length).toBe(0)

    // "Confirm apply" fires the apply=true mutation.
    fireEvent.click(screen.getByRole('button', { name: /confirm apply/i }))
    await waitFor(() => {
      const applyCallsAfter = fetchMock.mock.calls.filter(call => {
        const url = String(call[0])
        const init = call[1] as RequestInit | undefined
        if (init?.method !== 'POST' || !url.includes('/admin/projects/prune')) return false
        const body = init.body ? JSON.parse(String(init.body)) : {}
        return body.apply === true
      })
      expect(applyCallsAfter.length).toBe(1)
    })
  })
})
