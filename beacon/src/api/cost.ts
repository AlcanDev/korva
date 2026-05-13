import { useQuery } from "@tanstack/react-query";
import { adminFetch } from "./_fetch";

// Phase 8.6 — wrapper TanStack Query para /admin/cost/summary.

export interface CostBucket {
	name: string;
	family?: string;
	input_tokens: number;
	output_tokens: number;
	cache_read: number;
	count: number;
	cost_usd: number;
}

export interface DailyCost {
	date: string;
	tokens: number;
	cost_usd: number;
}

export interface CostSummary {
	window_days: number;
	from: string;
	to: string;
	total_usd: number;
	total_tokens: number;
	input_tokens: number;
	output_tokens: number;
	cache_read: number;
	cache_hit_pct: number;
	savings_usd: number;
	reduction_pct: number;
	by_model: CostBucket[];
	by_project: CostBucket[];
	daily: DailyCost[];
	interactions_count: number;
}

export function useCostSummary(days = 30) {
	return useQuery({
		queryKey: ["cost", "summary", days],
		queryFn: () => adminFetch<CostSummary>(`/admin/cost/summary?days=${days}`),
		refetchInterval: 60_000,
	});
}

// Phase 9.2 — anomaly detector hook.

export interface Anomaly {
	kind: "daily_spike" | "project_spike";
	subject: string;
	tokens: number;
	baseline_avg: number;
	baseline_std: number;
	z_score: number;
	severity: "warning" | "danger";
	suggestion: string;
}

export interface AnomaliesResponse {
	window_days: number;
	anomalies: Anomaly[];
}

export function useCostAnomalies(days = 30) {
	return useQuery({
		queryKey: ["cost", "anomalies", days],
		queryFn: () =>
			adminFetch<AnomaliesResponse>(`/admin/cost/anomalies?days=${days}`),
		refetchInterval: 60_000,
	});
}
