import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { ToastProvider, useToast } from "../Toast";

// Phase 8.1 — pin the toast contract: push → visible, dismiss → gone,
// auto-dismiss after duration for non-error tones, errors stick until
// dismissed manually. We use fake timers so the test runs instantly and
// is reliable on slow CI.

function wrap({ children }: { children: React.ReactNode }) {
	return <ToastProvider>{children}</ToastProvider>;
}

describe("Toast", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it("push adds a toast to the stack and renders it", () => {
		const view = render(
			<ToastProvider>
				<TriggerButton tone="success" title="Saved" message="All good" />
			</ToastProvider>,
		);
		act(() => {
			fireEvent.click(view.getByText("trigger"));
		});
		expect(screen.getByText("Saved")).toBeTruthy();
		expect(screen.getByText("All good")).toBeTruthy();
	});

	it("auto-dismisses non-error toasts after their duration", () => {
		const view = render(
			<ToastProvider>
				<TriggerButton tone="info" title="Heads up" />
			</ToastProvider>,
		);
		act(() => {
			fireEvent.click(view.getByText("trigger"));
		});
		expect(screen.getByText("Heads up")).toBeTruthy();
		act(() => {
			vi.advanceTimersByTime(5001);
		});
		expect(screen.queryByText("Heads up")).toBeNull();
	});

	it("errors stick until the user dismisses them", () => {
		const view = render(
			<ToastProvider>
				<TriggerButton tone="error" title="Boom" message="It broke" />
			</ToastProvider>,
		);
		act(() => {
			fireEvent.click(view.getByText("trigger"));
		});
		act(() => {
			vi.advanceTimersByTime(20_000);
		});
		// Still there after 20 s.
		expect(screen.getByText("Boom")).toBeTruthy();

		act(() => {
			fireEvent.click(screen.getByLabelText("Dismiss"));
		});
		expect(screen.queryByText("Boom")).toBeNull();
	});

	it("custom duration of 0 makes any tone sticky", () => {
		const view = render(
			<ToastProvider>
				<TriggerButton tone="success" title="Sticky" duration={0} />
			</ToastProvider>,
		);
		act(() => {
			fireEvent.click(view.getByText("trigger"));
		});
		act(() => {
			vi.advanceTimersByTime(60_000);
		});
		expect(screen.getByText("Sticky")).toBeTruthy();
	});

	it("useToast throws outside the provider", () => {
		// renderHook without ToastProvider; the hook must throw to make the
		// developer mistake loud rather than render with a stale context.
		expect(() => renderHook(() => useToast())).toThrow(/ToastProvider/);
	});

	it("clear() removes every toast", () => {
		const { result } = renderHook(() => useToast(), { wrapper: wrap });
		act(() => {
			result.current.push({ tone: "info", title: "a" });
			result.current.push({ tone: "warning", title: "b" });
			result.current.push({ tone: "error", title: "c" });
		});
		expect(result.current.toasts.length).toBe(3);
		act(() => {
			result.current.clear();
		});
		expect(result.current.toasts.length).toBe(0);
	});
});

function TriggerButton({
	tone,
	title,
	message,
	duration,
}: {
	tone: "success" | "info" | "warning" | "error";
	title: string;
	message?: string;
	duration?: number;
}) {
	const { push } = useToast();
	return (
		<button
			type="button"
			onClick={() => push({ tone, title, message, duration })}
		>
			trigger
		</button>
	);
}
