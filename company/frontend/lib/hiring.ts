import "server-only";
import { db } from "./supabase";
import { getApplicationForCompany, getJobRole } from "./db";
import type { ApplicationEmail, ApplicationInterview, EmploymentType, InterviewStatus, JobPosting, WorkMode } from "./types";

/**
 * The job board's company side (migration 0025): publishing a role as a job,
 * answering applicants, scheduling interviews, and emailing candidates.
 * Every function is scoped to the signed-in company: a role or application
 * is looked up through the company first, so a crafted id for someone else's
 * candidate finds nothing.
 */

/** Thrown when a 0025 table or column isn't in this database yet. */
export class NotMigratedError extends Error {
  constructor() {
    super("The job board isn't set up in this database yet — it needs migration 0025_job_board_and_applications.");
  }
}

function check(error: { code?: string; message?: string } | null): void {
  if (!error) return;
  if (["PGRST204", "PGRST205", "42703", "42P01"].includes(error.code ?? "")) throw new NotMigratedError();
  throw new Error(error.message ?? "The database refused that.");
}

// ── Job postings ─────────────────────────────────────────────────────

export const WORK_MODES: WorkMode[] = ["remote", "hybrid", "onsite"];
export const EMPLOYMENT_TYPES: EmploymentType[] = ["full_time", "part_time", "contract", "internship"];

/**
 * Saves how a role appears on the job board. The first time it's listed,
 * `published_at` is stamped — that's the "posted on" date candidates see —
 * and it isn't moved by later edits or by unlisting and relisting.
 */
export async function setJobPosting(companyId: string, roleId: string, posting: Omit<JobPosting, "publishedAt">): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const role = await getJobRole(companyId, roleId);
  if (!role) throw new Error("Role not found");
  const { error } = await c
    .from("job_roles")
    .update({
      listed: posting.listed,
      self_assessment: posting.selfAssessment,
      description: posting.description || null,
      location: posting.location || null,
      work_mode: posting.workMode,
      employment_type: posting.employmentType,
      ...(posting.listed && !role.posting.publishedAt ? { published_at: new Date().toISOString() } : {}),
    })
    .eq("id", roleId)
    .eq("company_id", companyId);
  check(error);
}

// ── Answering an applicant ───────────────────────────────────────────

/**
 * Invites someone who applied from the job board to the role's assessment:
 * the same `assessments` row a direct invitation writes (its task is then
 * generated for them by candidate/backend), linked to their existing
 * application, which moves from applied to invited.
 */
export async function inviteApplicant(companyId: string, applicationId: string, dueDate?: string): Promise<{ assessmentId: string; inviteToken: string | null }> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const application = await getApplicationForCompany(companyId, applicationId);
  if (!application) throw new Error("Candidate not found");
  if (application.assessmentId) throw new Error("This candidate already has an assessment for this role.");
  const role = await getJobRole(companyId, application.jobRoleId);
  if (!role) throw new Error("Role not found");

  const { data: row } = await c.from("candidate_applications").select("candidate_id").eq("id", applicationId).maybeSingle();

  const { data: assessment, error: assessmentError } = await c
    .from("assessments")
    .insert({
      company_id: companyId,
      template_id: null,
      job_role_id: role.id,
      task_status: "pending",
      invited_by: "company",
      candidate_email: application.candidateEmail,
      candidate_name: application.candidateName,
      candidate_id: (row as { candidate_id?: string } | null)?.candidate_id ?? null,
      role: role.title,
      due_date: dueDate || null,
    })
    .select("id, invite_token")
    .single();
  if (assessmentError || !assessment) throw assessmentError ?? new Error("Insert returned no row");

  const { error } = await c
    .from("candidate_applications")
    .update({ assessment_id: assessment.id, stage: "invited", stage_changed_at: new Date().toISOString() })
    .eq("id", applicationId);
  check(error);
  return { assessmentId: assessment.id as string, inviteToken: (assessment.invite_token as string) ?? null };
}

// ── Interviews ───────────────────────────────────────────────────────

function toInterview(r: Record<string, unknown>): ApplicationInterview {
  return {
    id: r.id as string,
    applicationId: r.application_id as string,
    title: r.title as string,
    startsAt: r.starts_at as string,
    durationMin: (r.duration_min as number) ?? 45,
    location: (r.location as string) ?? null,
    notes: (r.notes as string) ?? null,
    status: r.status as InterviewStatus,
    candidateMessage: (r.candidate_message as string) ?? null,
    updatedAt: r.updated_at as string,
  };
}

