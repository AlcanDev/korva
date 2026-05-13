import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from "react";

// Phase 7 — Button: one component for every CTA flavor we use in Beacon.
//
// Variants:
//   primary   — coral CTA (Run export, Save, Apply, …)
//   secondary — glass outline (Cancel, Refresh, …)
//   ghost     — transparent (sidebar items, toolbar icons)
//   danger    — red (destructive — Delete, Prune apply, …)
//   volt      — green volt CTA (Save verdict, Start sync, …)
//
// Loading state shows a spinner + dims the label; disabled prevents both.

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "volt";
export type ButtonSize = "sm" | "md" | "lg" | "icon";

const baseClass =
	"inline-flex items-center justify-center gap-2 font-medium tracking-tight transition-all " +
	"focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-[#05080F] " +
	"disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:translate-y-0";

const variantClass: Record<ButtonVariant, string> = {
	primary:
		"bg-coral text-white shadow-lg shadow-coral/20 hover:bg-orange-500 hover:shadow-coral/40 " +
		"hover:-translate-y-0.5 focus-visible:ring-coral/50",
	volt:
		"bg-volt text-[#03060C] shadow-lg shadow-volt/20 hover:bg-emerald-400 hover:shadow-volt/40 " +
		"hover:-translate-y-0.5 focus-visible:ring-volt/50",
	secondary:
		"bg-space-700/60 border border-white/10 text-ink-200 hover:bg-space-600/70 hover:border-white/20 " +
		"hover:-translate-y-0.5 focus-visible:ring-white/20",
	ghost:
		"text-ink-300 hover:text-ink-100 hover:bg-white/5 focus-visible:ring-white/15",
	danger:
		"bg-[#DA3633] text-white shadow-lg shadow-[#DA3633]/20 hover:bg-[#F85149] hover:shadow-[#F85149]/40 " +
		"hover:-translate-y-0.5 focus-visible:ring-[#DA3633]/50",
};

const sizeClass: Record<ButtonSize, string> = {
	sm: "px-2.5 py-1.5 text-xs rounded-md",
	md: "px-3.5 py-2 text-sm rounded-lg",
	lg: "px-5 py-2.5 text-base rounded-lg",
	icon: "h-8 w-8 rounded-md",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
	variant?: ButtonVariant;
	size?: ButtonSize;
	loading?: boolean;
	leftIcon?: ReactNode;
	rightIcon?: ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
	{
		variant = "secondary",
		size = "md",
		loading = false,
		leftIcon,
		rightIcon,
		disabled,
		className = "",
		children,
		type = "button",
		...rest
	},
	ref,
) {
	const cls = [baseClass, variantClass[variant], sizeClass[size], className]
		.filter(Boolean)
		.join(" ");
	return (
		<button
			ref={ref}
			type={type}
			disabled={disabled || loading}
			aria-busy={loading || undefined}
			className={cls}
			{...rest}
		>
			{loading ? (
				<span
					className="inline-block h-3.5 w-3.5 rounded-full border-2 border-current border-t-transparent animate-spin"
					aria-hidden
				/>
			) : leftIcon ? (
				<span aria-hidden>{leftIcon}</span>
			) : null}
			{size !== "icon" && children}
			{!loading && rightIcon ? <span aria-hidden>{rightIcon}</span> : null}
		</button>
	);
});
