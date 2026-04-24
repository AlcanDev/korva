import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AdminLogin from '@/pages/admin/AdminLogin'
import { I18nProvider } from '@/contexts/i18n'
import { useAdminStore } from '@/stores/admin'

vi.mock('@/api/admin', () => ({
  checkAdminKey: vi.fn(),
}))

import { checkAdminKey } from '@/api/admin'
const mockCheck = vi.mocked(checkAdminKey)

function renderLogin() {
  return render(
    <I18nProvider>
      <AdminLogin />
    </I18nProvider>
  )
}

describe('AdminLogin', () => {
  beforeEach(() => {
    useAdminStore.setState({ key: '', isAuthenticated: false })
    vi.clearAllMocks()
  })

  it('renders the login form', () => {
    renderLogin()
    expect(screen.getByText('Korva Admin')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Access Admin Panel/i })).toBeInTheDocument()
  })

  it('submit button is disabled when key is empty', () => {
    renderLogin()
    const button = screen.getByRole('button', { name: /Access Admin Panel/i })
    expect(button).toBeDisabled()
  })

  it('submit button enables when key is typed', async () => {
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'my-test-key')
    const button = screen.getByRole('button', { name: /Access Admin Panel/i })
    expect(button).not.toBeDisabled()
  })

  it('shows keyTooShort warning for short keys', async () => {
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'short')
    expect(screen.getByText(/looks too short/i)).toBeInTheDocument()
  })

  it('does not show keyTooShort warning for sufficiently long keys', async () => {
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'this-is-a-sufficiently-long-key-123')
    expect(screen.queryByText(/looks too short/i)).not.toBeInTheDocument()
  })

  it('calls checkAdminKey on submit', async () => {
    mockCheck.mockResolvedValue(true)
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'valid-key-32chars-abcdefghijklmnop')
    fireEvent.submit(input.closest('form')!)
    await waitFor(() => expect(mockCheck).toHaveBeenCalledWith('valid-key-32chars-abcdefghijklmnop'))
  })

  it('shows error message on invalid key', async () => {
    mockCheck.mockResolvedValue(false)
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'wrong-key-32chars-abcdefghijklmnop')
    fireEvent.submit(input.closest('form')!)
    await waitFor(() => expect(screen.getByText(/Invalid admin key/i)).toBeInTheDocument())
  })

  it('shows vault error when checkAdminKey throws', async () => {
    mockCheck.mockRejectedValue(new Error('Network error'))
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    await userEvent.type(input, 'some-key-32chars-abcdefghijklmnopq')
    fireEvent.submit(input.closest('form')!)
    await waitFor(() => expect(screen.getByText(/Could not connect to Vault/i)).toBeInTheDocument())
  })

  it('locks out after 5 failed attempts', async () => {
    mockCheck.mockResolvedValue(false)
    renderLogin()
    const input = screen.getByPlaceholderText(/Paste your admin key/i)
    const form = input.closest('form')!

    for (let i = 0; i < 5; i++) {
      await userEvent.clear(input)
      await userEvent.type(input, 'wrong-key-32chars-abcdefghijklmnop')
      fireEvent.submit(form)
      await waitFor(() => expect(mockCheck).toHaveBeenCalledTimes(i + 1))
    }

    await waitFor(() =>
      expect(screen.getByText(/Too many failed attempts/i)).toBeInTheDocument()
    )
    expect(screen.getByRole('button', { name: /Access Admin Panel/i })).toBeDisabled()
  })
})
