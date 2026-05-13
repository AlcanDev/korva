import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useMemo,
	useRef,
	useState,
	type KeyboardEvent,
	type ReactNode,
} from "react";
import { Search, X, CornerDownLeft } from "lucide-react";

// Phase 8.2 — Command palette (⌘K / Ctrl+K).
//
// The keyboard-first jump-to-anything UX patrons of Linear, Raycast, Notion.
// One provider; pages register items via useRegisterCommand(); a single
// modal overlay lists them with fuzzy filter + keyboard navigation.
//
// Design choices:
//   - In-memory registry keyed by command ID. Pages register on mount and
//     unregister on unmount. Stable across re-renders thanks to refs.
//   - Fuzzy matching is simple substring + prefix-boost. Works fine for the
//     few hundred items we'll have; we can swap fuse.js later if needed.
//   - Keyboard: ↑↓ navigate, Enter run, Esc close. ⌘K / Ctrl+K toggles.
//   - "Sections" group commands (Navigation, Actions, Help) for readability.

export interface CommandItem {
	id: string;
	label: string;
	hint?: string; // small grey text next to the label
	section?: string; // group header
	icon?: ReactNode;
	shortcut?: string; // displayed on the right ("g + h")
	keywords?: string[]; // extra search tokens that aren't the visible label
	run: () => void;
}

interface PaletteContextValue {
	registry: Map<string, CommandItem>;
	register: (item: CommandItem) => () => void;
	open: () => void;
	close: () => void;
	toggle: () => void;
	isOpen: boolean;
}

const Ctx = createContext<PaletteContextValue | null>(null);

export function CommandPaletteProvider({ children }: { children: ReactNode }) {
	const [isOpen, setIsOpen] = useState(false);
	// The registry lives in a ref so register/unregister doesn't trigger a
	// re-render of every consumer. We force a re-render via `tick` when the
	// modal is open so it picks up new entries.
	const registry = useRef<Map<string, CommandItem>>(new Map());
	const [, setTick] = useState(0);

	const register = useCallback((item: CommandItem) => {
		registry.current.set(item.id, item);
		setTick((n) => n + 1);
		return () => {
			registry.current.delete(item.id);
			setTick((n) => n + 1);
		};
	}, []);

	const open = useCallback(() => setIsOpen(true), []);
	const close = useCallback(() => setIsOpen(false), []);
	const toggle = useCallback(() => setIsOpen((o) => !o), []);

	// Global keybinding. Mod = Cmd on mac, Ctrl elsewhere.
	useEffect(() => {
		function handler(e: globalThis.KeyboardEvent) {
			const mod = e.metaKey || e.ctrlKey;
			if (mod && e.key.toLowerCase() === "k") {
				e.preventDefault();
				toggle();
			}
		}
		window.addEventListener("keydown", handler);
		return () => window.removeEventListener("keydown", handler);
	}, [toggle]);

	const value = useMemo(
		() => ({ registry: registry.current, register, open, close, toggle, isOpen }),
		[register, open, close, toggle, isOpen],
	);

	return (
		<Ctx.Provider value={value}>
			{children}
			{isOpen && <CommandPaletteModal onClose={close} />}
		</Ctx.Provider>
	);
}

export function useCommandPalette(): PaletteContextValue {
	const ctx = useContext(Ctx);
	if (!ctx) {
		throw new Error("useCommandPalette must be used within a CommandPaletteProvider");
	}
	return ctx;
}

