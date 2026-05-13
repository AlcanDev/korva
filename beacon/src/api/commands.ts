import { useMutation, useQuery } from "@tanstack/react-query";
import { adminFetch, adminPost } from "./_fetch";

// Phase 7 — wrappers TanStack Query para /admin/commands*.
//
// GET  /admin/commands           lista whitelist + flag local_only
// POST /admin/commands/run       body {command: slug} → stdout/stderr/exit

export interface CommandListEntry {
	slug: string;
	description: string;
	argv: string;
}

export interface CommandListResponse {
	commands: CommandListEntry[];
	local_only: boolean;
}

export interface CommandRunResponse {
	slug: string;
	argv: string;
	exit_code: number;
	stdout: string;
	stderr: string;
	duration_ms: number;
	timed_out: boolean;
	truncated: boolean;
}

export function useCommandList() {
	return useQuery({
		queryKey: ["commands", "list"],
		queryFn: () => adminFetch<CommandListResponse>("/admin/commands"),
		staleTime: 5 * 60 * 1000, // whitelist barely moves
	});
}

export function useRunCommand() {
	return useMutation({
		mutationFn: (slug: string) =>
			adminPost<CommandRunResponse>("/admin/commands/run", { command: slug }),
	});
}
