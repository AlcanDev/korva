import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route, useLocation } from 'react-router'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Observatory, { OBSERVATORY_BASE } from '../Observatory'
import { I18nProvider } from '@/contexts/i18n'

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

function jsonResponse(body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  )
}

const systemStatusFixture = {
  ide: [],
  vault: { running: true, port: 7437, pid: 1, uptime_sec: 42, version: 'test' },
  hive: { enabled: false, pending_outbox: 0, consecutive_errors: 0 },
  sentinel: {
    enabled: true,
    hooks_installed: [],
    rules_total: 4,
    builtin_count: 4,
    custom_count: 0,
    rules_path: '',
    profile: 'standard',
  },
  lore: { active_scrolls: [], available_scrolls_count: 0 },
  skills: { installed_count: 0, last_sync_at: null, sync_status: 'ok' },
  license: { tier: 'community', expiration_at: null, seats_used: 0, seats_total: 0 },
  sessions: { total: 0, active_24h: 0 },
  observations: { total: 0, by_type: {} },
  prompts: { total: 0 },
}

const tokenStatsFixture = {
  totals: {
    input_tokens: 0,
    output_tokens: 0,
    cache_read: 0,
    cache_creation: 0,
    interactions_count: 0,
    estimated_count: 0,
  },
  cache_hit_pct: 0,
  reduction_pct_estimated: 0,
  baseline_naive_tokens: 0,
  baseline_dir: '/tmp',
  by_model: {},
  by_project: {},
  daily: [],
}

const fetchMock = vi.fn(async (input?: RequestInfo | URL | string | null) => {
  const url = input == null ? '' : typeof input === 'string' ? input : String(input)
  if (url.includes('/admin/system-status')) return jsonResponse(systemStatusFixture)
  if (url.includes('/admin/tokens/stats')) return jsonResponse(tokenStatsFixture)
  if (url.includes('/admin/activity')) return jsonResponse({ interactions: [], total: 0, limit: 50, offset: 0 })
  if (url.includes('/admin/config')) return jsonResponse({
    scope: 'local',
    path: '/tmp/korva.config.json',
    hash: 'h',
    schema_version: '1',
    exists: true,
    config: {
      version: '1',
      project: 'test',
      team: '',
      country: 'CL',
      agent: 'claude',
      vault: { port: 7437, auto_start: true },
      lore: { active_scrolls: [] },
      sentinel: { enabled: true, hooks: ['pre-commit'] },
      hive: { enabled: false, endpoint: '', interval_minutes: 15, allowed_types: [] },
      license: {},
    },
  })
  if (url.includes('/admin/sentinel/rules')) return jsonResponse({
    profile: 'standard',
    rules_path: '/tmp/rules.yaml',
    builtin: [],
    custom: [],
  })
  if (url.includes('/admin/conflicts')) return jsonResponse({
    conflicts: [],
    count: 0,
    status: 'pending',
  })
  if (url.includes('/admin/projects/suggestions')) return jsonResponse({
    proposals: [],
    count: 0,
  })
  if (url.includes('/admin/projects')) return jsonResponse({
    projects: [],
    count: 0,
  })
  if (url.includes('/admin/commands')) return jsonResponse({
    commands: [],
    local_only: true,
  })
  if (url.includes('/admin/cost/summary')) return jsonResponse({
    window_days: 30,
    from: '2026-04-12T00:00:00Z',
    to: '2026-05-12T00:00:00Z',
    total_usd: 0,
    total_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_read: 0,
    cache_hit_pct: 0,
    savings_usd: 0,
    reduction_pct: 0,
    by_model: [],
    by_project: [],
    daily: [],
    interactions_count: 0,
  })
  if (url.includes('/admin/privacy/stats')) return jsonResponse({
    total_events: 0,
    total_chars_removed: 0,
    by_type: {},
    since: '2026-05-12T00:00:00Z',
    since_unix: 1736640000,
  })
  if (url.includes('/admin/cost/anomalies')) return jsonResponse({
    window_days: 30,
    anomalies: [],
  })
  if (url.includes('/admin/graph')) return jsonResponse({
    project: 'test',
    nodes: [],
    edges: [],
    truncated: false,
  })
  return jsonResponse({})
})
vi.stubGlobal('fetch', fetchMock)

// Exposes the current URL so click tests can assert on it.
function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="probe-url">{loc.pathname}</div>
}

function renderAt(initialPath: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <I18nProvider>
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={[initialPath]}>
          <Routes>
            <Route path="/admin/observatory/*" element={<Observatory />} />
          </Routes>
          <LocationProbe />
        </MemoryRouter>
      </QueryClientProvider>
    </I18nProvider>,
  )
}

beforeEach(() => fetchMock.mockClear())

