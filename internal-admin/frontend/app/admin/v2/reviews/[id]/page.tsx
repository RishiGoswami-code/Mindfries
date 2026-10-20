import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { fetchV2Review } from "@/lib/v2";
import { PageHeader, Pill, StatCard } from "@/components/ui";

export const metadata: Metadata = { title: "V2 Review" };
export const dynamic = "force-dynamic";

export default async function V2ReviewDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const data = await fetchV2Review(id);
  if (!data) notFound();

  const { session, submission, report, events } = data;
  const o = report.outcome;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/admin/v2/reviews" className="text-xs text-faint hover:underline">← all reviews</Link>
        <PageHeader eyebrow={`${session.product_slug} · ${session.role}`} title={session.assessment_title || session.bank_task_id} />
      </div>

      {/* Objective outcome */}
      <div className="rounded-xl border border-hair bg-surface p-5">
        <div className="flex items-center gap-3">
          <span className="eyebrow">Outcome</span>
          {!submission ? (
            <Pill tone="gray">in progress</Pill>
          ) : !o.graded ? (
            <Pill tone="amber">awaiting grade</Pill>
          ) : o.passed ? (
            <Pill tone="green">passed hidden tests</Pill>
          ) : (
            <Pill tone="coral">failed hidden tests</Pill>
          )}
        </div>
        {o.graded && o.check && (
          <div className="mt-2 text-xs text-faint">check: <code>{o.check.join(" ")}</code></div>
        )}
        {!o.graded && submission && (
          <div className="mt-2 text-sm text-dim">The code is submitted and stored — run the grader to get pass/fail.</div>
        )}
      </div>

      {/* Reviewer flags */}
      {report.flags.length > 0 && (
        <div className="rounded-xl border border-[#fbd0d4] bg-[#fff1f2] p-5">
          <div className="eyebrow mb-2 text-[#d93a44]">Worth a look</div>
          <ul className="space-y-1.5 text-sm text-[#9f2b33]">
            {report.flags.map((f, i) => <li key={i}>• {f}</li>)}
          </ul>
        </div>
      )}

      {/* Effort */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard label="Active time" value={`${report.effort.active_minutes}m`} hint={`of ${report.effort.window_minutes}m window`} />
        <StatCard label="Files changed" value={report.effort.files_changed} />
        <StatCard label="Lines" value={`+${report.effort.lines_added} / −${report.effort.lines_removed}`} />
        <StatCard label="Commits" value={report.effort.commits} />
      </div>

      {/* AI usage */}
      <div className="space-y-2 rounded-xl border border-hair bg-surface p-5">
        <div className="eyebrow">AI usage</div>
        <div className="text-sm text-dim">
          {report.ai.agents_detected.length === 0
            ? "No coding-agent processes detected."
            : `Coding agents detected: ${report.ai.agents_detected.join(", ")}${report.ai.agents_left_running ? " (left running)" : ""}`}
        </div>
        {report.ai.ai_extensions.length > 0 && (
          <div className="text-sm text-dim">AI editor extensions: {report.ai.ai_extensions.join(", ")}</div>
        )}
        <div className="text-sm text-dim">
          Attribution: {report.ai.candidate_attributed_lines} line(s) candidate · {report.ai.agent_attributed_lines} line(s) agent.
        </div>
        <div className="text-xs text-faint">Locked task files {report.effort.task_files_intact ? "unchanged" : "were modified"}.</div>
      </div>

      {/* What changed */}
      {report.changes && report.changes.length > 0 && (
        <div className="rounded-xl border border-hair bg-surface p-5">
          <div className="eyebrow mb-3">What changed</div>
          <ul className="space-y-1.5 text-sm">
            {report.changes.map((c) => (
              <li key={c.path} className="flex items-center justify-between gap-4">
                <code className="truncate">{c.path}</code>
                <span className="shrink-0 text-faint">+{c.lines_added} / −{c.lines_removed}{c.origin ? ` · ${c.origin}` : ""}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Coverage — honest about what this task can and can't show */}
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-xl border border-hair bg-surface p-5">
          <div className="eyebrow mb-2">This task assesses</div>
          <ul className="space-y-1 text-sm text-dim">
            {report.coverage.competencies.map((c, i) => <li key={i}>• {c}</li>)}
          </ul>
        </div>
        <div className="rounded-xl border border-hair bg-surface p-5">
          <div className="eyebrow mb-2">It can&apos;t assess</div>
          <ul className="space-y-1 text-sm text-faint">
            {report.coverage.cannot_assess.map((c, i) => <li key={i}>• {c}</li>)}
          </ul>
        </div>
      </div>

      {/* Timeline */}
      <div className="rounded-xl border border-hair bg-surface p-5">
        <div className="eyebrow mb-3">Timeline</div>
        <ul className="space-y-1 text-xs">
          {events.map((e) => (
            <li key={e.seq} className="flex gap-3">
              <span className="tabular-nums text-faint">{new Date(e.occurred_at).toLocaleTimeString()}</span>
              <span className="font-medium">{e.type}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
