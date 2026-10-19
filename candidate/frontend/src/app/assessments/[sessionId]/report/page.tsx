import type { Metadata } from "next";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import clsx from "clsx";
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  CheckCircle2,
  ChevronRight,
  Clock,
  Code2,
  FileText,
  Folder,
  Lightbulb,
  ListChecks,
  MessageSquare,
  Play,
  Sparkles,
  SquareTerminal,
  UserRound,
  Workflow,
  type LucideIcon,
} from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { ReportAutoRefresh } from "@/components/assessments/ReportAutoRefresh";
import { DownloadButton } from "@/components/report/DownloadButton";
import { DiffView, Observations, Panel, Pill, TestResults, Timeline } from "@/components/report/Parts";
import { LocalTime } from "@/components/jobs/LocalTime";
import { BackendAuthError, BackendError, getSession, getSessionReport, sessionEventsOrUndefined, type EvidenceItemView, type ReportView, type SessionView } from "@/lib/backend/client";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import type { CandidateProfile } from "@/lib/profile/types";
import { loadReportActivity, type ReportActivity } from "@/lib/report/data";
import { clock, shortTimeline } from "@/lib/report/trail";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Evidence report · Mindfries",
  description: "What the workspace observed during your session.",
};

// Real, per-request data — a report's status can change between one request
// and the next while evaluation is running.
export const dynamic = "force-dynamic";

const RECOMMENDATION: Record<string, { label: string; tone: "good" | "warn" | "bad" }> = {
  strong_hire: { label: "Strong hire", tone: "good" },
  hire: { label: "Hire", tone: "good" },
  lean_no: { label: "Leaning no", tone: "warn" },
  no_hire: { label: "No hire", tone: "bad" },
};

const STATUS_COPY: Record<Exclude<ReportView["status"], "ready">, { title: string; body: string }> = {
  pending: { title: "Waiting to start", body: "Your submission was received. Evaluation hasn't started yet — this page updates on its own." },
  generating: { title: "Generating your report", body: "The evaluation agents are reading your session now. This usually takes under a minute." },
  failed: { title: "Evaluation didn't complete", body: "" },
};

type Section = "summary" | "code" | "workflow" | "reasoning" | "interview" | "artifacts";

const SECTIONS: Array<{ id: Section; label: string; icon: LucideIcon; blurb: string }> = [
  { id: "summary", label: "Summary", icon: FileText, blurb: "" },
  { id: "code", label: "Code evaluation", icon: Code2, blurb: "Changes, diffs and test runs" },
  { id: "workflow", label: "Workflow", icon: Workflow, blurb: "Timeline and activity" },
  { id: "reasoning", label: "Reasoning", icon: Lightbulb, blurb: "Understanding and approach" },
  { id: "interview", label: "Interview", icon: MessageSquare, blurb: "Questions and answers" },
  { id: "artifacts", label: "Artifacts", icon: Folder, blurb: "Your files as you left them" },
];

/** Which agents' observations each tab shows — integrity/proctoring flags are kept strictly on the company side. */
const CATEGORIES: Record<Exclude<Section, "summary" | "artifacts">, string[]> = {
  code: ["code_evaluation", "provenance", "rubric"],
  workflow: ["workflow", "integrity"],
  reasoning: ["reasoning", "ai_usage"],
  interview: ["interview", "interview_status"],
};

async function loadReportData(sessionId: string) {
  try {
    const [session, report] = await Promise.all([getSession(sessionId), getSessionReport(sessionId)]);
    return { session, report };
  } catch (err) {
    if (err instanceof BackendAuthError) redirect(`/login?next=/assessments/${sessionId}/report`);
    if (err instanceof BackendError && err.status === 404) notFound();
    throw err;
  }
}

/**
 * The evidence report for one session.
 *
 * When the candidate submits an assessment, reports are kept strictly on the
 * company side until the hiring team reviews and explicitly releases them.
 * If not yet shared, the candidate sees the Under Review / Submission Received view.
 *
 * Once shared by the company, the candidate can view the evaluation and feedback.
 * Internal proctoring flags and reviewer notes always remain hidden from candidates.
 */
