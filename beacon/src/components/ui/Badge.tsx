import type { ReactNode } from "react";

// Phase 7 — Badge: one component for the dozens of "labelled status" tokens
// scattered across the dashboard (tier, conflict status, phase, …).

export type BadgeTone =
	| "neutral"
	| "success"
	| "warning"
	| "danger"
	| "info"
	| "cyan"
	| "volt"
	| "coral"
	| "purple";

const toneClass: Record<BadgeTone, string> = {
	neutral: "bg-space-700/60 border-white/10 text-ink-300",
	success: "bg-volt-dim border-volt/30 text-volt",
	warning: "bg-amber-400/10 border-amber-400/30 text-amber-400",
	danger: "bg-[#F85149]/10 border-[#F85149]/30 text-[#F85149]",
	info: "bg-cyan-400/10 border-cyan-400/30 text-cyan-400",
	cyan: "bg-cyan-400/10 border-cyan-400/30 text-cyan-300",
	volt: "bg-volt-dim border-volt/30 text-volt",
	coral: "bg-coral-dim border-coral/30 text-coral",
	purple: "bg-purple-400/10 border-purple-400/30 text-purple-400",
};

export interface BadgeProps {
	tone?: BadgeTone;
	children: ReactNode;
	leftIcon?: ReactNode;
	className?: string;
	mono?: boolean;
}

export function Badge({
	tone = "neutral",
	children,
	leftIcon,
	mono = false,
	className = "",
}: BadgeProps) {
	const cls = [
		"inline-flex items-center gap-1 px-2 py-0.5 rounded-full border text-[11px] font-medium",
		mono ? "font-mono tracking-wider" : "tracking-tight",
		toneClass[tone],
		className,
	].join(" ");
	return (
		<span className={cls}>
			{leftIcon ? <span aria-hidden>{leftIcon}</span> : null}
			{children}
		</span>
	);
}

// StatusDot: a tiny coloured circle, optionally pulsing. Pairs with Badge or
// stands alone in tables. Used to signal phase (running/idle/error/disabled).

export type DotState = "running" | "idle" | "warning" | "error" | "disabled" | "info";

const dotClass: Record<DotState, string> = {
	running: "bg-volt shadow-[0_0_8px_rgba(0,245,160,0.6)]",
	idle: "bg-ink-500",
	warning: "bg-amber-400 shadow-[0_0_8px_rgba(251,191,36,0.5)]",
	error: "bg-[#F85149] shadow-[0_0_8px_rgba(248,81,73,0.5)]",
	disabled: "bg-ink-600",
	info: "bg-cyan-400 shadow-[0_0_8px_rgba(34,211,238,0.5)]",
};

export function StatusDot({
	state,
	pulse = false,
	size = 8,
}: {
	state: DotState;
	pulse?: boolean;
	size?: number;
}) {
	return (
		<span
			aria-hidden
			className={`inline-block rounded-full ${dotClass[state]} ${pulse ? "animate-pulse" : ""}`}
			style={{ width: size, height: size }}
		/>
	);
}
