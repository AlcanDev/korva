// Phase 7 — Sparkline: a tiny inline line chart, no axes, no labels. Drops
// into a MetricCard, table cell, or anywhere a single-glance trend reads
// better than a number. Pure SVG — no chart library, no extra bundle.

export interface SparklineProps {
	data: number[];
	color?: string; // CSS color (use one of the design-system tokens)
	width?: number | string;
	height?: number | string;
	strokeWidth?: number;
	fillOpacity?: number;
	className?: string;
}

export function Sparkline({
	data,
	color = "var(--color-cyan-400)",
	width = "100%",
	height = "100%",
	strokeWidth = 1.5,
	fillOpacity = 0.15,
	className = "",
}: SparklineProps) {
	if (!data || data.length < 2) {
		return (
			<svg width={width} height={height} className={className} aria-hidden>
				<line x1="0" y1="50%" x2="100%" y2="50%" stroke="var(--color-glass)" strokeWidth="1" />
			</svg>
		);
	}
	const min = Math.min(...data);
	const max = Math.max(...data);
	const range = max - min || 1;
	const viewW = 100;
	const viewH = 32;
	const step = viewW / (data.length - 1);

	const points = data.map((v, i) => {
		const x = i * step;
		const y = viewH - ((v - min) / range) * viewH;
		return { x, y };
	});

	const linePath = points
		.map((p, i) => `${i === 0 ? "M" : "L"}${p.x.toFixed(2)},${p.y.toFixed(2)}`)
		.join(" ");
	const areaPath = `${linePath} L${viewW},${viewH} L0,${viewH} Z`;

	return (
		<svg
			viewBox={`0 0 ${viewW} ${viewH}`}
			preserveAspectRatio="none"
			width={width}
			height={height}
			className={className}
			aria-hidden
		>
			<path d={areaPath} fill={color} fillOpacity={fillOpacity} />
			<path d={linePath} fill="none" stroke={color} strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" />
		</svg>
	);
}
