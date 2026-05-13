import type { HTMLAttributes, ReactNode } from "react";

// Phase 7 — Card: the foundation of every panel in the new design.
//
// Variants:
//   - "default": subtle border on a flat surface (best for dense data tables)
//   - "glass":   blurred translucent panel (best for hero sections / metrics)
//   - "elevated": stronger shadow + brighter border (best for modal-ish content)
//
// Tone shifts the border accent toward a brand color so panels of different
// purpose visually cluster without needing a header.

export type CardVariant = "default" | "glass" | "elevated";
export type CardTone = "neutral" | "volt" | "cyan" | "coral" | "purple" | "amber";

const baseByVariant: Record<CardVariant, string> = {
	default: "bg-[#0D1117] border border-[#1F2937]",
	glass: "beacon-glass",
	elevated: "bg-[#111827] border border-white/10 shadow-card-hover",
};

const toneAccent: Record<CardTone, string> = {
	neutral: "",
	volt: "hover:border-volt/40 hover:shadow-glow-volt",
	cyan: "hover:border-cyan-400/40 hover:shadow-glow-cyan",
	coral: "hover:border-coral/40 hover:shadow-glow-coral",
	purple: "hover:border-purple-400/40",
	amber: "hover:border-amber-400/40",
};

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
	variant?: CardVariant;
	tone?: CardTone;
	interactive?: boolean;
	children?: ReactNode;
}

export function Card({
	variant = "default",
	tone = "neutral",
	interactive = false,
	className = "",
	children,
	...rest
}: CardProps) {
	const cls = [
		baseByVariant[variant],
		"rounded-xl transition-all duration-200",
		interactive ? "cursor-pointer hover:-translate-y-0.5" : "",
		tone !== "neutral" ? toneAccent[tone] : "",
		className,
	]
		.filter(Boolean)
		.join(" ");
	return (
		<div className={cls} {...rest}>
			{children}
		</div>
	);
}

export function CardHeader({
	title,
	subtitle,
	icon,
	actions,
}: {
	title: ReactNode;
	subtitle?: ReactNode;
	icon?: ReactNode;
	actions?: ReactNode;
}) {
	return (
		<header className="flex items-start justify-between gap-3 p-4 border-b border-white/5">
			<div className="flex items-start gap-2.5">
				{icon ? <span className="mt-0.5 text-ink-400">{icon}</span> : null}
				<div>
					<h3 className="text-sm font-semibold text-ink-100 leading-tight">{title}</h3>
					{subtitle ? (
						<p className="text-xs text-ink-400 mt-0.5 leading-snug">{subtitle}</p>
					) : null}
				</div>
			</div>
			{actions ? <div className="flex items-center gap-1.5">{actions}</div> : null}
		</header>
	);
}

export function CardBody({
	children,
	className = "",
}: {
	children: ReactNode;
	className?: string;
}) {
	return <div className={`p-4 ${className}`}>{children}</div>;
}

export function CardFooter({
	children,
	className = "",
}: {
	children: ReactNode;
	className?: string;
}) {
	return (
		<footer className={`px-4 py-3 border-t border-white/5 ${className}`}>{children}</footer>
	);
}
