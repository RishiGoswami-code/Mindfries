"use server";

import { revalidatePath } from "next/cache";
import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { mailerReady } from "@/lib/mailer";
import { isMissingSchema } from "@/lib/profile/extras";
import { canWithdraw, type Stage } from "./shape";

/**
 * What a candidate does on the job board and with their applications
 * (migration 0025). Each action acts only on the signed-in candidate's own
 * applications — the id in the request picks which one, never whose.
 */

export type JobResult = { ok: true; applicationId?: string } | { ok: false; error: string };

const NOT_SET_UP = "The job board isn't set up on the server yet.";
const MAX_NOTE = 2000;
const UUID = /^[0-9a-f-]{36}$/i;

function refresh() {
  revalidatePath("/jobs");
  revalidatePath("/applications");
  revalidatePath("/dashboard");
  revalidatePath("/assessments");
}

/**
 * Applies to a job on the board. The application goes to the hiring team as
 * "applied"; they choose who takes the assessment. If the company opened
 * this role's assessment to every applicant, the invitation is made now —
 * the same invitation a company writes, its task then generated for this
 * candidate by candidate/backend — and the application starts at "invited".
 */
export async function applyToJob(roleId: string, coverNoteRaw: string): Promise<JobResult> {
  const candidate = await currentCandidate();
  if (!candidate) return { ok: false, error: "Sign in again and retry." };
  const c = db();
  if (!c) return { ok: false, error: "The database isn't connected." };
  if (!UUID.test(roleId)) return { ok: false, error: "That job isn't on the board any more." };
  const coverNote = String(coverNoteRaw ?? "").trim().slice(0, MAX_NOTE) || null;
  const email = candidate.email.toLowerCase();

  const { data: role, error: roleError } = await c
    .from("job_roles")
    .select("id, title, company_id, self_assessment")
    .eq("id", roleId)
    .eq("listed", true)
    .eq("status", "open")
    .maybeSingle();
  if (roleError) return { ok: false, error: isMissingSchema(roleError) ? NOT_SET_UP : "Couldn't apply just now — try again." };
  if (!role) return { ok: false, error: "That job isn't on the board any more." };

  const { data: existing } = await c
    .from("candidate_applications")
    .select("id")
    .eq("job_role_id", roleId)
    .or(`candidate_id.eq.${candidate.id},candidate_email.eq."${email.replace(/["\\]/g, "")}"`)
    .limit(1);
  if (existing && existing.length > 0) return { ok: false, error: "You've already applied to this job — it's in your Applications." };

  const { data: me } = await c.from("candidate_users").select("name, email_verified_at").eq("id", candidate.id).maybeSingle();
  const name = (me?.name as string | undefined) || candidate.name || null;

  let assessmentId: string | null = null;
  if (role.self_assessment) {
    // Where confirming an email is possible, an invitation is only taken up
    // from a confirmed address (lib/invitations.ts, acceptInvitation); until
    // then it waits for the candidate to accept it on their dashboard.
    const accepted = !mailerReady() || !!me?.email_verified_at;
    const { data: assessment, error } = await c
      .from("assessments")
      .insert({
        company_id: role.company_id,
        template_id: null,
        job_role_id: role.id,
        task_status: "pending",
        invited_by: "company",
        candidate_email: email,
        candidate_name: name,
        candidate_id: candidate.id,
        role: role.title,
        ...(accepted ? { accepted_at: new Date().toISOString() } : {}),
      })
      .select("id")
      .single();
    if (error || !assessment) return { ok: false, error: "Couldn't apply just now — try again." };
    assessmentId = assessment.id as string;
  }

  const { data: application, error } = await c
    .from("candidate_applications")
    .insert({
      job_role_id: role.id,
      assessment_id: assessmentId,
      candidate_id: candidate.id,
      candidate_email: email,
      candidate_name: name,
      source: "applied",
      cover_note: coverNote,
      stage: assessmentId ? "invited" : "applied",
      stage_changed_at: new Date().toISOString(),
    })
    .select("id")
    .single();
  if (error || !application) {
    // Don't leave an invitation behind that no application points at.
    if (assessmentId) await c.from("assessments").delete().eq("id", assessmentId);
    return { ok: false, error: error && isMissingSchema(error) ? NOT_SET_UP : "Couldn't apply just now — try again." };
  }

  refresh();
  return { ok: true, applicationId: application.id as string };
}

