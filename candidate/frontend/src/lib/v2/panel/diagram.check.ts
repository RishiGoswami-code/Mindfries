/**
 * Checks diagram.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/v2/panel/diagram.check.ts
 */
import { bounds, edgePath, fitZoom, moved, withEdge, withoutNode } from "./diagram.ts";
import type { Diagram } from "./types";

// The mock-up's diagram, as mock.ts has it.
const node = (id: string, x: number, y: number, w: number) => ({ id, x, y, w, h: 64, title: id, tone: "blue" as const, icon: "box" as const });
const mock: Diagram = {
  title: "Untitled Diagram",
  nodes: [node("client", 20, 250, 160), node("lb", 215, 250, 155), node("auth", 425, 150, 185), node("user", 425, 232, 185), node("content", 425, 314, 185), node("db", 665, 250, 160), node("storage", 185, 470, 180), node("cache", 470, 470, 145)],
  edges: [
    { id: "e1", from: "client", to: "lb" },
    { id: "e7", from: "content", to: "db" },
    { id: "e8", from: "content", to: "cache" },
  ],
  groups: [{ id: "services", nodes: ["auth", "user", "content"] }],
};

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}

const a = { x: 0, y: 0, w: 100, h: 40 };
check("right of: side to side", edgePath(a, { x: 200, y: 100, w: 100, h: 40 }) === "M100 20 H150 V120 H200", edgePath(a, { x: 200, y: 100, w: 100, h: 40 }));
check("left of: side to side", edgePath({ x: 200, y: 0, w: 100, h: 40 }, a) === "M200 20 H150 V20 H100");
check("below: top to bottom", edgePath(a, { x: 20, y: 120, w: 100, h: 40 }) === "M50 40 V80 H70 V120");
check("above: bottom to top", edgePath({ x: 0, y: 200, w: 100, h: 40 }, a) === "M50 200 V120 H50 V40");

const b = bounds([a, { x: 150, y: 60, w: 50, h: 50 }], 10);
check("bounds with padding", b.x === -10 && b.y === -10 && b.w === 220 && b.h === 130, JSON.stringify(b));
check("bounds of nothing", bounds([]).w === 0);

const d = mock;
check("the mock fits as is in a big canvas", fitZoom(d, 2000, 2000) === 1);
const z = fitZoom(d, 560, 560);
check("the mock shrinks to fit the panel", z < 1 && z > 0.5, String(z));
check("never below 30%", fitZoom(d, 10, 10) === 0.3);

check("moved stays on the canvas", moved(d.nodes[0], -500, 3).x === 0 && moved(d.nodes[0], -500, 3).y === d.nodes[0].y + 3);
const without = withoutNode(d, "content");
check("removing a node drops its edges", !without.edges.some((e) => e.from === "content" || e.to === "content") && without.nodes.length === d.nodes.length - 1);
check("and leaves it out of its group", without.groups?.[0].nodes.join() === "auth,user");
check("no self-loops", withEdge(d, "db", "db") === d);
check("no duplicate edges", withEdge(d, "client", "lb") === d);
check("new edge added", withEdge(d, "db", "cache").edges.length === d.edges.length + 1);

if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall passed");
