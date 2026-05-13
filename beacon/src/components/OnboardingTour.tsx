import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
	Zap,
	ShieldCheck,
	DollarSign,
	Network,
	Terminal,
	Sparkles,
	Search,
	ArrowRight,
	ArrowLeft,
	X,
} from "lucide-react";
import { useNavigate } from "react-router";
import { Button } from "@/components/ui";

// Phase 9.4 — Onboarding tour.
//
// First-time visitors see a guided sequence that calls out:
//   - ⌘K command palette (productivity multiplier)
//   - Live activity, Privacy meter, Cost & ROI (differentiators)
//   - Knowledge graph + Commands runner (visceral wins)
//   - A "you're ready" CTA pointing at vault_save
//
// Persistence: localStorage key `korva.tour.completed`. Auto-arms on first
// admin login. Also reachable via the ⌘K palette so curious operators can
// re-watch it (registered in GlobalCommands).
//
// Implementation choice: a centred modal instead of spotlight-style
// callouts pinned to DOM targets. Spotlight tours need refs into every
// page; the centred modal works regardless of which route is mounted,
// is fully keyboard-navigable, and runs in jsdom (so we can unit-test
// the progression).

const STORAGE_KEY = "korva.tour.completed";

export function hasCompletedTour(): boolean {
	if (typeof window === "undefined") return true;
	return window.localStorage.getItem(STORAGE_KEY) === "1";
}

export function markTourCompleted(): void {
	if (typeof window === "undefined") return;
	window.localStorage.setItem(STORAGE_KEY, "1");
}

export function resetTour(): void {
	if (typeof window === "undefined") return;
	window.localStorage.removeItem(STORAGE_KEY);
}

interface TourStep {
	icon: ReactNode;
	tone: "volt" | "cyan" | "purple" | "coral" | "amber";
	title: string;
	body: ReactNode;
	cta?: { label: string; to?: string };
}

const STEPS: TourStep[] = [
	{
		icon: <Sparkles size={28} />,
		tone: "volt",
		title: "Welcome to Korva Beacon",
		body: (
			<>
				The local-first command centre for your AI engineering team. Persistent memory,
				architecture guardrails, and live observability — without anything leaving
				your machine.
			</>
		),
	},
	{
		icon: <Search size={28} />,
		tone: "cyan",
		title: "⌘K opens everything",
		body: (
			<>
				Press <kbd className="font-mono text-[11px] bg-space-700 border border-white/10 rounded px-1.5 py-0.5">⌘K</kbd> or{" "}
				<kbd className="font-mono text-[11px] bg-space-700 border border-white/10 rounded px-1.5 py-0.5">Ctrl K</kbd> from
				anywhere to navigate, run a command, or jump to a section. Fuzzy match across
				every action.
			</>
		),
	},
	{
		icon: <Zap size={28} />,
		tone: "volt",
		title: "Live activity, in real time",
		body: (
			<>
				The Observatory's <strong>Live</strong> tab streams every observation, command,
				and conflict the moment it happens via Server-Sent Events. Open it on a side
				monitor during a code review.
			</>
		),
		cta: { label: "Open Live", to: "/admin/observatory/live" },
	},
	{
		icon: <ShieldCheck size={28} />,
		tone: "cyan",
		title: "Privacy you can see",
		body: (
			<>
				The <strong>Privacy</strong> meter counts every password, token, and api_key
				the filter has redacted process-wide. Local-first isn't a promise — it's a
				number on the dashboard.
			</>
		),
		cta: { label: "Open Privacy", to: "/admin/observatory/privacy" },
	},
	{
		icon: <DollarSign size={28} />,
		tone: "coral",
		title: "Cost & ROI, no spreadsheet",
		body: (
			<>
				The <strong>Cost</strong> tab translates tokens into USD per model + project,
				shows the daily curve, and flags any day whose usage exceeded the historical
				baseline by 2 σ — so finance never sees the bill before you do.
			</>
		),
		cta: { label: "Open Cost", to: "/admin/observatory/cost" },
	},
	{
		icon: <Network size={28} />,
		tone: "purple",
		title: "Your knowledge as a graph",
		body: (
			<>
				The <strong>Graph</strong> tab renders every observation as a node and every
				relation (supersedes, conflicts, related, scoped, compatible) as an edge.
				Click around to see how decisions cluster.
			</>
		),
		cta: { label: "Open Graph", to: "/admin/observatory/graph" },
	},
	{
		icon: <Terminal size={28} />,
		tone: "amber",
		title: "Run korva from the browser",
		body: (
			<>
				The <strong>Commands</strong> tab fires curated <code>korva</code> CLI commands
				(status, doctor, projects list, …) with one click and shows the output in a
				styled terminal. No more "switch to tmux" for a quick health check.
			</>
		),
		cta: { label: "Open Commands", to: "/admin/observatory/commands" },
	},
	{
		icon: <Sparkles size={28} />,
		tone: "volt",
		title: "You're ready",
		body: (
			<>
				Save your first observation by asking any MCP-connected editor to call{" "}
				<code className="font-mono bg-space-700/60 border border-white/10 rounded px-1.5 py-0.5">
					vault_save
				</code>
				. It'll show up under <strong>Live</strong>, in the dashboard counters, and
				become a node in the graph — all without leaving your machine.
			</>
		),
	},
];

