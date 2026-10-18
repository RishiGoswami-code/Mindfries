import "server-only";
import { db } from "./supabase";
import { inviteSecret, signInviteToken } from "./auth/invite-token";
import { normalizeAssistantConfig } from "./assistant";
import { citedIds, describeEvent, type Moment } from "./moments";
import { normalizeInterviewConfig } from "./interview";
import type { CameraRecord, FlaggedMoment, TaskVariant } from "./types";
import type {
  ApplicationStage,
  AssessmentReport,
  CandidateApplication,
  CandidateReport,
  CompanyBilling,
  CompanyRole,
  CompanyUser,
  DueCandidate,
  InterviewExchange,
  InvitationResponse,
  GameTemplate,
  JobRole,
  RoleStatus,
  RoleVisibility,
  SessionSummary,
  StageCounts,
} from "./types";

// Data access for the Company Portal. Every read returns [] when Supabase
// isn't wired yet, so pages render empty states instead of crashing
// pre-setup — same convention as candidate/frontend and internal-admin's
// own lib/db.ts.
//
// Every function here takes companyId explicitly and every caller must pass
// it from the signed-in session (lib/auth/company-users.ts's
// currentCompanyUser()), never from a client-supplied value — that's what
// keeps one company's roles and candidates from ever being reachable by
// another's session.

/* eslint-disable @typescript-eslint/no-explicit-any */

function toTemplate(r: any): GameTemplate {
  return {
    id: r.id,
    name: r.name,
    taskVariant: r.task_variant,
    techStack: r.tech_stack ?? [],
    durationMin: r.duration_min ?? 60,
    own: !!r.company_id,
  };
}

/**
 * The tasks a company can attach to a role: Mindfries' published library,
 * plus the ones it generated for itself (status 'private', owned by it —
 * supabase/migrations/0017_task_generation.sql). Never another company's.
 */
export async function listPublishedTemplates(companyId?: string): Promise<GameTemplate[]> {
  const c = db();
  if (!c) return [];
  const { data: library } = await c
    .from("game_templates")
    .select("*")
    .eq("status", "published")
    .order("name", { ascending: true });
  // company_id doesn't exist before migration 0017; the error then just
  // means there are no company-owned tasks yet.
  const { data: own } = companyId
    ? await c.from("game_templates").select("*").eq("status", "private").eq("company_id", companyId).order("created_at", { ascending: false })
    : { data: [] };
  // A library task has no owner. Filtered here rather than in the query so
  // the query still runs on a database without the column.
  return [...(own ?? []), ...(library ?? []).filter((r: any) => !r.company_id)].map(toTemplate);
}

/**
 * Saves a generated task as this company's own and attaches it to the role.
 * It is stored 'private': outside the published library, so no other
 * company can attach it and it never appears in the candidates' open pool.
 */
export async function createCompanyTemplate(
  companyId: string,
  roleId: string,
  task: {
    name: string; taskVariant: string; techStack: string[]; durationMin: number;
    taskBrief: string; starterFiles: Record<string, string>; solutionFiles: Record<string, string>; verification: unknown;
  },
): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const role = await getJobRole(companyId, roleId);
  if (!role) throw new Error("Role not found");
  const { data, error } = await c
    .from("game_templates")
    .insert({
      name: task.name, task_variant: task.taskVariant, tech_stack: task.techStack, duration_min: task.durationMin,
      task_brief: task.taskBrief, starter_files: task.starterFiles, solution_files: task.solutionFiles,
      verification: task.verification, company_id: companyId, status: "private",
    })
    .select("id")
    .single();
  if (error && (error.code === "PGRST204" || error.code === "42703")) {
    throw new Error("Saving a generated task isn't available yet — the database is missing migration 0017_task_generation.");
  }
  if (error || !data) throw error ?? new Error("Could not save the task");
  await setJobRoleTemplate(companyId, roleId, data.id);
}

function toJobRole(r: any): JobRole {
  return {
    id: r.id,
    companyId: r.company_id,
    templateId: r.template_id ?? null,
    templateName: r.game_templates?.name ?? null,
    title: r.title,
    techStack: r.tech_stack ?? [],
    durationMin: r.duration_min ?? null,
    visibility: r.visibility,
    status: r.status,
    createdAt: r.created_at,
    // Absent before migration 0014 has run; normalizing fills the defaults either way.
    interviewConfig: normalizeInterviewConfig(r.interview_config),
    // Absent before migration 0015 has run; same.
    assistantConfig: normalizeAssistantConfig(r.assistant_config),
    // Absent before migration 0022 has run.
    assessmentBrief: r.assessment_brief ?? "",
    taskVariant: TASK_VARIANTS.includes(r.task_variant) ? r.task_variant : "bug_fix",
    // Absent before migration 0028 has run: treat as a legacy role with no role-level task.
    taskStatus: ["none", "pending", "generating", "ready", "failed"].includes(r.task_status) ? r.task_status : "none",
    taskError: r.task_error ?? null,
    // Absent before migration 0025 has run: an unlisted role with an empty posting.
    posting: {
      listed: r.listed === true,
      selfAssessment: r.self_assessment === true,
      description: r.description ?? "",
      location: r.location ?? "",
      workMode: ["remote", "hybrid", "onsite"].includes(r.work_mode) ? r.work_mode : null,
      employmentType: ["full_time", "part_time", "contract", "internship"].includes(r.employment_type) ? r.employment_type : null,
      publishedAt: r.published_at ?? null,
    },
  };
}

export const TASK_VARIANTS: TaskVariant[] = ["bug_fix", "feature", "refactor", "debug"];

