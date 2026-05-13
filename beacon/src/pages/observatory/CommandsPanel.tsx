import { useMemo, useState } from "react";
import {
	Terminal as TerminalIcon,
	Play,
	Activity,
	HeartPulse,
	Cloud,
	FolderTree,
	Settings,
	Tag,
	Wand2,
	AlertCircle,
} from "lucide-react";
import { useCommandList, useRunCommand, type CommandListEntry } from "@/api/commands";
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
	StatusDot,
} from "@/components/ui";

// Phase 7 — UI sobre /admin/commands*. Catálogo de comandos seguros
// (whitelisteados en el backend) que el operador puede correr con 1 click,
// con la salida formateada en una terminal estilizada.
//
// Solo trabaja cuando el endpoint reporta `local_only: true` — es decir,
// cuando el vault está escuchando en 127.0.0.1. En cualquier otro host la
// pantalla queda en "disabled" para evitar la falsa expectativa de poder
// disparar procesos remotos.

// Icon por slug — mantiene los botones visualmente distintos sin pelearse
// con la whitelist del backend (cualquier slug nuevo cae al ícono default).
const ICON_BY_SLUG: Record<string, React.ReactNode> = {
	status: <Activity size={14} />,
	doctor: <HeartPulse size={14} />,
	"hive-status": <Cloud size={14} />,
	"projects-list": <FolderTree size={14} />,
	"projects-suggest": <Wand2 size={14} />,
	"config-show": <Settings size={14} />,
	version: <Tag size={14} />,
};

export default function CommandsPanel() {
	const { data, isLoading, error } = useCommandList();
	const run = useRunCommand();
	const [selectedSlug, setSelectedSlug] = useState<string | null>(null);

	const selected = useMemo(
		() => data?.commands.find((c) => c.slug === selectedSlug) ?? null,
		[data, selectedSlug],
	);

	const localOnly = data?.local_only ?? false;

	return (
		<div className="p-6 max-w-7xl mx-auto space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Operator console"
				icon={<TerminalIcon size={22} />}
				title="Commands"
				subtitle={
					<>
						Run curated <code className="font-mono text-ink-300">korva</code> CLI commands
						without leaving the dashboard. The backend whitelist is the only entry
						point — no shell access, no arbitrary input.
					</>
				}
				badge={{
					tone: localOnly ? "success" : "danger",
					label: localOnly ? "Local vault" : "Remote vault — disabled",
				}}
			/>

			{error && (
				<ErrorBanner
					title="Couldn't load the command catalogue"
					message={String(error)}
				/>
			)}

			{!localOnly && data && (
				<Card variant="elevated" className="p-5">
					<div className="flex items-start gap-3">
						<AlertCircle size={18} className="text-amber-400 shrink-0 mt-0.5" />
						<div>
							<p className="text-sm font-medium text-ink-100 mb-1">
								This vault is not bound to localhost
							</p>
							<p className="text-xs text-ink-400 leading-relaxed">
								For safety, the command runner refuses to spawn processes when the
								dashboard is reached through a non-loopback host. To enable it,
								access the dashboard via{" "}
								<code className="font-mono text-ink-300">http://127.0.0.1:7437</code>{" "}
								(or wherever your vault is bound locally).
							</p>
						</div>
					</div>
				</Card>
			)}

			<div className="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-5">
				{/* Catalogue */}
				<Card variant="default">
					<CardHeader title="Catalogue" subtitle="Click a command to run it." />
					<CardBody className="!p-2">
						{isLoading ? (
							<div className="space-y-2">
								<Skeleton height={42} />
								<Skeleton height={42} />
								<Skeleton height={42} />
							</div>
						) : !data || data.commands.length === 0 ? (
							<EmptyState title="No commands available" />
						) : (
							<ul className="space-y-1">
								{data.commands.map((cmd) => (
									<CommandRow
										key={cmd.slug}
										command={cmd}
										active={cmd.slug === selectedSlug}
										disabled={!localOnly}
										running={run.isPending && run.variables === cmd.slug}
										onSelect={() => {
											setSelectedSlug(cmd.slug);
											if (localOnly) run.mutate(cmd.slug);
										}}
									/>
								))}
							</ul>
						)}
					</CardBody>
				</Card>

				{/* Output */}
				<Card variant="default" className="overflow-hidden">
					<CardHeader
						title={selected ? `Output — ${selected.slug}` : "Output"}
						subtitle={
							selected ? (
								<code className="font-mono text-ink-300">{selected.argv}</code>
							) : (
								"Pick a command on the left to see its captured stdout/stderr here."
							)
						}
						icon={<TerminalIcon size={14} />}
						actions={
							selected && localOnly ? (
								<Button
									size="sm"
									variant="ghost"
									leftIcon={<Play size={12} />}
									loading={run.isPending}
									onClick={() => run.mutate(selected.slug)}
								>
									Re-run
								</Button>
							) : null
						}
					/>
					<CommandOutput
						loading={run.isPending}
						result={run.data ?? null}
						error={run.error}
						hasSelection={Boolean(selected)}
					/>
				</Card>
			</div>
		</div>
	);
}

