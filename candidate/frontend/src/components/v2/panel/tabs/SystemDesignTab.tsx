"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import clsx from "clsx";
import {
  Archive, Circle, Database, Diamond, Ellipsis, FileText, Layers, Lock, MousePointer2, MoveUpRight, Pencil, Redo2, Scan, Split,
  Square, Type, Undo2, User, Users, Box as BoxIcon, type LucideIcon,
} from "lucide-react";
import type { Diagram, DiagramNode, NodeTone } from "@/lib/v2/panel/types";
import { bounds, edgePath, fitZoom, moved, withEdge, withoutNode } from "@/lib/v2/panel/diagram";

type Tool = "select" | "rect" | "circle" | "diamond" | "arrow" | "text" | "frame";

const TONES: Record<NodeTone, { box: string; icon: string }> = {
  blue: { box: "border-[#9DBBF5] bg-[#EEF4FF]", icon: "text-[#2F5BD3]" },
  purple: { box: "border-[#C9B6F2] bg-[#F5F0FF]", icon: "text-[#7C4DDB]" },
  green: { box: "border-[#A7DDB5] bg-[#EFFAF2]", icon: "text-[#1F8A4C]" },
  sky: { box: "border-[#A9D6F2] bg-[#EEF8FE]", icon: "text-[#1C7DC0]" },
  amber: { box: "border-[#F2D58E] bg-[#FFF8E6]", icon: "text-[#B7791F]" },
  red: { box: "border-[#F2B1B1] bg-[#FFF0F0]", icon: "text-[#D23B3B]" },
  violet: { box: "border-[#C6B9F0] bg-[#F4F0FF]", icon: "text-[#6B48D6]" },
};

const ICONS: Record<DiagramNode["icon"], LucideIcon> = {
  client: Users, balancer: Split, lock: Lock, user: User, file: FileText, database: Database, storage: Archive, cache: Layers, box: BoxIcon,
};

const TOOLS: { id: Tool; label: string; icon: LucideIcon }[] = [
  { id: "select", label: "Select and move", icon: MousePointer2 },
  { id: "rect", label: "Box", icon: Square },
  { id: "circle", label: "Circle", icon: Circle },
  { id: "diamond", label: "Decision", icon: Diamond },
  { id: "arrow", label: "Connect two boxes", icon: MoveUpRight },
  { id: "text", label: "Text", icon: Type },
  { id: "frame", label: "Fit to view", icon: Scan },
];

const ZOOMS = [0.5, 0.75, 1, 1.25, 1.5];

/**
 * The System Design canvas, as in the mock-up: boxes and elbow arrows on a
 * dotted board, with a toolbar, zoom, and undo. Boxes are dragged with the
 * select tool; the arrow tool joins two boxes; Delete removes the selected
 * one. Step 6 saves the diagram as evidence; for now it lives in the panel.
 */
