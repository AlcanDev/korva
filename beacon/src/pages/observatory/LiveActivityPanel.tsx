import { useMemo } from "react";
import {
	Activity,
	BookOpen,
	Cloud,
	Download,
	GitMerge,
	Save,
	Terminal,
	type LucideIcon,
} from "lucide-react";
import { useActivityFeed, type ActivityEvent, type EventKind } from "@/api/events";
import {
	Badge,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	PageHero,
	StatusDot,
} from "@/components/ui";

// Phase 8.5 — UI over /admin/events SSE. Shows a live stream of every
// observation save, command run, conflict, etc. as they happen. The
// connection state (connected/disconnected) is surfaced via a pulsing
// StatusDot in the header so the operator always knows the channel is
// healthy.
//
// Designed to feel like a Slack / Linear timeline: most-recent at top,
// per-event icon + label + timestamp + meta. Filter by kind to focus on
// one signal (e.g. only conflicts).

const ICON_BY_KIND: Record<EventKind, LucideIcon> = {
	observation_saved: Save,
	session_started: BookOpen,
	session_ended: BookOpen,
	conflict_detected: GitMerge,
	command_run: Terminal,
	export_written: Download,
	hive_phase_changed: Cloud,
};

const TONE_BY_KIND: Record<EventKind, "success" | "info" | "warning" | "danger" | "purple" | "cyan"> = {
	observation_saved: "success",
	session_started: "cyan",
	session_ended: "info",
	conflict_detected: "warning",
	command_run: "purple",
	export_written: "success",
	hive_phase_changed: "info",
};

const LABEL_BY_KIND: Record<EventKind, string> = {
	observation_saved: "Observation saved",
	session_started: "Session started",
	session_ended: "Session ended",
	conflict_detected: "Conflict detected",
	command_run: "Command run",
	export_written: "Obsidian export",
	hive_phase_changed: "Hive phase changed",
};

export default function LiveActivityPanel() {
	const { events, connected, error } = useActivityFeed();

	// Aggregate counts per kind for the header summary.
	const summary = useMemo(() => {
		const m = new Map<EventKind, number>();
		for (const e of events) m.set(e.kind, (m.get(e.kind) ?? 0) + 1);
		return Array.from(m.entries());
	}, [events]);

	return (
		<div className="p-4 sm:p-6 max-w-6xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Real-time observability"
				icon={<Activity size={22} />}
				title="Live activity"
				subtitle="Server-Sent Events stream every observation, command, and conflict the moment it happens. Open this on a side monitor during a code review."
				badge={{
					tone: connected ? "success" : error ? "danger" : "neutral",
					label: (
						<span className="inline-flex items-center gap-1.5">
							<StatusDot
								state={connected ? "running" : error ? "error" : "idle"}
								pulse={connected}
							/>
							{connected ? "Live" : error ? "Disconnected" : "Connecting…"}
						</span>
					),
				}}
			/>

			{summary.length > 0 && (
				<div className="flex flex-wrap gap-1.5">
					{summary.map(([kind, count]) => (
						<Badge key={kind} tone={TONE_BY_KIND[kind]} mono>
							{LABEL_BY_KIND[kind]} · {count}
						</Badge>
					))}
				</div>
			)}

			<Card>
				<CardHeader
					title="Stream"
					subtitle={
						events.length === 0
							? "Waiting for the first event…"
							: `${events.length} event(s) buffered (most recent first)`
					}
				/>
				<CardBody className="!p-0">
					{events.length === 0 ? (
						<div className="p-4">
							<EmptyState
								tone="cyan"
								icon={<Activity size={22} />}
								title="No activity yet"
								description="Save an observation from your editor or run a korva CLI command. Events show up here within milliseconds."
								hint="vault_save → live stream"
							/>
						</div>
					) : (
						<ul className="divide-y divide-white/5">
							{events.map((ev, i) => (
								<ActivityRow key={`${ev.at}-${i}`} ev={ev} />
							))}
						</ul>
					)}
				</CardBody>
			</Card>
		</div>
	);
}

function ActivityRow({ ev }: { ev: ActivityEvent }) {
	const Icon = ICON_BY_KIND[ev.kind] ?? Activity;
	const tone = TONE_BY_KIND[ev.kind] ?? "info";
	const meta = ev.meta ?? {};
	return (
		<li className="flex items-start gap-3 px-4 py-3 hover:bg-white/3 transition-colors">
			<span
				className={`shrink-0 mt-0.5 ${tone === "success" ? "text-volt" : tone === "warning" ? "text-amber-400" : tone === "danger" ? "text-[#F85149]" : tone === "purple" ? "text-purple-400" : tone === "cyan" ? "text-cyan-400" : "text-ink-400"}`}
				aria-hidden
			>
				<Icon size={14} />
			</span>
			<div className="flex-1 min-w-0">
				<div className="flex items-center gap-2 flex-wrap text-xs">
					<span className="text-ink-100 font-medium">{LABEL_BY_KIND[ev.kind]}</span>
					{ev.project ? (
						<Badge tone="cyan" mono>
							{ev.project}
						</Badge>
					) : null}
					{ev.actor ? (
						<span className="text-ink-400">
							by <span className="text-ink-200">{ev.actor}</span>
						</span>
					) : null}
				</div>
				{ev.title ? (
					<p className="text-sm text-ink-200 mt-0.5 truncate">{ev.title}</p>
				) : null}
				{Object.keys(meta).length > 0 ? (
					<div className="flex flex-wrap gap-1.5 mt-1.5">
						{Object.entries(meta).map(([k, v]) => (
							<code
								key={k}
								className="text-[10px] font-mono bg-space-700/60 border border-white/8 rounded px-1.5 py-0.5 text-ink-300"
							>
								{k}: {String(v)}
							</code>
						))}
					</div>
				) : null}
			</div>
			<time
				className="text-[10px] font-mono text-ink-500 shrink-0 mt-1 tabular-nums"
				dateTime={ev.at}
			>
				{formatTime(ev.at)}
			</time>
		</li>
	);
}

function formatTime(iso: string): string {
	try {
		const d = new Date(iso);
		return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
	} catch {
		return iso;
	}
}
