"use server";

import { revalidatePath } from "next/cache";
import { ForbiddenError, requireCompanyPermission } from "@/lib/auth/company-users";
import { getApplicationForCompany } from "@/lib/db";
import { closeInterview, recordApplicationEmail, rescheduleInterview, scheduleInterview, type InterviewInput } from "@/lib/hiring";
import { mailerReady, sendMail } from "@/lib/mailer";
import { candidateWantsEmail, type CandidateEmailKind } from "@/lib/candidate-settings";

/**
 * The hiring team's side of an application on the candidate page:
 * scheduling interviews and emailing the candidate. Every action re-checks
 * permission and looks the application up through the signed-in company.
 *
 * Email is the only messaging: it goes from Mindfries' address with the
 * sender's own address as reply-to, so the candidate's answer lands in the
 * sender's inbox. Each one is recorded (application_emails) whether or not
 * it could be sent, so both sides see what was said. A candidate who turned
 * that kind of email off in their Settings isn't emailed; it is still
 * recorded, and they read it in their Applications.
 */

export type HiringState = { error: string | null; notice: string | null };

const text = (v: unknown, max: number) => (typeof v === "string" ? v.slice(0, max).trim() : "");

function refresh(applicationId: string, roleId: string) {
  revalidatePath(`/candidates/${applicationId}`);
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/candidates");
  revalidatePath("/dashboard");
}

/** The form sends the moment in UTC (worked out in the scheduler's browser) and their timezone, for the email. */
function readInterview(form: FormData): { input: InterviewInput; timeZone: string } | { error: string } {
  const title = text(form.get("title"), 120);
  const startsAt = text(form.get("startsAtIso"), 40);
  const when = new Date(startsAt);
  const duration = Number(form.get("durationMin"));
  if (!title) return { error: "Say what the interview is, e.g. Technical interview." };
  if (!startsAt || Number.isNaN(when.getTime())) return { error: "Pick a date and time." };
  if (when.getTime() < Date.now() - 5 * 60_000) return { error: "That time has already passed." };
  if (!Number.isInteger(duration) || duration < 10 || duration > 480) return { error: "Length should be between 10 minutes and 8 hours." };
  const timeZone = text(form.get("timeZone"), 64) || "UTC";
  return {
    input: { title, startsAt: when.toISOString(), durationMin: duration, location: text(form.get("location"), 500), notes: text(form.get("notes"), 2000) },
    timeZone,
  };
}

function formatWhen(iso: string, timeZone: string): string {
  try {
    return new Intl.DateTimeFormat("en-GB", { dateStyle: "full", timeStyle: "short", timeZone, timeZoneName: "short" }).format(new Date(iso));
  } catch {
    return new Intl.DateTimeFormat("en-GB", { dateStyle: "full", timeStyle: "short", timeZone: "UTC", timeZoneName: "short" }).format(new Date(iso));
  }
}

/**
 * Sends an email to the candidate (when email is set up) and records it
 * either way. Returns what happened, in words for the form.
 */
async function emailCandidate(opts: {
  kind: CandidateEmailKind;
  companyId: string;
  companyName: string;
  senderEmail: string;
  senderName: string;
  applicationId: string;
  to: string;
  subject: string;
  body: string;
}): Promise<string> {
  let delivered = false;
  let error: string | null = null;
  if (!mailerReady()) {
    error = "Email isn't set up on this portal";
  } else if (!(await candidateWantsEmail(opts.to, opts.kind))) {
    error = "they've turned these emails off";
  } else {
    try {
      await sendMail({
        to: opts.to,
        subject: opts.subject,
        text: `${opts.body}\n\n— ${opts.senderName || opts.companyName}, ${opts.companyName}\nReply to this email to answer them directly.`,
        replyTo: opts.senderEmail,
      });
      delivered = true;
    } catch (e) {
      error = e instanceof Error ? e.message : "The mail service refused it";
    }
  }
  await recordApplicationEmail({
    companyId: opts.companyId,
    applicationId: opts.applicationId,
    senderEmail: opts.senderEmail,
    senderName: opts.senderName,
    subject: opts.subject,
    body: opts.body,
    delivered,
    error,
  });
  return delivered ? `Emailed ${opts.to}.` : `Saved, but not emailed (${error}). They'll see it in their Applications.`;
}

