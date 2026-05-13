import type { ReactNode } from "react";
import { Badge } from "./Badge";

// Phase 7 — PageHero: the consistent header used at the top of every admin
// page. Eyebrow (optional mono label), big title (gradient color allowed),
// subtitle, and an actions slot on the right (e.g. "Refresh", "Export…").
//
// Drops into Tailwind grids; the mesh background is supplied by the layout.

export interface PageHeroProps {
	eyebrow?: string;
	title: ReactNode;
	subtitle?: ReactNode;
	icon?: ReactNode;
	badge?: { tone: "success" | "info" | "warning" | "danger" | "neutral"; label: ReactNode };
	actions?: ReactNode;
	className?: string;
}

export function PageHero({
	eyebrow,
	title,
	subtitle,
	icon,
	badge,
	actions,
	className = "",
}: PageHeroProps) {
	return (
		<header
			className={`relative overflow-hidden rounded-xl sm:rounded-2xl beacon-glass px-4 sm:px-6 py-5 sm:py-7 mb-5 sm:mb-6 ${className}`}
		>
			<div className="absolute inset-0 pointer-events-none opacity-50 beacon-grid-overlay" />
			<div className="relative flex flex-col md:flex-row md:items-center md:justify-between gap-4">
				<div className="flex-1 min-w-0">
					{eyebrow ? (
						<p className="text-[11px] font-mono uppercase tracking-[0.18em] text-ink-400 mb-2">
							{eyebrow}
						</p>
					) : null}
					<div className="flex items-center gap-2 sm:gap-3 flex-wrap">
						{icon ? <span className="text-ink-300 shrink-0">{icon}</span> : null}
						<h1 className="font-display font-700 text-xl sm:text-2xl md:text-3xl text-ink-100 leading-tight tracking-tight">
							{title}
						</h1>
						{badge ? <Badge tone={badge.tone}>{badge.label}</Badge> : null}
					</div>
					{subtitle ? (
						<p className="text-xs sm:text-sm text-ink-400 mt-2 max-w-3xl leading-relaxed">
							{subtitle}
						</p>
					) : null}
				</div>
				{actions ? <div className="flex items-center gap-2 flex-wrap">{actions}</div> : null}
			</div>
		</header>
	);
}
