import clsx from "clsx";
import { ChevronRight, Flag } from "lucide-react";
import Link from "next/link";
import type { DiffLine, FileDiff } from "@/lib/report/diff";
import { clock, type ReportFlag, type TestRun, type TimelineItem } from "@/lib/report/trail";

/**
 * The evidence report's building blocks: cards, the side-by-side diff, the
 * timeline, the flag list and the test ring. Markup only — every number in
 * them is passed in from the session's recorded activity.
 */

export function Panel({ icon, title, action, className, children }: { icon?: React.ReactNode; title: React.ReactNode; action?: React.ReactNode; className?: string; children: React.ReactNode }) {
  return (
    <section className={clsx("min-w-0 rounded-[22px] border border-white/90 bg-white/75 p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur sm:p-6 print:break-inside-avoid print:shadow-none", className)}>
      <div className="flex items-center gap-3.5">
        {icon && <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#EFEAFD] text-[#5B3BE0]">{icon}</span>}
        <h2 className="min-w-0 flex-1 text-[19px] font-semibold text-[#100E2E]">{title}</h2>
        {action}
      </div>
      <div className="mt-4">{children}</div>
    </section>
  );
}

export function Pill({ tone, children }: { tone: "green" | "red" | "amber" | "gray" | "violet"; children: React.ReactNode }) {
  return (
    <span
      className={clsx(
        "inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-[12.5px] font-medium whitespace-nowrap",
        tone === "green" && "bg-[#E3F7EC] text-[#14693a]",
        tone === "red" && "bg-[#FDECEF] text-[#B01D38]",
        tone === "amber" && "bg-[#FDF0E1] text-[#9A4B07]",
        tone === "gray" && "bg-[#F1F0F6] text-[#5E5A82]",
        tone === "violet" && "bg-[#EFEAFD] text-[#4B2BB8]"
      )}
    >
      {children}
    </span>
  );
}

/** One column of a side-by-side diff. */
function DiffColumn({ title, lines, side, badge }: { title: string; lines: DiffLine[]; side: "before" | "after"; badge?: React.ReactNode }) {
  const shown = lines.filter((l) => l.kind === "same" || (side === "before" ? l.kind === "removed" : l.kind === "added"));
  return (
    <div className="min-w-0">
      <div className="flex h-11 items-center justify-between gap-2 border-b border-[#ECE9F6] px-4">
        <span className="text-[13.5px] font-medium text-[#4C4874]">{title}</span>
        {badge}
      </div>
      <div className="overflow-x-auto">
        <pre className="min-w-full py-2 font-mono text-[12.5px] leading-[1.6] text-[#1B1840]">
          {shown.length === 0 ? (
            <span className="block px-4 py-2 font-sans text-[13px] text-[#8A86A8]">{side === "before" ? "The file didn't exist." : "The file was deleted."}</span>
          ) : (
            shown.map((l, i) => {
              const n = l.kind === "same" ? (side === "before" ? l.before : l.after) : l.kind === "removed" ? l.before : l.after;
              const changed = l.kind !== "same";
              return (
                <span key={i} className={clsx("flex", changed && (side === "before" ? "bg-[#FDECEF]" : "bg-[#E6F7EC]"))}>
                  <span className="w-11 shrink-0 pr-3 text-right text-[#A9A5C2] select-none">{n}</span>
                  <span className={clsx("pr-4 whitespace-pre", changed && (side === "before" ? "text-[#8F1D33]" : "text-[#0F5530]"))}>{l.text || " "}</span>
                </span>
              );
            })
          )}
        </pre>
      </div>
    </div>
  );
}

/**
 * A file's change, the task's version beside the candidate's. `window`
 * crops to the first change and a few lines around it — the Summary's
 * glimpse; the Code evaluation tab shows the whole file.
 */
export function DiffView({ diff, window, pasteBadge }: { diff: FileDiff; window?: number; pasteBadge?: React.ReactNode }) {
  if (diff.tooLarge) {
    return (
      <p className="rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[13.5px] text-[#4C4874]">
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
    <div className="overflow-hidden rounded-2xl border border-[#ECE9F6] bg-white">
      <div className="grid md:grid-cols-2 md:divide-x md:divide-[#ECE9F6]">
        <DiffColumn title="Before" lines={lines} side="before" />
        <div className="border-t border-[#ECE9F6] md:border-t-0">
          <DiffColumn title="After" lines={lines} side="after" badge={pasteBadge} />
        </div>
      </div>
      {cropped && <p className="border-t border-[#ECE9F6] px-4 py-2 text-[12.5px] text-[#8A86A8]">Showing the first change. The whole file is under Code evaluation.</p>}
    </div>
  );
}

export function Timeline({ items }: { items: TimelineItem[] }) {
  return (
    <ol className="relative">
      {items.map((t, i) => (
        <li key={`${t.id}-${i}`} className={clsx("relative flex items-start gap-4 rounded-xl py-2.5 pr-2 pl-7", t.tone === "flag" && "bg-[#FDECEF]")}>
          {i < items.length - 1 && <span aria-hidden className="absolute top-6 bottom-[-14px] left-[11px] w-px bg-[#DCD6F4]" />}
          <span
            aria-hidden
            className={clsx(
              "absolute top-[15px] left-[7px] h-[9px] w-[9px] rounded-full ring-4 ring-white",
              t.tone === "flag" ? "bg-[#DC2643]" : t.tone === "bad" ? "bg-[#E3A63B]" : t.tone === "good" ? "bg-[#1A9E6B]" : "bg-[#5B3BE0]"
            )}
          />
          <span className={clsx("w-[56px] shrink-0 font-mono text-[13px] tabular-nums", t.tone === "flag" ? "text-[#B01D38]" : "text-[#6E6A8F]")}>{clock(t.offset)}</span>
          <span className={clsx("min-w-0 flex-1 text-[14px]", t.tone === "flag" ? "font-medium text-[#B01D38]" : "text-[#2A2654]")}>{t.text}</span>
          {t.tone === "flag" && <Flag size={15} className="mt-0.5 shrink-0 fill-[#DC2643] text-[#DC2643]" />}
        </li>
      ))}
    </ol>
  );
}

const SEVERITY_PILL = { high: { tone: "red", label: "High" }, medium: { tone: "amber", label: "Medium" }, low: { tone: "gray", label: "Low" } } as const;

export function FlagList({ flags, href }: { flags: ReportFlag[]; href?: string }) {
  if (flags.length === 0) return <p className="rounded-2xl bg-[#F3FBF6] px-4 py-3 text-[13.5px] text-[#14693a]">Nothing was flagged during this session.</p>;
  return (
    <ul className="divide-y divide-[#F0EEF7]">
      {flags.map((f) => {
        const body = (
          <>
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#FDECEF] text-[#DC2643]">
              <Flag size={18} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-[14.5px] font-medium text-[#100E2E]">{f.title}</span>
              <span className="block text-[13px] text-[#6E6A8F]">{[f.detail, `at ${clock(f.offset)}`].filter(Boolean).join(" · ")}</span>
            </span>
            <Pill tone={SEVERITY_PILL[f.severity].tone}>{SEVERITY_PILL[f.severity].label}</Pill>
            {href && <ChevronRight size={17} className="shrink-0 text-[#8A86A8]" />}
          </>
        );
        return (
          <li key={f.id}>
            {href ? (
              <Link href={href} className="flex items-center gap-3.5 rounded-xl px-1 py-3 hover:bg-[#FAF9FE]">
                {body}
              </Link>
            ) : (
              <div className="flex items-center gap-3.5 px-1 py-3">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** The ring and the passed/failed bars for one test run. */
export function TestResults({ run }: { run: TestRun | null }) {
  if (!run || run.passed === null || run.failed === null) {
    return <p className="rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[13.5px] text-[#4C4874]">No test run that could be read was recorded in this session.</p>;
  }
  const total = run.passed + run.failed;
  const share = total ? run.passed / total : 0;
  const r = 52;
  const circ = 2 * Math.PI * r;
  const good = run.failed === 0;
  const rows = [
    { label: "Passed", value: run.passed, color: "bg-[#22C55E]" },
    { label: "Failed", value: run.failed, color: "bg-[#EF4444]" },
  ];
  return (
    <div className="flex flex-col items-center gap-6 sm:flex-row sm:gap-10">
      <div className="relative h-[140px] w-[140px] shrink-0">
        <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90" aria-hidden>
          <circle cx="60" cy="60" r={r} fill="none" stroke="#EEF0F4" strokeWidth="12" />
          <circle cx="60" cy="60" r={r} fill="none" stroke={good ? "#22C55E" : "#E3A63B"} strokeWidth="12" strokeLinecap="round" strokeDasharray={`${share * circ} ${circ}`} />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[26px] font-semibold text-[#100E2E] tabular-nums">
            {run.passed}/{total}
          </span>
          <span className="text-[13px] text-[#6E6A8F]">Passed</span>
        </div>
      </div>
      <div className="w-full min-w-0 flex-1">
        {rows.map((row) => (
          <div key={row.label} className="flex items-center gap-4 border-b border-[#F0EEF7] py-2.5">
            <span className="w-16 text-[14px] text-[#4C4874]">{row.label}</span>
            <span className="w-8 text-[14px] font-medium tabular-nums">{row.value}</span>
            <span className="h-2.5 flex-1 overflow-hidden rounded-full bg-[#EEF0F4]">
              <span className={clsx("block h-full rounded-full", row.color)} style={{ width: `${total ? (row.value / total) * 100 : 0}%` }} />
            </span>
          </div>
        ))}
        <div className="flex items-center gap-4 py-2.5">
          <span className="w-16 text-[14px] font-semibold text-[#100E2E]">Total</span>
          <span className="w-8 text-[14px] font-semibold tabular-nums">{total}</span>
        </div>
        <p className="text-[12.5px] text-[#8A86A8]">
          The last run, at {clock(run.offset)}: <code className="font-mono">{run.command}</code>
        </p>
      </div>
    </div>
  );
}

/** The observations an agent wrote, with their trail citations ("[E12]") taken out. */
export function Observations({ items, empty }: { items: Array<{ category: string; observation: string }>; empty: string }) {
  if (items.length === 0) return <p className="text-[14px] text-[#6E6A8F]">{empty}</p>;
  return (
    <div className="space-y-3">
      {items.map((item, i) => (
        <p key={i} className="rounded-2xl bg-[#FAF9FE] px-4 py-3.5 text-[14.5px] leading-relaxed whitespace-pre-line text-[#1B1840]">
          {item.observation.replace(/ ?\[E\d+\]/g, "")}
        </p>
      ))}
    </div>
  );
}
