import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { BarChart, DonutChart, LineChart, Sparkline } from "../index";

// Phase 7 — smoke + contract tests for the SVG-only chart primitives.
// We test the public surface (renders without crashing + key data ends up
// in the DOM); pixel-perfect visuals belong to a screenshot suite, not unit.

describe("Sparkline", () => {
	it("renders a flat line when data has fewer than 2 points", () => {
		const { container } = render(<Sparkline data={[5]} />);
		// Falls back to a single <line> placeholder.
		expect(container.querySelector("line")).toBeTruthy();
		expect(container.querySelector("path")).toBeNull();
	});

	it("draws line + area paths for a normal series", () => {
		const { container } = render(<Sparkline data={[1, 2, 3, 4, 5]} />);
		const paths = container.querySelectorAll("path");
		expect(paths.length).toBe(2); // area + line
	});
});

describe("LineChart", () => {
	it("renders one path per series", () => {
		const series = [
			{ name: "Saved", color: "var(--color-volt)", data: [1, 4, 2, 6, 3] },
			{ name: "Failed", color: "var(--color-rose-400)", data: [0, 1, 0, 2, 1] },
		];
		const { container } = render(
			<LineChart xLabels={["a", "b", "c", "d", "e"]} series={series} />,
		);
		const lines = container.querySelectorAll("path[stroke]");
		expect(lines.length).toBe(2);
	});

	it("uses role=img for accessibility", () => {
		render(
			<LineChart
				xLabels={["a", "b"]}
				series={[{ name: "x", color: "var(--color-cyan-400)", data: [1, 2] }]}
			/>,
		);
		expect(screen.getByRole("img")).toBeTruthy();
	});
});

describe("DonutChart", () => {
	it("renders one segment per data point + legend rows", () => {
		const data = [
			{ label: "decision", value: 12, color: "var(--color-volt)" },
			{ label: "pattern", value: 7, color: "var(--color-cyan-400)" },
			{ label: "bugfix", value: 3, color: "var(--color-coral)" },
		];
		const { container } = render(
			<DonutChart data={data} centerLabel="Total" />,
		);
		// 1 ring background + 3 segment circles = 4 circles in total.
		const circles = container.querySelectorAll("circle");
		expect(circles.length).toBe(4);
		// Legend lists every label.
		expect(screen.getByText("decision")).toBeTruthy();
		expect(screen.getByText("pattern")).toBeTruthy();
		expect(screen.getByText("bugfix")).toBeTruthy();
	});

	it("centre label + total are rendered when provided", () => {
		render(
			<DonutChart
				data={[{ label: "a", value: 1, color: "var(--color-volt)" }]}
				centerLabel="Total"
				centerValue={42}
			/>,
		);
		expect(screen.getByText("Total")).toBeTruthy();
		expect(screen.getByText("42")).toBeTruthy();
	});
});

describe("BarChart", () => {
	it("renders rows sorted descending and caps at maxRows", () => {
		const data = [
			{ label: "korva", value: 12 },
			{ label: "vault", value: 5 },
			{ label: "beacon", value: 9 },
			{ label: "sentinel", value: 1 },
		];
		render(<BarChart data={data} maxRows={2} />);
		const labels = screen.getAllByText(/korva|vault|beacon|sentinel/i);
		// Should only render the top 2 (korva, beacon).
		const visibleLabels = labels.map((n) => n.textContent);
		expect(visibleLabels).toContain("korva");
		expect(visibleLabels).toContain("beacon");
		expect(visibleLabels).not.toContain("sentinel");
		expect(visibleLabels).not.toContain("vault");
	});

	it("shows the empty message when data is empty", () => {
		render(<BarChart data={[]} emptyMessage="Nothing yet" />);
		expect(screen.getByText("Nothing yet")).toBeTruthy();
	});
});
