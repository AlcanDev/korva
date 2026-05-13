// Phase 7 — BarChart: horizontal-bar comparison. Used for "top N projects",
// "errors by reason code", "observations per author". Horizontal bars are
// chosen on purpose — labels read left-to-right, no rotated text, scales
// gracefully to dozens of categories without breaking the layout.

export interface BarChartItem {
	label: string;
	value: number;
	color?: string;
}

export interface BarChartProps {
	data: BarChartItem[];
	formatValue?: (n: number) => string;
	maxRows?: number;
	emptyMessage?: string;
	className?: string;
}

export function BarChart({
	data,
	formatValue = (n) => n.toLocaleString(),
	maxRows = 10,
	emptyMessage = "No data",
	className = "",
}: BarChartProps) {
	if (data.length === 0) {
		return (
			<p className={`text-xs text-ink-400 italic ${className}`}>{emptyMessage}</p>
		);
	}
	const sorted = [...data].sort((a, b) => b.value - a.value).slice(0, maxRows);
	const max = Math.max(...sorted.map((d) => d.value), 1);
	return (
		<ul className={`space-y-1.5 ${className}`}>
			{sorted.map((row) => {
				const pct = (row.value / max) * 100;
				return (
					<li key={row.label}>
						<div className="flex items-center justify-between text-[11px] mb-1">
							<span className="text-ink-200 truncate max-w-[60%]" title={row.label}>
								{row.label}
							</span>
							<span className="font-mono text-ink-300">{formatValue(row.value)}</span>
						</div>
						<div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
							<div
								className="h-full rounded-full transition-all duration-500"
								style={{
									width: `${pct}%`,
									background:
										row.color ??
										"linear-gradient(90deg, var(--color-cyan-400), var(--color-indigo-400))",
								}}
							/>
						</div>
					</li>
				);
			})}
		</ul>
	);
}