// ── Catalogue row ──────────────────────────────────────────────────────────

function CommandRow({
	command,
	active,
	disabled,
	running,
	onSelect,
}: {
	command: CommandListEntry;
	active: boolean;
	disabled: boolean;
	running: boolean;
	onSelect: () => void;
}) {
	return (
		<li>
			<button
				type="button"
				onClick={onSelect}
				disabled={disabled}
				aria-current={active}
				className={`w-full text-left rounded-md px-3 py-2.5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
					active
						? "bg-volt-dim border border-volt/30"
						: "border border-transparent hover:bg-white/3 hover:border-white/10"
				}`}
			>
				<div className="flex items-center gap-2.5">
					<span
						className={`shrink-0 ${active ? "text-volt" : "text-ink-400"}`}
					>
						{ICON_BY_SLUG[command.slug] ?? <Play size={14} />}
					</span>
					<div className="flex-1 min-w-0">
						<p className="text-sm text-ink-100 truncate">{command.description}</p>
						<p className="text-[10px] font-mono text-ink-400 truncate">
							{command.argv}
						</p>
					</div>
					{running ? <StatusDot state="info" pulse /> : null}
				</div>
			</button>
		</li>
	);
}

// ── Output panel ───────────────────────────────────────────────────────────

function CommandOutput({
	loading,
	result,
	error,
	hasSelection,
}: {
	loading: boolean;
	result: import("@/api/commands").CommandRunResponse | null;
	error: unknown;
	hasSelection: boolean;
}) {
	if (loading) {
		return (
			<div className="p-5">
				<Skeleton height={14} className="mb-2" />
				<Skeleton height={14} width="90%" className="mb-2" />
				<Skeleton height={14} width="75%" className="mb-2" />
				<Skeleton height={14} width="60%" />
			</div>
		);
	}
	if (error) {
		return (
			<div className="p-5">
				<ErrorBanner title="Command failed to start" message={String(error)} />
			</div>
		);
	}
	if (!result) {
		return (
			<div className="p-8 text-center">
				<p className="text-sm text-ink-400">
					{hasSelection ? "Press Re-run to execute." : "No output yet."}
				</p>
			</div>
		);
	}
	const ok = result.exit_code === 0 && !result.timed_out;
	return (
		<div className="p-4 space-y-3">
			<div className="flex items-center gap-2 flex-wrap text-[11px]">
				<Badge tone={ok ? "success" : result.timed_out ? "warning" : "danger"} mono>
					{ok ? "exit 0" : result.timed_out ? "TIMED OUT" : `exit ${result.exit_code}`}
				</Badge>
				<Badge tone="info" mono>
					{result.duration_ms}ms
				</Badge>
				{result.truncated && (
					<Badge tone="warning" mono>
						OUTPUT TRUNCATED
					</Badge>
				)}
			</div>
			<pre className="terminal terminal-body whitespace-pre-wrap break-words text-[12.5px] max-h-[480px] overflow-auto">
				{result.stdout || (
					<span className="text-ink-500 italic">(no stdout)</span>
				)}
				{result.stderr && (
					<>
						{"\n\n"}
						<span className="text-[#FF6B6B]">{result.stderr}</span>
					</>
				)}
			</pre>
		</div>
	);
}
