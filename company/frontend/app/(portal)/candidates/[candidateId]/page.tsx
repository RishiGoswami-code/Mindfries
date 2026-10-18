import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { currentCompanyUser } from "@/lib/auth/company-users";
import { can } from "@/lib/auth/permissions";
import { getApplicationForCompany, getCandidateReport, getInvitationResponse, getJobRole } from "@/lib/db";
import { listApplicationEmails, listInterviews } from "@/lib/hiring";
import { getCandidateProfile } from "@/lib/candidate-profile";
import { EmptyState, PageHeader, Pill } from "@/components/ui";
import { fmtDate, recommendationLabel, sessionStatusTone, stageLabel, stageTone } from "@/lib/format";
import { CandidateProfileCard } from "./CandidateProfileCard";
import { CandidateSnapshot } from "./CandidateSnapshot";
import { EvidenceList } from "./EvidenceList";
import { ReviewerDecisionForm } from "./ReviewerDecisionForm";
import { ReportAutoRefresh } from "./ReportAutoRefresh";
import { ReportShareControl } from "./ReportShareControl";
import { ApplicantCard, EmailPanel, InterviewsPanel } from "./HiringPanels";
import { loadReportActivity } from "@/lib/report/data";
import { CandidateEvidenceReport } from "@/components/report/CandidateEvidenceReport";

/** The tab shows the candidate's name. */
export async function generateMetadata({ params }: { params: Promise<{ candidateId: string }> }): Promise<Metadata> {
  const user = await currentCompanyUser();
  if (!user) return { title: "Candidate" };
  const application = await getApplicationForCompany(user.companyId, (await params).candidateId);
  return { title: application ? (application.candidateName ?? application.candidateEmail) : "Candidate" };
}

export const dynamic = "force-dynamic";