export async function listJobRoles(companyId: string): Promise<JobRole[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("job_roles")
    .select("*, game_templates(name)")
    .eq("company_id", companyId)
    .order("created_at", { ascending: false });
  return (data ?? []).map(toJobRole);
}

/**
 * Creates a role with no assessment attached (template_id stays null) —
 * per this session's scope call, a company can open a role now and the
 * assessment-template picker (IMPLEMENTATION.md §3.3) is a separate,
 * later integration, not a precondition for this to work.
 */
export async function createJobRole(input: {
  companyId: string;
  title: string;
  techStack: string[];
  durationMin: number | null;
  visibility: RoleVisibility;
  assessmentBrief?: string;
  taskVariant?: TaskVariant;
}): Promise<JobRole> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c
    .from("job_roles")
    .insert({
      company_id: input.companyId,
      title: input.title,
      tech_stack: input.techStack,
      duration_min: input.durationMin,
      visibility: input.visibility,
      ...(input.assessmentBrief ? { assessment_brief: input.assessmentBrief } : {}),
      ...(input.taskVariant ? { task_variant: input.taskVariant } : {}),
    })
    .select("*")
    .single();
  if (error || !data) throw error ?? new Error("Insert returned no row");
  return toJobRole(data);
}

export async function getJobRole(companyId: string, roleId: string): Promise<JobRole | null> {
  const c = db();
  if (!c) return null;
  const { data } = await c
    .from("job_roles")
    .select("*, game_templates(name)")
    .eq("company_id", companyId)
    .eq("id", roleId)
    .maybeSingle();
  return data ? toJobRole(data) : null;
}

/**
 * What a role's assessment tests: the brief its one shared task is generated
 * from, and the kind of task. Saving the brief regenerates the role's task
 * (task_status back to 'pending'), so an edited description produces a new task
 * — the old template_id stays until the new one is ready, so candidates already
 * invited keep the task they were given (their assessments copied it at invite
 * time; only future invites get the new one).
 */
export async function setJobRoleBrief(companyId: string, roleId: string, brief: string, taskVariant: TaskVariant): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c
    .from("job_roles")
    .update({ assessment_brief: brief || null, task_variant: taskVariant, task_status: "pending", task_attempts: 0, task_error: null })
    .eq("company_id", companyId)
    .eq("id", roleId);
  if (error) throw error;
}

/** IMPLEMENTATION.md §3.3/§3.7's deferred picker — attach (or detach, templateId=null) a published template after the role already exists. */
export async function setJobRoleTemplate(companyId: string, roleId: string, templateId: string | null): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  // The id comes from a form. It has to be one this company may use: a
  // published library task, or a private one of its own — not another
  // company's private task whose id someone has got hold of.
  if (templateId) {
    const { data: template } = await c.from("game_templates").select("*").eq("id", templateId).maybeSingle();
    const allowed = template && ((template.status === "published" && !template.company_id) || (template.status === "private" && template.company_id === companyId));
    if (!allowed) throw new Error("That assessment isn't available to attach.");
  }
  const { error } = await c
    .from("job_roles")
    .update({ template_id: templateId })
    .eq("company_id", companyId)
    .eq("id", roleId);
  if (error) throw error;
}

/** Closes (or reopens) a role — same shape as setJobRoleTemplate. A closed role stops being this app's own concern beyond display; it doesn't retroactively touch any candidate already in its pipeline. */
export async function setJobRoleStatus(companyId: string, roleId: string, status: RoleStatus): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c
    .from("job_roles")
    .update({ status })
    .eq("company_id", companyId)
    .eq("id", roleId);
  if (error) throw error;
}

/**
 * Saves how the AI interview runs for a role. The value is normalized first,
 * so what's stored is always a complete, in-range config — the backend
 * normalizes again on read, but there's no reason to store junk.
 */
export async function setJobRoleInterview(companyId: string, roleId: string, config: unknown): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c
    .from("job_roles")
    .update({ interview_config: normalizeInterviewConfig(config) })
    .eq("company_id", companyId)
    .eq("id", roleId)
    .select("id");
  // PGRST204 / 42703: the column isn't there yet — say what's actually wrong
  // instead of a raw Postgres message about a schema cache.
  if (error && (error.code === "PGRST204" || error.code === "42703")) {
    throw new Error("Interview settings aren't available yet — the database is missing migration 0014_interview_config.");
  }
  if (error) throw error;
  if (!data || data.length === 0) throw new Error("Role not found");
}

/**
 * Saves whether candidates for a role get the AI assistant and how many
 * messages they may send it. Normalized first, like the interview settings.
 */
export async function setJobRoleAssistant(companyId: string, roleId: string, config: unknown): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c
    .from("job_roles")
    .update({ assistant_config: normalizeAssistantConfig(config) })
    .eq("company_id", companyId)
    .eq("id", roleId)
    .select("id");
  if (error && (error.code === "PGRST204" || error.code === "42703")) {
    throw new Error("Assistant settings aren't available yet — the database is missing migration 0015_assistant_config.");
  }
  if (error) throw error;
  if (!data || data.length === 0) throw new Error("Role not found");
}

const EMPTY_STAGE_COUNTS: StageCounts = {
  applied: 0,
  invited: 0,
  in_progress: 0,
  completed: 0,
  shortlisted: 0,
  rejected: 0,
  interviewing: 0,
  offered: 0,
  hired: 0,
  withdrawn: 0,
  declined: 0,
};

/** Per-role pipeline stage counts (IMPLEMENTATION.md §9) — one query per role for now; fine at MVP volumes. */
export async function stageCountsForRole(roleId: string): Promise<StageCounts> {
  const c = db();
  if (!c) return { ...EMPTY_STAGE_COUNTS };
  const { data } = await c.from("candidate_applications").select("stage").eq("job_role_id", roleId);
  const counts = { ...EMPTY_STAGE_COUNTS };
  for (const row of data ?? []) {
    const stage = row.stage as keyof StageCounts;
    if (stage in counts) counts[stage] += 1;
  }
  return counts;
}

