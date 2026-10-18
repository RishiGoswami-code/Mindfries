import React from "react";
import Link from "next/link";
import { ChevronRight, Flag } from "lucide-react";
import type { DiffLine, FileDiff } from "@/lib/report/diff";
import { clock, type ReportFlag, type TestRun, type TimelineItem } from "@/lib/report/trail";

export function Panel({
  icon,
  title,
  action,
  className = "",
  children,
}: {
  icon?: React.ReactNode;
  title: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section className={`min-w-0 rounded-2xl border border-hair bg-surface p-5 shadow-sm sm:p-6 print:break-inside-avoid ${className}`}>
      <div className="flex items-center gap-3">
        {icon && <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#f4f0ff] text-[#7c3aed]">{icon}</span>}
        <h2 className="min-w-0 flex-1 text-[17px] font-bold text-ink">{title}</h2>
        {action}
      </div>
      <div className="mt-4">{children}</div>
    </section>
  );
}

export function Pill({
  tone,
  children,
}: {
  tone: "green" | "red" | "amber" | "gray" | "violet";
  children: React.ReactNode;
}) {
  const tones = {
    green: "bg-[#e8f8ee] text-[#15a34a] border-[#bbf0cc]",
    red: "bg-[#feeef0] text-[#e11d48] border-[#fecdd3]",
    amber: "bg-[#fff8eb] text-[#d97706] border-[#fde68a]",
    gray: "bg-surface-2 text-dim border-hair",
    violet: "bg-[#f4f0ff] text-[#7c3aed] border-[#ddd6fe]",
  };
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-0.5 text-[12px] font-semibold whitespace-nowrap ${tones[tone]}`}>
      {children}
    </span>
  );
}

function DiffColumn({
  title,
  lines,
  side,
  badge,
}: {
  title: string;
  lines: DiffLine[];
  side: "before" | "after";
  badge?: React.ReactNode;
}) {
  const shown = lines.filter((l) => l.kind === "same" || (side === "before" ? l.kind === "removed" : l.kind === "added"));
  return (
    <div className="min-w-0">
      <div className="flex h-10 items-center justify-between gap-2 border-b border-hair px-4 bg-surface-2">
        <span className="text-[13px] font-semibold text-dim">{title}</span>
        {badge}
      </div>
      <div className="overflow-x-auto">
        <pre className="min-w-full py-2 font-mono text-[12px] leading-[1.6] text-ink">
          {shown.length === 0 ? (
            <span className="block px-4 py-2 font-sans text-[13px] text-faint">
              {side === "before" ? "The file didn't exist." : "The file was deleted."}
            </span>
          ) : (
            shown.map((l, i) => {
              const n = l.kind === "same" ? (side === "before" ? l.before : l.after) : l.kind === "removed" ? l.before : l.after;
              const changed = l.kind !== "same";
              return (
                <span key={i} className={`flex ${changed ? (side === "before" ? "bg-[#feeef0]" : "bg-[#e8f8ee]") : ""}`}>
                  <span className="w-10 shrink-0 pr-3 text-right text-faint select-none">{n}</span>
                  <span className={`pr-4 whitespace-pre ${changed ? (side === "before" ? "text-[#b91c1c] font-medium" : "text-[#15803d] font-medium") : ""}`}>
                    {l.text || " "}
                  </span>
                </span>
              );
            })
          )}
        </pre>
      </div>
    </div>
  );
}

export function DiffView({
  diff,
  window,
  pasteBadge,
}: {
  diff: FileDiff;
  window?: number;
  pasteBadge?: React.ReactNode;
}) {
  if (diff.tooLarge) {
    return (
      <p className="rounded-xl bg-surface-2 px-4 py-3 text-[13.5px] text-dim">
        This file is too large to show line by line here: {diff.added} lines added and {diff.removed} removed.
      </p>
    );
  }
  let lines = diff.lines;
  let cropped = false;
  if (window) {
    const first = Math.max(0, lines.findIndex((l) => l.kind !== "same"));
    const from = Math.max(0, first - 2);
    cropped = from > 0 || lines.length > from + window;
    lines = lines.slice(from, from + window);
  }
  return (
    <div className="overflow-hidden rounded-xl border border-hair bg-surface">
      <div className="grid md:grid-cols-2 md:divide-x md:divide-hair">
        <DiffColumn title="Before (Task starter)" lines={lines} side="before" />
        <div className="border-t border-hair md:border-t-0">
          <DiffColumn title="After (Candidate submission)" lines={lines} side="after" badge={pasteBadge} />
        </div>
      </div>
      {cropped && (
        <p className="border-t border-hair px-4 py-2 text-[12px] text-dim">
          Showing the first change. The full file diff is available in the Code evaluation tab.
        </p>
      )}
    </div>
  );
}

export function Timeline({ items }: { items: TimelineItem[] }) {
  return (
    <ol className="relative space-y-1">
      {items.map((t, i) => (
        <li
          key={`${t.id}-${i}`}
          className={`relative flex items-start gap-4 rounded-xl py-2 pr-2 pl-7 ${t.tone === "flag" ? "bg-[#feeef0]" : ""}`}
        >
          {i < items.length - 1 && <span aria-hidden className="absolute top-5 bottom-[-14px] left-[11px] w-px bg-hair-bright" />}
          <span
            aria-hidden
            className={`absolute top-[13px] left-[7px] h-[9px] w-[9px] rounded-full ring-4 ring-white ${
              t.tone === "flag" ? "bg-[#e11d48]" : t.tone === "bad" ? "bg-[#d97706]" : t.tone === "good" ? "bg-[#15a34a]" : "bg-[#7c3aed]"
            }`}
          />
          <span className={`w-[54px] shrink-0 font-mono text-[12.5px] tabular-nums ${t.tone === "flag" ? "text-[#b91c1c] font-bold" : "text-dim"}`}>
            {clock(t.offset)}
          </span>
          <span className={`min-w-0 flex-1 text-[13.5px] ${t.tone === "flag" ? "font-semibold text-[#b91c1c]" : "text-ink"}`}>
            {t.text}
          </span>
          {t.tone === "flag" && <Flag size={14} className="mt-0.5 shrink-0 fill-[#e11d48] text-[#e11d48]" />}
        </li>
      ))}
    </ol>
  );
}

const SEVERITY_PILL = {
  high: { tone: "red" as const, label: "High" },
  medium: { tone: "amber" as const, label: "Medium" },
  low: { tone: "gray" as const, label: "Low" },
};

export function FlagList({
  flags,
  onSelect,
}: {
  flags: ReportFlag[];
  onSelect?: () => void;
}) {
  if (flags.length === 0) {
    return <p className="rounded-xl bg-[#e8f8ee] px-4 py-3 text-[13.5px] font-medium text-[#15803d]">Nothing was flagged during this session.</p>;
  }
  return (
    <ul className="divide-y divide-hair">
      {flags.map((f) => {
        const body = (
          <>
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-[#feeef0] text-[#e11d48]">
              <Flag size={16} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-[14px] font-semibold text-ink">{f.title}</span>
              <span className="block text-[12.5px] text-dim">{[f.detail, `at ${clock(f.offset)}`].filter(Boolean).join(" · ")}</span>
            </span>
            <Pill tone={SEVERITY_PILL[f.severity].tone}>{SEVERITY_PILL[f.severity].label}</Pill>
            {onSelect && <ChevronRight size={16} className="shrink-0 text-faint" />}
          </>
        );
        return (
          <li key={f.id}>
            {onSelect ? (
              <button
                type="button"
                onClick={onSelect}
                className="flex w-full items-center gap-3.5 rounded-xl px-1 py-2.5 text-left transition-colors hover:bg-surface-2"
              >
                {body}
              </button>
            ) : (
              <div className="flex items-center gap-3.5 px-1 py-2.5">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export function TestResults({ run }: { run: TestRun | null }) {
  if (!run || run.passed === null || run.failed === null) {
    return <p className="rounded-xl bg-surface-2 px-4 py-3 text-[13.5px] text-dim">No test run that could be parsed was recorded.</p>;
  }
  const total = run.passed + run.failed;
  const share = total ? run.passed / total : 0;
  const r = 50;
  const circ = 2 * Math.PI * r;
  const good = run.failed === 0;
  const rows = [
    { label: "Passed", value: run.passed, color: "bg-[#15a34a]" },
    { label: "Failed", value: run.failed, color: "bg-[#e11d48]" },
  ];
  return (
    <div className="flex flex-col items-center gap-6 sm:flex-row sm:gap-8">
      <div className="relative h-[130px] w-[130px] shrink-0">
        <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90" aria-hidden>
          <circle cx="60" cy="60" r={r} fill="none" stroke="rgba(20, 22, 30, 0.08)" strokeWidth="11" />
          <circle
            cx="60"
            cy="60"
            r={r}
            fill="none"
            stroke={good ? "#15a34a" : "#d97706"}
            strokeWidth="11"
            strokeLinecap="round"
            strokeDasharray={`${share * circ} ${circ}`}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[24px] font-extrabold text-ink tabular-nums">
            {run.passed}/{total}
          </span>
          <span className="text-[12px] font-medium text-dim">Passed</span>
        </div>
      </div>
      <div className="w-full min-w-0 flex-1">
        {rows.map((row) => (
          <div key={row.label} className="flex items-center gap-4 border-b border-hair py-2">
            <span className="w-16 text-[13.5px] font-medium text-dim">{row.label}</span>
            <span className="w-8 text-[13.5px] font-bold tabular-nums">{row.value}</span>
            <span className="h-2 flex-1 overflow-hidden rounded-full bg-surface-2 border border-hair">
              <span className={`block h-full rounded-full ${row.color}`} style={{ width: `${total ? (row.value / total) * 100 : 0}%` }} />
            </span>
          </div>
        ))}
        <div className="flex items-center gap-4 py-2">
          <span className="w-16 text-[13.5px] font-bold text-ink">Total</span>
          <span className="w-8 text-[13.5px] font-bold tabular-nums">{total}</span>
        </div>
        <p className="mt-1 text-[12px] text-dim">
          Command: <code className="font-mono text-ink bg-surface-2 px-1.5 py-0.5 rounded">{run.command}</code> (at {clock(run.offset)})
        </p>
      </div>
    </div>
  );
}

export function Observations({
  items,
  empty,
}: {
  items: Array<{ category: string; observation: string }>;
  empty: string;
}) {
  if (items.length === 0) return <p className="text-[13.5px] text-dim">{empty}</p>;
  return (
    <div className="space-y-3">
      {items.map((item, i) => (
        <div key={i} className="rounded-xl border border-hair bg-surface-2 p-4 text-[14px] leading-relaxed whitespace-pre-line text-ink">
          {item.observation.replace(/ ?\[E\d+\]/g, "")}
        </div>
      ))}
    </div>
  );
}
