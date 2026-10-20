import type { Diagram, DiagramNode } from "./types";

/** The System Design canvas's geometry, kept free of React so it's testable. */

export interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** The smallest box around the nodes, padded. */
export function bounds(nodes: Box[], pad = 0): Box {
  if (nodes.length === 0) return { x: 0, y: 0, w: 0, h: 0 };
  const x = Math.min(...nodes.map((n) => n.x)) - pad;
  const y = Math.min(...nodes.map((n) => n.y)) - pad;
  const r = Math.max(...nodes.map((n) => n.x + n.w)) + pad;
  const b = Math.max(...nodes.map((n) => n.y + n.h)) + pad;
  return { x, y, w: r - x, h: b - y };
}

/**
 * An elbow connector from a to b, as in the mock-up: side to side when b is
 * to the right or left, top to bottom when it's above or below. Returns an
 * SVG path, ending where the arrowhead goes.
 */
export function edgePath(a: Box, b: Box): string {
  const acx = a.x + a.w / 2, acy = a.y + a.h / 2;
  const bcx = b.x + b.w / 2, bcy = b.y + b.h / 2;
  const horizontal = b.x >= a.x + a.w || b.x + b.w <= a.x;
  if (horizontal) {
    const right = bcx > acx;
    const x1 = right ? a.x + a.w : a.x;
    const x2 = right ? b.x : b.x + b.w;
    const mx = Math.round((x1 + x2) / 2);
    return `M${x1} ${acy} H${mx} V${bcy} H${x2}`;
  }
  const down = bcy > acy;
  const y1 = down ? a.y + a.h : a.y;
  const y2 = down ? b.y : b.y + b.h;
  const my = Math.round((y1 + y2) / 2);
  return `M${acx} ${y1} V${my} H${bcx} V${y2}`;
}

/** The zoom that fits the diagram into width × height, never above 1. */
export function fitZoom(d: Diagram, width: number, height: number, margin = 32): number {
  const b = bounds(d.nodes, margin);
  if (b.w === 0 || b.h === 0) return 1;
  return Math.max(0.3, Math.min(1, width / (b.x + b.w), height / (b.y + b.h)));
}

/** A node moved by (dx, dy), kept on the canvas. */
export function moved(n: DiagramNode, dx: number, dy: number): DiagramNode {
  return { ...n, x: Math.max(0, Math.round(n.x + dx)), y: Math.max(0, Math.round(n.y + dy)) };
}

/** A diagram with node `id` and every edge touching it removed. */
export function withoutNode(d: Diagram, id: string): Diagram {
  return {
    ...d,
    nodes: d.nodes.filter((n) => n.id !== id),
    edges: d.edges.filter((e) => e.from !== id && e.to !== id),
    groups: d.groups?.map((g) => ({ ...g, nodes: g.nodes.filter((n) => n !== id) })).filter((g) => g.nodes.length > 0),
  };
}

/** Adds an edge a → b unless it's a self-loop or already there. */
export function withEdge(d: Diagram, from: string, to: string): Diagram {
  if (from === to || d.edges.some((e) => e.from === from && e.to === to)) return d;
  return { ...d, edges: [...d.edges, { id: `e${Date.now().toString(36)}${d.edges.length}`, from, to }] };
}
