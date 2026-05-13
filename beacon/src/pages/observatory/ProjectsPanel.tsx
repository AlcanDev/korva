import { useMemo, useState } from "react";
import {
	type ConsolidationProposal,
	useConsolidateProjects,
	useProjectSuggestions,
	useProjects,
	usePruneProjects,
} from "@/api/projects";
import {
	AlertCircle,
	Check,
	FolderTree,
	GitMerge,
	RefreshCw,
	Trash2,
} from "lucide-react";
import {
	Badge,
	Button,
	Card,
	CardBody,
	CardHeader,
	EmptyState,
	ErrorBanner,
	MetricCard,
	PageHero,
	Skeleton,
	Tabs,
} from "@/components/ui";
import { BarChart } from "@/components/charts";

// Phase 7 — Refresh visual del Projects panel.
//
// Tres flujos del operador, ahora sobre el design-system Space:
//   1. Inventory  — métricas + bar chart "top proyectos" + tabla con tooltips
//   2. Consolidate — propuestas en cards con doble columna; el botón Merge
//                    siempre lleva la acción al canónico actualizado
//   3. Prune       — dry-run primero, confirmación de doble paso antes de
//                    aplicar (sin botón rojo accidental)

type Tab = "list" | "suggestions" | "prune";

export default function ProjectsPanel() {
	const [tab, setTab] = useState<Tab>("list");

	const tabs = [
		{ value: "list" as const, label: "Inventory" },
		{ value: "suggestions" as const, label: "Consolidate" },
		{ value: "prune" as const, label: "Prune empty" },
	];

	return (
		<div className="p-6 max-w-7xl mx-auto space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Project hygiene"
				icon={<FolderTree size={22} />}
				title="Projects"
				subtitle={
					<>
						Inspect, consolidate, and prune the projects your vault tracks.
						Variant names like <code className="font-mono">alpha</code>/
						<code className="font-mono">Alpha</code> and orphan sessions from
						abandoned MCP runs are exactly what these tools clean up.
					</>
				}
			/>

			<Tabs<Tab> value={tab} onChange={setTab} tabs={tabs} />

			<div className="space-y-4">
				{tab === "list" && <ProjectsList />}
				{tab === "suggestions" && <ConsolidateView />}
				{tab === "prune" && <PruneView />}
			</div>
		</div>
	);
}

// ── Inventory ────────────────────────────────────────────────────────────────