export function SystemDesignTab({ diagram, onChange }: { diagram: Diagram; onChange: (d: Diagram) => void }) {
  const board = useRef<HTMLDivElement>(null);
  const [tool, setTool] = useState<Tool>("select");
  const [zoom, setZoom] = useState(1);
  const [selected, setSelected] = useState<string | null>(null);
  const [connectFrom, setConnectFrom] = useState<string | null>(null);
  const [past, setPast] = useState<Diagram[]>([]);
  const [future, setFuture] = useState<Diagram[]>([]);
  const [title, setTitle] = useState(diagram.title);
  const drag = useRef<{ id: string; x: number; y: number; start: Diagram } | null>(null);

  const commit = useCallback(
    (next: Diagram, before: Diagram = diagram) => {
      if (next === before) return;
      setPast((p) => [...p.slice(-49), before]);
      setFuture([]);
      onChange(next);
    },
    [diagram, onChange],
  );

  const fit = useCallback(() => {
    const el = board.current;
    if (el) setZoom(fitZoom(diagram, el.clientWidth, el.clientHeight, 60));
  }, [diagram]);

  // Fit once the board has a size.
  useLayoutEffect(() => {
    fit();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const undo = () => {
    const prev = past[past.length - 1];
    if (!prev) return;
    setPast(past.slice(0, -1));
    setFuture([diagram, ...future]);
    onChange(prev);
  };
  const redo = () => {
    const next = future[0];
    if (!next) return;
    setFuture(future.slice(1));
    setPast([...past, diagram]);
    onChange(next);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
      if ((e.key === "Delete" || e.key === "Backspace") && selected) {
        commit(withoutNode(diagram, selected));
        setSelected(null);
      } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z") {
        e.preventDefault();
        if (e.shiftKey) redo();
        else undo();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const point = (e: React.PointerEvent) => {
    const r = board.current!.getBoundingClientRect();
    return { x: (e.clientX - r.left + board.current!.scrollLeft) / zoom, y: (e.clientY - r.top + board.current!.scrollTop) / zoom };
  };

  const onBoardDown = (e: React.PointerEvent) => {
    if (e.target !== e.currentTarget) return;
    setSelected(null);
    setConnectFrom(null);
    if (tool === "rect" || tool === "circle" || tool === "diamond" || tool === "text") {
      const p = point(e);
      const id = `n${Date.now().toString(36)}`;
      const n: DiagramNode = {
        id, x: Math.round(p.x - 70), y: Math.round(p.y - 28), w: 150, h: 56,
        title: tool === "text" ? "Note" : tool === "diamond" ? "Decision" : tool === "circle" ? "Actor" : "New service",
        tone: tool === "text" ? "amber" : tool === "diamond" ? "purple" : tool === "circle" ? "sky" : "blue",
        icon: "box",
      };
      commit({ ...diagram, nodes: [...diagram.nodes, moved(n, 0, 0)] });
      setSelected(id);
      setTool("select");
    }
  };

  const onNodeDown = (e: React.PointerEvent, n: DiagramNode) => {
    e.stopPropagation();
    if (tool === "arrow") {
      if (!connectFrom) setConnectFrom(n.id);
      else {
        commit(withEdge(diagram, connectFrom, n.id));
        setConnectFrom(null);
      }
      return;
    }
    setSelected(n.id);
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    drag.current = { id: n.id, x: e.clientX, y: e.clientY, start: diagram };
  };

  const onNodeMove = (e: React.PointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const dx = (e.clientX - d.x) / zoom, dy = (e.clientY - d.y) / zoom;
    onChange({ ...d.start, nodes: d.start.nodes.map((n) => (n.id === d.id ? moved(n, dx, dy) : n)) });
  };

  const onNodeUp = () => {
    const d = drag.current;
    drag.current = null;
    if (d) commit(diagram, d.start);
  };

  const rename = (id: string, text: string) => {
    commit({ ...diagram, nodes: diagram.nodes.map((n) => (n.id === id ? { ...n, title: text || n.title } : n)) });
  };

  const byId = new Map(diagram.nodes.map((n) => [n.id, n]));
  const extent = bounds(diagram.nodes, 60);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-[#EEF1F7] px-4 py-3">
        <label className="flex items-center gap-2 rounded-lg border border-[#E3E8F2] px-3 py-1.5 text-sm text-[#10183A]">
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={() => onChange({ ...diagram, title })}
            className="w-[130px] bg-transparent outline-none"
            aria-label="Diagram name"
          />
          <Pencil className="h-3.5 w-3.5 text-[#8A91AB]" aria-hidden />
        </label>
        <div className="mx-auto flex items-center gap-0.5 rounded-xl border border-[#E3E8F2] bg-white p-1" role="toolbar" aria-label="Drawing tools">
          {TOOLS.map((t) => {
            const Icon = t.icon;
            return (
              <button
                key={t.id}
                type="button"
                title={t.label}
                aria-label={t.label}
                aria-pressed={tool === t.id}
                onClick={() => (t.id === "frame" ? fit() : setTool(t.id))}
                className={clsx("flex h-8 w-8 items-center justify-center rounded-lg", tool === t.id ? "bg-[#EAF0FE] text-[#2F5BD3]" : "text-[#3B4568] hover:bg-[#F4F6FB]")}
              >
                <Icon className="h-[18px] w-[18px]" />
              </button>
            );
          })}
          <span className="flex h-8 w-8 items-center justify-center text-[#8A91AB]" aria-hidden>
            <Ellipsis className="h-[18px] w-[18px]" />
          </span>
        </div>
        <select
          value={ZOOMS.includes(zoom) ? zoom : "fit"}
          onChange={(e) => (e.target.value === "fit" ? fit() : setZoom(Number(e.target.value)))}
          className="rounded-lg border border-[#E3E8F2] bg-white px-2 py-1.5 text-sm text-[#10183A]"
          aria-label="Zoom"
        >
          {!ZOOMS.includes(zoom) && <option value="fit">{Math.round(zoom * 100)}%</option>}
          {ZOOMS.map((z) => (
            <option key={z} value={z}>
              {z * 100}%
            </option>
          ))}
        </select>
        <div className="flex items-center rounded-lg border border-[#E3E8F2]">
          <button type="button" onClick={undo} disabled={!past.length} className="p-1.5 text-[#3B4568] disabled:opacity-40" aria-label="Undo" title="Undo">
            <Undo2 className="h-4 w-4" />
          </button>
          <button type="button" onClick={redo} disabled={!future.length} className="p-1.5 text-[#3B4568] disabled:opacity-40" aria-label="Redo" title="Redo">
            <Redo2 className="h-4 w-4" />
          </button>
        </div>
      </div>
      {tool === "arrow" && (
        <p className="border-b border-[#EEF1F7] bg-[#F6F9FF] px-4 py-1.5 text-xs text-[#2F5BD3]">
          {connectFrom ? `From ${byId.get(connectFrom)?.title}: now click the box it goes to.` : "Click the box an arrow starts from."}
        </p>
      )}
      <div
        ref={board}
        className={clsx("relative flex-1 overflow-auto", tool !== "select" && tool !== "arrow" ? "cursor-crosshair" : "cursor-default")}
        style={{ backgroundImage: "radial-gradient(#D7DDEA 1px, transparent 1px)", backgroundSize: `${20 * zoom}px ${20 * zoom}px` }}
        onPointerDown={onBoardDown}
        data-testid="design-board"
      >
        {/* The sizer takes the scaled size, so scrollbars appear only when the scaled diagram doesn't fit. */}
        <div className="pointer-events-none" style={{ width: (extent.x + extent.w) * zoom, height: (extent.y + extent.h) * zoom }} aria-hidden />
        <div className="absolute left-0 top-0 origin-top-left" style={{ width: extent.x + extent.w, height: extent.y + extent.h, transform: `scale(${zoom})` }}>
          <svg className="pointer-events-none absolute inset-0 overflow-visible" width={extent.x + extent.w} height={extent.y + extent.h} aria-hidden>
            <defs>
              <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0 0L10 5L0 10z" fill="#2A3355" />
              </marker>
              <marker id="arrow-dash" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                <path d="M0 0L10 5L0 10z" fill="#8A91AB" />
              </marker>
            </defs>
            {diagram.groups?.map((g) => {
              const b = bounds(g.nodes.map((id) => byId.get(id)).filter((n): n is DiagramNode => !!n), 14);
              return <rect key={g.id} x={b.x} y={b.y} width={b.w} height={b.h} rx="12" fill="none" stroke="#C5CCDD" strokeDasharray="6 5" />;
            })}
            {diagram.edges.map((e) => {
              const a = byId.get(e.from), b = byId.get(e.to);
              if (!a || !b) return null;
              return (
                <path
                  key={e.id}
                  d={edgePath(a, b)}
                  fill="none"
                  stroke={e.dashed ? "#8A91AB" : "#2A3355"}
                  strokeWidth="1.5"
                  strokeDasharray={e.dashed ? "5 4" : undefined}
                  markerEnd={`url(#${e.dashed ? "arrow-dash" : "arrow"})`}
                />
              );
            })}
          </svg>
          {diagram.nodes.map((n) => {
            const Icon = ICONS[n.icon];
            const tone = TONES[n.tone];
            return (
              <div
                key={n.id}
                role="button"
                tabIndex={0}
                aria-label={`${n.title}${n.subtitle ? `, ${n.subtitle}` : ""}`}
                aria-pressed={selected === n.id}
                onPointerDown={(e) => onNodeDown(e, n)}
                onPointerMove={onNodeMove}
                onPointerUp={onNodeUp}
                onDoubleClick={() => {
                  const t = window.prompt("Rename", n.title);
                  if (t !== null) rename(n.id, t.trim());
                }}
                className={clsx(
                  "absolute flex select-none items-center gap-3 rounded-xl border px-4 shadow-[0_1px_2px_rgba(16,24,58,0.06)]",
                  tone.box,
                  tool === "select" ? "cursor-grab active:cursor-grabbing" : "cursor-pointer",
                  (selected === n.id || connectFrom === n.id) && "ring-2 ring-[#2F5BD3] ring-offset-2",
                )}
                style={{ left: n.x, top: n.y, width: n.w, height: n.h }}
              >
                <Icon className={clsx("h-5 w-5 shrink-0", tone.icon)} aria-hidden />
                <div className="min-w-0 leading-tight">
                  <div className="truncate text-[14px] font-medium text-[#10183A]">{n.title}</div>
                  {n.subtitle && <div className="truncate text-[12px] text-[#6B7393]">{n.subtitle}</div>}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
