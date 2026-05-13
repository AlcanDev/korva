import { useMutation, useQuery } from "@tanstack/react-query";
import { adminFetch, adminPost } from "./_fetch";

// Phase 10.2 — wrapper TanStack Query para /admin/mcp/*.

export interface MCPToolProp {
	type: string;
	description?: string;
	enum?: string[];
}

export interface MCPToolSchema {
	type: string;
	properties: Record<string, MCPToolProp>;
	required?: string[];
}

export interface MCPTool {
	name: string;
	description: string;
	input_schema: MCPToolSchema;
}

export interface MCPToolsResponse {
	tools: MCPTool[];
	profile: string;
}

export interface MCPInvokeResponse {
	tool: string;
	result: unknown;
}

export function useMCPTools() {
	return useQuery({
		queryKey: ["mcp", "tools"],
		queryFn: () => adminFetch<MCPToolsResponse>("/admin/mcp/tools"),
		staleTime: 5 * 60 * 1000,
	});
}

export function useMCPInvoke() {
	return useMutation({
		mutationFn: (body: { tool: string; args: Record<string, unknown> }) =>
			adminPost<MCPInvokeResponse>("/admin/mcp/invoke", body),
	});
}