function toApplication(r: any): CandidateApplication {
  return {
    id: r.id,
    jobRoleId: r.job_role_id,
    assessmentId: r.assessment_id ?? null,
    candidateName: r.candidate_name ?? null,
    candidateEmail: r.candidate_email,
    stage: r.stage,
    score: r.score ?? null,
    sectionScores: r.section_scores ?? {},
    timeTakenMin: r.time_taken_min ?? null,
    completedAt: r.completed_at ?? null,
    createdAt: r.created_at,
    invite: null,
    // Absent before migration 0025 has run: every application then came from an invitation.
    source: r.source === "applied" ? "applied" : "invited",
    coverNote: r.cover_note ?? null,
  };
}

/**
 * Updates applications' stage and, from migration 0025 on, when it changed
 * (the candidate's timeline dates each step by it). Without 0025 the column
 * isn't there, so the stage is written alone rather than failing.
 */
async function writeStage(
  apply: (patch: Record<string, unknown>) => PromiseLike<{ error: { code?: string } | null }>,
  stage: ApplicationStage,
): Promise<void> {
  let { error } = await apply({ stage, stage_changed_at: new Date().toISOString() });
  if (error && (error.code === "PGRST204" || error.code === "42703")) ({ error } = await apply({ stage }));
  if (error) throw error;
}

/** Fills in each application's invitation — its link and whether its task exists yet. */
async function withInvites(applications: CandidateApplication[]): Promise<CandidateApplication[]> {
  const c = db();
  const ids = applications.map((a) => a.assessmentId).filter((id): id is string => !!id);
  if (!c || ids.length === 0) return applications;
  const { data } = await c.from("assessments").select("id, invite_token, task_status, task_error").in("id", ids);
  const byId = new Map((data ?? []).map((r: any) => [r.id as string, r]));
  return applications.map((a) => {
    const r = a.assessmentId ? byId.get(a.assessmentId) : undefined;
    return r ? { ...a, invite: { token: r.invite_token ?? null, taskStatus: r.task_status ?? "ready", taskError: r.task_error ?? null } } : a;
  });
}

export async function listApplicationsForRole(roleId: string): Promise<CandidateApplication[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("candidate_applications")
    .select("*")
    .eq("job_role_id", roleId)
    .order("created_at", { ascending: false });
  return withInvites((data ?? []).map(toApplication));
}

/**
 * Invites a candidate to a role. Writes a real `assessments` row first —
 * the same table internal-admin's own createInvitation (and
 * candidate/backend's Go CreateInvitation) write to, so the candidate
 * app's existing session/report lifecycle picks this up unchanged — then a
 * linked `candidate_applications` row for this app's own pipeline board.
 * `assessments.template_id` carries over from the role's own template_id,
 * which is null until that picker is built (IMPLEMENTATION.md §3.7) — an
 * invite works either way, matching how `assessments.template_id` has
 * always been nullable.
 *
 * Not a single transaction — Supabase's client doesn't expose one across
 * two tables here, the same tradeoff internal-admin's addOnboarded
 * accepts — ordered so a failure on the second insert leaves an
 * `assessments` row unlinked rather than the invite silently not
 * happening at all.
 */
/**
 * The task state a new invitation to a role starts in. The task is generated
 * once per role now (migration 0028), so an invitation inherits it rather than
 * generating its own:
 *   - role has a ready task  → inherit it: template set, 'ready', no "preparing"
 *   - role's task is being made → wait: no template yet, 'pending'; the backend
 *     fills it in when the role's task is ready (SaveRoleTask backfills)
 *   - legacy role with no role-level task ('none') → start one now, so this and
 *     every future invite share one task instead of each generating its own
 *   - role's task failed → block: the company has to fix the brief first
 */
async function roleInviteTask(
  c: NonNullable<ReturnType<typeof db>>,
  companyId: string,
  roleId: string,
): Promise<{ templateId: string | null; taskStatus: "ready" | "pending" }> {
  const { data: role } = await c
    .from("job_roles")
    .select("template_id, task_status")
    .eq("company_id", companyId)
    .eq("id", roleId)
    .maybeSingle();
  if (!role) throw new Error("Role not found");
  if (role.task_status === "failed") {
    throw new Error("This role's assessment task couldn't be generated. Edit the role's brief and save it to regenerate the task before inviting candidates.");
  }
  if (role.template_id && role.task_status === "ready") {
    return { templateId: role.template_id, taskStatus: "ready" };
  }
  if (role.task_status === "none") {
    // A role from before role-level tasks: start generating one now, guarded so
    // concurrent invites don't each kick it off.
    await c
      .from("job_roles")
      .update({ task_status: "pending", task_attempts: 0, task_error: null })
      .eq("company_id", companyId)
      .eq("id", roleId)
      .eq("task_status", "none");
  }
  return { templateId: null, taskStatus: "pending" };
}

