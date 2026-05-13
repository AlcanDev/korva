import { useState } from "react";
import { AlertCircle, Check, Download, FileText, Folder } from "lucide-react";
import { useExportObsidian } from "@/api/export";
import { useProjects } from "@/api/projects";
import {
	Badge,
	Button,
	Card,
	CardBody,
	CardHeader,
	ErrorBanner,
	PageHero,
} from "@/components/ui";
import { DonutChart, CHART_PALETTE } from "@/components/charts";

// Phase 7 — Refresh visual del Export panel. Mismo flujo (form → run →
// resultado) pero usando los primitivos del nuevo design system + un donut
// con la distribución por tipo del último export.

const OBSERVATION_TYPES = [
	"decision",
	"pattern",
	"bugfix",
	"learning",
	"context",
	"antipattern",
	"task",
	"feature",
	"refactor",
	"discovery",
	"incident",
] as const;

// Colour ramp for the donut — type → token. Order matches the design palette
// progression we use elsewhere so dashboards feel cohesive.
const TYPE_COLOR: Record<string, string> = {
	decision: CHART_PALETTE.volt,
	pattern: CHART_PALETTE.cyan,
	bugfix: CHART_PALETTE.rose,
	learning: CHART_PALETTE.indigo,
	context: CHART_PALETTE.purple,
	antipattern: CHART_PALETTE.amber,
	task: CHART_PALETTE.emerald,
	feature: CHART_PALETTE.coral,
	refactor: CHART_PALETTE.cyan,
	discovery: CHART_PALETTE.purple,
	incident: CHART_PALETTE.rose,
};

export default function ExportPanel() {
	const [out, setOut] = useState("");
	const [project, setProject] = useState("");
	const [obsType, setObsType] = useState("");

	const projects = useProjects();
	const exporter = useExportObsidian();
	const ready = out.trim() !== "";

	function submit() {
		if (!ready || exporter.isPending) return;
		exporter.mutate({
			out: out.trim(),
			project: project || undefined,
			type: obsType || undefined,
		});
	}

	return (
		<div className="p-6 max-w-5xl mx-auto space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Knowledge export"
				icon={<Download size={22} />}
				title="Obsidian export"
				subtitle={
					<>
						Render the vault as Obsidian-flavored markdown. Each observation becomes
						a note with YAML frontmatter and{" "}
						<code className="font-mono text-ink-300">[[wikilinks]]</code>. Re-running
						over the same directory is safe — notes are rewritten from the live
						store state.
					</>
				}
			/>

			{/* Form */}
			<Card>
				<CardHeader title="Export configuration" icon={<Folder size={14} />} />
				<CardBody className="space-y-4">
					<div>
						<label
							htmlFor="export-out"
							className="block text-[10px] uppercase tracking-wider text-ink-400 mb-1.5"
						>
							Output directory <span className="text-coral">*</span>
						</label>
						<input
							id="export-out"
							type="text"
							value={out}
							onChange={(e) => setOut(e.target.value)}
							placeholder="/Users/me/vaults/korva-vault"
							className="w-full bg-space-900 border border-white/10 rounded-md px-3 py-2 text-sm text-ink-100 font-mono focus:border-volt focus:outline-none focus:ring-2 focus:ring-volt/20"
						/>
						<p className="text-[11px] text-ink-500 mt-1.5 leading-relaxed">
							Path is resolved relative to the vault process. Place it under your
							Obsidian vaults folder so File → Open vault picks it up immediately.
						</p>
					</div>

					<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label
								htmlFor="export-project"
								className="block text-[10px] uppercase tracking-wider text-ink-400 mb-1.5"
							>
								Project (optional)
							</label>
							<select
								id="export-project"
								value={project}
								onChange={(e) => setProject(e.target.value)}
								className="w-full bg-space-900 border border-white/10 rounded-md px-3 py-2 text-sm text-ink-100 focus:border-volt focus:outline-none"
							>
								<option value="">All projects</option>
								{(projects.data?.projects ?? []).map((p) => (
									<option key={p.name} value={p.name}>
										{p.name} ({p.observation_count})
									</option>
								))}
							</select>
						</div>
						<div>
							<label
								htmlFor="export-type"
								className="block text-[10px] uppercase tracking-wider text-ink-400 mb-1.5"
							>
								Type (optional)
							</label>
							<select
								id="export-type"
								value={obsType}
								onChange={(e) => setObsType(e.target.value)}
								className="w-full bg-space-900 border border-white/10 rounded-md px-3 py-2 text-sm text-ink-100 focus:border-volt focus:outline-none"
							>
								<option value="">All types</option>
								{OBSERVATION_TYPES.map((t) => (
									<option key={t} value={t}>
										{t}
									</option>
								))}
							</select>
						</div>
					</div>

					<div className="flex items-center gap-2 pt-1">
						<Button
							variant="volt"
							onClick={submit}
							disabled={!ready || exporter.isPending}
							loading={exporter.isPending}
							leftIcon={<Download size={12} />}
						>
							{exporter.isPending ? "Exporting…" : "Run export"}
						</Button>
						{!ready && (
							<span className="text-[11px] text-ink-500 inline-flex items-center gap-1.5">
								<AlertCircle size={11} /> An output directory is required
							</span>
						)}
					</div>
				</CardBody>
			</Card>

			{exporter.error && (
				<ErrorBanner title="Export failed" message={String(exporter.error)} />
			)}

			{exporter.data && <ExportResultCard result={exporter.data} />}
		</div>
	);
}

