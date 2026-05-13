import type { ReactNode } from "react";

// Phase 7 — Tabs: keyboard-navigable, accessible tab strip. Drives the
// internal sectioning of dense panels (ProjectsPanel, CommandsPanel, etc.).
//
// Caller owns the active state — keeps it simple, makes it trivial to bind
// to route params later if we want shareable deep-links.

export interface TabItem<T extends string = string> {
	value: T;
	label: ReactNode;
	icon?: ReactNode;
	badge?: ReactNode;
}

export interface TabsProps<T extends string = string> {
	value: T;
	onChange: (value: T) => void;
	tabs: TabItem<T>[];
	className?: string;
	variant?: "underline" | "pill";
}

export function Tabs<T extends string = string>({
	value,
	onChange,
	tabs,
	className = "",
	variant = "underline",
}: TabsProps<T>) {
	if (variant === "pill") {
		return (
			<div className={`inline-flex p-1 rounded-lg bg-space-700/60 border border-white/5 ${className}`}>
				{tabs.map((tab) => {
					const active = tab.value === value;
					return (
						<button
							key={tab.value}
							type="button"
							onClick={() => onChange(tab.value)}
							role="tab"
							aria-selected={active}
							className={`inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
								active
									? "bg-space-500 text-ink-100 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
									: "text-ink-400 hover:text-ink-200"
							}`}
						>
							{tab.icon ? <span aria-hidden>{tab.icon}</span> : null}
							{tab.label}
							{tab.badge ? (
								<span className="ml-1 text-[10px] font-mono text-ink-500">
									{tab.badge}
								</span>
							) : null}
						</button>
					);
				})}
			</div>
		);
	}
	// Underline (default).
	return (
		<nav
			role="tablist"
			className={`flex flex-wrap border-b border-white/5 ${className}`}
		>
			{tabs.map((tab) => {
				const active = tab.value === value;
				return (
					<button
						key={tab.value}
						type="button"
						onClick={() => onChange(tab.value)}
						role="tab"
						aria-selected={active}
						className={`inline-flex items-center gap-1.5 px-4 py-2.5 text-xs font-medium border-b-2 transition-colors -mb-px ${
							active
								? "border-volt text-ink-100"
								: "border-transparent text-ink-400 hover:text-ink-200"
						}`}
					>
						{tab.icon ? <span aria-hidden>{tab.icon}</span> : null}
						{tab.label}
						{tab.badge ? (
							<span className="ml-1 text-[10px] font-mono text-ink-500">
								{tab.badge}
							</span>
						) : null}
					</button>
				);
			})}
		</nav>
	);
}
