import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Film, Play, BookOpen, MessageSquare, ArrowLeft, ArrowRight, Pause } from "lucide-react";
import { adminFetch } from "@/api/_fetch";
import {
	Badge,
	Button,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	ErrorBanner,
	PageHero,
	Skeleton,
} from "@/components/ui";

// Phase 10.3 — Session replay panel.
//
// Lets an operator pick a session and step through what happened
// inside it: every observation, every interaction, ordered chrono-
// logically. Auto-play toggles a 1.5s timer that advances the cursor.

interface ReplayEntry {
	kind: "session_start" | "session_end" | "observation" | "interaction";
	at: string;
	title: string;
	project?: string;
	author?: string;
	obs_type?: string;
	model?: string;
	tokens?: number;
	duration_ms?: number;
	id?: string;
	excerpt_in?: string;
	excerpt_out?: string;
	body?: string;
}

interface ReplayResponse {
	session_id: string;
	project?: string;
	agent?: string;
	goal?: string;
	started_at: string;
	ended_at?: string;
	entries: ReplayEntry[];
	total: number;
}

interface SessionRow {
	id: string;
	project: string;
	agent?: string;
	goal?: string;
	started_at: string;
	ended_at?: string | null;
	obs_count?: number;
}

function useRecentSessions() {
	return useQuery({
		queryKey: ["replay", "sessions"],
		queryFn: () =>
			adminFetch<{ sessions: SessionRow[] }>("/admin/sessions?limit=30"),
	});
}

function useReplay(id: string | null) {
	return useQuery({
		queryKey: ["replay", id],
		queryFn: () =>
			adminFetch<ReplayResponse>(`/admin/sessions/${encodeURIComponent(id ?? "")}/replay`),
		enabled: Boolean(id),
	});
}

const KIND_META: Record<ReplayEntry["kind"], { label: string; tone: "success" | "info" | "purple" | "warning"; icon: React.ReactNode }> = {
	session_start: { label: "Session start", tone: "success", icon: <Play size={13} /> },
	session_end:   { label: "Session end",   tone: "warning", icon: <Pause size={13} /> },
	observation:   { label: "Observation",   tone: "info",    icon: <BookOpen size={13} /> },
	interaction:   { label: "Interaction",   tone: "purple",  icon: <MessageSquare size={13} /> },
};

