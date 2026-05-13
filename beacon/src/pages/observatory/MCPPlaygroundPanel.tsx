import { useMemo, useState } from "react";
import { Play, Plug, Code2 } from "lucide-react";
import { useMCPTools, useMCPInvoke, type MCPTool } from "@/api/mcp";
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

// Phase 10.2 — MCP playground.
//
// Lets devs (and curious operators) invoke read-only MCP tools from the
// dashboard. Picks a tool from a dropdown, fills its JSON args in a
// textarea (with a "generate template from schema" helper), hits Run, and
// sees the raw JSON response.
//
// Critical safety contract: hard-capped to the Readonly profile server-side.
// Even if the UI lets you type vault_save into the textarea, the backend
// will refuse — verified by the playground test suite.

export default function MCPPlaygroundPanel() {
	const { data, isLoading, error } = useMCPTools();
	const invoke = useMCPInvoke();
	const [selectedName, setSelectedName] = useState<string>("");
	const [argsText, setArgsText] = useState<string>("{}");
	const [parseError, setParseError] = useState<string | null>(null);

	const tools = data?.tools ?? [];
	const selected = useMemo(
		() => tools.find((t) => t.name === selectedName) ?? null,
		[tools, selectedName],
	);

	// Auto-select the first tool on load.
	if (!selectedName && tools.length > 0) {
		setSelectedName(tools[0].name);
	}

	function fillTemplate(tool: MCPTool) {
		const template: Record<string, unknown> = {};
		// Required first, then nudge a sensible default per type.
		for (const req of tool.input_schema.required ?? []) {
			const prop = tool.input_schema.properties[req];
			template[req] = defaultForType(prop?.type ?? "string");
		}
		setArgsText(JSON.stringify(template, null, 2));
		setParseError(null);
	}

	function run() {
		if (!selected) return;
		setParseError(null);
		let args: Record<string, unknown>;
		try {
			args = argsText.trim() ? JSON.parse(argsText) : {};
		} catch (e) {
			setParseError(String(e));
			return;
		}
		invoke.mutate({ tool: selected.name, args });
	}

	return (
		<div className="p-4 sm:p-6 max-w-6xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Developer console"
				icon={<Plug size={22} />}
				title="MCP playground"
				subtitle="Invoke read-only MCP tools from the dashboard. Same surface your AI editor talks to — no terminal needed."
				badge={{ tone: "info", label: data?.profile ?? "readonly" }}
			/>

			{error ? <ErrorBanner title="Couldn't load MCP tools" message={String(error)} /> : null}

			{isLoading ? (
				<Skeleton height={400} />
			) : (
				<div className="grid grid-cols-1 lg:grid-cols-[320px_1fr] gap-4">
					{/* Tool list */}
					<Card>
						<CardHeader title="Tools" subtitle={`${tools.length} available`} />
						<CardBody className="!p-2">
							{tools.length === 0 ? (
								<EmptyState
									tone="cyan"
									icon={<Plug size={20} />}
									title="No tools exposed"
									compact
								/>
							) : (
								<ul className="space-y-1">
									{tools.map((t) => (
										<li key={t.name}>
											<button
												type="button"
												onClick={() => setSelectedName(t.name)}
												aria-current={t.name === selectedName}
												className={`w-full text-left rounded-md px-3 py-2 text-xs transition-colors ${
													t.name === selectedName
														? "bg-cyan-400/10 border border-cyan-400/30"
														: "border border-transparent hover:bg-white/3 hover:border-white/10"
												}`}
											>
												<p className="font-mono text-ink-100">{t.name}</p>
												<p className="text-[10px] text-ink-400 mt-0.5 line-clamp-2">
													{t.description}
												</p>
											</button>
										</li>
									))}
								</ul>
							)}
						</CardBody>
					</Card>

					{/* Args + result */}
					<div className="space-y-4">
						<Card>
							<CardHeader
								title={selected ? selected.name : "Pick a tool"}
								subtitle={
									selected ? (
										<span className="text-ink-300">{selected.description}</span>
									) : (
										"Choose a tool on the left and Korva will show its JSON schema here."
									)
								}
								icon={<Code2 size={14} />}
								actions={
									selected ? (
										<>
											<Button
												size="sm"
												variant="ghost"
												onClick={() => fillTemplate(selected)}
											>
												Fill template
											</Button>
											<Button
												size="sm"
												variant="volt"
												leftIcon={<Play size={12} />}
												onClick={run}
												loading={invoke.isPending}
											>
												{invoke.isPending ? "Running…" : "Run"}
											</Button>
										</>
									) : null
								}
							/>
							<CardBody className="space-y-3">
								{selected ? (
									<div className="space-y-3">
										<div>
											<p className="text-[10px] uppercase tracking-wider text-ink-400 mb-1.5">
												Args (JSON)
											</p>
											<textarea
												value={argsText}
												onChange={(e) => setArgsText(e.target.value)}
												spellCheck={false}
												rows={8}
												className="w-full bg-space-900 border border-white/10 rounded-md px-3 py-2 text-xs text-ink-100 font-mono focus:border-volt focus:outline-none"
												placeholder='{"project": "korva", "q": "ULID"}'
											/>
										</div>
										{selected.input_schema?.required &&
										selected.input_schema.required.length > 0 ? (
											<p className="text-[11px] text-ink-400">
												Required:{" "}
												<span className="font-mono text-ink-200">
													{selected.input_schema.required.join(", ")}
												</span>
											</p>
										) : null}
										<SchemaSummary tool={selected} />
									</div>
								) : (
									<p className="text-xs text-ink-400">Nothing selected yet.</p>
								)}
								{parseError ? (
									<ErrorBanner title="JSON parse error" message={parseError} />
								) : null}
								{invoke.error ? (
									<ErrorBanner title="Tool failed" message={String(invoke.error)} />
								) : null}
							</CardBody>
						</Card>

						{invoke.data ? (
							<Card>
								<CardHeader
									title="Result"
									icon={<Code2 size={14} />}
									actions={<Badge tone="success" mono>OK</Badge>}
								/>
								<CardBody>
									<pre className="bg-space-900 border border-white/8 rounded-md p-3 text-[11px] font-mono text-ink-200 overflow-auto max-h-[440px] leading-relaxed">
										{JSON.stringify(invoke.data.result, null, 2)}
									</pre>
								</CardBody>
							</Card>
						) : null}
					</div>
				</div>
			)}
		</div>
	);
}

function SchemaSummary({ tool }: { tool: MCPTool }) {
	const props = Object.entries(tool.input_schema?.properties ?? {});
	if (props.length === 0) return null;
	return (
		<details className="text-xs">
			<summary className="cursor-pointer text-ink-300 hover:text-ink-100 select-none">
				Show full schema ({props.length} props)
			</summary>
			<dl className="mt-2 space-y-1.5">
				{props.map(([name, p]) => (
					<div key={name} className="grid grid-cols-[120px_1fr] gap-2 items-start">
						<dt className="font-mono text-ink-200">{name}</dt>
						<dd className="text-ink-400 leading-snug">
							<span className="font-mono text-cyan-300 mr-2">{p.type}</span>
							{p.description ?? ""}
							{p.enum && p.enum.length > 0 ? (
								<span className="block mt-0.5 text-[10px] font-mono text-ink-500">
									enum: {p.enum.join(", ")}
								</span>
							) : null}
						</dd>
					</div>
				))}
			</dl>
		</details>
	);
}

function defaultForType(t: string): unknown {
	switch (t) {
		case "string":
			return "";
		case "number":
		case "integer":
			return 0;
		case "boolean":
			return false;
		case "array":
			return [];
		case "object":
			return {};
		default:
			return null;
	}
}
