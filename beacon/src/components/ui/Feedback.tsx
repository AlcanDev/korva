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

// EmptyState: friendly placeholder when a query returns no rows. Sits inside
// a Card or as a standalone block — accepts an optional CTA so callers can
// nudge the user toward the next action.
export function EmptyState({
	icon,
	title,
	description,
	action,
	className = "",
}: {
	icon?: ReactNode;
	title: string;
	description?: ReactNode;
	action?: ReactNode;
	className?: string;
}) {
	return (
		<div
			className={`flex flex-col items-center justify-center text-center px-6 py-10 rounded-xl border border-dashed border-white/10 bg-space-800/30 ${className}`}
		>
			{icon ? <div className="mb-3 text-ink-500">{icon}</div> : null}
			<p className="text-sm text-ink-200 font-medium">{title}</p>
			{description ? (
				<p className="text-xs text-ink-400 mt-1.5 max-w-md leading-relaxed">
					{description}
				</p>
			) : null}
			{action ? <div className="mt-4">{action}</div> : null}
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
