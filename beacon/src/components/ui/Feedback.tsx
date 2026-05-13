import type { ReactNode } from "react";

// Phase 7 — Feedback primitives: small but high-traffic UI bits.

// Spinner: tiny inline loader. Used by buttons, async tables, and the
// terminal output panel while a command is running.
export function Spinner({
	size = 14,
	className = "",
}: {
	size?: number;
	className?: string;
}) {
	return (
		<span
			role="status"
			aria-label="Loading"
			className={`inline-block rounded-full border-2 border-current border-t-transparent animate-spin ${className}`}
			style={{ width: size, height: size }}
		/>
	);
}

// Skeleton: shimmering placeholder rectangle. Lighter than a real spinner for
// layout-stable loading states (table rows, metric cards).
export function Skeleton({
	width = "100%",
	height = 16,
	className = "",
	rounded = "rounded-md",
}: {
	width?: number | string;
	height?: number | string;
	className?: string;
	rounded?: string;
}) {
	return (
		<span
			aria-hidden
			className={`block ${rounded} bg-gradient-to-r from-white/5 via-white/10 to-white/5 bg-[length:200%_100%] animate-shimmer ${className}`}
			style={{
				width: typeof width === "number" ? `${width}px` : width,
				height: typeof height === "number" ? `${height}px` : height,
			}}
		/>
	);
}

// EmptyState: friendly placeholder when a query returns no rows. Phase 8.3
// adds an optional `tone` prop that paints the icon + a soft glow behind it,
// `hints` so we can show "tip" copy below the CTA, and a `compact` variant
// that shrinks the padding for use inside dense Cards.
//
// The visual is unmistakably Korva: a dotted-border surface plus a tone-
// coloured halo behind the icon. Beats the generic "(none)" we used to ship.

export type EmptyStateTone = "neutral" | "volt" | "cyan" | "coral" | "purple" | "amber";

const emptyToneRing: Record<EmptyStateTone, string> = {
	neutral: "before:bg-white/5",
	volt: "before:bg-volt/10",
	cyan: "before:bg-cyan-400/10",
	coral: "before:bg-coral/10",
	purple: "before:bg-purple-400/10",
	amber: "before:bg-amber-400/10",
};

const emptyToneIcon: Record<EmptyStateTone, string> = {
	neutral: "text-ink-500",
	volt: "text-volt",
	cyan: "text-cyan-400",
	coral: "text-coral",
	purple: "text-purple-400",
	amber: "text-amber-400",
};

export function EmptyState({
	icon,
	title,
	description,
	action,
	hint,
	tone = "neutral",
	compact = false,
	className = "",
}: {
	icon?: ReactNode;
	title: string;
	description?: ReactNode;
	action?: ReactNode;
	hint?: ReactNode;
	tone?: EmptyStateTone;
	compact?: boolean;
	className?: string;
}) {
	const padding = compact ? "px-4 py-6" : "px-6 py-10";
	return (
		<div
			className={`flex flex-col items-center justify-center text-center rounded-xl border border-dashed border-white/10 bg-space-800/30 ${padding} ${className}`}
		>
			{icon ? (
				<div
					className={`relative mb-3 flex items-center justify-center w-12 h-12 before:absolute before:inset-0 before:rounded-full before:blur-xl ${emptyToneRing[tone]}`}
				>
					<span className={`relative ${emptyToneIcon[tone]}`} aria-hidden>
						{icon}
					</span>
				</div>
			) : null}
			<p className="text-sm text-ink-100 font-medium">{title}</p>
			{description ? (
				<p className="text-xs text-ink-400 mt-1.5 max-w-md leading-relaxed">
					{description}
				</p>
			) : null}
			{action ? <div className="mt-4">{action}</div> : null}
			{hint ? (
				<p className="mt-3 text-[10px] font-mono text-ink-500 uppercase tracking-wider">
					{hint}
				</p>
			) : null}
		</div>
	);
}

// ErrorBanner: red rounded notice for inline errors (mutation failures,
// network outages). Less aggressive than a modal, more visible than a toast.
export function ErrorBanner({
	title,
	message,
	className = "",
}: {
	title?: string;
	message: ReactNode;
	className?: string;
}) {
	return (
		<div
			role="alert"
			className={`rounded-lg border border-[#F85149]/30 bg-[#F85149]/8 px-4 py-3 text-xs ${className}`}
		>
			{title ? (
				<p className="font-semibold text-[#F85149] mb-1">{title}</p>
			) : null}
			<p className="text-ink-200 leading-relaxed">{message}</p>
		</div>
	);
}