export default async function SessionReportPage({ params, searchParams }: { params: Promise<{ sessionId: string }>; searchParams: Promise<{ section?: string }> }) {
  const [{ sessionId }, { section: wanted }] = await Promise.all([params, searchParams]);
  const section: Section = SECTIONS.some((s) => s.id === wanted) ? (wanted as Section) : "summary";

  const [{ session, report }, viewer, profile] = await Promise.all([loadReportData(sessionId), currentCandidate(), getProfile()]);
  const activity = viewer ? await loadReportActivity(sessionId, viewer.id, session.startedAt) : null;

  // Confidentiality gate: If the hiring team has not released the report to the candidate,
  // show the reassuring "Under Review / Submission Received" state.
  if (!report.sharedWithCandidate) {
    return (
      <SubmissionPendingView
        session={session}
        activity={activity}
        profile={profile}
        viewerName={viewer?.name}
      />
    );
  }

  const isTerminal = report.status === "ready" || report.status === "failed";
  const evidence = report.status === "ready" ? (report.evidence ?? []) : [];
  const href = (s: Section) => `/assessments/${encodeURIComponent(sessionId)}/report${s === "summary" ? "" : `?section=${s}`}`;

  return (
    <CandidateShell sessionName={viewer?.name} profile={profile}>
      {!isTerminal && <ReportAutoRefresh live={await sessionEventsOrUndefined(sessionId)} />}

      <header className="mt-6 flex flex-col gap-5 px-1 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link href="/assessments" className="inline-flex items-center gap-1.5 text-[13.5px] text-[#4C4874] hover:text-[#100E2E] print:hidden">
            <ArrowLeft size={15} /> All assessments
          </Link>
          <div className="mt-3 flex items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-[#E3F7EC] px-3 py-1 text-[11.5px] font-semibold tracking-wide text-[#14693a] uppercase">
              <CheckCircle2 size={13} /> Shared by Company
            </span>
          </div>
          <h1 className={clsx(display.className, "mt-1.5 text-[38px] leading-tight font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[46px]")}>Session {session.startedAt.slice(0, 10)}</h1>
          <p className="mt-1.5 flex flex-wrap gap-x-2.5 text-[15px] text-[#6E6A8F]">
            <span>{session.durationMin} min assessment</span>
            <span aria-hidden>·</span>
            <span>Session {sessionId.slice(0, 8)}</span>
            {activity?.trail.lastAt && (
              <>
                <span aria-hidden>·</span>
                <span>
                  Last activity <LocalTime iso={activity.trail.lastAt} />
                </span>
              </>
            )}
          </p>
        </div>
        <DownloadButton />
      </header>

      {/* Shared announcement banner */}
      <div className="mt-4 flex items-center gap-3 rounded-2xl border border-[#CDEBD9] bg-[#F1FAF4] px-4 py-3 text-[14px] text-[#14693a]">
        <CheckCircle2 size={18} className="shrink-0 text-[#1A9E6B]" />
        <span>
          <strong>Report released:</strong> The hiring team has reviewed your submission and shared this assessment report with you
          {report.sharedAt ? <> on <LocalTime iso={report.sharedAt} /></> : ""}.
        </span>
      </div>

      <nav className="mt-6 flex gap-1 overflow-x-auto rounded-[20px] border border-white/90 bg-white/70 px-2 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur [scrollbar-width:none] print:hidden" aria-label="Report sections">
        {SECTIONS.map(({ id, label, icon: Icon }) => (
          <Link
            key={id}
            href={href(id)}
            aria-current={section === id ? "page" : undefined}
            className={clsx(
              "relative flex shrink-0 items-center gap-2.5 px-4 py-4 text-[15px] transition-colors",
              section === id ? "font-semibold text-[#4B2BB8] after:absolute after:inset-x-3 after:bottom-0 after:h-[3px] after:rounded-full after:bg-[#5B3BE0]" : "text-[#2A2654] hover:text-[#100E2E]"
            )}
          >
            <Icon size={19} strokeWidth={1.8} />
            {label}
          </Link>
        ))}
      </nav>

      <div className="mt-5 space-y-5">
        {activity && !activity.available && (
          <p className="rounded-2xl border border-[#F3DDBA] bg-[#FFF8EC] px-4 py-3 text-[13.5px] text-[#7A4B07]">
            The session&apos;s activity couldn&apos;t be read here, so only the evaluation is shown.
          </p>
        )}

        {section === "summary" && <Summary report={report} activity={activity} durationMin={session.durationMin} href={href} />}
        {section === "code" && <CodeSection activity={activity} evidence={evidence} ready={report.status === "ready"} />}
        {section === "workflow" && <WorkflowSection activity={activity} evidence={evidence} ready={report.status === "ready"} />}
        {section === "reasoning" && <ReasoningSection activity={activity} evidence={evidence} ready={report.status === "ready"} />}
        {section === "interview" && <InterviewSection activity={activity} evidence={evidence} ready={report.status === "ready"} />}
        {section === "artifacts" && <ArtifactsSection activity={activity} />}
      </div>
    </CandidateShell>
  );
}

function pick(evidence: EvidenceItemView[], section: keyof typeof CATEGORIES) {
  return evidence.filter((e) => CATEGORIES[section].includes(e.category));
}

const notReady = "The evaluation hasn't finished, so there are no observations here yet.";

// ── Submission Pending View (Shown before Company Releases Report) ────────────

function SubmissionPendingView({
  session,
  activity,
  profile,
  viewerName,
}: {
  session: SessionView;
  activity: ReportActivity | null;
  profile: CandidateProfile | null;
  viewerName?: string;
}) {
  const trail = activity?.trail;
  const lastTests = trail?.lastTests ?? null;

  return (
    <CandidateShell sessionName={viewerName} profile={profile}>
      <header className="mt-6 flex flex-col gap-5 px-1 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link href="/assessments" className="inline-flex items-center gap-1.5 text-[13.5px] text-[#4C4874] hover:text-[#100E2E]">
            <ArrowLeft size={15} /> All assessments
          </Link>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full bg-[#EFEAFD] px-3 py-1 text-[11.5px] font-semibold tracking-wide text-[#4B2BB8] uppercase">
              <CheckCircle2 size={13} /> Submission Received
            </span>
            <span className="inline-flex items-center gap-1.5 rounded-full bg-[#FFF8EC] px-3 py-1 text-[11.5px] font-semibold tracking-wide text-[#B45309] uppercase">
              <Clock size={13} /> Under Review
            </span>
          </div>
          <h1 className={clsx(display.className, "mt-2 text-[36px] leading-tight font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[44px]")}>
            Assessment Submitted
          </h1>
          <p className="mt-1.5 flex flex-wrap gap-x-2.5 text-[15px] text-[#6E6A8F]">
            <span>{session.durationMin} min assessment</span>
            <span aria-hidden>·</span>
            <span>Session {session.id.slice(0, 8)}</span>
            {activity?.trail.lastAt && (
              <>
                <span aria-hidden>·</span>
                <span>Submitted <LocalTime iso={activity.trail.lastAt} /></span>
              </>
            )}
          </p>
        </div>
      </header>

      <div className="mt-6 rounded-[24px] border border-white/90 bg-white/80 p-6 shadow-[0_12px_36px_-24px_rgba(76,58,160,0.35)] backdrop-blur sm:p-8">
        <div className="flex flex-col gap-6 sm:flex-row sm:items-start">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br from-[#EFEAFD] to-[#E2D9FC] text-[#5B3BE0] shadow-sm">
            <Sparkles size={28} />
          </span>
          <div className="min-w-0 flex-1">
            <h2 className="text-[22px] font-bold tracking-tight text-[#100E2E]">
              Thank you for completing your assessment!
            </h2>
            <p className="mt-2 text-[15px] leading-relaxed text-[#4C4874]">
              Your code, test runs, and interview answers have been safely received and delivered to the hiring team.
            </p>
            <div className="mt-4 rounded-2xl border border-[#F1EDFC] bg-[#FAF9FE] p-4 text-[14px] leading-relaxed text-[#5E5A82]">
              <p className="font-semibold text-[#100E2E]">What happens next?</p>
              <p className="mt-1">
                The hiring team is reviewing your assessment. To ensure a fair and comprehensive evaluation, detailed assessment reports remain private to the company until they are ready to release them.
              </p>
              <p className="mt-2">
                Once the company finishes reviewing your submission and releases the report, your evaluation results and feedback will become visible right here. You will also receive an in-app notification on your dashboard.
              </p>
            </div>
          </div>
        </div>

        <div className="mt-8 border-t border-[#F0EEF7] pt-6">
          <p className="text-[13px] font-semibold tracking-wider text-[#6E6A8F] uppercase">
            Submission overview
          </p>
          <div className="mt-3 grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="rounded-2xl border border-[#ECE9F6] bg-white p-4 shadow-sm">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#EAF1FE] text-[#2F6BDE]">
                <Clock size={18} />
              </span>
              <p className="mt-3 text-[13px] text-[#6E6A8F]">Time worked</p>
              <p className="mt-0.5 text-[20px] font-bold text-[#100E2E]">{trail?.workMinutes ?? session.elapsedMin} min</p>
              <p className="text-[12px] text-[#8A86A8]">of {session.durationMin} min allowed</p>
            </div>

            <div className="rounded-2xl border border-[#ECE9F6] bg-white p-4 shadow-sm">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#EFEAFD] text-[#5B3BE0]">
                <FileText size={18} />
              </span>
              <p className="mt-3 text-[13px] text-[#6E6A8F]">Files modified</p>
              <p className="mt-0.5 text-[20px] font-bold text-[#100E2E]">{activity?.diffs.length ?? trail?.editedPaths.length ?? 0}</p>
              <p className="text-[12px] text-[#8A86A8]">changes recorded</p>
            </div>

            <div className="rounded-2xl border border-[#ECE9F6] bg-white p-4 shadow-sm">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#E3F7EC] text-[#1A9E6B]">
                <Play size={18} />
              </span>
              <p className="mt-3 text-[13px] text-[#6E6A8F]">Test runs</p>
              <p className="mt-0.5 text-[20px] font-bold text-[#100E2E]">
                {lastTests && lastTests.passed !== null ? `${lastTests.passed} passed` : "Completed"}
              </p>
              <p className="text-[12px] text-[#8A86A8]">
                {lastTests?.failed ? `${lastTests.failed} failing` : "tests recorded"}
              </p>
            </div>

            <div className="rounded-2xl border border-[#ECE9F6] bg-white p-4 shadow-sm">
              <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#EFEAFD] text-[#5B3BE0]">
                <MessageSquare size={18} />
              </span>
              <p className="mt-3 text-[13px] text-[#6E6A8F]">Interview</p>
              <p className="mt-0.5 text-[20px] font-bold text-[#100E2E]">Recorded</p>
              <p className="text-[12px] text-[#8A86A8]">answers submitted</p>
            </div>
          </div>
        </div>

        <div className="mt-8 flex flex-wrap items-center gap-3 border-t border-[#F0EEF7] pt-6">
          <Link
            href="/assessments"
            className="inline-flex items-center gap-2 rounded-full bg-[#1F1B4D] px-5 py-2.5 text-[14px] font-semibold text-white transition-opacity hover:opacity-90"
          >
            Back to assessments
          </Link>
          <Link
            href="/dashboard"
            className="inline-flex items-center gap-2 rounded-full border border-[#DCD6F4] bg-white px-5 py-2.5 text-[14px] font-semibold text-[#2A2654] transition-colors hover:bg-[#FAF9FE]"
          >
            Candidate dashboard
          </Link>
        </div>
      </div>
    </CandidateShell>
  );
}

// ── Summary ────────────────────────────────────────────────────────────────

function Summary({ report, activity, durationMin, href }: { report: ReportView; activity: ReportActivity | null; durationMin: number; href: (s: Section) => string }) {
  const trail = activity?.trail;
  const top = activity?.diffs[0];
  const lastTests = trail?.lastTests ?? null;
  const incomplete = report.status === "ready" ? (report.evidence ?? []).filter((e) => e.category === "incomplete") : [];
  const cleanTimeline = (trail?.timeline ?? []).filter((t) => t.tone !== "flag");

  const tiles: Array<{ icon: LucideIcon; label: string; value: React.ReactNode; note?: React.ReactNode; tint: string; href?: string }> = [
    { icon: Clock, label: "Time worked", value: `${trail?.workMinutes ?? 0} min`, note: `of ${durationMin} allowed`, tint: "bg-[#EAF1FE] text-[#2F6BDE]" },
    {
      icon: FileText,
      label: "Files modified",
      value: activity?.diffs.length ?? trail?.editedPaths.length ?? 0,
      note: (activity?.diffs.map((d) => d.path) ?? trail?.editedPaths ?? []).slice(0, 2).join(", ") || undefined,
      tint: "bg-[#EFEAFD] text-[#5B3BE0]",
    },
    {
      icon: Play,
      label: "Tests",
      value: lastTests && lastTests.passed !== null ? `${lastTests.passed} / ${lastTests.passed + (lastTests.failed ?? 0)}` : "—",
      note: lastTests ? <span className={lastTests.failed ? "text-[#9A4B07]" : "text-[#14693a]"}>{lastTests.failed ? `${lastTests.failed} failing` : "passed"}</span> : "none run",
      tint: "bg-[#E3F7EC] text-[#1A9E6B]",
    },
    { icon: SquareTerminal, label: "Terminal commands", value: trail?.terminalCommands ?? 0, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
    {
      icon: CheckCircle2,
      label: "Report status",
      value: "Released",
      note: "Shared by company",
      tint: "bg-[#E3F7EC] text-[#1A9E6B]",
    },
  ];

  return (
    <>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,2fr)]">
        <RecommendationCard report={report} />
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-5">
          {tiles.map((t) => {
            const body = (
              <>
                <span className={clsx("flex h-10 w-10 items-center justify-center rounded-xl", t.tint)}>
                  <t.icon size={20} />
                </span>
                <p className="mt-4 text-[13.5px] text-[#6E6A8F]">{t.label}</p>
                <p className="mt-1 text-[22px] font-semibold text-[#100E2E] tabular-nums">{t.value}</p>
                {t.note && <p className="mt-0.5 truncate text-[13px] text-[#6E6A8F]">{t.note}</p>}
              </>
            );
            const cls = "relative block min-w-0 rounded-[20px] border border-white/90 bg-white/75 p-4 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)]";
            return t.href ? (
              <Link key={t.label} href={t.href} className={clsx(cls, "hover:bg-white")}>
                {body}
                <ChevronRight size={17} className="absolute top-1/2 right-3 text-[#8A86A8]" />
              </Link>
            ) : (
              <div key={t.label} className={cls}>
                {body}
              </div>
            );
          })}
        </div>
      </div>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)]">
        <div className="min-w-0 space-y-5">
          <Panel
            icon={<Code2 size={21} />}
            title="Code changes"
            action={
              top ? (
                <Link href={href("code")} className="inline-flex items-center gap-1.5 text-[14px] font-medium text-[#5B3BE0] hover:underline print:hidden">
                  View full diff <ArrowRight size={15} />
                </Link>
              ) : undefined
            }
          >
            {!top ? (
              <p className="text-[14px] text-[#6E6A8F]">
                {activity?.finalFiles ? "The files you left are the same as the task's." : "No saved copy of your files was recorded, so there's nothing to compare."}
              </p>
            ) : (
              <>
                <div className="mb-3 flex flex-wrap items-center gap-2">
                  <span className="mr-1 inline-flex items-center gap-2 text-[15px] font-medium text-[#100E2E]">
                    <FileText size={17} /> {top.path}
                  </span>
                  <Pill tone="green">+{top.added} lines</Pill>
                  <Pill tone="red">-{top.removed} lines</Pill>
                  {(activity?.diffs.length ?? 0) > 1 && <Pill tone="gray">+{activity!.diffs.length - 1} more files</Pill>}
                </div>
                <DiffView diff={top} window={14} />
              </>
            )}
          </Panel>

          <Panel icon={<ListChecks size={21} />} title="Test results">
            <TestResults run={lastTests} />
          </Panel>
        </div>

        <div className="min-w-0 space-y-5">
          <Panel icon={<Workflow size={21} />} title="Session timeline">
            {cleanTimeline.length > 0 ? (
              <>
                <Timeline items={shortTimeline(cleanTimeline).items} />
                <Link href={href("workflow")} className="mt-2 inline-flex items-center gap-1.5 text-[13.5px] font-medium text-[#5B3BE0] hover:underline print:hidden">
                  Full timeline <ArrowRight size={14} />
                </Link>
              </>
            ) : (
              <p className="text-[14px] text-[#6E6A8F]">No activity was recorded for this session.</p>
            )}
          </Panel>

          <Panel icon={<SquareTerminal size={20} />} title="Session highlights">
            <dl className="grid grid-cols-2 gap-3 text-[14px]">
              {[
                ["Terminal commands", trail?.terminalCommands ?? 0],
                ["Test runs", trail?.testRuns.length ?? 0],
                ["Time worked", `${trail?.workMinutes ?? 0} min`],
                ["Questions to assistant", trail?.assistant.filter((t) => t.role === "candidate").length ?? 0],
              ].map(([k, v]) => (
                <div key={String(k)} className="rounded-xl bg-[#FAF9FE] px-3 py-2.5">
                  <dt className="text-[12.5px] text-[#6E6A8F]">{k}</dt>
                  <dd className="text-[17px] font-semibold text-[#100E2E] tabular-nums">{v}</dd>
                </div>
              ))}
            </dl>
          </Panel>
        </div>
      </div>

      {incomplete.length > 0 && (
        <Panel title="Missing from this report">
          <Observations items={incomplete} empty="" />
        </Panel>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4 print:hidden">
        {SECTIONS.filter((s) => s.id !== "summary" && s.id !== "artifacts").map((s) => (
          <Link key={s.id} href={href(s.id)} className="flex items-center gap-3.5 rounded-[20px] border border-white/90 bg-white/75 p-4 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] hover:bg-white">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#EFEAFD] text-[#5B3BE0]">
              <s.icon size={21} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-[15px] font-semibold text-[#100E2E]">{s.label}</span>
              <span className="block truncate text-[13px] text-[#6E6A8F]">{s.blurb}</span>
            </span>
            <ChevronRight size={18} className="text-[#8A86A8]" />
          </Link>
        ))}
      </div>
    </>
  );
}

function RecommendationCard({ report }: { report: ReportView }) {
  if (report.status !== "ready") {
    const copy = STATUS_COPY[report.status];
    return (
      <div className="rounded-[20px] border border-[#E4E0F2] bg-white/80 p-5">
        <div className="flex items-center gap-2.5">
          {report.status !== "failed" && <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-[#5B3BE0]" aria-hidden />}
          <p className="text-[16px] font-semibold text-[#100E2E]">{copy.title}</p>
        </div>
        <p className="mt-2 text-[14px] leading-relaxed text-[#5E5A82]">{report.status === "failed" ? (report.error ?? "No reason was recorded.") : copy.body}</p>
      </div>
    );
  }
  const rec = report.recommendation ? RECOMMENDATION[report.recommendation] : undefined;
  const tone = rec?.tone ?? "none";
  return (
    <div
      className={clsx(
        "rounded-[20px] border p-5",
        tone === "bad" && "border-[#F7D5DC] bg-[#FDF1F3]",
        tone === "warn" && "border-[#F3DDBA] bg-[#FFF8EC]",
        tone === "good" && "border-[#CDEBD9] bg-[#F1FAF4]",
        tone === "none" && "border-[#E4E0F2] bg-white/80"
      )}
    >
      <div className="flex items-start gap-4">
        <span
          className={clsx(
            "flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-white",
            tone === "bad" ? "text-[#DC2643]" : tone === "warn" ? "text-[#C26410]" : tone === "good" ? "text-[#1A9E6B]" : "text-[#5B3BE0]"
          )}
        >
          <UserRound size={24} />
        </span>
        <div className="min-w-0">
          <p className="text-[14.5px] font-semibold text-[#100E2E]">Recommendation</p>
          <p className={clsx("text-[28px] leading-tight font-bold", tone === "bad" ? "text-[#DC2643]" : tone === "warn" ? "text-[#B45309]" : tone === "good" ? "text-[#15803D]" : "text-[#4C4874]")}>
            {rec?.label ?? "Not given"}
          </p>
        </div>
      </div>
      {report.summary && (
        <div className="mt-4 rounded-2xl bg-white/80 px-4 py-3">
          <p className="text-[14px] font-semibold text-[#100E2E]">Key reason</p>
          <p className={clsx("mt-1 line-clamp-5 text-[14px] leading-relaxed", tone === "bad" ? "text-[#8F1D33]" : "text-[#4C4874]")}>{report.summary.replace(/ ?\[E\d+\]/g, "")}</p>
        </div>
      )}
      <p className="mt-3 text-[12px] text-[#8A86A8]">Evaluation summary shared by the hiring team.</p>
    </div>
  );
}

// ── The other tabs ─────────────────────────────────────────────────────────

function CodeSection({ activity, evidence, ready }: { activity: ReportActivity | null; evidence: EvidenceItemView[]; ready: boolean }) {
  const diffs = activity?.diffs ?? [];
  const runs = activity?.trail.testRuns ?? [];
  return (
    <>
      <Panel icon={<Code2 size={21} />} title="What the code evaluation found">
        <Observations items={pick(evidence, "code")} empty={ready ? "The code evaluation recorded no observations." : notReady} />
      </Panel>
      <Panel icon={<FileText size={20} />} title={`Files changed (${diffs.length})`}>
        {diffs.length === 0 ? (
          <p className="text-[14px] text-[#6E6A8F]">{activity?.finalFiles ? "The files you left are the same as the task's." : "No saved copy of your files was recorded."}</p>
        ) : (
          <div className="space-y-6">
            {diffs.map((d) => (
              <div key={d.path}>
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <span className="mr-1 font-mono text-[14px] font-medium text-[#100E2E]">{d.path}</span>
                  {!d.beforeExists && <Pill tone="violet">New file</Pill>}
                  {!d.afterExists && <Pill tone="gray">Deleted</Pill>}
                  <Pill tone="green">+{d.added}</Pill>
                  <Pill tone="red">-{d.removed}</Pill>
                </div>
                <div className="max-h-[560px] overflow-auto rounded-2xl">
                  <DiffView diff={d} />
                </div>
              </div>
            ))}
          </div>
        )}
      </Panel>
      <Panel icon={<Play size={20} />} title={`Test runs (${runs.length})`}>
        {runs.length === 0 ? (
          <p className="text-[14px] text-[#6E6A8F]">No test runs were recorded.</p>
        ) : (
          <ul className="divide-y divide-[#F0EEF7]">
            {runs.map((r) => (
              <li key={r.id} className="flex flex-wrap items-center gap-x-4 gap-y-1 py-3">
                <span className="w-14 font-mono text-[13px] text-[#6E6A8F]">{clock(r.offset)}</span>
                <code className="min-w-0 flex-1 truncate font-mono text-[13px] text-[#1B1840]">{r.command}</code>
                {r.passed === null ? <Pill tone="gray">exit {r.exitCode}</Pill> : <Pill tone={r.failed ? "amber" : "green"}>{r.passed} passed · {r.failed} failed</Pill>}
                {r.failing.length > 0 && <p className="w-full pl-[72px] text-[12.5px] text-[#9A4B07]">Failing: {r.failing.join(", ")}</p>}
              </li>
            ))}
          </ul>
        )}
      </Panel>
    </>
  );
}

function WorkflowSection({ activity, evidence, ready }: { activity: ReportActivity | null; evidence: EvidenceItemView[]; ready: boolean }) {
  const trail = activity?.trail;
  const cleanTimeline = (trail?.timeline ?? []).filter((t) => t.tone !== "flag");
  return (
    <div className="grid gap-5 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
      <div className="min-w-0 space-y-5">
        <Panel icon={<Workflow size={21} />} title="How you worked">
          <Observations items={pick(evidence, "workflow")} empty={ready ? "The workflow evaluation recorded no observations." : notReady} />
        </Panel>
        <Panel icon={<Clock size={20} />} title="Full timeline">
          {cleanTimeline.length > 0 ? <Timeline items={cleanTimeline} /> : <p className="text-[14px] text-[#6E6A8F]">No activity was recorded.</p>}
        </Panel>
      </div>
      <div className="min-w-0 space-y-5">
        <Panel icon={<SquareTerminal size={20} />} title="At a glance">
          <dl className="grid grid-cols-2 gap-3 text-[14px]">
            {[
              ["Terminal commands", trail?.terminalCommands ?? 0],
              ["Test runs", trail?.testRuns.length ?? 0],
              ["Pastes", trail?.pastes.length ?? 0],
              ["Time away", `${trail?.awayMinutes ?? 0} min`],
              ["Questions to the assistant", trail?.assistant.filter((t) => t.role === "candidate").length ?? 0],
              ["Files edited", trail?.editedPaths.length ?? 0],
            ].map(([k, v]) => (
              <div key={String(k)} className="rounded-xl bg-[#FAF9FE] px-3 py-2.5">
                <dt className="text-[12.5px] text-[#6E6A8F]">{k}</dt>
                <dd className="text-[17px] font-semibold text-[#100E2E] tabular-nums">{v}</dd>
              </div>
            ))}
          </dl>
        </Panel>
      </div>
    </div>
  );
}

function Conversation({ turns, you, them }: { turns: Array<{ id: number; offset: number; role: "candidate" | "other"; text: string }>; you: string; them: string }) {
  return (
    <ol className="space-y-3">
      {turns.map((t) => (
        <li key={t.id} className={clsx("max-w-[88%] rounded-2xl px-4 py-3", t.role === "candidate" ? "ml-auto bg-[#EFEAFD]" : "bg-[#F6F5FA]")}>
          <p className="text-[12px] text-[#6E6A8F]">
            {t.role === "candidate" ? you : them} · {clock(t.offset)}
          </p>
          <p className="mt-1 text-[14px] leading-relaxed whitespace-pre-line text-[#1B1840]">{t.text}</p>
        </li>
      ))}
    </ol>
  );
}

function ReasoningSection({ activity, evidence, ready }: { activity: ReportActivity | null; evidence: EvidenceItemView[]; ready: boolean }) {
  const turns = activity?.trail.assistant ?? [];
  return (
    <>
      <Panel icon={<Lightbulb size={21} />} title="Understanding and approach">
        <Observations items={pick(evidence, "reasoning")} empty={ready ? "The reasoning evaluation recorded no observations." : notReady} />
      </Panel>
      <Panel icon={<Bot size={21} />} title={`Conversation with the workspace assistant (${turns.filter((t) => t.role === "candidate").length} questions)`}>
        {turns.length === 0 ? <p className="text-[14px] text-[#6E6A8F]">You didn&apos;t use the assistant in this session.</p> : <Conversation turns={turns} you="You" them="Assistant" />}
      </Panel>
    </>
  );
}

function InterviewSection({ activity, evidence, ready }: { activity: ReportActivity | null; evidence: EvidenceItemView[]; ready: boolean }) {
  const turns = activity?.trail.interview ?? [];
  return (
    <>
      <Panel icon={<MessageSquare size={21} />} title="What the interview showed">
        <Observations items={pick(evidence, "interview")} empty={ready ? "The interview evaluation recorded no observations." : notReady} />
      </Panel>
      <Panel icon={<UserRound size={21} />} title="Transcript">
        {turns.length === 0 ? <p className="text-[14px] text-[#6E6A8F]">No interview was recorded for this session.</p> : <Conversation turns={turns} you="You" them="Interviewer" />}
      </Panel>
    </>
  );
}

function ArtifactsSection({ activity }: { activity: ReportActivity | null }) {
  const files = activity?.finalFiles ? Object.entries(activity.finalFiles).sort(([a], [b]) => a.localeCompare(b)) : [];
  return (
    <div className="max-w-4xl">
      <Panel icon={<Folder size={20} />} title={`Your files (${files.length})`}>
        {files.length === 0 ? (
          <p className="text-[14px] text-[#6E6A8F]">No saved copy of your files was recorded.</p>
        ) : (
          <>
            {activity?.finalAt && (
              <p className="mb-3 text-[13px] text-[#6E6A8F]">
                As saved <LocalTime iso={activity.finalAt} />.
              </p>
            )}
            <div className="space-y-2">
              {files.map(([path, text]) => (
                <details key={path} className="group rounded-2xl border border-[#ECE9F6] bg-white">
                  <summary className="flex cursor-pointer list-none items-center gap-2.5 px-4 py-3 text-[14px] font-medium text-[#100E2E]">
                    <ChevronRight size={16} className="text-[#8A86A8] transition-transform group-open:rotate-90" />
                    <span className="min-w-0 flex-1 truncate font-mono">{path.replace(/^\/+/, "")}</span>
                    <span className="text-[12.5px] font-normal text-[#8A86A8]">{String(text).split("\n").length} lines</span>
                  </summary>
                  <pre className="max-h-[480px] overflow-auto border-t border-[#ECE9F6] px-4 py-3 font-mono text-[12.5px] leading-[1.6] text-[#1B1840]">{String(text)}</pre>
                </details>
              ))}
            </div>
          </>
        )}
      </Panel>
    </div>
  );
}