// useRegisterCommand: convenient hook for pages to declare commands. The
// `deps` array works like useEffect — change it when the run callback or
// metadata changes.
export function useRegisterCommand(item: CommandItem, deps: unknown[] = []) {
	const { register } = useCommandPalette();
	useEffect(() => {
		return register(item);
		// We deliberately let the caller manage deps so they don't have to
		// memoize the whole `item` object.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, deps);
}

// ── Modal ────────────────────────────────────────────────────────────────────

function CommandPaletteModal({ onClose }: { onClose: () => void }) {
	const { registry } = useCommandPalette();
	const [query, setQuery] = useState("");
	const [active, setActive] = useState(0);
	const inputRef = useRef<HTMLInputElement>(null);

	const items = useMemo(() => {
		const all = Array.from(registry.values());
		const q = query.trim().toLowerCase();
		if (!q) return all;
		return all
			.map((it) => ({ it, score: scoreCommand(it, q) }))
			.filter((x) => x.score > 0)
			.sort((a, b) => b.score - a.score)
			.map((x) => x.it);
	}, [registry, query]);

	// Reset the active cursor when the filter narrows so the cursor doesn't
	// point off the end.
	useEffect(() => {
		setActive(0);
	}, [query]);

	useEffect(() => {
		inputRef.current?.focus();
	}, []);

	// Group by section for the rendered output, but keep the flat index for
	// keyboard navigation so ↑↓ traverses across sections naturally.
	const grouped = useMemo(() => groupBySection(items), [items]);

	function runActive() {
		const target = items[active];
		if (target) {
			target.run();
			onClose();
		}
	}

	function onKey(e: KeyboardEvent<HTMLInputElement>) {
		if (e.key === "ArrowDown") {
			e.preventDefault();
			setActive((i) => Math.min(items.length - 1, i + 1));
		} else if (e.key === "ArrowUp") {
			e.preventDefault();
			setActive((i) => Math.max(0, i - 1));
		} else if (e.key === "Enter") {
			e.preventDefault();
			runActive();
		} else if (e.key === "Escape") {
			e.preventDefault();
			onClose();
		}
	}

	return (
		<div
			className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] px-4"
			role="dialog"
			aria-modal="true"
			aria-label="Command palette"
		>
			<button
				type="button"
				aria-label="Close palette"
				className="absolute inset-0 bg-black/60 backdrop-blur-sm"
				onClick={onClose}
			/>
			<div className="relative w-full max-w-xl beacon-glass-strong rounded-xl shadow-card-hover overflow-hidden animate-scale-in">
				<div className="flex items-center gap-2 px-3.5 py-2.5 border-b border-white/8">
					<Search size={15} className="text-ink-400 shrink-0" />
					<input
						ref={inputRef}
						type="text"
						value={query}
						onChange={(e) => setQuery(e.target.value)}
						onKeyDown={onKey}
						placeholder="Type a command or search…"
						className="flex-1 bg-transparent text-sm text-ink-100 placeholder:text-ink-500 outline-none"
						aria-controls="command-palette-list"
						aria-autocomplete="list"
					/>
					<button
						type="button"
						onClick={onClose}
						aria-label="Close"
						className="text-ink-400 hover:text-ink-100 transition-colors rounded p-1 -m-1"
					>
						<X size={14} />
					</button>
				</div>

				{items.length === 0 ? (
					<div className="px-4 py-6 text-center text-xs text-ink-400">
						No matches.
					</div>
				) : (
					<ul
						id="command-palette-list"
						role="listbox"
						aria-label="Commands"
						className="max-h-[60vh] overflow-y-auto py-1"
					>
						{grouped.map((group) => (
							<li key={group.section}>
								{group.section ? (
									<div className="px-3 py-1.5 text-[10px] uppercase tracking-wider text-ink-500 font-medium">
										{group.section}
									</div>
								) : null}
								<ul role="group">
									{group.items.map((it) => {
										const idx = items.indexOf(it);
										const isActive = idx === active;
										return (
											<li key={it.id}>
												<button
													type="button"
													role="option"
													aria-selected={isActive}
													onClick={() => {
														setActive(idx);
														it.run();
														onClose();
													}}
													onMouseEnter={() => setActive(idx)}
													className={`w-full flex items-center gap-2.5 px-3 py-2 text-sm text-left transition-colors ${
														isActive
															? "bg-volt-dim text-ink-100"
															: "text-ink-200 hover:bg-white/3"
													}`}
												>
													{it.icon ? (
														<span className="text-ink-400 shrink-0" aria-hidden>
															{it.icon}
														</span>
													) : null}
													<span className="flex-1 truncate">{it.label}</span>
													{it.hint ? (
														<span className="text-xs text-ink-500 truncate">
															{it.hint}
														</span>
													) : null}
													{it.shortcut ? (
														<kbd className="text-[10px] font-mono bg-space-700 border border-white/10 rounded px-1.5 py-0.5 text-ink-400">
															{it.shortcut}
														</kbd>
													) : null}
												</button>
											</li>
										);
									})}
								</ul>
							</li>
						))}
					</ul>
				)}

				<div className="px-3.5 py-2 border-t border-white/8 flex items-center justify-between text-[10px] text-ink-500 font-mono">
					<span className="inline-flex items-center gap-1.5">
						<kbd className="bg-space-700 border border-white/10 rounded px-1 py-0.5">
							↑↓
						</kbd>
						navigate
					</span>
					<span className="inline-flex items-center gap-1.5">
						<kbd className="bg-space-700 border border-white/10 rounded px-1 py-0.5">
							<CornerDownLeft size={9} className="inline" />
						</kbd>
						run
					</span>
					<span className="inline-flex items-center gap-1.5">
						<kbd className="bg-space-700 border border-white/10 rounded px-1 py-0.5">
							esc
						</kbd>
						close
					</span>
				</div>
			</div>
		</div>
	);
}

// ── Matching ────────────────────────────────────────────────────────────────

// scoreCommand returns 0 when the query doesn't match any token, otherwise a
// positive ranking score. Prefix matches on the label win over substring;
// keyword matches contribute a smaller bonus.
function scoreCommand(it: CommandItem, q: string): number {
	const label = it.label.toLowerCase();
	let score = 0;
	if (label.startsWith(q)) score += 100;
	else if (label.includes(q)) score += 50;
	if (it.section?.toLowerCase().includes(q)) score += 5;
	if (it.hint?.toLowerCase().includes(q)) score += 3;
	for (const kw of it.keywords ?? []) {
		if (kw.toLowerCase().includes(q)) score += 10;
	}
	return score;
}

function groupBySection(
	items: CommandItem[],
): { section: string; items: CommandItem[] }[] {
	const byKey = new Map<string, CommandItem[]>();
	for (const it of items) {
		const key = it.section ?? "";
		const arr = byKey.get(key) ?? [];
		arr.push(it);
		byKey.set(key, arr);
	}
	// Preserve insertion order so the layout stays predictable.
	return Array.from(byKey.entries()).map(([section, items]) => ({
		section,
		items,
	}));
}
