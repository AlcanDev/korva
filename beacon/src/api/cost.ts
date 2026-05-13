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
