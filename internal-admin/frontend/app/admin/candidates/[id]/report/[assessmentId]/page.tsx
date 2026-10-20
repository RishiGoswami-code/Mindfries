import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { listCandidateCards } from "@/lib/candidates";
import { getCandidateReport } from "@/lib/report/candidate-report";
import { loadReportActivity } from "@/lib/report/data";
import { CandidateEvidenceReport } from "@/components/report/CandidateEvidenceReport";
import { PageHeader, Pill } from "@/components/ui";
import { RECOMMENDATION } from "@/components/admin/CandidateCard";
import { fmtDate } from "@/lib/format";
import { DecisionForm } from "./DecisionForm";

export const metadata: Metadata = { title: "Evidence report" };

export const dynamic = "force-dynamic";

const uuid = /^[0-9a-f-]{36}$/i;

/**
 * One assessment's complete evidence report, as the company sees it — every
 * tab, the diffs, the timeline, the interview and its recordings, the
 * proctoring flags and stills — and the hiring decision, which only a person
 * makes: the company's reviewer, or a Mindfries admin here.
 */
export default async function AdminReportPage({ params }: { params: Promise<{ id: string; assessmentId: string }> }) {
  const { id, assessmentId } = await params;
  if (!uuid.test(id) || !uuid.test(assessmentId)) notFound();
  const [c] = await listCandidateCards(id);
  const assessment = c?.assessments.find((a) => a.id === assessmentId);
  if (!c || !assessment) notFound();

  const { session, report, interview, camera, screen, flags } = await getCandidateReport(assessmentId);
  const activity = session ? await loadReportActivity(session.id, session.startedAt, null, assessmentId) : null;
  const decision = report?.review?.recommendation ?? null;
  const rec = decision ? RECOMMENDATION[decision] : null;
  const ai = report?.status === "ready" && report.recommendation ? RECOMMENDATION[report.recommendation] : null;

  return (
    <div className="space-y-6">
      <Link href={`/admin/candidates/${id}`} className="text-sm text-dim hover:underline">
        ← {c.name || c.email}
      </Link>
      <PageHeader eyebrow={[assessment.company, assessment.role].filter(Boolean).join(" — ") || "Assessment"} title={`${c.name || c.email} — evidence report`}>
        {`invited ${fmtDate(assessment.invitedAt)}`}
        {report?.sharedWithCandidate ? " · shared with the candidate" : " · not shared with the candidate"}
      </PageHeader>

      <section className="hair-card p-5">
        <div className="eyebrow">Hiring decision</div>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          {rec ? <Pill tone={rec.tone}>{rec.label}</Pill> : <Pill tone="gray">Not decided</Pill>}
          {report?.review?.by && (
            <span className="text-xs text-dim">
              by {report.review.by}
              {report.review.at ? `, ${fmtDate(report.review.at)}` : ""}
            </span>
          )}
          {ai && <span className="text-xs text-dim">AI suggests: {ai.label}</span>}
        </div>
        {report?.review?.note && <p className="mt-2 text-sm text-dim">{report.review.note}</p>}
        <p className="mt-2 text-xs text-faint">The decision is a person&apos;s — the company&apos;s reviewer, or Mindfries. The AI only suggests.</p>
        {report ? (
          <DecisionForm candidateId={id} assessmentId={assessmentId} decision={decision} note={report.review?.note ?? null} />
        ) : (
          <p className="mt-3 text-sm text-dim">There&apos;s no report yet, so there&apos;s nothing to decide on.</p>
        )}
      </section>

      {session ? (
        <CandidateEvidenceReport session={session} report={report} activity={activity} interview={interview} camera={camera} screen={screen} flags={flags} />
      ) : (
        <p className="hair-card p-5 text-sm text-dim">The candidate hasn&apos;t started this assessment, so there is no report.</p>
      )}
    </div>
  );
}
