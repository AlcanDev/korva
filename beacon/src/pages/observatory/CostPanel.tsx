import { useMemo, useState } from "react";
import { DollarSign, TrendingDown, Coins, Sparkles, Cpu } from "lucide-react";
import { useCostSummary } from "@/api/cost";
import {
	Badge,
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
import { BarChart, CHART_PALETTE, DonutChart, LineChart, Sparkline } from "@/components/charts";

// Phase 8.6 — Cost & ROI dashboard. Designed for the CFO/CTO conversation:
//
//   - Hero metric: USD spent in window
//   - Beside it: tokens, cache-hit %, savings (cache rewriting paid off)
//   - Daily line of USD spent (Korva's actual cost curve)
//   - Donut by model (which family takes the biggest slice)
//   - Bar by project (which team is consuming)
//
// Every chart pulls from one endpoint so the panel renders in <300 ms even
// with months of data buffered.

type WindowDays = "7" | "30" | "90";

const TONE_BY_MODEL_FAMILY: Record<string, string> = {
	"Anthropic Claude Opus": CHART_PALETTE.purple,
	"Anthropic Claude 3 Opus": CHART_PALETTE.purple,
	"Anthropic Claude Sonnet 4": CHART_PALETTE.cyan,
	"Anthropic Claude 3.5 Sonnet": CHART_PALETTE.cyan,
	"Anthropic Claude 3 Haiku": CHART_PALETTE.amber,
	"OpenAI GPT-4o": CHART_PALETTE.emerald,
	"OpenAI GPT-4o mini": CHART_PALETTE.volt,
	"OpenAI GPT-4.1": CHART_PALETTE.emerald,
	"Google Gemini 2.0 Flash": CHART_PALETTE.coral,
	"Google Gemini 1.5 Pro": CHART_PALETTE.rose,
};

export default function CostPanel() {
	const [days, setDays] = useState<WindowDays>("30");
	const { data, isLoading, error } = useCostSummary(Number(days));

	const dailySeries = useMemo(() => {
		if (!data?.daily?.length) return null;
		const labels = data.daily.map((d) => d.date.slice(5)); // MM-DD
		return {
			labels,
			series: [
				{
					name: "USD",
					color: CHART_PALETTE.volt,
					data: data.daily.map((d) => Number(d.cost_usd.toFixed(2))),
				},
			],
			sparkData: data.daily.map((d) => d.cost_usd),
		};
	}, [data]);

	const byModelDonut = useMemo(() => {
		if (!data?.by_model?.length) return null;
		return data.by_model
			.filter((b) => b.cost_usd > 0)
			.sort((a, b) => b.cost_usd - a.cost_usd)
			.slice(0, 6)
			.map((b) => ({
				label: b.family ?? b.name,
				value: Number(b.cost_usd.toFixed(4)),
				color: TONE_BY_MODEL_FAMILY[b.family ?? ""] ?? CHART_PALETTE.indigo,
			}));
	}, [data]);

	const byProjectBar = useMemo(() => {
		if (!data?.by_project?.length) return null;
		return data.by_project
			.sort((a, b) => b.cost_usd - a.cost_usd)
			.map((b) => ({
				label: b.name,
				value: Number(b.cost_usd.toFixed(2)),
			}));
	}, [data]);

	return (
		<div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-4 sm:space-y-5 animate-fade-up">
			<PageHero
				eyebrow="Cost & ROI"
				icon={<DollarSign size={22} />}
				title="Cost & ROI"
				subtitle="What your AI actually costs — by model, by project, by day. The savings line shows what Korva's caching paid back."
				actions={
					<Tabs<WindowDays>
						variant="pill"
						value={days}
						onChange={setDays}
						tabs={[
							{ value: "7", label: "7d" },
							{ value: "30", label: "30d" },
							{ value: "90", label: "90d" },
						]}
					/>
				}
			/>

			{error ? <ErrorBanner title="Couldn't load cost summary" message={String(error)} /> : null}

			{isLoading ? (
				<div className="grid grid-cols-2 md:grid-cols-4 gap-3">
					<Skeleton height={108} />
					<Skeleton height={108} />
					<Skeleton height={108} />
					<Skeleton height={108} />
				</div>
			) : data ? (
				<>
					{/* Hero metric strip */}
					<div className="grid grid-cols-2 md:grid-cols-4 gap-3">
						<MetricCard
							label="Spent"
							value={`$${data.total_usd.toFixed(2)}`}
							hint={`last ${data.window_days} days`}
							tone="volt"
							icon={<DollarSign size={14} />}
							sparkline={
								dailySeries?.sparkData ? (
									<Sparkline data={dailySeries.sparkData} color="var(--color-volt)" />
								) : null
							}
						/>
						<MetricCard
							label="Tokens"
							value={Intl.NumberFormat().format(data.total_tokens)}
							hint={`${Intl.NumberFormat().format(data.interactions_count)} calls`}
							tone="cyan"
							icon={<Coins size={14} />}
						/>
						<MetricCard
							label="Cache hit"
							value={`${(data.cache_hit_pct * 100).toFixed(1)}%`}
							hint={`${Intl.NumberFormat().format(data.cache_read)} read`}
							tone="purple"
							icon={<Sparkles size={14} />}
						/>
						<MetricCard
							label="Savings"
							value={`$${data.savings_usd.toFixed(2)}`}
							hint="vs. uncached input"
							tone="coral"
							icon={<TrendingDown size={14} />}
						/>
					</div>

					{/* Daily curve */}
					<Card>
						<CardHeader
							title="USD spent per day"
							subtitle="A flat line is healthy; a slope to the right is your AI bill growing."
							icon={<DollarSign size={14} />}
						/>
						<CardBody>
							{dailySeries ? (
								<LineChart
									xLabels={dailySeries.labels}
									series={dailySeries.series}
									height={220}
									yFormatter={(n) => `$${n.toFixed(2)}`}
								/>
							) : (
								<EmptyState
									tone="cyan"
									icon={<DollarSign size={22} />}
									title="No spend yet"
									description="Once interactions land in this window the daily cost curve will appear here."
									hint="Run an AI session to populate"
								/>
							)}
						</CardBody>
					</Card>

					{/* Distribution by model + by project */}
					<div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
						<Card>
							<CardHeader title="By model" subtitle="Where the dollars go" icon={<Cpu size={14} />} />
							<CardBody>
								{byModelDonut?.length ? (
									<DonutChart
										data={byModelDonut}
										centerLabel="USD"
										centerValue={`$${data.total_usd.toFixed(2)}`}
										stroke={20}
										size={160}
									/>
								) : (
									<EmptyState
										tone="purple"
										icon={<Cpu size={22} />}
										title="No model usage in window"
										compact
									/>
								)}
							</CardBody>
						</Card>

						<Card>
							<CardHeader title="By project" subtitle="Top spenders" />
							<CardBody>
								{byProjectBar?.length ? (
									<BarChart
										data={byProjectBar}
										maxRows={8}
										formatValue={(n) => `$${n.toFixed(2)}`}
										emptyMessage="No per-project breakdown yet."
									/>
								) : (
									<EmptyState
										tone="coral"
										icon={<DollarSign size={22} />}
										title="No project breakdown"
										compact
									/>
								)}
							</CardBody>
						</Card>
					</div>

					<p className="text-[11px] text-ink-500 text-center">
						Prices are best-effort estimates from public pricing pages. Run{" "}
						<code className="font-mono text-ink-300">korva config show</code> if you need exact
						invoice-grade figures. <Badge tone="neutral" mono>estimated</Badge>
					</p>
				</>
			) : null}
		</div>
	);
}
