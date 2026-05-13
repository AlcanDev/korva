import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Lightbulb, ChevronRight } from "lucide-react";
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

// Phase 10.4 — Pattern insights panel.
//
// Surfaces topics that recur across multiple observations but aren't yet
// declared as a `pattern` row. The operator picks one and (optionally)
// promotes it via vault_save with type=pattern.

interface PatternSuggestion {
	phrase: string;
	project: string;
	count: number;
	sample_ids: string[];
	sample_titles: string[];
	already_pattern: boolean;
	severity: "info" | "high";
}

interface InsightsResponse {
	project: string;
	suggestions: PatternSuggestion[];
}

function useInsights(project: string, minCount: number) {
	return useQuery({
		queryKey: ["insights", project, minCount],
		queryFn: () => {
			const params = new URLSearchParams();
			if (project) params.set("project", project);
			params.set("min_count", String(minCount));
			return adminFetch<InsightsResponse>(`/admin/insights/patterns?${params}`);
		},
	});
}

export default function InsightsPanel() {
	const projects = useProjects();
	const [project, setProject] = useState("");
	const [minCount, setMinCount] = useState(3);

	const insights = useInsights(project, minCount);

	const firstProject = projects.data?.projects?.[0]?.name;
	if (!project && firstProject) setProject(firstProject);

	return (
		<div className="p-4 sm:p-6 max-w-6xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Pattern intelligence"
				icon={<Lightbulb size={22} />}
				title="Insights"
				subtitle="Topics that recur across multiple observations but aren't yet declared as patterns. Promote them to consolidate informal knowledge into canonical rows."
			/>

			<Card>
				<CardBody>
					<div className="flex items-center gap-3 flex-wrap">
						<label htmlFor="ins-project" className="text-[10px] uppercase tracking-wider text-ink-400">
							Project
						</label>
						<select
							id="ins-project"
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
						<label htmlFor="ins-min" className="text-[10px] uppercase tracking-wider text-ink-400">
							Min count
						</label>
						<input
							id="ins-min"
							type="number"
							min={2}
							max={20}
							value={minCount}
							onChange={(e) => setMinCount(Number(e.target.value))}
							className="bg-space-900 border border-white/10 rounded-md px-2 py-1.5 text-sm text-ink-100 w-20 focus:border-volt focus:outline-none"
						/>
					</div>
				</CardBody>
			</Card>

			{insights.error ? <ErrorBanner title="Couldn't load insights" message={String(insights.error)} /> : null}

			<Card>
				<CardHeader
					title={`${insights.data?.suggestions.length ?? 0} pattern candidate(s)`}
					subtitle="Sorted by severity then count. Click a row to see the sample observations."
				/>
				<CardBody className="!p-0">
					{insights.isLoading ? (
						<Skeleton height={200} />
					) : !insights.data || insights.data.suggestions.length === 0 ? (
						<div className="p-4">
							<EmptyState
								tone="volt"
								icon={<Lightbulb size={22} />}
								title="No recurring patterns yet"
								description={`Save more observations in ${project || "this project"} — when the same topic appears in ${minCount}+ entries it'll show up here.`}
								hint="More obs → smarter suggestions"
							/>
						</div>
					) : (
						<ul className="divide-y divide-white/5">
							{insights.data.suggestions.map((s) => (
								<SuggestionRow key={s.phrase} s={s} />
							))}
						</ul>
					)}
				</CardBody>
			</Card>
		</div>
	);
}

function SuggestionRow({ s }: { s: PatternSuggestion }) {
	const [open, setOpen] = useState(false);
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
				<div className="flex-1 min-w-0">
					<div className="flex items-center gap-2 flex-wrap text-xs">
						<span className="font-mono text-ink-100">"{s.phrase}"</span>
						<Badge tone={s.severity === "high" ? "warning" : "neutral"} mono>
							{s.count} occurrences
						</Badge>
						{s.already_pattern && <Badge tone="success" mono>already a pattern</Badge>}
					</div>
					{!s.already_pattern && (
						<p className="text-[11px] text-ink-400 mt-1">
							Consider declaring this as a formal <code>pattern</code> observation so future
							sessions can reference it directly.
						</p>
					)}
				</div>
			</button>
			{open && (
				<div className="px-4 pb-3 pl-11">
					<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-2">
						Sample observations
					</p>
					<ul className="space-y-1.5">
						{s.sample_titles.map((t, i) => (
							<li key={s.sample_ids[i]} className="text-xs text-ink-300 flex items-start gap-2">
								<code className="font-mono text-ink-500 shrink-0">{s.sample_ids[i].slice(-8)}</code>
								<span className="truncate">{t}</span>
							</li>
						))}
					</ul>
				</div>
			)}
		</li>
	);
}
