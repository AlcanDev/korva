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
import { useI18n } from "@/contexts/i18n";

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
	const { t } = useI18n();
	const tx = t.projects;
	const [tab, setTab] = useState<Tab>("list");

	const tabs = [
		{ value: "list" as const, label: tx.tabInventory },
		{ value: "suggestions" as const, label: tx.tabConsolidate },
		{ value: "prune" as const, label: tx.tabPrune },
	];

	return (
		<div className="p-6 max-w-7xl mx-auto space-y-5 animate-fade-up">
			<PageHero
				eyebrow={tx.eyebrow}
				icon={<FolderTree size={22} />}
				title={tx.title}
				subtitle={tx.subtitle}
			/>

			<Tabs<Tab> value={tab} onChange={setTab} tabs={tabs} />

			<div className="space-y-4">
				{tab === "list" && <ProjectsList tx={tx} />}
				{tab === "suggestions" && <ConsolidateView tx={tx} />}
				{tab === "prune" && <PruneView tx={tx} />}
			</div>
		</div>
	);
}

type ProjectsLang = ReturnType<typeof useI18n>["t"]["projects"];

// ── Inventory ────────────────────────────────────────────────────────────────

function ProjectsList({ tx }: { tx: ProjectsLang }) {
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
					label={tx.metricProjects}
					value={data?.count ?? 0}
					tone="cyan"
					hint={tx.metricProjectsHint}
				/>
				<MetricCard
					label={tx.metricObservations}
					value={aggregates.totalObs.toLocaleString()}
					tone="volt"
					hint={tx.metricObservationsHint}
				/>
				<MetricCard
					label={tx.metricSessions}
					value={aggregates.totalSess.toLocaleString()}
					tone="purple"
					hint={tx.metricSessionsHint}
				/>
			</div>

			<div className="grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-4 mt-4">
				{/* Table */}
				<Card>
					<CardHeader
						title={tx.inventoryCount(data?.count ?? 0)}
						actions={
							<Button
								size="sm"
								variant="ghost"
								leftIcon={<RefreshCw size={11} className={isFetching ? "animate-spin" : ""} />}
								onClick={() => refetch()}
								disabled={isFetching}
							>
								{tx.refresh}
							</Button>
						}
					/>
					{projects.length === 0 ? (
						<CardBody>
							<EmptyState
								tone="cyan"
								icon={<FolderTree size={22} />}
								title={tx.emptyTitle}
								description={tx.emptyDesc}
								hint="vault_save → start tracking"
							/>
						</CardBody>
					) : (
						<div className="overflow-x-auto">
							<table className="w-full text-sm">
								<thead className="text-[10px] uppercase tracking-wider text-ink-400 bg-white/3">
									<tr>
										<th className="text-left py-2 px-4 font-medium">{tx.columnProject}</th>
										<th className="text-right py-2 px-4 font-medium">{tx.columnObservations}</th>
										<th className="text-right py-2 px-4 font-medium">{tx.columnSessions}</th>
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
					<CardHeader title={tx.topProjects} subtitle={tx.topProjectsHint} />
					<CardBody>
						<BarChart data={aggregates.topRows} maxRows={8} />
					</CardBody>
				</Card>
			</div>
		</>
	);
}

// ── Consolidate ──────────────────────────────────────────────────────────────

function ConsolidateView({ tx }: { tx: ProjectsLang }) {
	const { data, isLoading, error, refetch, isFetching } = useProjectSuggestions();
	const consolidate = useConsolidateProjects();

	if (isLoading) return <Skeleton height={120} />;
	if (error) return <ErrorBanner message={String(error)} />;

	const proposals = data?.proposals ?? [];
	return (
		<Card>
			<CardHeader
				title={tx.consolidateCount(data?.count ?? 0)}
				subtitle={tx.consolidateSubtitle}
				actions={
					<Button
						size="sm"
						variant="ghost"
						leftIcon={<RefreshCw size={11} className={isFetching ? "animate-spin" : ""} />}
						onClick={() => refetch()}
						disabled={isFetching}
					>
						{tx.rescan}
					</Button>
				}
			/>
			<CardBody>
				{proposals.length === 0 ? (
					<EmptyState
						tone="volt"
						icon={<GitMerge size={22} />}
						title={tx.consolidateEmpty}
						description={tx.consolidateEmptyDesc}
						hint="Korva normalizes names automatically"
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
								tx={tx}
							/>
						))}
						{consolidate.isSuccess && (
							<div className="rounded-lg border border-volt/30 bg-volt-dim px-3 py-2 text-xs flex items-center gap-2">
								<Check size={12} className="text-volt" />
								<span className="text-ink-200">
									{tx.mergeSuccess(
										consolidate.data?.observations_updated ?? 0,
										consolidate.data?.canonical ?? "",
									)}
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
	tx,
}: {
	proposal: ConsolidationProposal;
	onMerge: (canonical: string, sources: string[]) => void;
	pending: boolean;
	tx: ProjectsLang;
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
						{tx.canonicalName}
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
					{pending ? tx.merging : tx.mergeIntoCanonical}
				</Button>
			</div>
			<div className="mt-3">
				<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1.5">
					{tx.sourcesLabel}
				</p>
				<div className="flex flex-wrap gap-1.5">
					{sources.length === 0 ? (
						<span className="text-[11px] text-ink-500 italic">
							{tx.noOtherVariants}
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

function PruneView({ tx }: { tx: ProjectsLang }) {
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
			<CardHeader title={tx.pruneTitle} subtitle={tx.pruneSubtitle} />
			<CardBody className="space-y-3">
				<div className="flex items-center gap-2 flex-wrap">
					<Button
						variant="secondary"
						leftIcon={<RefreshCw size={12} className={prune.isPending && !confirmApply ? "animate-spin" : ""} />}
						onClick={runDryRun}
						disabled={prune.isPending}
					>
						{prune.isPending && !confirmApply ? tx.pruneScanning : tx.pruneDryRun}
					</Button>
					{data && empty.length > 0 && !data.dry_run && (
						<Badge tone="success" leftIcon={<Check size={11} />}>
							{tx.pruneRemoved(data.sessions_removed)}
						</Badge>
					)}
					{data && empty.length > 0 && data.dry_run && (
						!confirmApply ? (
							<Button
								variant="danger"
								leftIcon={<Trash2 size={12} />}
								onClick={() => setConfirmApply(true)}
							>
								{tx.pruneApply}
							</Button>
						) : (
							<>
								<Badge tone="warning" leftIcon={<AlertCircle size={11} />}>
									{tx.pruneConfirmWarning(empty.length)}
								</Badge>
								<Button
									variant="danger"
									leftIcon={<Trash2 size={12} />}
									loading={prune.isPending}
									onClick={runApply}
								>
									{tx.pruneConfirm}
								</Button>
								<Button variant="ghost" onClick={() => setConfirmApply(false)}>
									{tx.cancel}
								</Button>
							</>
						)
					)}
				</div>
				{prune.error && <ErrorBanner message={String(prune.error)} />}
				{data && empty.length === 0 && (
					<EmptyState
						tone="volt"
						icon={<Check size={22} />}
						title={tx.pruneEmptyTitle}
						description={tx.pruneEmptyDesc}
						hint="Nothing to clean — you're tidy"
					/>
				)}
				{data && empty.length > 0 && (
					<div className="overflow-x-auto rounded-lg border border-white/8">
						<table className="w-full text-sm">
							<thead className="text-[10px] uppercase tracking-wider text-ink-400 bg-white/3">
								<tr>
									<th className="text-left py-2 px-4 font-medium">{tx.columnProject}</th>
									<th className="text-right py-2 px-4 font-medium">{tx.columnSessions}</th>
									<th className="text-right py-2 px-4 font-medium">{tx.columnPrompts}</th>
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