function ProjectsList() {
	const { data, isLoading, error, refetch, isFetching } = useProjects();

	const aggregates = useMemo(() => {
		const projects = data?.projects ?? [];
		const totalObs = projects.reduce((acc, p) => acc + p.observation_count, 0);
		const totalSess = projects.reduce((acc, p) => acc + p.session_count, 0);
		const topRows = projects.map((p) => ({ label: p.name, value: p.observation_count }));
		return { totalObs, totalSess, topRows };
	}, [data]);

	if (isLoading)
		return (
			<div className="grid grid-cols-1 md:grid-cols-3 gap-3">
				<Skeleton height={92} />
				<Skeleton height={92} />
				<Skeleton height={92} />
			</div>
		);
	if (error) return <ErrorBanner message={String(error)} />;

	const projects = data?.projects ?? [];
	return (
		<>
			{/* Metric strip */}
			<div className="grid grid-cols-2 md:grid-cols-3 gap-3">
				<MetricCard
					label="Projects"
					value={data?.count ?? 0}
					tone="cyan"
					hint="distinct names in the vault"
				/>
				<MetricCard
					label="Observations"
					value={aggregates.totalObs.toLocaleString()}
					tone="volt"
					hint="across all projects"
				/>
				<MetricCard
					label="Sessions"
					value={aggregates.totalSess.toLocaleString()}
					tone="purple"
					hint="across all projects"
				/>
			</div>

			<div className="grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-4 mt-4">
				{/* Table */}
				<Card>
					<CardHeader
						title={`${data?.count ?? 0} project(s) tracked`}
						actions={
							<Button
								size="sm"
								variant="ghost"
								leftIcon={<RefreshCw size={11} className={isFetching ? "animate-spin" : ""} />}
								onClick={() => refetch()}
								disabled={isFetching}
							>
								Refresh
							</Button>
						}
					/>
					{projects.length === 0 ? (
						<CardBody>
							<EmptyState
								title="No projects yet"
								description="The vault hasn't recorded any observations or sessions."
							/>
						</CardBody>
					) : (
						<div className="overflow-x-auto">
							<table className="w-full text-sm">
								<thead className="text-[10px] uppercase tracking-wider text-ink-400 bg-white/3">
									<tr>
										<th className="text-left py-2 px-4 font-medium">Project</th>
										<th className="text-right py-2 px-4 font-medium">Observations</th>
										<th className="text-right py-2 px-4 font-medium">Sessions</th>
									</tr>
								</thead>
								<tbody>
									{projects.map((p) => (
										<tr
											key={p.name}
											className="border-t border-white/5 hover:bg-white/3 transition-colors"
										>
											<td className="py-2.5 px-4 text-ink-100 font-mono">{p.name}</td>
											<td className="py-2.5 px-4 text-right text-ink-300 font-mono">
												{p.observation_count.toLocaleString()}
											</td>
											<td className="py-2.5 px-4 text-right text-ink-300 font-mono">
												{p.session_count.toLocaleString()}
											</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					)}
				</Card>

				{/* Top-projects bar chart */}
				<Card>
					<CardHeader title="Top projects" subtitle="by observation count" />
					<CardBody>
						<BarChart
							data={aggregates.topRows}
							maxRows={8}
							emptyMessage="Save observations to populate this chart."
						/>
					</CardBody>
				</Card>
			</div>
		</>
	);
}

// ── Consolidate ──────────────────────────────────────────────────────────────

function ConsolidateView() {
	const { data, isLoading, error, refetch, isFetching } = useProjectSuggestions();
	const consolidate = useConsolidateProjects();

	if (isLoading) return <Skeleton height={120} />;
	if (error) return <ErrorBanner message={String(error)} />;

	const proposals = data?.proposals ?? [];
	return (
		<Card>
			<CardHeader
				title={`${data?.count ?? 0} merge candidate(s)`}
				subtitle="Variant names that normalize to the same canonical form."
				actions={
					<Button
						size="sm"
						variant="ghost"
						leftIcon={<RefreshCw size={11} className={isFetching ? "animate-spin" : ""} />}
						onClick={() => refetch()}
						disabled={isFetching}
					>
						Re-scan
					</Button>
				}
			/>
			<CardBody>
				{proposals.length === 0 ? (
					<EmptyState
						title="No variants found"
						description="Every project has a unique normalized name."
					/>
				) : (
					<div className="space-y-3">
						{proposals.map((p) => (
							<ProposalCard
								key={p.canonical}
								proposal={p}
								onMerge={(canonical, sources) =>
									consolidate.mutate({ canonical, sources })
								}
								pending={consolidate.isPending}
							/>
						))}
						{consolidate.isSuccess && (
							<div className="rounded-lg border border-volt/30 bg-volt-dim px-3 py-2 text-xs flex items-center gap-2">
								<Check size={12} className="text-volt" />
								<span className="text-ink-200">
									Merged{" "}
									<span className="font-mono text-volt">
										{consolidate.data?.observations_updated ?? 0}
									</span>{" "}
									observation(s) into{" "}
									<code className="font-mono text-volt">
										{consolidate.data?.canonical}
									</code>
								</span>
							</div>
						)}
						{consolidate.error && <ErrorBanner message={String(consolidate.error)} />}
					</div>
				)}
			</CardBody>
		</Card>
	);
}

function ProposalCard({
	proposal,
	onMerge,
	pending,
}: {
	proposal: ConsolidationProposal;
	onMerge: (canonical: string, sources: string[]) => void;
	pending: boolean;
}) {
	const [canonical, setCanonical] = useState(proposal.canonical);
	const sources = proposal.variants.map((v) => v.name).filter((n) => n !== canonical);
	const selectId = `canonical-${proposal.canonical}`;
	return (
		<div className="rounded-lg border border-white/8 bg-space-800/50 p-4">
			<div className="grid grid-cols-1 md:grid-cols-[1fr_auto] gap-3 items-end">
				<div>
					<label
						htmlFor={selectId}
						className="block text-[10px] uppercase tracking-wider text-ink-400 mb-1"
					>
						Canonical name
					</label>
					<select
						id={selectId}
						value={canonical}
						onChange={(e) => setCanonical(e.target.value)}
						className="w-full bg-space-900 border border-white/10 rounded-md px-3 py-1.5 text-sm text-ink-100 focus:border-volt focus:outline-none"
					>
						{proposal.variants.map((v) => (
							<option key={v.name} value={v.name}>
								{v.name} ({v.observation_count} obs)
							</option>
						))}
					</select>
				</div>
				<Button
					variant="volt"
					onClick={() => onMerge(canonical, sources)}
					disabled={pending || sources.length === 0}
					loading={pending}
					leftIcon={<GitMerge size={12} />}
				>
					Merge into canonical
				</Button>
			</div>
			<div className="mt-3">
				<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1.5">
					Sources (will be folded into canonical)
				</p>
				<div className="flex flex-wrap gap-1.5">
					{sources.length === 0 ? (
						<span className="text-[11px] text-ink-500 italic">
							(no other variants — change canonical to merge)
						</span>
					) : (
						sources.map((s) => (
							<Badge key={s} tone="cyan" mono>
								{s}
							</Badge>
						))
					)}
				</div>
			</div>
		</div>
	);
}

// ── Prune ────────────────────────────────────────────────────────────────────

function PruneView() {
	const prune = usePruneProjects();
	const [confirmApply, setConfirmApply] = useState(false);

	function runDryRun() {
		setConfirmApply(false);
		prune.mutate({ apply: false });
	}
	function runApply() {
		prune.mutate({ apply: true }, { onSuccess: () => setConfirmApply(false) });
	}

	const data = prune.data;
	const empty = data?.empty ?? [];

	return (
		<Card>
			<CardHeader
				title="Prune empty projects"
				subtitle={
					<>
						Empty projects own sessions but zero observations. Pruning drops the
						orphan sessions; observations are never touched.
					</>
				}
			/>
			<CardBody className="space-y-3">
				<div className="flex items-center gap-2 flex-wrap">
					<Button
						variant="secondary"
						leftIcon={<RefreshCw size={12} className={prune.isPending && !confirmApply ? "animate-spin" : ""} />}
						onClick={runDryRun}
						disabled={prune.isPending}
					>
						{prune.isPending && !confirmApply ? "Scanning…" : "Dry-run scan"}
					</Button>
					{data && empty.length > 0 && !data.dry_run && (
						<Badge tone="success" leftIcon={<Check size={11} />}>
							Removed {data.sessions_removed} session(s)
						</Badge>
					)}
					{data && empty.length > 0 && data.dry_run && (
						!confirmApply ? (
							<Button
								variant="danger"
								leftIcon={<Trash2 size={12} />}
								onClick={() => setConfirmApply(true)}
							>
								Apply…
							</Button>
						) : (
							<>
								<Badge tone="warning" leftIcon={<AlertCircle size={11} />}>
									This deletes {empty.length} project's sessions. Sure?
								</Badge>
								<Button
									variant="danger"
									leftIcon={<Trash2 size={12} />}
									loading={prune.isPending}
									onClick={runApply}
								>
									Confirm apply
								</Button>
								<Button variant="ghost" onClick={() => setConfirmApply(false)}>
									Cancel
								</Button>
							</>
						)
					)}
				</div>
				{prune.error && <ErrorBanner message={String(prune.error)} />}
				{data && empty.length === 0 && (
					<EmptyState
						title="No empty projects"
						description="Every project with sessions also has at least one observation."
					/>
				)}
				{data && empty.length > 0 && (
					<div className="overflow-x-auto rounded-lg border border-white/8">
						<table className="w-full text-sm">
							<thead className="text-[10px] uppercase tracking-wider text-ink-400 bg-white/3">
								<tr>
									<th className="text-left py-2 px-4 font-medium">Project</th>
									<th className="text-right py-2 px-4 font-medium">Sessions</th>
									<th className="text-right py-2 px-4 font-medium">Prompts</th>
								</tr>
							</thead>
							<tbody>
								{empty.map((e) => (
									<tr key={e.project} className="border-t border-white/5">
										<td className="py-2.5 px-4 text-ink-100 font-mono">{e.project}</td>
										<td className="py-2.5 px-4 text-right text-ink-300 font-mono">
											{e.session_count}
										</td>
										<td className="py-2.5 px-4 text-right text-ink-300 font-mono">
											{e.prompt_count}
										</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</CardBody>
		</Card>
	);
}

