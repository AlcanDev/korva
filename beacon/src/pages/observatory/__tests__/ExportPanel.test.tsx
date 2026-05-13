import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import ExportPanel from '../ExportPanel'

// Fase 6.2 — verifica el contrato del Export panel: el botón queda
// deshabilitado sin out, el submit dispara POST con la forma correcta,
// y la card de resultado muestra los contadores devueltos por el backend.

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

const exportFixture = {
  out_dir: '/tmp/vault',
  file_count: 7,
  project_count: 2,
  by_project: { korva: 5, 'vault-mcp': 2 },
  by_type: { decision: 4, pattern: 2, learning: 1 },
  generated_at: '2026-05-12T20:00:00Z',
}

let fetchMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  fetchMock = vi.fn(async (input?: RequestInfo | URL | string | null, init?: RequestInit) => {
    const url = input == null ? '' : typeof input === 'string' ? input : String(input)
    const method = init?.method ?? 'GET'
    if (method === 'GET' && url.includes('/admin/projects'))
      return jsonResponse({ projects: [{ name: 'korva', observation_count: 5, session_count: 1 }], count: 1 })
    if (method === 'POST' && url.includes('/admin/export/obsidian')) {
      return jsonResponse(exportFixture)
    }
    return jsonResponse({})
  })
  vi.stubGlobal('fetch', fetchMock)
})

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <ExportPanel />
    </QueryClientProvider>,
  )
}

describe('ExportPanel', () => {
  it('disables the Run button until an output directory is provided', () => {
    renderPanel()
    const btn = screen.getByRole('button', { name: /run export/i }) as HTMLButtonElement
    expect(btn.disabled).toBe(true)
  })

  it('fires POST /admin/export/obsidian with the form values', async () => {
    renderPanel()
    const input = screen.getByLabelText(/output directory/i)
    fireEvent.change(input, { target: { value: '/tmp/vault' } })

    const btn = screen.getByRole('button', { name: /run export/i })
    fireEvent.click(btn)

    await waitFor(() => {
      const exportCall = fetchMock.mock.calls.find(call => {
        const url = String(call[0])
        const init = call[1] as RequestInit | undefined
        return init?.method === 'POST' && url.includes('/admin/export/obsidian')
      })
      expect(exportCall).toBeTruthy()
      const init = exportCall![1] as RequestInit
      const body = JSON.parse(String(init.body))
      expect(body.out).toBe('/tmp/vault')
      // project and type were left blank → must be omitted (undefined) so the
      // backend interprets that as "all".
      expect(body.project).toBeUndefined()
      expect(body.type).toBeUndefined()
    })
  })

  it('renders the result card with counts and the output path', async () => {
    renderPanel()
    fireEvent.change(screen.getByLabelText(/output directory/i), {
      target: { value: '/tmp/vault' },
    })
    fireEvent.click(screen.getByRole('button', { name: /run export/i }))

    expect(await screen.findByText(/export written/i)).toBeTruthy()
    expect(screen.getByText('/tmp/vault')).toBeTruthy()
    expect(screen.getByText('7')).toBeTruthy() // file_count
    expect(screen.getByText('decision:')).toBeTruthy()
  })
})
