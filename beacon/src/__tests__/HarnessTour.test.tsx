import { describe, expect, it, vi, beforeEach } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import {
  HarnessTour,
  hasCompletedHarnessTour,
  markHarnessTourCompleted,
  resetHarnessTour,
} from '@/components/HarnessTour'

// Phase 15.C — verify the harness tour contract:
//   - Render shows step 1 with "Next" when open
//   - open=false renders nothing (no dialog leak under unrelated tests)
//   - Next/Back walk through all 5 steps in order
//   - Keyboard nav: Escape closes (not completed), Enter / ArrowRight
//     advance, ArrowLeft retreats
//   - Backdrop click closes without marking completed
//   - The CTA on the final step closes the tour AND marks it completed
//   - localStorage persistence helpers (has/mark/reset) round-trip
//   - Re-opening the tour resets progress to step 1

describe('HarnessTour', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('renders nothing when open=false', () => {
    render(<HarnessTour open={false} onClose={vi.fn()} />)
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('renders the first step when open=true', () => {
    render(<HarnessTour open={true} onClose={vi.fn()} />)
    expect(screen.getByRole('dialog', { name: /harness tour/i })).toBeInTheDocument()
    expect(screen.getByText(/welcome to the harness dashboard/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /next step/i })).toBeInTheDocument()
    // Step counter "1 / 5".
    expect(screen.getByText(/1 \/ 5/)).toBeInTheDocument()
  })

  it('Next walks all the way to the final step then becomes a CTA', () => {
    const onClose = vi.fn()
    render(<HarnessTour open={true} onClose={onClose} />)
    // Steps 1 → 5: 4 clicks of "Next" lands on the CTA step.
    const titles = [
      /spec_ready means awaiting human approval/i,
      /lint specs before approval/i,
      /wire the ci gate in two minutes/i,
      /you're ready/i,
    ]
    for (const title of titles) {
      fireEvent.click(screen.getByRole('button', { name: /next step/i }))
      expect(screen.getByText(title)).toBeInTheDocument()
    }
    // Final step swaps the Next button for the CTA link.
    expect(screen.queryByRole('button', { name: /next step|finish tour/i })).toBeNull()
    expect(screen.getByRole('link', { name: /read the docs/i })).toBeInTheDocument()
  })

  it('Back returns to the previous step and is disabled at step 1', () => {
    render(<HarnessTour open={true} onClose={vi.fn()} />)
    // Back is disabled on the first step.
    const back0 = screen.getByRole('button', { name: /previous step/i })
    expect(back0).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: /next step/i }))
    expect(screen.getByText(/spec_ready means awaiting human approval/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /previous step/i }))
    expect(screen.getByText(/welcome to the harness dashboard/i)).toBeInTheDocument()
  })

  it('clicking the CTA on the final step marks completed and calls onClose(true)', () => {
    const onClose = vi.fn()
    render(<HarnessTour open={true} onClose={onClose} />)
    for (let i = 0; i < 4; i++) {
      fireEvent.click(screen.getByRole('button', { name: /next step/i }))
    }
    fireEvent.click(screen.getByRole('link', { name: /read the docs/i }))
    expect(onClose).toHaveBeenCalledWith(true)
    expect(hasCompletedHarnessTour()).toBe(true)
  })

  it('Escape closes the tour without marking completed', () => {
    const onClose = vi.fn()
    render(<HarnessTour open={true} onClose={onClose} />)
    act(() => {
      fireEvent.keyDown(window, { key: 'Escape' })
    })
    expect(onClose).toHaveBeenCalledWith(false)
    expect(hasCompletedHarnessTour()).toBe(false)
  })

  it('Enter advances and ArrowLeft retreats', () => {
    render(<HarnessTour open={true} onClose={vi.fn()} />)
    act(() => {
      fireEvent.keyDown(window, { key: 'Enter' })
    })
    expect(screen.getByText(/spec_ready means awaiting human approval/i)).toBeInTheDocument()
    act(() => {
      fireEvent.keyDown(window, { key: 'ArrowRight' })
    })
    expect(screen.getByText(/lint specs before approval/i)).toBeInTheDocument()
    act(() => {
      fireEvent.keyDown(window, { key: 'ArrowLeft' })
    })
    expect(screen.getByText(/spec_ready means awaiting human approval/i)).toBeInTheDocument()
  })

  it('clicking the backdrop closes without marking completed', () => {
    const onClose = vi.fn()
    render(<HarnessTour open={true} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: /^close tour$/i }))
    expect(onClose).toHaveBeenCalledWith(false)
    expect(hasCompletedHarnessTour()).toBe(false)
  })

  it('clicking the X button closes without marking completed', () => {
    const onClose = vi.fn()
    render(<HarnessTour open={true} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: /^close$/i }))
    expect(onClose).toHaveBeenCalledWith(false)
    expect(hasCompletedHarnessTour()).toBe(false)
  })

  it('reopening resets progress to step 1', () => {
    const { rerender } = render(<HarnessTour open={true} onClose={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: /next step/i }))
    fireEvent.click(screen.getByRole('button', { name: /next step/i }))
    expect(screen.getByText(/3 \/ 5/)).toBeInTheDocument()
    // Close and reopen — progress should reset.
    rerender(<HarnessTour open={false} onClose={vi.fn()} />)
    rerender(<HarnessTour open={true} onClose={vi.fn()} />)
    expect(screen.getByText(/1 \/ 5/)).toBeInTheDocument()
    expect(screen.getByText(/welcome to the harness dashboard/i)).toBeInTheDocument()
  })

  it('progress dots show one active dot per step', () => {
    const { container } = render(<HarnessTour open={true} onClose={vi.fn()} />)
    // Active dot has w-6, inactive dots are w-1.5 — counting w-6 verifies "exactly one active".
    expect(container.querySelectorAll('.w-6').length).toBe(1)
    act(() => {
      fireEvent.click(screen.getByRole('button', { name: /next step/i }))
    })
    expect(container.querySelectorAll('.w-6').length).toBe(1)
  })

  it('storage helpers round-trip', () => {
    expect(hasCompletedHarnessTour()).toBe(false)
    markHarnessTourCompleted()
    expect(hasCompletedHarnessTour()).toBe(true)
    resetHarnessTour()
    expect(hasCompletedHarnessTour()).toBe(false)
  })

  it('does not auto-complete when the operator dismisses then re-mounts', () => {
    const onClose = vi.fn()
    const { rerender } = render(<HarnessTour open={true} onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: /^close$/i }))
    expect(hasCompletedHarnessTour()).toBe(false)
    // Re-render closed → next visit will reopen the tour because the
    // dashboard checks hasCompletedHarnessTour() on mount.
    rerender(<HarnessTour open={false} onClose={onClose} />)
    expect(hasCompletedHarnessTour()).toBe(false)
  })
})
