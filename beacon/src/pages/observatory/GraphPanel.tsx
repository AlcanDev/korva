import { useMemo, useState } from "react";
import { Network, RefreshCw } from "lucide-react";
import { useKnowledgeGraph } from "@/api/graph";
import { useProjects } from "@/api/projects";
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
import { KnowledgeGraph } from "@/components/charts";

// Phase 9.3 — Knowledge Graph panel.
//
// The visceral differentiator. Nobody else in this space surfaces the
// relationships between decisions / patterns / bugfixes as a navigable
// graph. The operator picks a project, sees the cluster of observations,
// the lines connecting them, and can click a node to read details.
//
// The force-directed simulation runs in the browser (no D3), capping at
// ~150 nodes per project so it stays snappy. The backend filters dangling
// edges so we never paint a line to a node that wasn't returned.

export default function GraphPanel() {
	const projects = useProjects();
	const [project, setProject] = useState<string>("");
	// Default-select the first project once the list loads.
	const firstProject = projects.data?.projects?.[0]?.name;
	if (!project && firstProject) {
		setProject(firstProject);
	}

	const graph = useKnowledgeGraph(project);

	const counts = useMemo(() => {
		if (!graph.data) return null;
		const byType = new Map<string, number>();
		for (const n of graph.data.nodes) {
			byType.set(n.type, (byType.get(n.type) ?? 0) + 1);
		}
		return Array.from(byType.entries()).sort((a, b) => b[1] - a[1]);
	}, [graph.data]);

	return (
		<div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Knowledge map"
				icon={<Network size={22} />}
				title="Knowledge graph"
				subtitle="Every observation is a node. Every relation (supersedes, conflicts, related, scoped, compatible) is an edge. Pick a project and see how your team's decisions cluster."
				badge={
					graph.data?.truncated
						? { tone: "warning", label: `Showing first ${graph.data.nodes.length}` }
						: undefined
				}
				actions={
					project ? (
						<Button
							variant="ghost"
							size="sm"
							leftIcon={<RefreshCw size={11} className={graph.isFetching ? "animate-spin" : ""} />}
							onClick={() => graph.refetch()}
							disabled={graph.isFetching}
						>
							Refresh
						</Button>
					) : null
				}
			/>

			{/* Project picker */}
			<Card>
				<CardBody>
					<div className="flex items-center gap-3 flex-wrap">
						<label htmlFor="graph-project" className="text-[10px] uppercase tracking-wider text-ink-400">
							Project
						</label>
						<select
							id="graph-project"
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
						{counts && counts.length > 0 && (
							<div className="flex items-center gap-1.5 flex-wrap">
								{counts.map(([type, count]) => (
									<Badge key={type} tone="neutral" mono>
										{type}: {count}
									</Badge>
								))}
							</div>
						)}
					</div>
				</CardBody>
			</Card>

			{graph.error ? <ErrorBanner title="Couldn't load graph" message={String(graph.error)} /> : null}

			<Card>
				<CardHeader
					title={project ? `${project} — ${graph.data?.nodes.length ?? 0} nodes, ${graph.data?.edges.length ?? 0} edges` : "Pick a project"}
					subtitle="Drag-free force-directed layout. Hover a node to see its title; click to open."
				/>
				<CardBody className="!p-2">
					{graph.isLoading ? (
						<Skeleton height={500} />
					) : !project ? (
						<EmptyState
							tone="cyan"
							icon={<Network size={22} />}
							title="Select a project"
							description="The graph populates with every observation in the project and the relations between them."
						/>
					) : !graph.data || graph.data.nodes.length === 0 ? (
						<EmptyState
							tone="purple"
							icon={<Network size={22} />}
							title="No observations in this project yet"
							description="Save observations from any AI editor to start populating the graph. Conflicts and supersedes relations appear automatically."
							hint="vault_save → relations form"
						/>
					) : (
						<KnowledgeGraph
							nodes={graph.data.nodes}
							edges={graph.data.edges}
							height={520}
						/>
					)}
				</CardBody>
			</Card>
		</div>
	);
}