export async function inviteCandidateToRole(input: {
  companyId: string;
  jobRoleId: string;
  candidateEmail: string;
  candidateName?: string;
  dueDate?: string;
}): Promise<CandidateApplication> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");

  const role = await getJobRole(input.companyId, input.jobRoleId);
  if (!role) throw new Error("Role not found");
  const task = await roleInviteTask(c, input.companyId, input.jobRoleId);

  const { data: assessment, error: assessmentError } = await c
    .from("assessments")
    .insert({
      company_id: input.companyId,
      // Inherited from the role's one shared task (migration 0028): set when the
      // role already has it, filled in by the backend when it's still being made.
      template_id: task.templateId,
      job_role_id: input.jobRoleId,
      task_status: task.taskStatus,
      invited_by: "company",
      candidate_email: input.candidateEmail,
      candidate_name: input.candidateName || null,
      role: role.title,
      due_date: input.dueDate || null,
    })
    .select("id, invite_token")
    .single();
  if (assessmentError || !assessment) throw assessmentError ?? new Error("Insert returned no row");

  const { data, error } = await c
    .from("candidate_applications")
    .insert({
      job_role_id: input.jobRoleId,
      assessment_id: assessment.id,
      candidate_email: input.candidateEmail,
      candidate_name: input.candidateName || null,
      stage: "invited",
    })
    .select("*")
    .single();
  if (error || !data) throw error ?? new Error("Insert returned no row");
  return { ...toApplication(data), invite: { token: assessment.invite_token ?? null, taskStatus: task.taskStatus, taskError: null } };
}

/**
 * Invites a list of candidates to one role at once — the point of per-role
 * tasks: the company gives a list instead of filling the form per person, and
 * they all share the role's one task. Emails already invited to this role are
 * skipped (reported back), not re-invited. Returns what was invited and what
 * was skipped and why.
 */
export async function bulkInviteCandidatesToRole(input: {
  companyId: string;
  jobRoleId: string;
  candidates: { email: string; name?: string }[];
  dueDate?: string;
}): Promise<{ invited: CandidateApplication[]; skipped: { email: string; reason: string }[] }> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");

  const role = await getJobRole(input.companyId, input.jobRoleId);
  if (!role) throw new Error("Role not found");
  // Decided once: every candidate in this batch shares the role's one task.
  const task = await roleInviteTask(c, input.companyId, input.jobRoleId);

  const skipped: { email: string; reason: string }[] = [];
  // Normalise, validate and de-duplicate the list itself.
  const byEmail = new Map<string, { email: string; name?: string }>();
  for (const cand of input.candidates) {
    const email = cand.email.trim().toLowerCase();
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) {
      skipped.push({ email: cand.email, reason: "not a valid email" });
      continue;
    }
    if (byEmail.has(email)) continue; // same email twice in the list
    byEmail.set(email, { email, name: cand.name?.trim() || undefined });
  }

  // Skip anyone already invited to this role.
  if (byEmail.size > 0) {
    const { data: existing } = await c
      .from("candidate_applications")
      .select("candidate_email")
      .eq("job_role_id", input.jobRoleId)
      .in("candidate_email", [...byEmail.keys()]);
    for (const row of existing ?? []) {
      const email = (row.candidate_email ?? "").toLowerCase();
      if (byEmail.delete(email)) skipped.push({ email, reason: "already invited to this role" });
    }
  }

  const toInvite = [...byEmail.values()];
  if (toInvite.length === 0) return { invited: [], skipped };

  const { data: assessments, error: aErr } = await c
    .from("assessments")
    .insert(
      toInvite.map((cand) => ({
        company_id: input.companyId,
        template_id: task.templateId,
        job_role_id: input.jobRoleId,
        task_status: task.taskStatus,
        invited_by: "company",
        candidate_email: cand.email,
        candidate_name: cand.name || null,
        role: role.title,
        due_date: input.dueDate || null,
      })),
    )
    .select("id, invite_token, candidate_email");
  if (aErr || !assessments) throw aErr ?? new Error("Insert returned no rows");

  const { data: apps, error: appErr } = await c
    .from("candidate_applications")
    .insert(
      assessments.map((a) => {
        const cand = toInvite.find((t) => t.email === (a.candidate_email ?? "").toLowerCase());
        return {
          job_role_id: input.jobRoleId,
          assessment_id: a.id,
          candidate_email: a.candidate_email,
          candidate_name: cand?.name || null,
          stage: "invited" as const,
        };
      }),
    )
    .select("*");
  if (appErr || !apps) throw appErr ?? new Error("Insert returned no rows");

  const tokenByEmail = new Map(assessments.map((a) => [(a.candidate_email ?? "").toLowerCase(), a.invite_token ?? null]));
  const invited = apps.map((a) => ({
    ...toApplication(a),
    invite: { token: tokenByEmail.get((a.candidate_email ?? "").toLowerCase()) ?? null, taskStatus: task.taskStatus, taskError: null },
  }));
  return { invited, skipped };
}

/**
 * Asks for an invitation's task to be made again, after it failed. Scoped
 * by company, and only for an invitation that hasn't been started.
 */
export async function retryInvitationTask(companyId: string, assessmentId: string): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c
    .from("assessments")
    .update({ task_status: "pending", task_attempts: 0, task_error: null })
    .eq("id", assessmentId)
    .eq("company_id", companyId)
    .eq("status", "invited")
    .eq("task_status", "failed");
  if (error) throw error;
}

/**
 * Records that the invitation was mailed. Scoped by company as well as id,
 * like every write here.
 */
export async function markInvitationEmailed(companyId: string, assessmentId: string): Promise<void> {
  const c = db();
  if (!c) return;
  await c.from("assessments").update({ invite_emailed_at: new Date().toISOString() }).eq("id", assessmentId).eq("company_id", companyId);
}

/** What the candidate did with an invitation. Null when the application has no invitation behind it. */
export async function getInvitationResponse(companyId: string, assessmentId: string | null): Promise<InvitationResponse | null> {
  const c = db();
  if (!c || !assessmentId) return null;
  const { data } = await c
    .from("assessments")
    .select("invite_emailed_at, accepted_at, declined_at, decline_reason")
    .eq("id", assessmentId)
    .eq("company_id", companyId)
    .maybeSingle();
  if (!data) return null;
  return {
    emailedAt: data.invite_emailed_at ?? null,
    acceptedAt: data.accepted_at ?? null,
    declinedAt: data.declined_at ?? null,
    declineReason: data.decline_reason ?? null,
  };
}

