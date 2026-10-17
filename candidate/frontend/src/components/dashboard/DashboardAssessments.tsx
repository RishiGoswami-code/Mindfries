import Link from "next/link";
import clsx from "clsx";
import { ArrowRight, FileText } from "lucide-react";
import { InvitationAnswer } from "@/components/assessments/InvitationAnswer";
import { sortByAttention, statusLabels, UNAVAILABLE, type Assessment } from "@/lib/dashboard/data";
import { display } from "@/lib/dashboard/fonts";

const PREVIEW_COUNT = 3;

const PILL: Record<Assessment["status"], string> = {
  invited: "bg-[#ECE6FD] text-[#4B2BB8]",
  "in-progress": "bg-[#FBE3F2] text-[#8A2F6B]",
  submitted: "bg-[#DDF5EF] text-[#1E5B4F]",
  closed: "bg-[#EEECF4] text-[#6E6A8F]",
};

/**
 * The dashboard's preview of the candidate's assessments: the three most
 * worth acting on (sortByAttention), with "View all" when there are more.
 *
 * `items` undefined means the list couldn't be read, which is said plainly
 * (UNAVAILABLE) rather than shown as an empty list. Every row's action is
 * the real one: an unanswered invitation asks Accept / Decline first, then
 * Start opens onboarding; in progress resumes the session; submitted opens
 * its report.
 */
export function DashboardAssessments({ items: loaded }: { items?: Assessment[] }) {
  const items = loaded ?? [];
  const preview = sortByAttention(items).slice(0, PREVIEW_COUNT);

  return (
    <section className="rounded-[22px] border border-white/90 bg-white/55 p-6 shadow-[0_10px_30px_-20px_rgba(76,58,160,0.3)] backdrop-blur">
      <div className="flex items-center gap-5">
        <h2 className={clsx(display.className, "text-[25px] font-medium tracking-tight text-[#100E2E]")}>Your assessments</h2>
        <span className="flex h-[30px] min-w-[40px] items-center justify-center rounded-full bg-[#EEEBFA] px-3 text-[14px] font-medium tabular-nums text-[#1B1840]">
          {items.length}
        </span>
        {items.length > PREVIEW_COUNT && (
          <Link href="/assessments" className="ml-auto text-[13.5px] font-medium text-[#5B3BE0] hover:underline">
            View all →
          </Link>
        )}
      </div>

      {preview.length === 0 ? (
        <div className="mt-5 flex flex-col items-center rounded-2xl border border-dashed border-[#DCD7EE] px-6 pt-6 pb-8 text-center">
          <span className="flex h-[78px] w-[78px] items-center justify-center rounded-full bg-[#EFEBFB]">
            <FileText size={32} strokeWidth={1.6} className="text-[#6A47E6]" />
          </span>
          <p className={clsx(display.className, "mt-4 text-[23px] font-medium text-[#100E2E]")}>
            {loaded ? "Nothing here yet" : UNAVAILABLE.title}
          </p>
          <p className="mt-2 max-w-md text-[15px] text-[#7A6CB5]">
            {loaded ? "Assessments you're invited to will show up here." : UNAVAILABLE.body}
          </p>
        </div>
      ) : (
        <ul className="mt-5 space-y-3">
          {preview.map((assessment) => (
            <AssessmentRow key={assessment.id} assessment={assessment} />
          ))}
        </ul>
      )}
    </section>
  );
}

function AssessmentRow({ assessment }: { assessment: Assessment }) {
  const { status, invitation } = assessment;
  const unanswered = status === "invited" && !!invitation && !invitation.accepted;
  const label = invitation?.declined ? "Declined" : status === "invited" && invitation?.accepted ? "Accepted" : statusLabels[status];

  return (
    <li
      className={clsx(
        "flex flex-wrap items-center gap-x-5 gap-y-3 rounded-2xl bg-white/70 px-5 py-4 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.9)]",
        status === "closed" && "opacity-60"
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2.5">
          <span className={clsx("rounded-full px-2.5 py-0.5 text-[11px] font-semibold tracking-wide uppercase", PILL[status])}>{label}</span>
          <span className="text-[12.5px] text-[#6E6A8F]">{assessment.due}</span>
        </div>
        <p className="mt-1.5 truncate text-[16px] font-medium text-[#100E2E]">{assessment.role}</p>
        <p className="truncate text-[13px] text-[#6E6A8F]">
          {[assessment.company, assessment.location, ...assessment.tags].filter(Boolean).join(" · ")}
        </p>
      </div>

      {unanswered && <InvitationAnswer assessmentId={assessment.id} />}

      {status === "invited" && !unanswered && (
        <RowLink href={`/onboarding?template=${encodeURIComponent(assessment.id)}`}>Start</RowLink>
      )}

      {/* The clock is the server's and kept running; resuming picks the session back up. */}
      {status === "in-progress" && assessment.sessionId && (
        <RowLink href={`/ide?session=${encodeURIComponent(assessment.sessionId)}`}>Resume</RowLink>
      )}

      {status === "submitted" && assessment.sessionId && (
        <RowLink href={`/assessments/${encodeURIComponent(assessment.sessionId)}/report`} subtle>
          View report
        </RowLink>
      )}
    </li>
  );
}

function RowLink({ href, children, subtle = false }: { href: string; children: React.ReactNode; subtle?: boolean }) {
  return (
    <Link
      href={href}
      className={clsx(
        "inline-flex shrink-0 items-center gap-1.5 rounded-full px-4 py-2 text-[13px] font-semibold transition-opacity hover:opacity-90",
        subtle ? "bg-[#EEEBFA] text-[#3D22A8]" : "bg-[#1F1B4D] text-white"
      )}
    >
      {children}
      <ArrowRight size={14} />
    </Link>
  );
}
