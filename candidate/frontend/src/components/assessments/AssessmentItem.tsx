import Link from "next/link";
import clsx from "clsx";
import { ArrowRight } from "lucide-react";
import { InvitationAnswer } from "./InvitationAnswer";
import { statusLabels, type Assessment } from "@/lib/dashboard/data";
import { display } from "@/lib/dashboard/fonts";

/**
 * One assessment, as a row (the dashboard and the Assessments list view) or
 * a card (the grid view). Both share `AssessmentAction`, so what a button
 * does can't differ between the two:
 *
 * - an invitation not yet answered asks Accept / Decline first;
 * - then, once the candidate's own task has been made, Start opens the
 *   onboarding wizard — until then it says the task is being prepared;
 * - in progress resumes the session — the clock is the server's and kept
 *   running, so nothing is gained by leaving and coming back;
 * - submitted opens its report.
 */

const PILL: Record<Assessment["status"], string> = {
  invited: "bg-[#ECE6FD] text-[#4B2BB8]",
  "in-progress": "bg-[#FBE3F2] text-[#8A2F6B]",
  submitted: "bg-[#DDF5EF] text-[#1E5B4F]",
  closed: "bg-[#EEECF4] text-[#6E6A8F]",
};

function statusText({ status, invitation }: Assessment): string {
  if (invitation?.declined) return "Declined";
  if (status === "invited" && invitation?.accepted) return "Accepted";
  return statusLabels[status];
}

function details(a: Assessment): string {
  return [a.company, a.location, ...a.tags].filter(Boolean).join(" · ");
}

export function AssessmentRow({ assessment }: { assessment: Assessment }) {
  return (
    <li
      className={clsx(
        "flex flex-wrap items-center gap-x-5 gap-y-3 rounded-2xl bg-white/70 px-5 py-4 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.9)]",
        assessment.status === "closed" && "opacity-60"
      )}
    >
      {/* At least 220px wide, so on a phone the action wraps onto its own line instead of squeezing the role name. */}
      <div className="min-w-[220px] flex-1">
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <span className={clsx("rounded-full px-2.5 py-0.5 text-[11px] font-semibold tracking-wide uppercase", PILL[assessment.status])}>
            {statusText(assessment)}
          </span>
          <span className="text-[12.5px] text-[#6E6A8F]">{assessment.due}</span>
          {assessment.match !== undefined && <span className="ml-auto text-[12.5px] text-[#6E6A8F]">{assessment.match}% match</span>}
        </div>
        <p className="mt-1.5 truncate text-[16px] font-medium text-[#100E2E]">{assessment.role}</p>
        <p className="truncate text-[13px] text-[#6E6A8F]">{details(assessment)}</p>
        <TaskNote assessment={assessment} />
      </div>
      <AssessmentAction assessment={assessment} />
    </li>
  );
}

export function AssessmentCard({ assessment }: { assessment: Assessment }) {
  return (
    <li
      className={clsx(
        "flex min-h-[220px] flex-col rounded-[20px] bg-white/75 p-5 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.95),0_10px_30px_-22px_rgba(76,58,160,0.45)]",
        assessment.status === "closed" && "opacity-60"
      )}
    >
      <div className="flex items-center gap-2.5">
        <span className={clsx("rounded-full px-2.5 py-0.5 text-[11px] font-semibold tracking-wide uppercase", PILL[assessment.status])}>
          {statusText(assessment)}
        </span>
        {assessment.match !== undefined && <span className="ml-auto text-[12.5px] text-[#6E6A8F]">{assessment.match}% match</span>}
      </div>
      <h3 className={clsx(display.className, "mt-3 text-[23px] leading-tight font-medium text-[#100E2E] [overflow-wrap:anywhere]")}>
        {assessment.role}
      </h3>
      <p className="mt-1.5 text-[13px] text-[#6E6A8F]">{details(assessment)}</p>
      <div className="mt-2">
        <TaskNote assessment={assessment} />
      </div>
      <div className="mt-auto flex flex-wrap items-end justify-between gap-3 pt-5">
        <span className="text-[12.5px] text-[#6E6A8F]">{assessment.due}</span>
        <AssessmentAction assessment={assessment} />
      </div>
    </li>
  );
}

/**
 * Each invitation gets a task written for that candidate after it is sent;
 * until it is ready there is nothing to start. An older backend doesn't
 * say, which counts as ready, as it always was.
 */
function taskReady(a: Assessment): boolean {
  return !a.taskStatus || a.taskStatus === "ready";
}

/** Said under the due date while the task is being made — before an invitation is answered too, so accepting doesn't lead to a Start that isn't there yet. */
function TaskNote({ assessment }: { assessment: Assessment }) {
  if (assessment.status !== "invited" || taskReady(assessment)) return null;
  return (
    <span role="status" className={clsx("text-[12.5px] font-medium", assessment.taskStatus === "failed" ? "text-[#a6203c]" : "text-[#4B2BB8]")}>
      {assessment.taskStatus === "failed" ? "Couldn't be prepared — the team that invited you can see this" : "Task being prepared…"}
    </span>
  );
}

function AssessmentAction({ assessment }: { assessment: Assessment }) {
  const { status, invitation, sessionId } = assessment;
  const unanswered = status === "invited" && !!invitation && !invitation.accepted;

  if (unanswered) return <InvitationAnswer assessmentId={assessment.id} />;
  if (status === "invited" && !taskReady(assessment)) return null; // TaskNote says why there's no Start yet
  if (status === "invited") return <ActionLink href={`/onboarding?template=${encodeURIComponent(assessment.id)}`}>Start</ActionLink>;
  if (status === "in-progress" && sessionId) return <ActionLink href={`/ide?session=${encodeURIComponent(sessionId)}`}>Resume</ActionLink>;
  if (status === "submitted" && sessionId)
    return (
      <ActionLink href={`/assessments/${encodeURIComponent(sessionId)}/report`} subtle>
        {assessment.reportShared ? "View report" : "Under review"}
      </ActionLink>
    );
  return null;
}

function ActionLink({ href, children, subtle = false }: { href: string; children: React.ReactNode; subtle?: boolean }) {
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
