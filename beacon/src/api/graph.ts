import { useQuery } from "@tanstack/react-query";
import { adminFetch } from "./_fetch";

// Phase 9.3 — wrapper TanStack Query para /admin/graph.

export interface GraphNode {
	id: string;
	label: string;
	type: string;
	project: string;
	topic_key?: string;
}

export interface GraphEdge {
	source: string;
	target: string;
	relation: string;
	confidence: number;
}

export interface GraphResponse {
	project: string;
	nodes: GraphNode[];
	edges: GraphEdge[];
	truncated: boolean;
}

export function useKnowledgeGraph(project: string, limit = 150) {
	return useQuery({
		queryKey: ["graph", project, limit],
		queryFn: () =>
			adminFetch<GraphResponse>(
				`/admin/graph?project=${encodeURIComponent(project)}&limit=${limit}`,
			),
		enabled: Boolean(project),
	});
}
