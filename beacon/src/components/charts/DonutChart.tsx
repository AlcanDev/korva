// Phase 7 — DonutChart: distribution over categories. Used for "observations
// by type", "sessions by agent", etc. Centre carries a big total and an
// optional caption; segments are clickable when the caller passes onSelect.

export interface DonutSegment {
	label: string;
	value: number;
	color: string;
}

export interface DonutChartProps {
	data: DonutSegment[];
	size?: number;
	stroke?: number;
	centerLabel?: string;
	centerValue?: string | number;
	onSelect?: (segment: DonutSegment) => void;
	className?: string;
}

export function DonutChart({
	data,
	size = 160,
	stroke = 22,
	centerLabel,
	centerValue,
	onSelect,
	className = "",
}: DonutChartProps) {
	const total = data.reduce((acc, d) => acc + d.value, 0) || 1;
	const radius = (size - stroke) / 2;
	const circumference = 2 * Math.PI * radius;
	let offset = 0;

	const totalForCenter = centerValue ?? data.reduce((acc, d) => acc + d.value, 0);

	return (
		<div className={`flex items-center gap-4 ${className}`}>
			<div className="relative shrink-0" style={{ width: size, height: size }}>
				<svg
					viewBox={`0 0 ${size} ${size}`}
					width={size}
					height={size}
					role="img"
					aria-label="Donut chart"
					className="-rotate-90"
				>
					<circle
						cx={size / 2}
						cy={size / 2}
						r={radius}
						fill="none"
						stroke="rgba(255,255,255,0.06)"
						strokeWidth={stroke}
					/>
					{data.map((d) => {
						const dash = (d.value / total) * circumference;
						const node = (
							<circle
								key={d.label}
								cx={size / 2}
								cy={size / 2}
								r={radius}
								fill="none"
								stroke={d.color}
								strokeWidth={stroke}
								strokeDasharray={`${dash} ${circumference - dash}`}
								strokeDashoffset={-offset}
								strokeLinecap="butt"
								style={{
									cursor: onSelect ? "pointer" : "default",
									transition: "stroke-width 0.2s",
								}}
								onClick={onSelect ? () => onSelect(d) : undefined}
							>
								<title>{`${d.label}: ${d.value} (${((d.value / total) * 100).toFixed(1)}%)`}</title>
							</circle>
						);
						offset += dash;
						return node;
					})}
				</svg>
				<div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
					<span className="font-display font-700 text-2xl text-ink-100 leading-none">
						{totalForCenter}
					</span>
					{centerLabel ? (
						<span className="text-[10px] uppercase tracking-wider text-ink-400 mt-1">
							{centerLabel}
						</span>
					) : null}
				</div>
			</div>

			{/* Legend */}
			<ul className="flex-1 space-y-1.5">
				{data.map((d) => {
					const pct = (d.value / total) * 100;
					return (
						<li key={d.label} className="flex items-center gap-2 text-xs">
							<span
								aria-hidden
								className="inline-block w-2.5 h-2.5 rounded-sm shrink-0"
								style={{ background: d.color }}
							/>
							<span className="text-ink-200 flex-1 truncate" title={d.label}>
								{d.label}
							</span>
							<span className="font-mono text-ink-300">{d.value}</span>
							<span className="font-mono text-ink-500 w-10 text-right">
								{pct.toFixed(0)}%
							</span>
						</li>
					);
				})}
			</ul>
		</div>
	);
}