function ExportResultCard({
	result,
}: {
	result: {
		out_dir: string;
		file_count: number;
		project_count: number;
		by_type: Record<string, number>;
		by_project?: Record<string, number>;
		generated_at: string;
	};
}) {
	const donutData = Object.entries(result.by_type)
		.map(([label, value]) => ({
			label,
			value,
			color: TYPE_COLOR[label] ?? CHART_PALETTE.indigo,
		}))
		.sort((a, b) => b.value - a.value);

	return (
		<Card tone="volt" variant="glass">
			<CardHeader
				icon={<Check size={14} className="text-volt" />}
				title="Export written"
				subtitle={
					<span className="font-mono text-ink-300">{result.out_dir}</span>
				}
				actions={
					<>
						<Badge tone="success" mono>
							{result.file_count} files
						</Badge>
						<Badge tone="cyan" mono>
							{result.project_count} projects
						</Badge>
					</>
				}
			/>
			<CardBody className="grid grid-cols-1 md:grid-cols-[1fr_240px] gap-4 items-start">
				<div>
					<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-2">
						Output breakdown
					</p>
					<DonutChart
						data={donutData}
						centerLabel="Files"
						centerValue={result.file_count}
						stroke={18}
						size={140}
					/>
				</div>
				<div className="text-xs space-y-3">
					<div>
						<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1">
							Generated at
						</p>
						<p className="font-mono text-ink-200">
							{new Date(result.generated_at)
								.toISOString()
								.replace("T", " ")
								.slice(0, 19)}
						</p>
					</div>
					<div>
						<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1.5">
							Next step
						</p>
						<p className="text-ink-300 leading-relaxed">
							Open the folder in Obsidian via{" "}
							<span className="text-ink-100">File → Open vault</span> and start
							browsing the wikilinks under each project's{" "}
							<code className="font-mono">_index.md</code>.
						</p>
					</div>
					<div className="flex items-center gap-1.5 flex-wrap pt-1">
						{donutData.slice(0, 6).map((d) => (
							<span
								key={d.label}
								className="text-[10px] font-mono inline-flex items-center gap-1"
							>
								<FileText size={9} className="text-ink-400" />
								<span className="text-ink-300">
									{d.label}: {d.value}
								</span>
							</span>
						))}
					</div>
				</div>
			</CardBody>
		</Card>
	);
}
