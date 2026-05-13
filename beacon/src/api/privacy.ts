import { useQuery } from "@tanstack/react-query";
import { adminFetch } from "./_fetch";

// Phase 9.1 — wrapper TanStack Query para /admin/privacy/stats.

export type RedactionType =
	| "password"
	| "token"
	| "secret"
	| "api_key"
	| "private_key"
	| "client_secret"
	| "vault_role_id"
	| "vault_secret_id"
	| "bearer_token"
	| "private_tag"
	| "custom_keyword";

export interface PrivacyStats {
	total_events: number;
	total_chars_removed: number;
	by_type: Partial<Record<RedactionType, number>>;
	since: string;
	since_unix: number;
}

export function usePrivacyStats() {
	return useQuery({
		queryKey: ["privacy", "stats"],
		queryFn: () => adminFetch<PrivacyStats>("/admin/privacy/stats"),
		refetchInterval: 5_000, // pulse: every 5 s
	});
}