/** An application's interviews, soonest first. Empty — not an error — before migration 0025. */
export async function listInterviews(companyId: string, applicationId: string): Promise<ApplicationInterview[]> {
  const c = db();
  if (!c) return [];
  const { data, error } = await c
    .from("application_interviews")
    .select("*")
    .eq("company_id", companyId)
    .eq("application_id", applicationId)
    .order("starts_at", { ascending: true });
  if (error) return [];
  return (data ?? []).map(toInterview);
}

export interface InterviewInput {
  title: string;
  startsAt: string;
  durationMin: number;
  location: string;
  notes: string;
}

/**
 * Schedules an interview and moves the application to interviewing — unless
 * it's already further on (offered, hired) or closed, which a new interview
 * doesn't undo.
 */
export async function scheduleInterview(companyId: string, applicationId: string, input: InterviewInput): Promise<ApplicationInterview> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const application = await getApplicationForCompany(companyId, applicationId);
  if (!application) throw new Error("Candidate not found");
  const { data, error } = await c
    .from("application_interviews")
    .insert({
      application_id: applicationId,
      company_id: companyId,
      title: input.title,
      starts_at: input.startsAt,
      duration_min: input.durationMin,
      location: input.location || null,
      notes: input.notes || null,
    })
    .select("*")
    .single();
  check(error);
  if (!["offered", "hired", "rejected", "withdrawn", "declined", "interviewing"].includes(application.stage)) {
    await c.from("candidate_applications").update({ stage: "interviewing", stage_changed_at: new Date().toISOString() }).eq("id", applicationId);
  }
  return toInterview(data as Record<string, unknown>);
}

/** Moves an interview to a new time or place; it's scheduled again, waiting for the candidate to confirm. */
export async function rescheduleInterview(companyId: string, interviewId: string, input: InterviewInput): Promise<ApplicationInterview> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c
    .from("application_interviews")
    .update({
      title: input.title,
      starts_at: input.startsAt,
      duration_min: input.durationMin,
      location: input.location || null,
      notes: input.notes || null,
      status: "scheduled",
      candidate_message: null,
      updated_at: new Date().toISOString(),
    })
    .eq("id", interviewId)
    .eq("company_id", companyId)
    .select("*")
    .maybeSingle();
  check(error);
  if (!data) throw new Error("Interview not found");
  return toInterview(data as Record<string, unknown>);
}

/** The hiring team's side of an interview's end: it happened, or it's off. */
export async function closeInterview(companyId: string, interviewId: string, status: "completed" | "cancelled"): Promise<ApplicationInterview> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c
    .from("application_interviews")
    .update({ status, updated_at: new Date().toISOString() })
    .eq("id", interviewId)
    .eq("company_id", companyId)
    .select("*")
    .maybeSingle();
  check(error);
  if (!data) throw new Error("Interview not found");
  return toInterview(data as Record<string, unknown>);
}

// ── Email ────────────────────────────────────────────────────────────

/** What was emailed to this candidate about this application, newest first. Empty before migration 0025. */
export async function listApplicationEmails(companyId: string, applicationId: string): Promise<ApplicationEmail[]> {
  const c = db();
  if (!c) return [];
  const { data, error } = await c
    .from("application_emails")
    .select("id, subject, body, sent_by_name, delivered, error, sent_at")
    .eq("company_id", companyId)
    .eq("application_id", applicationId)
    .order("sent_at", { ascending: false });
  if (error) return [];
  return (data ?? []).map((r) => ({
    id: r.id as string,
    subject: r.subject as string,
    body: r.body as string,
    sentByName: (r.sent_by_name as string) ?? null,
    delivered: r.delivered === true,
    error: (r.error as string) ?? null,
    sentAt: r.sent_at as string,
  }));
}

/** Keeps a record of an email — sent or not — so both sides can see what was said. */
export async function recordApplicationEmail(input: {
  companyId: string;
  applicationId: string;
  senderEmail: string;
  senderName: string;
  subject: string;
  body: string;
  delivered: boolean;
  error: string | null;
}): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data: sender } = await c
    .from("company_users")
    .select("id")
    .eq("company_id", input.companyId)
    .ilike("email", input.senderEmail)
    .maybeSingle();
  const { error } = await c.from("application_emails").insert({
    application_id: input.applicationId,
    company_id: input.companyId,
    sent_by: (sender as { id?: string } | null)?.id ?? null,
    sent_by_name: input.senderName || null,
    subject: input.subject,
    body: input.body,
    delivered: input.delivered,
    error: input.error,
  });
  check(error);
}
