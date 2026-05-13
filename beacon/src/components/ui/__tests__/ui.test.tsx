import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import {
	Badge,
	Button,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	ErrorBanner,
	MetricCard,
	Skeleton,
	Spinner,
	StatusDot,
	Tabs,
} from "../index";

// Phase 7 — sanity tests for the new UI primitives. We test public contract
// (props -> rendered output / event firing) rather than implementation
// details; the goal is to catch regressions when the design system evolves.

describe("Card", () => {
	it("renders children and respects the className prop", () => {
		render(
			<Card className="custom-class" data-testid="card">
				hello
			</Card>,
		);
		const card = screen.getByTestId("card");
		expect(card.textContent).toBe("hello");
		expect(card.className).toContain("custom-class");
	});

	it("applies the glass variant", () => {
		render(<Card variant="glass" data-testid="glass" />);
		expect(screen.getByTestId("glass").className).toContain("beacon-glass");
	});

	it("CardHeader renders title + subtitle + actions", () => {
		render(
			<Card>
				<CardHeader title="Hi" subtitle="A subtitle" actions={<span>x</span>} />
				<CardBody>body</CardBody>
			</Card>,
		);
		expect(screen.getByText("Hi")).toBeTruthy();
		expect(screen.getByText("A subtitle")).toBeTruthy();
		expect(screen.getByText("x")).toBeTruthy();
	});
});

describe("Button", () => {
	it("fires onClick when not loading", () => {
		const onClick = vi.fn();
		render(
			<Button onClick={onClick} variant="primary">
				Run
			</Button>,
		);
		fireEvent.click(screen.getByRole("button", { name: /run/i }));
		expect(onClick).toHaveBeenCalledTimes(1);
	});

	it("disables and aria-busy=true while loading", () => {
		const onClick = vi.fn();
		render(
			<Button onClick={onClick} loading>
				Run
			</Button>,
		);
		const btn = screen.getByRole("button") as HTMLButtonElement;
		expect(btn.disabled).toBe(true);
		expect(btn.getAttribute("aria-busy")).toBe("true");
		fireEvent.click(btn);
		expect(onClick).not.toHaveBeenCalled();
	});

	it("renders left icon and not the label when size=icon", () => {
		render(
			<Button size="icon" leftIcon={<svg data-testid="icn" />}>
				hidden
			</Button>,
		);
		expect(screen.getByTestId("icn")).toBeTruthy();
		expect(screen.queryByText("hidden")).toBeNull();
	});
});

describe("Badge / StatusDot", () => {
	it("applies tone classes", () => {
		render(<Badge tone="success">OK</Badge>);
		const node = screen.getByText("OK");
		expect(node.className).toContain("text-volt");
	});

	it("StatusDot picks the right palette per state", () => {
		const { rerender, container } = render(<StatusDot state="running" />);
		expect(container.firstChild!).toHaveProperty("className");
		expect((container.firstChild as HTMLElement).className).toContain("bg-volt");
		rerender(<StatusDot state="error" />);
		expect((container.firstChild as HTMLElement).className).toContain("bg-[#F85149]");
	});
});

describe("MetricCard", () => {
	it("renders label + value + trend", () => {
		render(
			<MetricCard
				label="Observations"
				value="1,234"
				trend={{ value: 12.5, direction: "up", label: "vs last week" }}
			/>,
		);
		expect(screen.getByText("Observations")).toBeTruthy();
		expect(screen.getByText("1,234")).toBeTruthy();
		expect(screen.getByText(/12\.5%/)).toBeTruthy();
		expect(screen.getByText(/vs last week/)).toBeTruthy();
	});

	it("formats negative trend with the down arrow class", () => {
		const { container } = render(
			<MetricCard label="Errors" value="3" trend={{ value: -8.3, direction: "down" }} />,
		);
		// The absolute value is rendered.
		expect(screen.getByText(/8\.3%/)).toBeTruthy();
		// And the trend span is coloured red.
		const trend = container.querySelector(".text-\\[\\#F85149\\]");
		expect(trend).toBeTruthy();
	});
});

describe("Feedback", () => {
	it("Spinner has role=status", () => {
		render(<Spinner />);
		expect(screen.getByRole("status")).toBeTruthy();
	});

	it("Skeleton renders with custom dimensions", () => {
		const { container } = render(<Skeleton width={120} height={20} />);
		const sk = container.firstChild as HTMLElement;
		expect(sk.style.width).toBe("120px");
		expect(sk.style.height).toBe("20px");
	});

	it("EmptyState renders title + optional CTA", () => {
		render(
			<EmptyState
				title="Nothing here"
				description="Try again later"
				action={<button type="button">Retry</button>}
			/>,
		);
		expect(screen.getByText("Nothing here")).toBeTruthy();
		expect(screen.getByText("Try again later")).toBeTruthy();
		expect(screen.getByRole("button", { name: /retry/i })).toBeTruthy();
	});

	it("ErrorBanner has role=alert", () => {
		render(<ErrorBanner title="Oops" message="Network down" />);
		const banner = screen.getByRole("alert");
		expect(banner.textContent).toContain("Network down");
	});
});

describe("Tabs", () => {
	it("renders all tabs and fires onChange on click", () => {
		const onChange = vi.fn();
		const tabs = [
			{ value: "a", label: "A" },
			{ value: "b", label: "B" },
		];
		render(<Tabs value="a" onChange={onChange} tabs={tabs} />);
		fireEvent.click(screen.getByRole("tab", { name: "B" }));
		expect(onChange).toHaveBeenCalledWith("b");
	});

	it("marks the active tab via aria-selected", () => {
		const tabs = [
			{ value: "a", label: "A" },
			{ value: "b", label: "B" },
		];
		render(<Tabs value="b" onChange={vi.fn()} tabs={tabs} />);
		const tabB = screen.getByRole("tab", { name: "B" });
		const tabA = screen.getByRole("tab", { name: "A" });
		expect(tabB.getAttribute("aria-selected")).toBe("true");
		expect(tabA.getAttribute("aria-selected")).toBe("false");
	});

	it("pill variant renders without crashing", () => {
		const tabs = [
			{ value: "a", label: "A" },
			{ value: "b", label: "B" },
		];
		render(<Tabs variant="pill" value="a" onChange={vi.fn()} tabs={tabs} />);
		expect(screen.getByRole("tab", { name: "A" })).toBeTruthy();
	});
});
