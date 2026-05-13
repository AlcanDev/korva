// Phase 7 — Beacon charts barrel. Pure-SVG components — no extra library.

export { Sparkline } from "./Sparkline";
export type { SparklineProps } from "./Sparkline";

export { LineChart } from "./LineChart";
export type { LineChartProps, LineChartSeries } from "./LineChart";

export { DonutChart } from "./DonutChart";
export type { DonutChartProps, DonutSegment } from "./DonutChart";

export { BarChart } from "./BarChart";
export type { BarChartProps, BarChartItem } from "./BarChart";

export { KnowledgeGraph } from "./KnowledgeGraph";
export type {
	KnowledgeGraphProps,
	KnowledgeGraphNode,
	KnowledgeGraphEdge,
} from "./KnowledgeGraph";

// Convenience palette aligned with the design system. Use these so charts
// look visually consistent across pages.
export const CHART_PALETTE = {
	volt: "var(--color-volt)",
	cyan: "var(--color-cyan-400)",
	indigo: "var(--color-indigo-400)",
	purple: "var(--color-purple-400)",
	coral: "var(--color-coral)",
	amber: "var(--color-amber-400)",
	emerald: "var(--color-emerald-400)",
	rose: "var(--color-rose-400)",
} as const;