export interface CandidateApplicationWithRole extends CandidateApplication {
  roleTitle: string;
}

/** Cross-role candidate list (§4's "All candidates" screen) — filtered through job_roles so one company never sees another's rows. */
export async function listApplicationsForCompany(companyId: string): Promise<CandidateApplicationWithRole[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("candidate_applications")
    .select("*, job_roles!inner(id, title, company_id)")
    .eq("job_roles.company_id", companyId)
    .order("created_at", { ascending: false });
  return (data ?? []).map((r: any) => ({ ...toApplication(r), roleTitle: r.job_roles?.title ?? "—" }));
}

/**
 * Moves a candidate to a new pipeline stage — Phase 2's per-candidate
 * version of the original plan's "bulk actions"; a multi-select bulk-action
 * bar is still outstanding, this is one candidate at a time. Scoped through
 * job_roles the same way every other write here is, so a request can't move
 * a candidate that belongs to a different company's role.
 */
export async function setApplicationStage(companyId: string, applicationId: string, stage: ApplicationStage): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const existing = await getApplicationForCompany(companyId, applicationId);
  if (!existing) throw new Error("Candidate not found");
  await writeStage((patch) => c.from("candidate_applications").update(patch).eq("id", applicationId), stage);
}

/**
 * The bulk version of setApplicationStage — one query, not N. Scoped the
 * same way: the role must belong to this company, and the update itself is
 * pinned to `job_role_id = roleId`, so a crafted id for someone else's
 * candidate just doesn't match a row rather than silently touching it.
 */
export async function bulkSetApplicationStage(
  companyId: string,
  roleId: string,
  applicationIds: string[],
  stage: ApplicationStage,
): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  if (applicationIds.length === 0) return;
  const role = await getJobRole(companyId, roleId);
  if (!role) throw new Error("Role not found");
  await writeStage((patch) => c.from("candidate_applications").update(patch).eq("job_role_id", roleId).in("id", applicationIds), stage);
}

/** One candidate's detail page — scoped through job_roles so one company can never open another's candidate by id. */
export async function getApplicationForCompany(companyId: string, applicationId: string): Promise<CandidateApplicationWithRole | null> {
  const c = db();
  if (!c) return null;
  const { data } = await c
    .from("candidate_applications")
    .select("*, job_roles!inner(id, title, company_id)")
    .eq("id", applicationId)
    .eq("job_roles.company_id", companyId)
    .maybeSingle();
  if (!data) return null;
  return { ...toApplication(data), roleTitle: (data as any).job_roles?.title ?? "—" };
}

/**
 * Reads the session and evidence-based report for a candidate's assessment
 * — direct Supabase reads of the shared `sessions` / `assessment_reports` /
 * `evidence_items` tables, not a call to candidate/backend's Go API (see
 * lib/types.ts's CandidateReport for why: that API authenticates by
 * forwarding the *candidate's own* session cookie, which this app never
 * has). Returns nulls rather than throwing when nothing exists yet — a
 * candidate who hasn't started, or whose report hasn't been generated, is
 * an honest empty state, not an error.
 */
export async function getCandidateReport(assessmentId: string | null): Promise<CandidateReport> {
  const none = { total: 0, expired: 0, stills: [] };
  const empty: CandidateReport = { session: null, report: null, interview: [], camera: none, screen: none, flags: [] };
  if (!assessmentId) return empty;
  const c = db();
  if (!c) return empty;

  const { data: sessionRow } = await c
    .from("sessions")
    .select("*")
    .eq("assessment_id", assessmentId)
    .order("started_at", { ascending: false })
    .limit(1)
    .maybeSingle();
  if (!sessionRow) return empty;

  // A sandbox session is marked by its sandbox_ready event (candidate/backend
  // writes it when the machine holds the task); the sandbox id itself is
  // cleared once the session is submitted.
  const { count: sandboxReady } = await c
    .from("activity_events")
    .select("id", { count: "exact", head: true })
    .eq("session_id", sessionRow.id)
    .eq("event_type", "sandbox_ready");

  const session: SessionSummary = {
    id: sessionRow.id,
    status: sessionRow.status,
    sandboxHealth: sessionRow.sandbox_health,
    workspace: (sandboxReady ?? 0) > 0 ? "sandbox" : "browser",
    progressPct: sessionRow.progress_pct ?? 0,
    durationMin: sessionRow.duration_min ?? 60,
    elapsedMin: sessionRow.elapsed_min ?? 0,
    startedAt: sessionRow.started_at,
  };

  const { data: reportRow } = await c.from("assessment_reports").select("*").eq("session_id", session.id).maybeSingle();
  const [interview, camera, screen, flags] = await Promise.all([
    getInterviewExchanges(session.id),
    getCameraRecord(session.id, session.startedAt),
    getCameraRecord(session.id, session.startedAt, "screen_snapshot"),
    getFlaggedMoments(session.id, session.startedAt),
  ]);
  if (!reportRow) return { session, report: null, interview, camera, screen, flags };

  const { data: evidenceRows } = await c
    .from("evidence_items")
    .select("*")
    .eq("report_id", reportRow.id)
    .order("created_at", { ascending: true });

  const report: AssessmentReport = {
    id: reportRow.id,
    status: reportRow.status,
    recommendation: reportRow.recommendation ?? null,
    summary: reportRow.summary ?? null,
    error: reportRow.error ?? null,
    evidence: (evidenceRows ?? []).map((e: any) => ({ id: e.id, category: e.category, observation: e.observation })),
    moments: [],
    annotations: [],
    review:
      reportRow.reviewer_recommendation || reportRow.reviewer_note
        ? {
            recommendation: reportRow.reviewer_recommendation ?? null,
            note: reportRow.reviewer_note ?? null,
            by: reportRow.reviewed_by ?? null,
            at: reportRow.reviewed_at ?? null,
          }
        : null,
  };
  report.moments = await getMoments(session.id, session.startedAt, citedIds(report.evidence.map((e) => e.observation)));

  // Absent before migration 0016 has run — then there are simply no notes yet.
  const { data: noteRows } = await c
    .from("report_annotations")
    .select("id, category, kind, body, author_name, created_at")
    .eq("report_id", reportRow.id)
    .order("created_at", { ascending: true });
  report.annotations = (noteRows ?? []).map((n: any) => ({
    id: n.id,
    category: n.category ?? null,
    kind: n.kind === "correction" ? "correction" : "note",
    body: n.body,
    authorName: n.author_name,
    createdAt: n.created_at,
  }));

  return { session, report, interview, camera, screen, flags };
}

