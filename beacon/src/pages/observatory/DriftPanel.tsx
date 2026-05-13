import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, ChevronRight, Scale } from "lucide-react";
import { adminFetch } from "@/api/_fetch";
import { useProjects } from "@/api/projects";
import {
	Badge,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	ErrorBanner,
	PageHero,
	Skeleton,
} from "@/components/ui";

// Phase 10.5 — Decision drift alerts panel.
//
// Lists pairs (decision, later observation) where the newer row may have
// violated the older decision. Heuristic — operator decides whether each
// alert is real.

interface DriftAlert {
	decision_id: string;
	decision_title: string;
	decision_created: string;
	violator_id: string;
	violator_title: string;
	violator_type: string;
	violator_created: string;
	overlap_score: number;
	project: string;
	severity: "info" | "warning" | "danger";
	reason: string;
}

interface DriftResponse {
	project: string;
	window_days: number;
	alerts: DriftAlert[];
}

function useDrift(project: string, days: number) {
	return useQuery({
		queryKey: ["drift", project, days],
		queryFn: () => {
			const params = new URLSearchParams();
			if (project) params.set("project", project);
			params.set("days", String(days));
			return adminFetch<DriftResponse>(`/admin/drift/decisions?${params}`);
		},
		refetchInterval: 60_000,
	});
}

const SEV_TONE: Record<DriftAlert["severity"], "info" | "warning" | "danger"> = {
	info: "info",
	warning: "warning",
	danger: "danger",
};

export default function DriftPanel() {
	const projects = useProjects();
	const [project, setProject] = useState("");
	const [days, setDays] = useState(30);

	const drift = useDrift(project, days);
	const firstProject = projects.data?.projects?.[0]?.name;
	if (!project && firstProject) setProject(firstProject);

	const alerts = drift.data?.alerts ?? [];

	return (
		<div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Decision integrity"
				icon={<Scale size={22} />}
				title="Decision drift"
				subtitle="Pairs (decision, later observation) where the newer row may have violated the older decision. Korva surfaces candidates; you decide what's real."
				badge={alerts.length > 0 ? { tone: "warning", label: `${alerts.length} candidates` } : undefined}
			/>

			<Card>
				<CardBody>
					<div className="flex items-center gap-3 flex-wrap">
						<label htmlFor="drift-project" className="text-[10px] uppercase tracking-wider text-ink-400">
							Project
						</label>
						<select
							id="drift-project"
							value={project}
							onChange={(e) => setProject(e.target.value)}
							className="bg-space-900 border border-white/10 rounded-md px-3 py-1.5 text-sm text-ink-100 focus:border-volt focus:outline-none min-w-[200px]"
						>
							{(projects.data?.projects ?? []).map((p) => (
								<option key={p.name} value={p.name}>
									{p.name} ({p.observation_count})
								</option>
							))}
						</select>
						<label htmlFor="drift-days" className="text-[10px] uppercase tracking-wider text-ink-400">
							Window (days)
						</label>
						<input
							id="drift-days"
							type="number"
							min={1}
							max={365}
							value={days}
							onChange={(e) => setDays(Number(e.target.value))}
							className="bg-space-900 border border-white/10 rounded-md px-2 py-1.5 text-sm text-ink-100 w-20 focus:border-volt focus:outline-none"
						/>
					</div>
				</CardBody>
			</Card>

			{drift.error ? <ErrorBanner title="Couldn't load drift" message={String(drift.error)} /> : null}

			<Card>
				<CardHeader
					title={`${alerts.length} candidate violation(s)`}
					subtitle="Sorted by severity then overlap score. Highest-overlap pairs first."
				/>
				<CardBody className="!p-0">
					{drift.isLoading ? (
						<Skeleton height={200} />
					) : alerts.length === 0 ? (
						<div className="p-4">
							<EmptyState
								tone="volt"
								icon={<Scale size={22} />}
								title="No drift detected"
								description={`No newer ${project ? `${project} ` : ""}observations in the last ${days} days share enough terminology with a prior decision to count as drift.`}
								hint="Healthy decisions hold up"
							/>
						</div>
					) : (
						<ul className="divide-y divide-white/5">
							{alerts.map((a, i) => (
								<DriftRow key={`${a.decision_id}-${a.violator_id}-${i}`} a={a} />
							))}
						</ul>
					)}
				</CardBody>
			</Card>
		</div>
	);
}

function DriftRow({ a }: { a: DriftAlert }) {
	const [open, setOpen] = useState(false);
	const tone = SEV_TONE[a.severity];
	return (
		<li>
			<button
				type="button"
				onClick={() => setOpen((o) => !o)}
				className="w-full flex items-start gap-3 px-4 py-3 hover:bg-white/3 transition-colors text-left"
				aria-expanded={open}
			>
				<ChevronRight
					size={14}
					className={`shrink-0 mt-1 text-ink-400 transition-transform ${open ? "rotate-90" : ""}`}
				/>
				<span
					className={`shrink-0 mt-0.5 ${tone === "danger" ? "text-[#F85149]" : tone === "warning" ? "text-amber-400" : "text-cyan-400"}`}
				>
					<AlertTriangle size={14} />
				</span>
				<div className="flex-1 min-w-0">
					<div className="flex items-center gap-2 flex-wrap text-xs">
						<Badge tone={tone} mono>
							{a.severity}
						</Badge>
						<Badge tone="cyan" mono>{a.violator_type}</Badge>
						<span className="text-ink-400">overlap</span>
						<span className={tone === "danger" ? "text-[#F85149] font-mono" : tone === "warning" ? "text-amber-400 font-mono" : "text-cyan-400 font-mono"}>
							{(a.overlap_score * 100).toFixed(0)}%
						</span>
					</div>
					<div className="grid grid-cols-1 md:grid-cols-2 gap-2 mt-2 text-xs">
						<div>
							<p className="text-[10px] uppercase tracking-wider text-ink-500 mb-0.5">Decision</p>
							<p className="text-ink-100 truncate">{a.decision_title}</p>
							<p className="text-[10px] text-ink-500 mt-0.5 font-mono">
								{new Date(a.decision_created).toLocaleDateString()}
							</p>
						</div>
						<div>
							<p className="text-[10px] uppercase tracking-wider text-ink-500 mb-0.5">Possible violator</p>
							<p className="text-ink-100 truncate">{a.violator_title}</p>
							<p className="text-[10px] text-ink-500 mt-0.5 font-mono">
								{new Date(a.violator_created).toLocaleDateString()}
							</p>
						</div>
					</div>
				</div>
			</button>
			{open && (
				<div className="px-4 pb-3 pl-11">
					<p className="text-xs text-ink-300 leading-relaxed">{a.reason}</p>
					<p className="text-[10px] font-mono text-ink-500 mt-2">
						decision id <span className="text-ink-300">{a.decision_id.slice(-12)}</span>
						{"  "}·{"  "}
						violator id <span className="text-ink-300">{a.violator_id.slice(-12)}</span>
					</p>
				</div>
			)}
		</li>
	);
}
