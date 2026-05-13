import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useRef,
	useState,
	type ReactNode,
} from "react";
import { CheckCircle2, AlertTriangle, XCircle, Info, X } from "lucide-react";

// Phase 8.1 — Toast system. Replaces inline ErrorBanner / success-card
// scaffolding scattered across mutations with a single notification stack
// in the corner. Idiomatic for modern dashboards (Linear, Vercel, Stripe)
// and dramatically reduces the visual noise of inline result strips.
//
// Contract:
//   useToast() → { push(toast), dismiss(id), toasts }
//   <ToastProvider>: mounts the stack once near the root
//   <ToastViewport>: where the toasts render (positioned fixed)
//
// Auto-dismisses after 5 s for non-error tones; errors stick until clicked
// so the operator can copy the message before it disappears.

export type ToastTone = "success" | "warning" | "error" | "info";

export interface ToastItem {
	id: string;
	tone: ToastTone;
	title: string;
	message?: string;
	duration?: number; // ms; 0 = sticky
}

interface ToastInput {
	tone?: ToastTone;
	title: string;
	message?: string;
	duration?: number;
}

interface ToastContextValue {
	toasts: ToastItem[];
	push: (toast: ToastInput) => string;
	dismiss: (id: string) => void;
	clear: () => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let toastSeq = 0;
function nextId(): string {
	toastSeq += 1;
	return `toast-${Date.now()}-${toastSeq}`;
}

export function ToastProvider({ children }: { children: ReactNode }) {
	const [toasts, setToasts] = useState<ToastItem[]>([]);
	// Track timers per-toast so we can clear them on early dismiss.
	const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

	const dismiss = useCallback((id: string) => {
		setToasts((prev) => prev.filter((t) => t.id !== id));
		const handle = timers.current.get(id);
		if (handle) {
			clearTimeout(handle);
			timers.current.delete(id);
		}
	}, []);

	const push = useCallback(
		(input: ToastInput) => {
			const id = nextId();
			const tone = input.tone ?? "info";
			// Errors stick by default (operator may want to copy the message);
			// other tones auto-dismiss after 5 s unless caller overrides.
			const duration =
				input.duration !== undefined
					? input.duration
					: tone === "error"
						? 0
						: 5000;
			const item: ToastItem = {
				id,
				tone,
				title: input.title,
				message: input.message,
				duration,
			};
			setToasts((prev) => [...prev, item]);
			if (duration > 0) {
				const handle = setTimeout(() => dismiss(id), duration);
				timers.current.set(id, handle);
			}
			return id;
		},
		[dismiss],
	);

	const clear = useCallback(() => {
		// Snapshot the timer map and clear each so we don't fire stale
		// dismissals after a hard reset.
		for (const h of timers.current.values()) clearTimeout(h);
		timers.current.clear();
		setToasts([]);
	}, []);

	// Clean every pending timer when the provider unmounts (route changes
	// during dev, hot reload, etc.) to avoid leaks.
	useEffect(() => {
		const map = timers.current;
		return () => {
			for (const h of map.values()) clearTimeout(h);
			map.clear();
		};
	}, []);

	const value = useMemo(
		() => ({ toasts, push, dismiss, clear }),
		[toasts, push, dismiss, clear],
	);

	return (
		<ToastContext.Provider value={value}>
			{children}
			<ToastViewport toasts={toasts} onDismiss={dismiss} />
		</ToastContext.Provider>
	);
}

export function useToast(): ToastContextValue {
	const ctx = useContext(ToastContext);
	if (!ctx) {
		throw new Error("useToast must be used within a ToastProvider");
	}
	return ctx;
}

// useToastOptional: returns null instead of throwing when outside the
// provider. Useful in unit-test scaffolding that may not mount providers.
export function useToastOptional(): ToastContextValue | null {
	return useContext(ToastContext);
}

const toneStyles: Record<
	ToastTone,
	{ border: string; bg: string; icon: ReactNode; iconClass: string }
> = {
	success: {
		border: "border-volt/30",
		bg: "bg-volt-dim",
		icon: <CheckCircle2 size={16} />,
		iconClass: "text-volt",
	},
	warning: {
		border: "border-amber-400/30",
		bg: "bg-amber-400/10",
		icon: <AlertTriangle size={16} />,
		iconClass: "text-amber-400",
	},
	error: {
		border: "border-[#F85149]/30",
		bg: "bg-[#F85149]/10",
		icon: <XCircle size={16} />,
		iconClass: "text-[#F85149]",
	},
	info: {
		border: "border-cyan-400/30",
		bg: "bg-cyan-400/10",
		icon: <Info size={16} />,
		iconClass: "text-cyan-400",
	},
};

function ToastViewport({
	toasts,
	onDismiss,
}: {
	toasts: ToastItem[];
	onDismiss: (id: string) => void;
}) {
	return (
		<div
			role="region"
			aria-live="polite"
			aria-label="Notifications"
			className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-[calc(100vw-2rem)] sm:w-96 pointer-events-none"
		>
			{toasts.map((t) => {
				const s = toneStyles[t.tone];
				return (
					<div
						key={t.id}
						role="status"
						className={`pointer-events-auto rounded-lg border ${s.border} ${s.bg} beacon-glass-strong backdrop-blur-xl shadow-card-hover px-3.5 py-3 flex items-start gap-3 animate-slide-up`}
					>
						<span className={`shrink-0 mt-0.5 ${s.iconClass}`} aria-hidden>
							{s.icon}
						</span>
						<div className="flex-1 min-w-0">
							<p className="text-sm font-medium text-ink-100 leading-snug">
								{t.title}
							</p>
							{t.message ? (
								<p className="text-xs text-ink-300 mt-1 leading-relaxed break-words">
									{t.message}
								</p>
							) : null}
						</div>
						<button
							type="button"
							aria-label="Dismiss"
							onClick={() => onDismiss(t.id)}
							className="shrink-0 text-ink-400 hover:text-ink-100 transition-colors rounded p-1 -m-1"
						>
							<X size={13} />
						</button>
					</div>
				);
			})}
		</div>
	);
}