const TONE_RING: Record<TourStep["tone"], string> = {
	volt: "ring-volt/30 bg-volt/10 text-volt",
	cyan: "ring-cyan-400/30 bg-cyan-400/10 text-cyan-400",
	purple: "ring-purple-400/30 bg-purple-400/10 text-purple-400",
	coral: "ring-coral/30 bg-coral/10 text-coral",
	amber: "ring-amber-400/30 bg-amber-400/10 text-amber-400",
};

export interface OnboardingTourProps {
	open: boolean;
	onClose: (completed: boolean) => void;
}

export function OnboardingTour({ open, onClose }: OnboardingTourProps) {
	const [step, setStep] = useState(0);
	const navigate = useNavigate();
	const total = STEPS.length;
	const current = STEPS[step];

	// Reset to step 0 on open.
	useEffect(() => {
		if (open) setStep(0);
	}, [open]);

	function close(completed: boolean) {
		if (completed) markTourCompleted();
		onClose(completed);
	}

	function next() {
		if (step + 1 >= total) {
			close(true);
			return;
		}
		setStep((s) => s + 1);
	}

	function prev() {
		setStep((s) => Math.max(0, s - 1));
	}

	function runCta() {
		if (!current.cta?.to) return;
		navigate(current.cta.to);
		close(true);
	}

	if (!open) return null;

	return (
		<div
			role="dialog"
			aria-modal="true"
			aria-label="Welcome tour"
			className="fixed inset-0 z-50 flex items-center justify-center px-4"
		>
			<button
				type="button"
				aria-label="Close tour"
				className="absolute inset-0 bg-black/70 backdrop-blur-sm"
				onClick={() => close(false)}
			/>
			<div className="relative w-full max-w-lg beacon-glass-strong rounded-2xl shadow-card-hover overflow-hidden animate-scale-in">
				<button
					type="button"
					onClick={() => close(false)}
					aria-label="Close"
					className="absolute top-3 right-3 text-ink-400 hover:text-ink-100 transition-colors rounded p-1"
				>
					<X size={16} />
				</button>

				<div className="px-8 pt-8 pb-2 text-center">
					<div
						className={`mx-auto w-16 h-16 rounded-2xl ring-1 flex items-center justify-center mb-5 ${TONE_RING[current.tone]}`}
					>
						{current.icon}
					</div>
					<h2 className="font-display font-700 text-xl text-ink-100 mb-3">
						{current.title}
					</h2>
					<p className="text-sm text-ink-300 leading-relaxed">{current.body}</p>
				</div>

				{/* Progress dots */}
				<div className="flex items-center justify-center gap-1.5 py-4">
					{STEPS.map((_, i) => (
						<span
							key={i}
							aria-hidden
							className={`h-1.5 rounded-full transition-all ${
								i === step ? "w-6 bg-ink-200" : "w-1.5 bg-white/15"
							}`}
						/>
					))}
				</div>

				<div className="px-6 pb-6 flex items-center justify-between gap-2">
					<Button
						variant="ghost"
						size="sm"
						leftIcon={<ArrowLeft size={12} />}
						onClick={prev}
						disabled={step === 0}
					>
						Back
					</Button>

					<div className="flex items-center gap-2">
						{current.cta?.to ? (
							<Button variant="secondary" size="sm" onClick={runCta}>
								{current.cta.label}
							</Button>
						) : null}
						{step + 1 < total ? (
							<Button variant="volt" size="sm" rightIcon={<ArrowRight size={12} />} onClick={next}>
								Next
							</Button>
						) : (
							<Button variant="volt" size="sm" onClick={next}>
								Got it
							</Button>
						)}
					</div>
				</div>

				<p className="text-[10px] text-center text-ink-500 pb-3 px-4">
					You can replay this any time with{" "}
					<kbd className="font-mono bg-space-700 border border-white/10 rounded px-1">
						⌘K
					</kbd>{" "}
					→ "Open welcome tour".
				</p>
			</div>
		</div>
	);
}

// useAutoTour decides whether to auto-open the tour on mount based on
// localStorage state. Caller renders <OnboardingTour open={open} … />
// using the returned `open` flag.
export function useAutoTour(): {
	open: boolean;
	openTour: () => void;
	closeTour: (completed: boolean) => void;
} {
	const [open, setOpen] = useState(false);
	const completed = useMemo(() => hasCompletedTour(), []);

	useEffect(() => {
		if (!completed) setOpen(true);
	}, [completed]);

	return {
		open,
		openTour: () => setOpen(true),
		closeTour: () => setOpen(false),
	};
}
