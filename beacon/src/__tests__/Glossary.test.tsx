import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { TermTooltip, GlossaryCallout } from '@/components/Glossary'
import { I18nProvider } from '@/contexts/i18n'

function Wrap({ children }: { children: React.ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>
}

describe('TermTooltip', () => {
  it('renders children text', () => {
    render(
      <Wrap>
        <TermTooltip term="Scroll" definition="A knowledge file.">
          Scroll
        </TermTooltip>
      </Wrap>
    )
    expect(screen.getByText('Scroll')).toBeInTheDocument()
  })

  it('shows tooltip on mouse enter', () => {
    render(
      <Wrap>
        <TermTooltip term="Vault" definition="The memory database.">
          Vault
        </TermTooltip>
      </Wrap>
    )
    const trigger = screen.getByRole('button', { name: /Definition: Vault/i })
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
    fireEvent.mouseEnter(trigger)
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    expect(screen.getByText('The memory database.')).toBeInTheDocument()
  })

  it('hides tooltip on mouse leave', () => {
    render(
      <Wrap>
        <TermTooltip term="Hive" definition="Public scroll library.">
          Hive
        </TermTooltip>
      </Wrap>
    )
    const trigger = screen.getByRole('button', { name: /Definition: Hive/i })
    fireEvent.mouseEnter(trigger)
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    fireEvent.mouseLeave(trigger)
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('shows tooltip on focus and hides on blur', () => {
    render(
      <Wrap>
        <TermTooltip term="MCP" definition="Model Context Protocol.">
          MCP
        </TermTooltip>
      </Wrap>
    )
    const trigger = screen.getByRole('button', { name: /Definition: MCP/i })
    fireEvent.focus(trigger)
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    fireEvent.blur(trigger)
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })
})

describe('GlossaryCallout', () => {
  beforeEach(() => { localStorage.clear() })

  it('renders 8 glossary terms when expanded', () => {
    render(
      <Wrap>
        <GlossaryCallout />
      </Wrap>
    )
    // GlossaryCallout defaults closed — click header div to open
    const header = screen.getByText(/Korva Glossary/i)
    fireEvent.click(header.closest('div')!)

    const termNames = ['Scroll', 'Skill', 'Vault', 'Observation', 'Hive', 'Sentinel', 'SDD', 'MCP']
    for (const term of termNames) {
      expect(screen.getAllByText(term).length).toBeGreaterThanOrEqual(1)
    }
  })

  it('starts collapsed and toggles open', () => {
    render(
      <Wrap>
        <GlossaryCallout />
      </Wrap>
    )
    // Scroll definition text not visible initially (collapsed)
    expect(screen.queryByText(/A Markdown file/i)).not.toBeInTheDocument()
    // Click the header container div (the clickable parent, not the text span)
    const headerSpan = screen.getByText(/Korva Glossary/i)
    fireEvent.click(headerSpan.closest('div')!)
    expect(screen.getByText(/A Markdown file/i)).toBeInTheDocument()
  })
})