function interviewMail(kind: "scheduled" | "moved", companyName: string, roleTitle: string, input: InterviewInput, timeZone: string) {
  const lines = [
    kind === "scheduled"
      ? `${companyName} has scheduled an interview with you for ${roleTitle}.`
      : `${companyName} has moved your interview for ${roleTitle}.`,
    "",
    `${input.title}`,
    `When: ${formatWhen(input.startsAt, timeZone)} (${input.durationMin} minutes)`,
    input.location ? `Where: ${input.location}` : null,
    input.notes ? `\n${input.notes}` : null,
    "",
    "Confirm it, or ask for another time, from Applications in the Mindfries candidate portal.",
  ].filter((l): l is string => l !== null);
  return { subject: `${kind === "scheduled" ? "Interview" : "Interview moved"}: ${input.title} — ${roleTitle}`, body: lines.join("\n") };
}

export async function scheduleInterviewAction(applicationId: string, _prev: HiringState, form: FormData): Promise<HiringState> {
  let session: Awaited<ReturnType<typeof requireCompanyPermission>>;
  try {
    session = await requireCompanyPermission("candidate:stage");
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", notice: null };
  }
  const read = readInterview(form);
  if ("error" in read) return { error: read.error, notice: null };
  try {
    const application = await getApplicationForCompany(session.companyId, applicationId);
    if (!application) return { error: "Candidate not found.", notice: null };
    await scheduleInterview(session.companyId, applicationId, read.input);
    const mail = interviewMail("scheduled", session.companyName, application.roleTitle, read.input, read.timeZone);
    const sent = await emailCandidate({ kind: "interviews", ...session, senderEmail: session.email, senderName: session.name, applicationId, to: application.candidateEmail, ...mail });
    refresh(applicationId, application.jobRoleId);
    return { error: null, notice: `Scheduled. ${sent}` };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't schedule that — try again.", notice: null };
  }
}

export async function rescheduleInterviewAction(applicationId: string, interviewId: string, _prev: HiringState, form: FormData): Promise<HiringState> {
  let session: Awaited<ReturnType<typeof requireCompanyPermission>>;
  try {
    session = await requireCompanyPermission("candidate:stage");
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", notice: null };
  }
  const read = readInterview(form);
  if ("error" in read) return { error: read.error, notice: null };
  try {
    const application = await getApplicationForCompany(session.companyId, applicationId);
    if (!application) return { error: "Candidate not found.", notice: null };
    await rescheduleInterview(session.companyId, interviewId, read.input);
    const mail = interviewMail("moved", session.companyName, application.roleTitle, read.input, read.timeZone);
    const sent = await emailCandidate({ kind: "interviews", ...session, senderEmail: session.email, senderName: session.name, applicationId, to: application.candidateEmail, ...mail });
    refresh(applicationId, application.jobRoleId);
    return { error: null, notice: `Moved. ${sent}` };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't move that — try again.", notice: null };
  }
}

/** Marks an interview done, or calls it off (and tells the candidate). Bound per button. */
export async function closeInterviewAction(applicationId: string, interviewId: string, status: "completed" | "cancelled"): Promise<void> {
  const session = await requireCompanyPermission("candidate:stage");
  const application = await getApplicationForCompany(session.companyId, applicationId);
  if (!application) throw new Error("Candidate not found.");
  const interview = await closeInterview(session.companyId, interviewId, status);
  if (status === "cancelled") {
    await emailCandidate({
      kind: "interviews",
      ...session,
      senderEmail: session.email,
      senderName: session.name,
      applicationId,
      to: application.candidateEmail,
      subject: `Interview cancelled: ${interview.title} — ${application.roleTitle}`,
      body: `${session.companyName} has cancelled your ${interview.title.toLowerCase()} for ${application.roleTitle}, which was due ${formatWhen(interview.startsAt, "UTC")}.`,
    });
  }
  refresh(applicationId, application.jobRoleId);
}

export async function sendEmailAction(applicationId: string, _prev: HiringState, form: FormData): Promise<HiringState> {
  let session: Awaited<ReturnType<typeof requireCompanyPermission>>;
  try {
    session = await requireCompanyPermission("candidate:stage");
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", notice: null };
  }
  const subject = text(form.get("subject"), 200);
  const body = text(form.get("body"), 10_000);
  if (!subject) return { error: "Add a subject.", notice: null };
  if (body.length < 2) return { error: "Write the message.", notice: null };
  try {
    const application = await getApplicationForCompany(session.companyId, applicationId);
    if (!application) return { error: "Candidate not found.", notice: null };
    const sent = await emailCandidate({ kind: "messages", ...session, senderEmail: session.email, senderName: session.name, applicationId, to: application.candidateEmail, subject, body });
    refresh(applicationId, application.jobRoleId);
    return { error: null, notice: sent };
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't send that — try again.", notice: null };
  }
}