const SNAPSHOTS_BUCKET = "proctor-snapshots";
/** How many stills the report page shows. A session has one every half minute; a reviewer wants a sense of it, not all 120. */
const CAMERA_SAMPLE = 24;

/**
 * The proctoring camera's stills for a session (candidate/frontend's
 * lib/ide/camera-snapshots.ts): an even sample of them, each with a signed
 * link valid for an hour. Only paths inside the session's own folder are
 * used — the events come from the candidate's browser.
 */
async function getCameraRecord(sessionId: string, startedAt: string, event: "camera_snapshot" | "screen_snapshot" = "camera_snapshot"): Promise<CameraRecord> {
  const c = db();
  const none: CameraRecord = { total: 0, expired: 0, stills: [] };
  if (!c) return none;
  const { data } = await c
    .from("activity_events")
    .select("event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("event_type", [event, `${event}_expired`])
    .order("occurred_at", { ascending: true });
  if (!data || data.length === 0) return none;

  const expired = data.filter((r: any) => r.event_type === `${event}_expired`).length;
  const live = data
    .filter((r: any) => r.event_type === event)
    .map((r: any) => ({ path: (r.payload ?? {}).path as unknown, at: r.occurred_at as string }))
    .filter((s): s is { path: string; at: string } => typeof s.path === "string" && s.path.startsWith(`${sessionId}/`) && !s.path.includes(".."));

  const step = Math.max(1, Math.ceil(live.length / CAMERA_SAMPLE));
  const sample = live.filter((_, i) => i % step === 0);
  const start = new Date(startedAt).getTime();
  const stills = (
    await Promise.all(
      sample.map(async (s) => {
        const { data: signed } = await c.storage.from(SNAPSHOTS_BUCKET).createSignedUrl(s.path, RECORDING_LINK_SECONDS);
        return signed?.signedUrl ? { url: signed.signedUrl, offsetSeconds: Math.max(0, (new Date(s.at).getTime() - start) / 1000) } : null;
      }),
    )
  ).filter((s): s is { url: string; offsetSeconds: number } => s !== null);
  return { total: live.length + expired, expired, stills };
}

const FLAG_LABEL: Record<string, string> = {
  left_workspace: "Left the workspace",
  fullscreen_exit: "Left fullscreen",
  screen_share_stopped: "Stopped sharing the screen",
  extra_display: "More than one display connected",
  external_paste: "Large paste from outside the workspace",
  face_missing: "Nobody in front of the camera",
  multiple_faces: "More than one face on camera",
  voices_detected: "Voices on the microphone",
  loud_noise: "Loud noise on the microphone",
  devtools_attempt: "Tried to open developer tools",
  screenshot_attempt: "Pressed PrintScreen",
  second_window: "Assessment open in a second window",
};

/**
 * The moments flagged live during a session (issue #120), each with signed
 * links to its camera still, screen still and audio clip. The paths come
 * from the candidate's browser, so only ones inside the session's own
 * folder are used. A file deleted at the end of its 90 days simply has no
 * link.
 */
async function getFlaggedMoments(sessionId: string, startedAt: string): Promise<FlaggedMoment[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("activity_events")
    .select("payload, occurred_at")
    .eq("session_id", sessionId)
    .eq("event_type", "activity_flag")
    .order("occurred_at", { ascending: true })
    .limit(200);
  if (!data || data.length === 0) return [];
  const start = new Date(startedAt).getTime();
  const ownPath = (p: unknown): p is string => typeof p === "string" && p.startsWith(`${sessionId}/`) && !p.includes("..");
  const link = async (p: unknown) => {
    if (!ownPath(p)) return null;
    const { data: signed } = await c.storage.from(SNAPSHOTS_BUCKET).createSignedUrl(p, RECORDING_LINK_SECONDS);
    return signed?.signedUrl ?? null;
  };
  return Promise.all(
    data
      .filter((r: any) => FLAG_LABEL[(r.payload ?? {}).kind])
      .map(async (r: any) => {
        const p = r.payload ?? {};
        const at = typeof p.at === "string" && !Number.isNaN(Date.parse(p.at)) ? Date.parse(p.at) : new Date(r.occurred_at).getTime();
        const detail =
          p.kind === "external_paste" && typeof p.detail?.chars === "number"
            ? `${p.detail.chars} characters`
            : p.kind === "multiple_faces" && typeof p.detail?.faces === "number"
              ? `${p.detail.faces} faces`
              : p.kind === "devtools_attempt" && typeof p.detail?.keys === "string"
                ? p.detail.keys
                : null;
        const [cameraUrl, screenUrl, audioUrl] = await Promise.all([link(p.cameraPath), link(p.screenPath), link(p.audioPath)]);
        return {
          kind: p.kind,
          label: FLAG_LABEL[p.kind],
          offsetSeconds: Math.max(0, (at - start) / 1000),
          detail,
          cameraUrl,
          screenUrl,
          audioUrl,
          lowConfidence: p.kind === "face_missing" || p.kind === "multiple_faces",
        };
      }),
  );
}

/**
 * The session events a report's evidence cites, as moments: how far into
 * the session, and what happened. Restricted to the session's own events, so
 * a citation can only ever resolve to something that session recorded.
 */
async function getMoments(sessionId: string, startedAt: string, ids: number[]): Promise<Moment[]> {
  const c = db();
  if (!c || ids.length === 0) return [];
  const { data } = await c
    .from("activity_events")
    .select("id, event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("id", ids.slice(0, 200));
  const start = new Date(startedAt).getTime();
  return (data ?? []).map((row: any) => ({
    id: Number(row.id),
    offsetSeconds: Math.max(0, (new Date(row.occurred_at).getTime() - start) / 1000),
    what: describeEvent(row.event_type, (row.payload ?? {}) as Record<string, unknown>),
  }));
}

/**
 * The report behind one of this company's applications. Every write to a
 * report goes through here: the application is looked up under the company
 * first, so a report can only be reached by the company whose candidate it
 * is about.
 */
async function reportIdForApplication(companyId: string, applicationId: string): Promise<string> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const application = await getApplicationForCompany(companyId, applicationId);
  if (!application?.assessmentId) throw new Error("Candidate not found");
  const { data: sessionRow } = await c
    .from("sessions")
    .select("id")
    .eq("assessment_id", application.assessmentId)
    .order("started_at", { ascending: false })
    .limit(1)
    .maybeSingle();
  if (!sessionRow) throw new Error("This candidate has no session yet");
  const { data: reportRow } = await c.from("assessment_reports").select("id").eq("session_id", sessionRow.id).maybeSingle();
  if (!reportRow) throw new Error("This candidate has no report yet");
  return reportRow.id;
}

const NEEDS_0016 = "Reviewing reports isn't available yet — the database is missing migration 0016_report_review.";
const missingSchema = (error: { code?: string } | null) =>
  !!error && ["PGRST204", "PGRST205", "42703", "42P01"].includes(error.code ?? "");

/** Adds a reviewer's note or correction to a report. */
export async function addReportAnnotation(
  companyId: string,
  applicationId: string,
  note: { category: string | null; kind: "note" | "correction"; body: string; authorName: string; authorEmail: string },
): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const reportId = await reportIdForApplication(companyId, applicationId);
  const { error } = await c.from("report_annotations").insert({
    report_id: reportId,
    category: note.category,
    kind: note.kind,
    body: note.body,
    author_name: note.authorName,
    author_email: note.authorEmail,
  });
  if (missingSchema(error)) throw new Error(NEEDS_0016);
  if (error) throw error;
}

