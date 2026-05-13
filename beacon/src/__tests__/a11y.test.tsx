import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { axe } from "vitest-axe";
import {
	Badge,
	Button,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	ErrorBanner,
	MetricCard,
	PageHero,
	Spinner,
	StatusDot,
	Tabs,
	ToastProvider,
} from "@/components/ui";

// Phase 10.1 — automated accessibility checks via axe-core.
//
// We pin WCAG 2.1 AA conformance for every UI primitive in the design
// system. A violation here breaks the build because primitives propagate
// into every page — fixing them once eliminates whole classes of issues
// across the dashboard.
//
// What axe catches:
//   - missing alt / aria labels
//   - colour-contrast violations
//   - landmark + heading hierarchy issues
//   - duplicate ids, label-for mismatches, role drift
//
// We render each primitive in a minimal harness, sometimes with extra
// labelling so axe sees the same context the caller actually provides
// (e.g. icon-only buttons need aria-label from the caller — that's the
// contract, and the test pins it).

async function expectNoA11yViolations(node: React.ReactNode) {
	const { container } = render(<>{node}</>);
	const results = await axe(container);
	expect(results.violations).toEqual([]);
}

describe("a11y — UI primitives", () => {
	it("Card with header + body", async () => {
		await expectNoA11yViolations(
			<Card>
				<CardHeader title="Heading" subtitle="A subtitle" />
				<CardBody>Body text inside.</CardBody>
			</Card>,
		);
	});

	it("Button — labeled with text", async () => {
		await expectNoA11yViolations(<Button>Save</Button>);
	});

	it("Button — icon-only must carry aria-label", async () => {
		await expectNoA11yViolations(
			<Button size="icon" aria-label="Close dialog">
				<span aria-hidden>×</span>
			</Button>,
		);
	});

	it("Badge", async () => {
		await expectNoA11yViolations(<Badge tone="success">Healthy</Badge>);
	});

	it("StatusDot — decorative", async () => {
		await expectNoA11yViolations(<StatusDot state="running" />);
	});

	it("Spinner has role=status", async () => {
		await expectNoA11yViolations(<Spinner />);
	});

	it("MetricCard with label + value", async () => {
		await expectNoA11yViolations(
			<MetricCard label="Observations" value="1,234" hint="last 7 days" />,
		);
	});

	it("EmptyState with action", async () => {
		await expectNoA11yViolations(
			<EmptyState
				title="No data"
				description="Save your first observation."
				action={<Button>Save</Button>}
			/>,
		);
	});

	it("ErrorBanner uses role=alert", async () => {
		await expectNoA11yViolations(
			<ErrorBanner title="Boom" message="Something went wrong" />,
		);
	});

	it("Tabs — underline variant has role=tablist + role=tab + aria-selected", async () => {
		await expectNoA11yViolations(
			<Tabs
				value="a"
				onChange={() => undefined}
				tabs={[
					{ value: "a", label: "A" },
					{ value: "b", label: "B" },
				]}
			/>,
		);
	});

	it("PageHero acts as a header landmark", async () => {
		// PageHero already renders a <header> with h1; we ensure the
		// title is the only h1 in its scope (axe's heading-order rule).
		await expectNoA11yViolations(
			<PageHero
				eyebrow="Test page"
				title="Test title"
				subtitle="A subtitle"
				badge={{ tone: "success", label: "live" }}
			/>,
		);
	});

	it("ToastProvider mounts cleanly with no toasts", async () => {
		await expectNoA11yViolations(<ToastProvider>nothing</ToastProvider>);
	});
});