/** The candidate's own application, or null. */
async function ownApplication(applicationId: string) {
  const candidate = await currentCandidate();
  const c = db();
  if (!candidate || !c || !UUID.test(applicationId)) return null;
  const email = candidate.email.toLowerCase().replace(/["\\]/g, "");
  const { data } = await c
    .from("candidate_applications")
    .select("id, stage, assessment_id")
    .eq("id", applicationId)
    .or(`candidate_id.eq.${candidate.id},candidate_email.eq."${email}"`)
    .maybeSingle();
  return data ? { c, candidate, app: data as { id: string; stage: Stage; assessment_id: string | null } } : null;
}

/**
 * Withdraws an application. An assessment not yet started is closed with
 * it (the backend only starts an open one), and interviews still to come
 * are cancelled on the candidate's side, so the hiring team sees why.
 */
export async function withdrawApplication(applicationId: string, reasonRaw: string): Promise<JobResult> {
  const own = await ownApplication(applicationId);
  if (!own) return { ok: false, error: "That application isn't yours, or isn't there any more." };
  const { c, app } = own;
  if (!canWithdraw(app)) {
    return { ok: false, error: app.stage === "in_progress" ? "Your assessment is under way — finish it, or let it run out, first." : "This application is already closed." };
  }
  const reason = String(reasonRaw ?? "").trim().slice(0, 500);
  const now = new Date().toISOString();

  if (app.assessment_id) {
    await c
      .from("assessments")
      .update({ status: "declined", declined_at: now, decline_reason: reason || "Withdrew the application" })
      .eq("id", app.assessment_id)
      .eq("status", "invited");
  }
  await c
    .from("application_interviews")
    .update({ status: "cancelled_by_candidate", candidate_message: reason || "Withdrew the application", updated_at: now })
    .eq("application_id", app.id)
    .in("status", ["scheduled", "confirmed", "reschedule_requested"]);
  const { error } = await c.from("candidate_applications").update({ stage: "withdrawn", stage_changed_at: now }).eq("id", app.id);
  if (error) return { ok: false, error: "Couldn't withdraw just now — try again." };

  refresh();
  return { ok: true };
}

export type InterviewResponse = "confirm" | "reschedule" | "cancel";

const NEXT_STATUS: Record<InterviewResponse, string> = {
  confirm: "confirmed",
  reschedule: "reschedule_requested",
  cancel: "cancelled_by_candidate",
};

/**
 * The candidate's answer to an interview: confirm it, ask for another time
 * (saying when suits them), or cancel it. The hiring team sees the answer
 * and the message on the candidate's page.
 */
export async function respondToInterview(interviewId: string, response: InterviewResponse, messageRaw: string): Promise<JobResult> {
  const candidate = await currentCandidate();
  const c = db();
  if (!candidate || !c) return { ok: false, error: "Sign in again and retry." };
  if (!UUID.test(interviewId) || !(response in NEXT_STATUS)) return { ok: false, error: "That interview isn't there any more." };
  const message = String(messageRaw ?? "").trim().slice(0, 1000);
  if (response === "reschedule" && message.length < 3) return { ok: false, error: "Say which days or times would suit you." };

  const { data: interview } = await c.from("application_interviews").select("id, application_id, status").eq("id", interviewId).maybeSingle();
  if (!interview) return { ok: false, error: "That interview isn't there any more." };
  const own = await ownApplication(interview.application_id as string);
  if (!own) return { ok: false, error: "That interview isn't yours." };
  if (!["scheduled", "confirmed", "reschedule_requested"].includes(interview.status as string)) {
    return { ok: false, error: "This interview has already been closed." };
  }

  const { error } = await c
    .from("application_interviews")
    .update({ status: NEXT_STATUS[response], candidate_message: message || null, updated_at: new Date().toISOString() })
    .eq("id", interviewId);
  if (error) return { ok: false, error: "Couldn't save that — try again." };

  refresh();
  return { ok: true };
}
