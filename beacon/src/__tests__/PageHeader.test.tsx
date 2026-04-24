import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Database } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { I18nProvider } from '@/contexts/i18n'

function renderHeader(props: Parameters<typeof PageHeader>[0]) {
  return render(
    <I18nProvider>
      <PageHeader {...props} />
    </I18nProvider>
  )
}

describe('PageHeader', () => {
  it('renders title and description', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Test Title',
      description: 'Test description text',
    })
    expect(screen.getByText('Test Title')).toBeInTheDocument()
    expect(screen.getByText('Test description text')).toBeInTheDocument()
  })

  it('renders badge when provided', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Title',
      description: 'Desc',
      badge: 'Teams',
    })
    expect(screen.getByText('Teams')).toBeInTheDocument()
  })

  it('does not render badge when not provided', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Title',
      description: 'Desc',
    })
    expect(screen.queryByText('Teams')).not.toBeInTheDocument()
  })

  it('renders CLI hint command when provided', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Title',
      description: 'Desc',
      hint: { command: 'korva vault list', label: 'list all' },
    })
    expect(screen.getByText('korva vault list')).toBeInTheDocument()
    expect(screen.getByText('list all')).toBeInTheDocument()
  })

  it('renders copy button for hint command', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Title',
      description: 'Desc',
      hint: { command: 'korva vault list' },
    })
    expect(screen.getByTitle('Copy command')).toBeInTheDocument()
  })

  it('renders actions when provided', () => {
    renderHeader({
      icon: <Database size={17} />,
      title: 'Title',
      description: 'Desc',
      actions: <button>Action</button>,
    })
    expect(screen.getByRole('button', { name: 'Action' })).toBeInTheDocument()
  })
})
