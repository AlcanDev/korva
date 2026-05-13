import { useEffect, useMemo, useRef, useState } from "react";

// Phase 9.3 — Force-directed knowledge graph in pure SVG.
//
// No D3, no react-flow, no extra dependency. The implementation:
//   - Random initial positions inside a viewBox
//   - Each frame applies repulsive force between every pair of nodes
//     (Coulomb-like) + attractive force along each edge (Hooke-like)
//   - Velocity is damped each frame so the simulation converges
//   - requestAnimationFrame loop; bails out once kinetic energy is
//     under a tiny threshold (no CPU wasted once stable)
//
// Designed for graphs up to ~200 nodes. Above that we'd want a quadtree
// (Barnes-Hut) or a server-precomputed layout; the current store cap is
// 500 anyway so this is fine.

export interface KnowledgeGraphNode {
	id: string;
	label: string;
	type: string;
}

export interface KnowledgeGraphEdge {
	source: string;
	target: string;
	relation: string;
}

export interface KnowledgeGraphProps {
	nodes: KnowledgeGraphNode[];
	edges: KnowledgeGraphEdge[];
	height?: number;
	onSelect?: (node: KnowledgeGraphNode) => void;
	colorByType?: Record<string, string>;
	className?: string;
}

interface SimNode extends KnowledgeGraphNode {
	x: number;
	y: number;
	vx: number;
	vy: number;
}

const VIEW_W = 800;
const VIEW_H = 500;
const RADIUS = 7;
// Tuned for a 800×500 canvas + ≤ 150 nodes.
const REPULSION = 800;
const SPRING = 0.04;
const SPRING_LENGTH = 90;
const CENTER_GRAVITY = 0.002;
const DAMPING = 0.85;
const MIN_KINETIC = 0.01; // stop the loop when this low
const MAX_FRAMES = 600;   // safety cap

const DEFAULT_COLOR: Record<string, string> = {
	decision: "var(--color-cyan-400)",
	pattern: "var(--color-volt)",
	bugfix: "var(--color-rose-400)",
	learning: "var(--color-purple-400)",
	context: "var(--color-amber-400)",
	antipattern: "var(--color-rose-500)",
	task: "var(--color-emerald-400)",
	feature: "var(--color-coral)",
	refactor: "var(--color-cyan-300)",
	discovery: "var(--color-purple-400)",
	incident: "var(--color-rose-500)",
};