export default function ReplayPanel() {
	const sessions = useRecentSessions();
	const [selected, setSelected] = useState<string | null>(null);
	const replay = useReplay(selected);
	const [cursor, setCursor] = useState(0);
	const [playing, setPlaying] = useState(false);

	const entries = replay.data?.entries ?? [];
	const active = entries[cursor];

	// Auto-select first session.
	const firstId = sessions.data?.sessions?.[0]?.id;
	if (!selected && firstId) {
		setSelected(firstId);
	}

	// Auto-play timer.
	useMemo(() => {
		if (!playing) return;
		const t = setTimeout(() => {
			if (cursor + 1 >= entries.length) {
				setPlaying(false);
				return;
			}
			setCursor((c) => c + 1);
		}, 1500);
		return () => clearTimeout(t);
	}, [playing, cursor, entries.length]);

	return (
		<div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Audit trail"
				icon={<Film size={22} />}
				title="Session replay"
				subtitle="Step through every observation and interaction inside a session, in chronological order. Useful for audits, post-mortems, and onboarding new team members."
			/>

			{replay.error ? <ErrorBanner title="Couldn't load replay" message={String(replay.error)} /> : null}

			<div className="grid grid-cols-1 lg:grid-cols-[280px_1fr] gap-4">
				{/* Session picker */}
				<Card>
					<CardHeader title="Sessions" subtitle="Most recent first" />
					<CardBody className="!p-2">
						{sessions.isLoading ? (
							<Skeleton height={200} />
						) : (sessions.data?.sessions ?? []).length === 0 ? (
							<EmptyState
								tone="cyan"
								icon={<Film size={20} />}
								title="No sessions yet"
								description="An MCP client starts a session when it talks to the vault."
								compact
							/>
						) : (
							<ul className="space-y-1 max-h-[520px] overflow-y-auto">
								{sessions.data!.sessions.map((s) => (
									<li key={s.id}>
										<button
											type="button"
											onClick={() => {
												setSelected(s.id);
												setCursor(0);
												setPlaying(false);
											}}
											aria-current={s.id === selected}
											className={`w-full text-left rounded-md px-3 py-2 transition-colors ${
												s.id === selected
													? "bg-volt-dim border border-volt/30"
													: "border border-transparent hover:bg-white/3 hover:border-white/10"
											}`}
										>
											<p className="text-xs font-mono text-ink-100 truncate">{s.project || "—"}</p>
											{s.goal && <p className="text-[10px] text-ink-400 truncate mt-0.5">{s.goal}</p>}
											<p className="text-[10px] text-ink-500 mt-0.5">
												{new Date(s.started_at).toLocaleString()}
											</p>
										</button>
									</li>
								))}
							</ul>
						)}
					</CardBody>
				</Card>

				{/* Replay viewer */}
				<Card>
					<CardHeader
						title={replay.data ? `${replay.data.project ?? "?"} · ${replay.data.agent ?? "?"}` : "Pick a session"}
						subtitle={replay.data?.goal}
						actions={
							replay.data ? (
								<>
									<Button
										size="sm"
										variant="ghost"
										leftIcon={<ArrowLeft size={12} />}
										onClick={() => setCursor((c) => Math.max(0, c - 1))}
										disabled={cursor === 0}
										aria-label="Previous entry"
									>
										Prev
									</Button>
									<Button
										size="sm"
										variant={playing ? "secondary" : "volt"}
										leftIcon={playing ? <Pause size={12} /> : <Play size={12} />}
										onClick={() => setPlaying((p) => !p)}
									>
										{playing ? "Pause" : "Play"}
									</Button>
									<Button
										size="sm"
										variant="ghost"
										rightIcon={<ArrowRight size={12} />}
										onClick={() => setCursor((c) => Math.min(entries.length - 1, c + 1))}
										disabled={cursor >= entries.length - 1}
										aria-label="Next entry"
									>
										Next
									</Button>
								</>
							) : null
						}
					/>
					<CardBody>
						{replay.isLoading ? (
							<Skeleton height={300} />
						) : !replay.data || entries.length === 0 ? (
							<EmptyState
								tone="cyan"
								icon={<Film size={22} />}
								title="No replay data"
								description="Select a session that has observations or interactions logged."
							/>
						) : (
							<>
								{/* Progress bar */}
								<div className="mb-4">
									<div className="flex items-center justify-between text-[10px] font-mono text-ink-400 mb-1">
										<span>
											step {cursor + 1} / {entries.length}
										</span>
										{active && (
											<span>{new Date(active.at).toLocaleString()}</span>
										)}
									</div>
									<div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
										<div
											className="h-full bg-volt rounded-full transition-all"
											style={{ width: `${((cursor + 1) / entries.length) * 100}%` }}
										/>
									</div>
								</div>

								{/* Active entry detail */}
								{active && <EntryDetail entry={active} />}

								{/* All entries timeline */}
								<div className="mt-6">
									<h3 className="text-[10px] uppercase tracking-wider text-ink-400 mb-2">
										Timeline
									</h3>
									<ul className="divide-y divide-white/5">
										{entries.map((e, i) => {
											const meta = KIND_META[e.kind];
											return (
												<li
													key={`${e.at}-${i}`}
													className={`flex items-start gap-3 px-2 py-2 transition-colors cursor-pointer ${
														i === cursor ? "bg-volt-dim" : "hover:bg-white/3"
													}`}
													onClick={() => {
														setCursor(i);
														setPlaying(false);
													}}
													onKeyDown={(ev) => {
														if (ev.key === "Enter" || ev.key === " ") {
															ev.preventDefault();
															setCursor(i);
															setPlaying(false);
														}
													}}
												>
													<span className={`shrink-0 mt-0.5 ${
														meta.tone === "success" ? "text-volt" :
														meta.tone === "warning" ? "text-amber-400" :
														meta.tone === "purple" ? "text-purple-400" :
														"text-cyan-400"
													}`}>{meta.icon}</span>
													<div className="flex-1 min-w-0">
														<div className="flex items-center gap-2 text-xs">
															<Badge tone={meta.tone} mono>{meta.label}</Badge>
															<span className="text-ink-200 truncate">{e.title}</span>
														</div>
														<p className="text-[10px] text-ink-500 mt-0.5 font-mono">
															{new Date(e.at).toLocaleTimeString()}
															{e.tokens ? ` · ${Intl.NumberFormat().format(e.tokens)} tokens` : ""}
															{e.duration_ms ? ` · ${e.duration_ms}ms` : ""}
														</p>
													</div>
												</li>
											);
										})}
									</ul>
								</div>
							</>
						)}
					</CardBody>
				</Card>
			</div>
		</div>
	);
}

function EntryDetail({ entry }: { entry: ReplayEntry }) {
	const meta = KIND_META[entry.kind];
	return (
		<div className="rounded-lg border border-white/8 bg-space-800/50 p-4 space-y-3">
			<div className="flex items-center gap-2 flex-wrap">
				<Badge tone={meta.tone} mono>{meta.label}</Badge>
				{entry.obs_type && <Badge tone="cyan" mono>{entry.obs_type}</Badge>}
				{entry.model && <Badge tone="purple" mono>{entry.model}</Badge>}
				{entry.tokens ? (
					<span className="text-[11px] font-mono text-ink-400">
						{Intl.NumberFormat().format(entry.tokens)} tokens
					</span>
				) : null}
			</div>
			<h3 className="text-base font-medium text-ink-100">{entry.title}</h3>
			{entry.body && (
				<pre className="text-xs font-mono text-ink-200 bg-space-900 border border-white/8 rounded p-3 whitespace-pre-wrap break-words leading-relaxed">
					{entry.body}
				</pre>
			)}
			{entry.excerpt_in && (
				<div>
					<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1">Prompt excerpt</p>
					<pre className="text-xs font-mono text-ink-200 bg-space-900 border border-white/8 rounded p-3 whitespace-pre-wrap leading-relaxed">
						{entry.excerpt_in}
					</pre>
				</div>
			)}
			{entry.excerpt_out && (
				<div>
					<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1">Response excerpt</p>
					<pre className="text-xs font-mono text-ink-200 bg-space-900 border border-white/8 rounded p-3 whitespace-pre-wrap leading-relaxed">
						{entry.excerpt_out}
					</pre>
				</div>
			)}
		</div>
	);
}
