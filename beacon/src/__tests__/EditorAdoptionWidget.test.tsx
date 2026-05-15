import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { EditorAdoptionWidget } from '@/components/EditorAdoptionWidget'
import { useAdminStore } from '@/stores/admin'

// Phase 18.D — verify the adoption widget contract:
//   - shows "no traffic yet" hint when total=0
//   - renders rows with editor name, count, and % share
//   - anonymous (empty editor) rows render as "anonymous"
//   - shows loading skeleton during fetch
//   - shows error hint on a non-2xx
//   - honors a custom window-days prop in the API call

function renderWithProviders(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

function stubFetch(handler: (url: string) => Response | Promise<Response>) {
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : (input as URL).toString()
    return handler(url)
  }) as unknown as typeof fetch
}

beforeEach(() => {
  useAdminStore.setState({
    key: 'k',
    sessionToken: '',
    authMode: 'key',
    isAuthenticated: true,
  })
})

describe('EditorAdoptionWidget', () => {
  it('renders the empty-state hint when total = 0', async () => {
    stubFetch(() =>
      new Response(JSON.stringify({ window_days: 7, total: 0, rows: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    renderWithProviders(<EditorAdoptionWidget />)
    await waitFor(() =>
      expect(screen.getByText(/No traffic yet/i)).toBeInTheDocument(),
    )
    // The hint must teach the reader HOW to opt in.
    expect(screen.getByText(/X-Korva-Editor/)).toBeInTheDocument()
  })

  it('renders one bar per editor with count + percentage', async () => {
    stubFetch(() =>
      new Response(
        JSON.stringify({
          window_days: 7,
          total: 100,
          rows: [
            { editor: 'cursor', count: 60, by_channel: { http: 40, mcp: 20 } },
            { editor: 'claude', count: 30, by_channel: { http: 0, mcp: 30 } },
            { editor: '', count: 10, by_channel: { http: 10, mcp: 0 } },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    renderWithProviders(<EditorAdoptionWidget />)
    await waitFor(() => expect(screen.getByText('cursor')).toBeInTheDocument())
    expect(screen.getByText('claude')).toBeInTheDocument()
    // Empty editor → "anonymous" label.
    expect(screen.getByText('anonymous')).toBeInTheDocument()
    // Percentage rendering: 60 / 100 = 60.0%
    expect(screen.getByText(/60 · 60\.0%/)).toBeInTheDocument()
    expect(screen.getByText(/30 · 30\.0%/)).toBeInTheDocument()
    expect(screen.getByText(/10 · 10\.0%/)).toBeInTheDocument()
    // Total headline.
    expect(screen.getByText('100')).toBeInTheDocument()
  })

  // Phase 19.D — the per-row tooltip + aria-label encode the
  // channel split so an operator scanning the widget can tell
  // "this editor only shows up via MCP" without leaving the page.
  it('surfaces the http/mcp channel split via tooltip + aria-label', async () => {
    stubFetch(() =>
      new Response(
        JSON.stringify({
          window_days: 7,
          total: 30,
          rows: [
            { editor: 'cursor', count: 15, by_channel: { http: 5, mcp: 10 } },
            { editor: 'claude', count: 10, by_channel: { http: 0, mcp: 10 } },
            { editor: 'aider', count: 5, by_channel: { http: 5, mcp: 0 } },
          ],
        }),
        { status: 200, headers: { 'Content-Type': 'application/json' } },
      ),
    )
    renderWithProviders(<EditorAdoptionWidget />)
    await waitFor(() => expect(screen.getByText('cursor')).toBeInTheDocument())

    // Mixed: "http N · mcp M".
    const cursorLabel = screen.getByLabelText(/cursor.*http 5.*mcp 10/i)
    expect(cursorLabel).toBeInTheDocument()
    // MCP-only label.
    expect(screen.getByLabelText(/claude.*mcp only/i)).toBeInTheDocument()
    // HTTP-only label.
    expect(screen.getByLabelText(/aider.*http only/i)).toBeInTheDocument()
  })

  it('renders the error hint on a non-2xx', async () => {
    stubFetch(() => new Response('boom', { status: 500 }))
    renderWithProviders(<EditorAdoptionWidget />)
    await waitFor(() =>
      expect(screen.getByText(/Could not load adoption data/i)).toBeInTheDocument(),
    )
  })

  it('passes a custom window-days into the API request URL', async () => {
    let capturedURL = ''
    stubFetch((url) => {
      capturedURL = url
      return new Response(JSON.stringify({ window_days: 30, total: 0, rows: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    })
    renderWithProviders(<EditorAdoptionWidget windowDays={30} />)
    await waitFor(() => expect(capturedURL).toMatch(/days=30/))
    // Header still surfaces the window for visual confirmation.
    expect(screen.getByText(/last 30d/i)).toBeInTheDocument()
  })

  it('renders the section landmark with the right aria-label', async () => {
    stubFetch(() =>
      new Response(JSON.stringify({ window_days: 7, total: 0, rows: [] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    renderWithProviders(<EditorAdoptionWidget />)
    const region = screen.getByRole('region', { name: /editor adoption/i })
    expect(region).toBeInTheDocument()
  })
})