export function KnowledgeGraph({
	nodes,
	edges,
	height = 500,
	onSelect,
	colorByType,
	className = "",
}: KnowledgeGraphProps) {
	const palette = useMemo(
		() => ({ ...DEFAULT_COLOR, ...(colorByType ?? {}) }),
		[colorByType],
	);

	// Build the sim node list lazily, reseed positions when the node id set
	// changes. We compare ids (stringified) instead of length so re-orders
	// don't trigger a full reset.
	const nodeIdsKey = useMemo(() => nodes.map((n) => n.id).join("|"), [nodes]);
	const [sim, setSim] = useState<SimNode[]>(() => seed(nodes));
	const [hoverId, setHoverId] = useState<string | null>(null);
	const rafRef = useRef<number | null>(null);
	const framesRef = useRef(0);

	useEffect(() => {
		setSim(seed(nodes));
		framesRef.current = 0;
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [nodeIdsKey]);

	// Index edges by source id for quick force lookup.
	const adjacency = useMemo(() => {
		const a = new Map<string, string[]>();
		for (const e of edges) {
			a.set(e.source, [...(a.get(e.source) ?? []), e.target]);
			a.set(e.target, [...(a.get(e.target) ?? []), e.source]);
		}
		return a;
	}, [edges]);

	// Simulation loop.
	useEffect(() => {
		if (sim.length === 0) return;
		let cancelled = false;
		function step() {
			if (cancelled) return;
			framesRef.current += 1;
			let ke = 0;
			setSim((prev) => {
				const next = prev.map((n) => ({ ...n }));
				for (let i = 0; i < next.length; i++) {
					let fx = 0;
					let fy = 0;
					// Repulsion against every other node.
					for (let j = 0; j < next.length; j++) {
						if (i === j) continue;
						const dx = next[i].x - next[j].x;
						const dy = next[i].y - next[j].y;
						const dist2 = Math.max(0.01, dx * dx + dy * dy);
						const f = REPULSION / dist2;
						fx += (dx / Math.sqrt(dist2)) * f;
						fy += (dy / Math.sqrt(dist2)) * f;
					}
					// Attraction along edges.
					for (const otherId of adjacency.get(next[i].id) ?? []) {
						const o = next.find((n) => n.id === otherId);
						if (!o) continue;
						const dx = o.x - next[i].x;
						const dy = o.y - next[i].y;
						const dist = Math.sqrt(dx * dx + dy * dy);
						const stretch = dist - SPRING_LENGTH;
						fx += (dx / dist) * SPRING * stretch;
						fy += (dy / dist) * SPRING * stretch;
					}
					// Centre gravity so disconnected nodes don't fly off.
					fx += (VIEW_W / 2 - next[i].x) * CENTER_GRAVITY;
					fy += (VIEW_H / 2 - next[i].y) * CENTER_GRAVITY;

					next[i].vx = (next[i].vx + fx) * DAMPING;
					next[i].vy = (next[i].vy + fy) * DAMPING;
					next[i].x += next[i].vx;
					next[i].y += next[i].vy;
					// Clamp inside the canvas.
					next[i].x = Math.max(RADIUS, Math.min(VIEW_W - RADIUS, next[i].x));
					next[i].y = Math.max(RADIUS, Math.min(VIEW_H - RADIUS, next[i].y));
					ke += next[i].vx * next[i].vx + next[i].vy * next[i].vy;
				}
				return next;
			});
			if (ke > MIN_KINETIC && framesRef.current < MAX_FRAMES) {
				rafRef.current = requestAnimationFrame(step);
			}
		}
		rafRef.current = requestAnimationFrame(step);
		return () => {
			cancelled = true;
			if (rafRef.current != null) cancelAnimationFrame(rafRef.current);
		};
	}, [sim.length, adjacency]);

	if (nodes.length === 0) {
		return (
			<div
				className={`flex items-center justify-center text-xs text-ink-400 italic ${className}`}
				style={{ height }}
			>
				No relationships yet — save observations and watch them link up.
			</div>
		);
	}

	const nodeById = new Map(sim.map((n) => [n.id, n]));

	return (
		<div className={`relative ${className}`} style={{ height }}>
			<svg
				viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
				preserveAspectRatio="xMidYMid meet"
				className="w-full h-full"
				role="img"
				aria-label={`Knowledge graph with ${nodes.length} nodes and ${edges.length} edges`}
			>
				{/* Edges first so nodes paint on top */}
				{edges.map((e, i) => {
					const a = nodeById.get(e.source);
					const b = nodeById.get(e.target);
					if (!a || !b) return null;
					return (
						<line
							key={`edge-${i}`}
							x1={a.x}
							y1={a.y}
							x2={b.x}
							y2={b.y}
							stroke="rgba(255,255,255,0.12)"
							strokeWidth={1}
						>
							<title>
								{e.relation}: {a.label} ↔ {b.label}
							</title>
						</line>
					);
				})}

				{/* Nodes */}
				{sim.map((n) => {
					const isHover = n.id === hoverId;
					const color = palette[n.type] ?? "var(--color-ink-400)";
					return (
						<g
							key={n.id}
							style={{ cursor: onSelect ? "pointer" : "default" }}
							onMouseEnter={() => setHoverId(n.id)}
							onMouseLeave={() => setHoverId(null)}
							onClick={() => onSelect?.(n)}
						>
							<circle
								cx={n.x}
								cy={n.y}
								r={isHover ? RADIUS + 2 : RADIUS}
								fill={color}
								stroke="var(--color-space-900)"
								strokeWidth={1.5}
								opacity={hoverId && !isHover ? 0.45 : 1}
							>
								<title>
									[{n.type}] {n.label}
								</title>
							</circle>
							{isHover && (
								<text
									x={n.x}
									y={n.y - RADIUS - 6}
									fontSize="10"
									fontFamily="var(--font-mono)"
									fill="var(--color-ink-100)"
									textAnchor="middle"
									pointerEvents="none"
								>
									{truncate(n.label, 32)}
								</text>
							)}
						</g>
					);
				})}
			</svg>
		</div>
	);
}

// seed places every node at a random spot inside the canvas. Random init
// avoids the symmetry trap where every node ends up at (0,0).
function seed(nodes: KnowledgeGraphNode[]): SimNode[] {
	return nodes.map((n) => ({
		...n,
		x: VIEW_W * 0.25 + Math.random() * VIEW_W * 0.5,
		y: VIEW_H * 0.25 + Math.random() * VIEW_H * 0.5,
		vx: 0,
		vy: 0,
	}));
}

function truncate(s: string, max: number): string {
	if (s.length <= max) return s;
	return s.slice(0, max - 1) + "…";
}
