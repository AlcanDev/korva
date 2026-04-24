import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { InfoCallout } from '@/components/PageHeader'

beforeEach(() => {
  localStorage.clear()
})

describe('InfoCallout', () => {
  it('renders children content', () => {
    render(<InfoCallout>Some info text</InfoCallout>)
    expect(screen.getByText('Some info text')).toBeInTheDocument()
  })

  it('renders title when provided', () => {
    render(<InfoCallout title="Important">Content</InfoCallout>)
    expect(screen.getByText('Important')).toBeInTheDocument()
  })

  it('is not collapsible by default — always shows content', () => {
    render(<InfoCallout title="Notice">Always visible</InfoCallout>)
    expect(screen.getByText('Always visible')).toBeInTheDocument()
  })

  it('collapsible starts open by default and toggles closed', () => {
    render(
      <InfoCallout title="Tip" collapsible id="test-callout">
        Tip body
      </InfoCallout>
    )
    expect(screen.getByText('Tip body')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Tip'))
    expect(screen.queryByText('Tip body')).not.toBeInTheDocument()
  })

  it('collapsible can start closed via defaultOpen={false}', () => {
    render(
      <InfoCallout title="Glossary" collapsible id="glossary" defaultOpen={false}>
        Hidden body
      </InfoCallout>
    )
    expect(screen.queryByText('Hidden body')).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('Glossary'))
    expect(screen.getByText('Hidden body')).toBeInTheDocument()
  })

  it('persists open state in localStorage', () => {
    const { unmount } = render(
      <InfoCallout title="Persist" collapsible id="persist-test">
        Persistent body
      </InfoCallout>
    )
    // Close it
    fireEvent.click(screen.getByText('Persist'))
    unmount()

    // Re-render — should remember closed state
    render(
      <InfoCallout title="Persist" collapsible id="persist-test">
        Persistent body
      </InfoCallout>
    )
    expect(screen.queryByText('Persistent body')).not.toBeInTheDocument()
  })
})
