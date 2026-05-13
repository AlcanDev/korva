import { useId, useMemo, useState } from "react";

// Phase 7 — LineChart: SVG-only time-series chart. One or more series,
// auto-fitted Y axis, grid lines, hover tooltip. Designed for the Dashboard
// "activity over time" + Tokens analytics, where we want a clean look
// without pulling in recharts/chart.js (≥80 KB extra).

export interface LineChartSeries {
	name: string;
	color: string;
	data: number[];
}

export interface LineChartProps {
	xLabels: string[];               // 1 label per data point, length matches series.data
	series: LineChartSeries[];
	height?: number;
	yFormatter?: (n: number) => string;
	className?: string;
}

export function LineChart({
	xLabels,
	series,
	height = 200,
	yFormatter = (n) => n.toLocaleString(),
	className = "",
}: LineChartProps) {
	const id = useId();
	const [hoverIdx, setHoverIdx] = useState<number | null>(null);

	const { paths, maxY, minY, viewW, viewH, gridY } = useMemo(() => {
		const viewW = 600;
		const viewH = 160;
		const allValues = series.flatMap((s) => s.data);
		const maxY = Math.max(...allValues, 1);
		// Round max up to a "nice" number so the Y-axis labels read clean.
		const niceMax = niceCeil(maxY);
		const minY = 0;
		const step = xLabels.length > 1 ? viewW / (xLabels.length - 1) : viewW;

		const paths = series.map((s) => {
			const pts = s.data.map((v, i) => {
				const x = i * step;
				const y = viewH - ((v - minY) / (niceMax - minY)) * viewH;
				return { x, y };
			});
			const linePath = pts
				.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(2)},${p.y.toFixed(2)}`)
				.join(" ");
			return { ...s, pts, linePath };
		});

		// Five horizontal grid lines (0%, 25%, 50%, 75%, 100%).
		const gridY = [0, 0.25, 0.5, 0.75, 1].map((pct) => ({
			y: viewH - pct * viewH,
			value: niceMax * pct,
		}));

		return { paths, maxY: niceMax, minY, viewW, viewH, gridY };
	}, [series, xLabels.length]);

	const hoveredLabel = hoverIdx !== null ? xLabels[hoverIdx] : null;
	const hoveredValues =
		hoverIdx !== null
			? paths.map((p) => ({ name: p.name, value: p.data[hoverIdx], color: p.color }))
			: [];

	return (
		<div className={`relative ${className}`} style={{ height }}>
			<svg
				viewBox={`0 -8 ${viewW + 60} ${viewH + 40}`}
				preserveAspectRatio="none"
				className="w-full h-full"
				role="img"
				aria-label={`Line chart with ${series.length} series`}
				onMouseLeave={() => setHoverIdx(null)}
				onMouseMove={(e) => {
					const svg = e.currentTarget;
					const rect = svg.getBoundingClientRect();
					const x = ((e.clientX - rect.left) / rect.width) * (viewW + 60) - 8;
					const idx = Math.round((x / viewW) * (xLabels.length - 1));
					if (idx >= 0 && idx < xLabels.length) setHoverIdx(idx);
				}}
			>
				{/* Grid */}
				{gridY.map((g, i) => (
					<g key={`grid-${i}`}>
						<line
							x1={0}
							x2={viewW}
							y1={g.y}
							y2={g.y}
							stroke="rgba(255,255,255,0.05)"
							strokeWidth="1"
						/>
						<text
							x={viewW + 6}
							y={g.y + 3}
							fontSize="9"
							fill="var(--color-ink-500)"
							fontFamily="var(--font-mono)"
						>
							{yFormatter(g.value)}
						</text>
					</g>
				))}

				{/* X axis line */}
				<line x1={0} x2={viewW} y1={viewH} y2={viewH} stroke="rgba(255,255,255,0.08)" />

				{/* Series areas + lines */}
				{paths.map((p, i) => (
					<g key={`series-${i}-${id}`}>
						<path d={p.linePath} fill="none" stroke={p.color} strokeWidth="1.8" strokeLinejoin="round" />
					</g>
				))}

				{/* Hover crosshair + dots */}
				{hoverIdx !== null && paths.length > 0 && paths[0].pts[hoverIdx] && (
					<g>
						<line
							x1={paths[0].pts[hoverIdx].x}
							x2={paths[0].pts[hoverIdx].x}
							y1={0}
							y2={viewH}
							stroke="rgba(255,255,255,0.12)"
							strokeWidth="1"
							strokeDasharray="2,2"
						/>
						{paths.map((p) =>
							p.pts[hoverIdx] ? (
								<circle
									key={`dot-${p.name}`}
									cx={p.pts[hoverIdx].x}
									cy={p.pts[hoverIdx].y}
									r="3"
									fill={p.color}
									stroke="var(--color-space-900)"
									strokeWidth="1.5"
								/>
							) : null,
						)}
					</g>
				)}

				{/* X labels (every Nth so they don't overlap) */}
				{xLabels.map((label, i) => {
					const skip = Math.max(1, Math.floor(xLabels.length / 6));
					if (i % skip !== 0 && i !== xLabels.length - 1) return null;
					const x = i * (xLabels.length > 1 ? viewW / (xLabels.length - 1) : 0);
					return (
						<text
							key={`xl-${i}`}
							x={x}
							y={viewH + 16}
							fontSize="9"
							fill="var(--color-ink-500)"
							fontFamily="var(--font-mono)"
							textAnchor={i === 0 ? "start" : i === xLabels.length - 1 ? "end" : "middle"}
						>
							{label}
						</text>
					);
				})}
			</svg>

			{/* Tooltip — positioned via percentage so it tracks the SVG viewBox */}
			{hoverIdx !== null && hoveredLabel && (
				<div
					className="absolute top-2 left-1/2 -translate-x-1/2 beacon-glass-strong rounded-lg px-3 py-2 text-xs shadow-card pointer-events-none"
					style={{ minWidth: 140 }}
				>
					<p className="font-mono text-[10px] text-ink-400 mb-1">{hoveredLabel}</p>
					<ul className="space-y-1">
						{hoveredValues.map((v) => (
							<li key={v.name} className="flex items-center gap-2 text-ink-200">
								<span
									aria-hidden
									className="inline-block w-2 h-2 rounded-full"
									style={{ background: v.color }}
								/>
								<span className="flex-1">{v.name}</span>
								<span className="font-mono">{yFormatter(v.value)}</span>
							</li>
						))}
					</ul>
				</div>
			)}

			{/* Legend */}
			{series.length > 1 && (
				<div className="mt-2 flex flex-wrap items-center gap-3 text-[11px] text-ink-400">
					{series.map((s) => (
						<span key={s.name} className="inline-flex items-center gap-1.5">
							<span
								aria-hidden
								className="inline-block w-2 h-2 rounded-full"
								style={{ background: s.color }}
							/>
							{s.name}
						</span>
					))}
				</div>
			)}

			{/* Hide invisible y-max placeholder so consumers can use this var. */}
			<span data-y-max={maxY} data-y-min={minY} aria-hidden hidden />
		</div>
	);
}

// niceCeil rounds a positive number up to a "nice" Y-axis ceiling. Keeps the
// chart from showing 1.0000003 as the max — picks 1, 5, 10, 50, 100, 500, …
function niceCeil(n: number): number {
	if (n <= 1) return 1;
	const pow = Math.pow(10, Math.floor(Math.log10(n)));
	const norm = n / pow;
	if (norm <= 1) return pow;
	if (norm <= 2) return 2 * pow;
	if (norm <= 5) return 5 * pow;
	return 10 * pow;
}
