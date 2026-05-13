import type { ReactNode } from "react";
import { ArrowDownRight, ArrowUpRight } from "lucide-react";
import { Card, type CardTone } from "./Card";

// Phase 7 — MetricCard: the hero metric block used across the dashboard.
//
// Shows a label, a big value, an optional trend (delta + direction), and an
// optional sparkline. Tone shifts the accent color of the value + sparkline.
// Designed to fit a 3 or 4-column grid; mobile collapses naturally.

export interface MetricCardProps {
	label: string;
	value: ReactNode;
	hint?: ReactNode;
	icon?: ReactNode;
	tone?: CardTone;
	trend?: {
		value: number; // absolute change, formatted by caller as %
		direction: "up" | "down" | "flat";
		label?: string; // "vs last 7 days"
	};
	sparkline?: ReactNode; // chart component (see Charts)
	className?: string;
}

const toneText: Record<CardTone, string> = {
	neutral: "text-ink-100",
	volt: "text-volt",
	cyan: "text-cyan-300",
	coral: "text-coral",
	purple: "text-purple-400",
	amber: "text-amber-400",
};

type TrendDirection = "up" | "down" | "flat";

const trendClass: Record<TrendDirection, string> = {
	up: "text-volt",
	down: "text-[#F85149]",
	flat: "text-ink-400",
};

export function MetricCard({
	label,
	value,
	hint,
	icon,
	tone = "neutral",
	trend,
	sparkline,
	className = "",
}: MetricCardProps) {
	return (
		<Card variant="glass" tone={tone} className={`p-4 ${className}`}>
			<div className="flex items-start justify-between gap-2 mb-3">
				<span className="text-[11px] uppercase tracking-wider text-ink-400 font-medium">
					{label}
				</span>
				{icon ? <span className="text-ink-400 opacity-80">{icon}</span> : null}
			</div>
			<div className={`text-3xl font-display font-700 leading-none ${toneText[tone]}`}>
				{value}
			</div>
			{(trend || hint || sparkline) && (
				<div className="mt-3 flex items-end justify-between gap-3">
					<div className="flex flex-col gap-1">
						{trend ? (
							<span
								className={`inline-flex items-center gap-1 text-[11px] font-mono ${trendClass[trend.direction]}`}
							>
								{trend.direction === "up" ? (
									<ArrowUpRight size={11} />
								) : trend.direction === "down" ? (
									<ArrowDownRight size={11} />
								) : (
									<span className="inline-block h-px w-2 bg-current" />
								)}
								{Math.abs(trend.value).toFixed(1)}%
								{trend.label ? (
									<span className="text-ink-500 ml-1">{trend.label}</span>
								) : null}
							</span>
						) : null}
						{hint ? <span className="text-[11px] text-ink-400">{hint}</span> : null}
					</div>
					{sparkline ? <div className="h-8 w-20 shrink-0">{sparkline}</div> : null}
				</div>
			)}
		</Card>
	);
}