/** Records (or, with nothing chosen and no note, clears) the reviewer's own decision on a report. */
export async function setReviewerDecision(
  companyId: string,
  applicationId: string,
  decision: { recommendation: string | null; note: string | null; by: string },
): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const reportId = await reportIdForApplication(companyId, applicationId);
  const cleared = !decision.recommendation && !decision.note;
  const { error } = await c
    .from("assessment_reports")
    .update({
      reviewer_recommendation: decision.recommendation,
      reviewer_note: decision.note,
      reviewed_by: cleared ? null : decision.by,
      reviewed_at: cleared ? null : new Date().toISOString(),
    })
    .eq("id", reportId);
  if (missingSchema(error)) throw new Error(NEEDS_0016);
  if (error) throw error;
}

const RECORDINGS_BUCKET = "interview-recordings";
const RECORDING_LINK_SECONDS = 3600;
const NO_ANSWER_TEXT = "(no answer given in the time allowed)";

/**
 * Rebuilds the interview from the session's evidence trail: "interview"
 * events are the questions and answers, in order (written by
 * candidate/backend's orchestrator), and "interview_recording" events point
 * at the clip uploaded for each answer (written by candidate/frontend once
 * an upload succeeds). Recordings sit in a private bucket; each gets a
 * signed link valid for an hour.
 *
 * Returns [] on any failure — a missing transcript must not take the whole
 * report page down with it.
 */