export default async function CandidateDetailPage({ params }: { params: Promise<{ candidateId: string }> }) {
  const { candidateId } = await params;
  const user = await currentCompanyUser();
  if (!user) notFound();

  const application = await getApplicationForCompany(user.companyId, candidateId);
  if (!application) notFound();

  const [{ session, report, interview, camera, screen, flags }, profile, invitation, role, interviews, emails] = await Promise.all([
    getCandidateReport(application.assessmentId),
    getCandidateProfile(application.candidateEmail),
    getInvitationResponse(user.companyId, application.assessmentId),
    getJobRole(user.companyId, application.jobRoleId),
    listInterviews(user.companyId, application.id),
    listApplicationEmails(user.companyId, application.id),
  ]);

  const activity = session
    ? await loadReportActivity(session.id, session.startedAt, null, application.assessmentId)
    : null;

  const closed = application.stage === "withdrawn" || application.stage === "declined";
  const pending = report?.status === "pending" || report?.status === "generating";
  const canReview = can("candidate:stage", user.role);

  return (
    <div className="space-y-6">
      <PageHeader eyebrow={application.roleTitle} title={application.candidateName ?? application.candidateEmail}>
        {application.candidateEmail}
      </PageHeader>

      <div className="flex flex-wrap items-center gap-1.5">
        <Pill tone={stageTone[application.stage]}>{stageLabel[application.stage]}</Pill>
        {application.score != null && <span className="mono text-sm text-dim">{application.score}%</span>}
        <span className="text-xs text-faint">
          {application.source === "applied" ? "Applied" : "Invited"} {fmtDate(application.createdAt)}
        </span>
        {invitation && (
          <span className="text-xs text-faint">
            · {invitation.emailedAt ? `emailed ${fmtDate(invitation.emailedAt)}` : "no invitation email was sent"}
            {invitation.acceptedAt && !invitation.declinedAt ? ` · accepted ${fmtDate(invitation.acceptedAt)}` : ""}
          </span>
        )}
      </div>

      {invitation?.declinedAt && (
        <div className="hair-card p-5">
          <div className="eyebrow">Declined by the candidate</div>
          <p className="mt-2 text-sm">
            {fmtDate(invitation.declinedAt)}
            {invitation.declineReason ? "" : " — no reason given."}
          </p>
          {invitation.declineReason && <p className="mt-2 text-sm whitespace-pre-line text-dim">&ldquo;{invitation.declineReason}&rdquo;</p>}
        </div>
      )}

      <CandidateSnapshot
        name={application.candidateName ?? profile?.name ?? ""}
        email={application.candidateEmail}
        profile={profile}
        score={application.score}
        sectionScores={application.sectionScores}
        recommendation={
          report?.review?.recommendation
            ? { value: report.review.recommendation, by: "reviewer" }
            : report?.status === "ready" && report.recommendation
              ? { value: report.recommendation, by: "ai" }
              : null
        }
        sessionStatus={session?.status ?? null}
      />

      {application.stage === "withdrawn" && (
        <div className="hair-card p-5">
          <div className="eyebrow">Withdrawn by the candidate</div>
          <p className="mt-2 text-sm text-dim">They pulled their application from the job board. Nothing more can be scheduled or sent for it.</p>
        </div>
      )}

      {application.source === "applied" && !application.assessmentId && !closed && (
        <ApplicantCard
          roleId={application.jobRoleId}
          applicationId={application.id}
          coverNote={application.coverNote}
          selfAssessment={role?.posting.selfAssessment ?? false}
          canInvite={can("candidate:invite", user.role)}
        />
      )}

      <CandidateProfileCard profile={profile} />

      <InterviewsPanel applicationId={application.id} interviews={interviews} canEdit={canReview && !closed} />
      <EmailPanel applicationId={application.id} emails={emails} canSend={canReview && !closed} candidateEmail={application.candidateEmail} />

      {!session && (invitation?.declinedAt || !application.assessmentId) ? null : !session ? (
        <EmptyState title="Hasn't started yet" hint="A session and report show up here once the candidate begins their assessment." />
      ) : (
        <>
          <div className="hair-card p-5">
            <div className="eyebrow">Session</div>
            <div className="mt-2 flex flex-wrap items-center gap-3 text-sm">
              <Pill tone={sessionStatusTone[session.status] ?? "gray"}>{session.status}</Pill>
              <span className="mono text-dim">
                {session.elapsedMin}/{session.durationMin}m
              </span>
              <span className="text-dim" title="Where the candidate worked. The browser workspace runs Python and JavaScript for real but can't install every package or run servers.">
                {session.workspace === "sandbox" ? "Worked on a sandbox machine" : "Worked in the browser workspace"}
              </span>
            </div>
          </div>

          {report && (
            <ReportShareControl
              applicationId={application.id}
              sharedWithCandidate={report.sharedWithCandidate}
              sharedAt={report.sharedAt}
              sharedBy={report.sharedBy}
              canManage={canReview}
            />
          )}

          {report?.review && (
            <div className="hair-card p-5">
              <div className="eyebrow">Reviewer&apos;s decision</div>
              <div className="mt-2 text-lg font-extrabold tracking-tight">
                {report.review.recommendation ? (recommendationLabel[report.review.recommendation] ?? report.review.recommendation) : "Not decided"}
              </div>
              {report.review.note && <p className="mt-2 text-sm whitespace-pre-line text-dim">{report.review.note}</p>}
              {report.review.by && (
                <p className="mt-2 text-xs text-faint">
                  {report.review.by}
                  {report.review.at ? ` · ${fmtDate(report.review.at)}` : ""}
                </p>
              )}
            </div>
          )}

          {canReview && report && (
            <div className="hair-card p-5">
              <div className="eyebrow">Review candidate</div>
              <div className="mt-2">
                <ReviewerDecisionForm applicationId={application.id} review={report.review} />
              </div>
            </div>
          )}

          {/* Full candidate evidence report with all tabs, diffs, timeline, interview video/audio replay, artifacts, and proctoring snapshots */}
          <CandidateEvidenceReport
            session={session}
            report={report}
            activity={activity}
            interview={interview}
            camera={camera}
            screen={screen}
            flags={flags}
          />

          {report && report.evidence.length > 0 && (
            <div className="space-y-4 pt-4 border-t border-hair">
              <div className="eyebrow">Evidence citations & annotations</div>
              <EvidenceList
                applicationId={application.id}
                evidence={report.evidence}
                moments={report.moments}
                annotations={report.annotations}
                canAnnotate={canReview}
              />
            </div>
          )}

          {pending && <ReportAutoRefresh />}
        </>
      )}
    </div>
  );
}