describe('Observatory navigation', () => {
  it('OBSERVATORY_BASE matches the parent route in Admin.tsx', () => {
    // If this constant drifts from the Admin.tsx route, the absolute hrefs
    // below break silently — this is a guard, not an implementation detail.
    expect(OBSERVATORY_BASE).toBe('/admin/observatory')
  })

  it('renders 13 sub-tabs with absolute hrefs from /admin/observatory/health', () => {
    renderAt('/admin/observatory/health')
    const nav = screen.getByRole('navigation', { name: /observatory sections/i })
    const hrefs = Array.from(nav.querySelectorAll('a')).map(
      (a) => (a as HTMLAnchorElement).getAttribute('href'),
    )
    expect(hrefs).toEqual([
      '/admin/observatory/health',
      '/admin/observatory/live',
      '/admin/observatory/cost',
      '/admin/observatory/privacy',
      '/admin/observatory/graph',
      '/admin/observatory/tokens',
      '/admin/observatory/activity',
      '/admin/observatory/commands',
      '/admin/observatory/conflicts',
      '/admin/observatory/projects',
      '/admin/observatory/export',
      '/admin/observatory/config',
      '/admin/observatory/sentinel',
    ])
  })

  it('keeps hrefs absolute regardless of where the user is', () => {
    renderAt('/admin/observatory/tokens')
    const nav = screen.getByRole('navigation', { name: /observatory sections/i })
    const hrefs = Array.from(nav.querySelectorAll('a')).map(
      (a) => (a as HTMLAnchorElement).getAttribute('href'),
    )
    expect(hrefs).toEqual([
      '/admin/observatory/health',
      '/admin/observatory/live',
      '/admin/observatory/cost',
      '/admin/observatory/privacy',
      '/admin/observatory/graph',
      '/admin/observatory/tokens',
      '/admin/observatory/activity',
      '/admin/observatory/commands',
      '/admin/observatory/conflicts',
      '/admin/observatory/projects',
      '/admin/observatory/export',
      '/admin/observatory/config',
      '/admin/observatory/sentinel',
    ])
  })

  it('clicking a tab navigates to the absolute URL (no path stacking)', async () => {
    renderAt('/admin/observatory/health')
    const tokensLink = screen.getByRole('link', { name: /tokens/i })
    fireEvent.click(tokensLink)
    await waitFor(() => {
      expect(screen.getByTestId('probe-url').textContent).toBe('/admin/observatory/tokens')
    })
  })

  it('chaining clicks does not concatenate paths', async () => {
    renderAt('/admin/observatory/health')
    fireEvent.click(screen.getByRole('link', { name: /tokens/i }))
    fireEvent.click(screen.getByRole('link', { name: /activity/i }))
    fireEvent.click(screen.getByRole('link', { name: /sentinel rules/i }))
    await waitFor(() => {
      expect(screen.getByTestId('probe-url').textContent).toBe('/admin/observatory/sentinel')
    })
  })

  it('marks only the active tab with the active style', () => {
    renderAt('/admin/observatory/activity')
    const activityLink = screen.getByRole('link', { name: /activity/i })
    expect(activityLink.className).toContain('border-[#388bfd]')

    const tokensLink = screen.getByRole('link', { name: /tokens/i })
    expect(tokensLink.className).not.toContain('border-[#388bfd]')
  })

  it('unknown sub-path falls back to /health (no blank screen)', async () => {
    renderAt('/admin/observatory/does-not-exist')
    await waitFor(() => {
      expect(screen.getByTestId('probe-url').textContent).toBe('/admin/observatory/health')
    })
  })

  it('bare /admin/observatory redirects to /admin/observatory/health', async () => {
    renderAt('/admin/observatory')
    await waitFor(() => {
      expect(screen.getByTestId('probe-url').textContent).toBe('/admin/observatory/health')
    })
  })

  it.each([
    ['health', /System Health/i],
    ['live', /Live activity/i],
    ['cost', /Cost & ROI/i],
    ['privacy', /Privacy meter/i],
    ['graph', /Knowledge graph/i],
    ['tokens', /Token Analytics/i],
    ['activity', /Activity Timeline/i],
    ['commands', /Commands/i],
    ['conflicts', /Conflicts/i],
    ['projects', /Projects/i],
    ['export', /Obsidian export/i],
    ['config', /Configuration/i],
    ['sentinel', /Sentinel Rules/i],
  ])('mounts the %s page heading at /admin/observatory/%s', async (slug, heading) => {
    renderAt(`/admin/observatory/${slug}`)
    await waitFor(() => {
      // Some pages take a tick to fetch and render; the heading is in the
      // initial render. We assert at least one element matches the heading.
      const elements = screen.getAllByText(heading)
      expect(elements.length).toBeGreaterThan(0)
    })
  })
})