async function getInterviewExchanges(sessionId: string): Promise<InterviewExchange[]> {
  const c = db();
  if (!c) return [];
  const { data, error } = await c
    .from("activity_events")
    .select("event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("event_type", ["interview", "interview_recording", "interview_recording_expired"])
    .order("occurred_at", { ascending: true });
  if (error || !data) return [];

  const exchanges: InterviewExchange[] = [];
  const clips = new Map<number, { path: string; kind: "audio" | "video"; seconds: number }>();
  // Recordings are deleted after 90 days (internal-admin's lib/retention.ts),
  // which leaves this event behind in place of the one pointing at the file.
  const expired = new Set<number>();
  for (const row of data) {
    const p = (row.payload ?? {}) as Record<string, unknown>;
    if (row.event_type === "interview_recording_expired") {
      if (typeof p.question === "number") expired.add(p.question);
      continue;
    }
    if (row.event_type === "interview_recording") {
      if (typeof p.path === "string" && typeof p.question === "number" && p.path.startsWith(`${sessionId}/`)) {
        clips.set(p.question, { path: p.path, kind: p.kind === "video" ? "video" : "audio", seconds: Number(p.seconds) || 0 });
      }
      continue;
    }
    if (typeof p.text !== "string") continue;
    if (p.role === "interviewer") {
      exchanges.push({ number: exchanges.length + 1, question: p.text, answer: null, seconds: null, timedOut: false, unanswered: false, recording: null, recordingExpired: false });
    } else if (p.role === "candidate" && exchanges.length > 0) {
      const last = exchanges[exchanges.length - 1];
      last.answer = p.text;
      last.seconds = typeof p.seconds === "number" ? p.seconds : null;
      last.timedOut = p.timedOut === true;
      last.unanswered = p.text === NO_ANSWER_TEXT;
    }
  }

  await Promise.all(
    exchanges.map(async (x) => {
      const clip = clips.get(x.number);
      if (!clip) {
        x.recordingExpired = expired.has(x.number);
        return;
      }
      const { data: signed } = await c.storage.from(RECORDINGS_BUCKET).createSignedUrl(clip.path, RECORDING_LINK_SECONDS);
      if (signed?.signedUrl) x.recording = { url: signed.signedUrl, kind: clip.kind, seconds: clip.seconds };
    }),
  );
  return exchanges;
}

/**
 * Every candidate with a real due date on their assessment — the Overview
 * calendar widget's data. `assessments.due_date` is set at invite time
 * (inviteCandidateToRole passes it straight through); joined here rather
 * than added to candidate_applications, since the due date belongs to the
 * assessment record, not the pipeline row. Sorted in JS rather than via a
 * nested-column `.order()` — small volumes at MVP scale, and Supabase's
 * embedded-resource ordering only reliably covers the base table's own
 * columns.
 */
export async function listUpcomingDueDates(companyId: string): Promise<DueCandidate[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("candidate_applications")
    .select("id, candidate_name, candidate_email, job_roles!inner(title, company_id), assessments!inner(due_date)")
    .eq("job_roles.company_id", companyId)
    .not("assessments.due_date", "is", null);

  return (data ?? [])
    .map((r: any) => ({
      applicationId: r.id,
      candidateName: r.candidate_name ?? null,
      candidateEmail: r.candidate_email,
      roleTitle: r.job_roles?.title ?? "—",
      dueDate: r.assessments?.due_date as string,
    }))
    .filter((d) => !!d.dueDate)
    .sort((a, b) => a.dueDate.localeCompare(b.dueDate));
}

function toCompanyUser(r: any): CompanyUser {
  return {
    id: r.id,
    companyId: r.company_id,
    email: r.email,
    name: r.name,
    role: r.role,
    status: r.status,
    createdAt: r.created_at,
  };
}

export async function listCompanyUsers(companyId: string): Promise<CompanyUser[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("company_users")
    .select("*")
    .eq("company_id", companyId)
    .order("created_at", { ascending: true });
  return (data ?? []).map(toCompanyUser);
}

/**
 * IMPLEMENTATION.md Phase 4: invite a teammate. Same insert + sign-token
 * shape as internal-admin's private createCompanyUserInvite (which this
 * generalizes — `role` is a parameter here, not hardcoded "admin", since
 * that helper only ever provisions the founding admin). Returns `token:
 * null` if COMPANY_INVITE_SECRET isn't configured — the row still gets
 * created (the token can be re-derived by any future call once the secret
 * exists, since nothing about the row itself depends on it) — the caller
 * decides whether that's an error worth surfacing.
 */
export async function inviteCompanyUser(input: {
  companyId: string;
  email: string;
  name: string;
  role: CompanyRole;
}): Promise<{ companyUser: CompanyUser; token: string | null }> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");

  const { data, error } = await c
    .from("company_users")
    .insert({ company_id: input.companyId, email: input.email, name: input.name, role: input.role, status: "invited" })
    .select("*")
    .single();
  if (error || !data) throw error ?? new Error("Insert returned no row");

  const secret = inviteSecret();
  const token = secret ? await signInviteToken({ companyUserId: data.id, email: input.email }, secret) : null;
  return { companyUser: toCompanyUser(data), token };
}

export async function setCompanyUserStatus(
  companyId: string,
  userId: string,
  status: "active" | "disabled",
): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c
    .from("company_users")
    .update({ status })
    .eq("company_id", companyId)
    .eq("id", userId);
  if (error) throw error;
}

/** Changes an existing teammate's role. Same shape as setCompanyUserStatus — the "can't leave zero active admins" guard lives in the caller (settings/team/actions.ts's setTeammateRole), same as it already does for status changes. */
export async function setCompanyUserRole(companyId: string, userId: string, role: CompanyRole): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c
    .from("company_users")
    .update({ role })
    .eq("company_id", companyId)
    .eq("id", userId);
  if (error) throw error;
}

/**
 * IMPLEMENTATION.md §3.4: plan/seats straight from `companies`, usage
 * derived from the same counts Overview already reads (lib/overview.ts) —
 * no Stripe, no invented numbers, display only. `seatsUsed` counts active
 * company_users only — an invited-but-not-yet-accepted teammate isn't
 * occupying a seat yet.
 */
export async function getCompanyBilling(companyId: string): Promise<CompanyBilling> {
  const c = db();
  if (!c) return { plan: "trial", seatsTotal: 0, seatsUsed: 0, openRoles: 0, candidatesInvited: 0 };

  const { data: company } = await c.from("companies").select("plan, seats").eq("id", companyId).maybeSingle();
  const [roles, users, applications] = await Promise.all([
    listJobRoles(companyId),
    listCompanyUsers(companyId),
    listApplicationsForCompany(companyId),
  ]);

  return {
    plan: company?.plan ?? "trial",
    seatsTotal: company?.seats ?? 0,
    seatsUsed: users.filter((u) => u.status === "active").length,
    openRoles: roles.filter((r) => r.status === "open").length,
    candidatesInvited: applications.length,
  };
}
