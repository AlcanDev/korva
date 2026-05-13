import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Routes, Route } from "react-router";
import { OnboardingTour, hasCompletedTour, markTourCompleted, resetTour } from "@/components/OnboardingTour";

// Phase 9.4 — verify the tour contract:
//   - Render shows step 0 with "Next"
//   - Next/Back advance + go back through the sequence
//   - Closing via X marks NOT completed
//   - Finishing the last "Got it" marks completed and closes
//   - hasCompletedTour reflects localStorage
//   - resetTour clears the flag

function wrap(node: React.ReactNode) {
	return (
		<MemoryRouter>
			<Routes>
				<Route path="/" element={node} />
			</Routes>
		</MemoryRouter>
	);
}

describe("OnboardingTour", () => {
	beforeEach(() => {
		window.localStorage.clear();
	});

	it("renders the first step when open", () => {
		render(wrap(<OnboardingTour open={true} onClose={vi.fn()} />));
		expect(screen.getByText(/welcome to korva beacon/i)).toBeTruthy();
		expect(screen.getByRole("button", { name: /next/i })).toBeTruthy();
	});

	it("does not render when open is false", () => {
		render(wrap(<OnboardingTour open={false} onClose={vi.fn()} />));
		expect(screen.queryByRole("dialog")).toBeNull();
	});

	it("Next advances and Back returns", () => {
		render(wrap(<OnboardingTour open={true} onClose={vi.fn()} />));
		fireEvent.click(screen.getByRole("button", { name: /next/i }));
		expect(screen.getByText(/⌘K opens everything/i)).toBeTruthy();
		fireEvent.click(screen.getByRole("button", { name: /back/i }));
		expect(screen.getByText(/welcome to korva beacon/i)).toBeTruthy();
	});

	it("closing via the X does not mark completed", () => {
		const onClose = vi.fn();
		render(wrap(<OnboardingTour open={true} onClose={onClose} />));
		fireEvent.click(screen.getByLabelText(/^close$/i));
		expect(onClose).toHaveBeenCalledWith(false);
		expect(hasCompletedTour()).toBe(false);
	});

	it('finishing the last step marks completed and calls onClose(true)', () => {
		const onClose = vi.fn();
		render(wrap(<OnboardingTour open={true} onClose={onClose} />));
		// Click Next until the last step then "Got it".
		// The tour has 8 steps — click Next 7 times, then "Got it".
		for (let i = 0; i < 7; i++) {
			fireEvent.click(screen.getByRole("button", { name: /next/i }));
		}
		fireEvent.click(screen.getByRole("button", { name: /got it/i }));
		expect(onClose).toHaveBeenCalledWith(true);
		expect(hasCompletedTour()).toBe(true);
	});

	it("markTourCompleted + resetTour interact correctly", () => {
		expect(hasCompletedTour()).toBe(false);
		markTourCompleted();
		expect(hasCompletedTour()).toBe(true);
		resetTour();
		expect(hasCompletedTour()).toBe(false);
	});

	it("progress dots show one active dot per step", () => {
		const { container } = render(
			wrap(<OnboardingTour open={true} onClose={vi.fn()} />),
		);
		// The active dot has w-6, inactive dots are w-1.5. Counting "w-6"
		// occurrences verifies exactly one active.
		const dots = container.querySelectorAll(".w-6");
		expect(dots.length).toBe(1);
		// Advance one step.
		act(() => {
			fireEvent.click(screen.getByRole("button", { name: /next/i }));
		});
		const dotsAfter = container.querySelectorAll(".w-6");
		expect(dotsAfter.length).toBe(1);
	});
});
