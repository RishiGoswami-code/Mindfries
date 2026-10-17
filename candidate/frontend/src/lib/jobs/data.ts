import "server-only";
import { db } from "@/lib/supabase";
import { isMissingSchema } from "@/lib/profile/extras";
import type { EmploymentType, Interview, InterviewStatus, JobListing, MyApplication, ReceivedEmail, Stage, WorkMode } from "./shape";

/**
 * Reads for the job board and the candidate's applications (migration
 * 0025). Before 0025 is applied these answer "not set up" rather than
 * failing, so the rest of the portal still loads.
 */

export type Read<T> = { available: true; value: T } | { available: false; value: T };

const WORK_MODES: WorkMode[] = ["remote", "hybrid", "onsite"];
const EMPLOYMENT: EmploymentType[] = ["full_time", "part_time", "contract", "internship"];

/* eslint-disable @typescript-eslint/no-explicit-any -- rows from the untyped Supabase client, narrowed field by field below */
function toListing(r: any, applicants: number): JobListing {
  return {
    id: r.id,
    title: r.title,
    companyName: r.companies?.name ?? "A company on Mindfries",
    companyWebsite: r.companies?.website ?? null,
    description: r.description ?? "",
    location: r.location ?? "",
    workMode: WORK_MODES.includes(r.work_mode) ? r.work_mode : null,
    employmentType: EMPLOYMENT.includes(r.employment_type) ? r.employment_type : null,
    techStack: Array.isArray(r.tech_stack) ? r.tech_stack : [],
    durationMin: r.duration_min ?? null,
    publishedAt: r.published_at ?? null,
    applicants,
    selfAssessment: r.self_assessment === true,
  };
}

const JOB_COLUMNS = "id, title, description, location, work_mode, employment_type, tech_stack, duration_min, published_at, self_assessment, listed, status, company_id, companies(name, website)";

async function applicantCounts(roleIds: string[]): Promise<Map<string, number>> {
  const c = db();
  const counts = new Map<string, number>();
  if (!c || roleIds.length === 0) return counts;
  const { data } = await c.from("candidate_applications").select("job_role_id").in("job_role_id", roleIds);
  for (const row of data ?? []) counts.set(row.job_role_id as string, (counts.get(row.job_role_id as string) ?? 0) + 1);
  return counts;
}

export interface JobFilters {
  q?: string;
  workMode?: WorkMode;
  employmentType?: EmploymentType;
}

/** Jobs on the board: listed by their company, and the role still open. Newest first. */
export async function listJobs(filters: JobFilters = {}): Promise<Read<JobListing[]>> {
  const c = db();
  if (!c) return { available: false, value: [] };
  let query = c.from("job_roles").select(JOB_COLUMNS).eq("listed", true).eq("status", "open").order("published_at", { ascending: false }).limit(200);
  if (filters.workMode) query = query.eq("work_mode", filters.workMode);
  if (filters.employmentType) query = query.eq("employment_type", filters.employmentType);
  const { data, error } = await query;
  if (error) {
    if (!isMissingSchema(error)) console.error("jobs: couldn't list jobs:", error.message);
    return { available: false, value: [] };
  }
  const rows = (data ?? []) as any[];
  const needle = (filters.q ?? "").trim().toLowerCase();
  const matched = needle
    ? rows.filter((r) => [r.title, r.companies?.name, r.location, r.description, ...(r.tech_stack ?? [])].some((f) => typeof f === "string" && f.toLowerCase().includes(needle)))
    : rows;
  const counts = await applicantCounts(matched.map((r) => r.id));
  return { available: true, value: matched.map((r) => toListing(r, counts.get(r.id) ?? 0)) };
}

/** One job, only while it's on the board. */
export async function getJob(id: string): Promise<JobListing | null> {
  const c = db();
  if (!c || !/^[0-9a-f-]{36}$/i.test(id)) return null;
  const { data, error } = await c.from("job_roles").select(JOB_COLUMNS).eq("id", id).eq("listed", true).eq("status", "open").maybeSingle();
  if (error || !data) return null;
  const counts = await applicantCounts([id]);
  return toListing(data, counts.get(id) ?? 0);
}

/**
 * "This candidate's applications" as a PostgREST filter: their account, or
 * the exact address an invitation was sent to. Exact match on purpose —
 * ilike treats _ and % as wildcards, and a_b@x.com must never match
 * acb@x.com. Emails are stored lower-cased by both portals; the value is
 * quoted so a + or . in it is read literally.
 */
function ownedBy(candidate: { id: string; email: string }): string {
  const email = candidate.email.toLowerCase().replace(/["\\]/g, "");
  return `candidate_id.eq.${candidate.id},candidate_email.eq."${email}"`;
}

const STAGES: Stage[] = ["applied", "invited", "in_progress", "completed", "shortlisted", "interviewing", "offered", "hired", "rejected", "declined", "withdrawn"];

/**
 * The signed-in candidate's applications: those made from the job board
 * and those that came from an invitation, matched by account or by the
 * email the invitation was sent to. Newest first, each with its assessment,
 * interviews and the emails the hiring team sent.
 */
export async function listMyApplications(candidate: { id: string; email: string }): Promise<Read<MyApplication[]>> {
  const c = db();
  if (!c) return { available: false, value: [] };
  const { data, error } = await c
    .from("candidate_applications")
    .select(
      "id, job_role_id, assessment_id, stage, source, cover_note, created_at, stage_changed_at, completed_at, candidate_email, candidate_id, " +
        "job_roles(id, title, location, work_mode, employment_type, published_at, self_assessment, listed, status, companies(name))"
    )
    .or(ownedBy(candidate))
    .order("created_at", { ascending: false });
  if (error) {
    if (!isMissingSchema(error)) console.error("jobs: couldn't list applications:", error.message);
    return { available: false, value: [] };
  }
  const rows = (data ?? []) as any[];
  if (rows.length === 0) return { available: true, value: [] };

  const appIds = rows.map((r) => r.id as string);
  const assessmentIds = rows.map((r) => r.assessment_id).filter(Boolean) as string[];
  const roleIds = [...new Set(rows.map((r) => r.job_role_id as string))];

  const [interviews, emails, assessments, sessions, counts] = await Promise.all([
    c.from("application_interviews").select("*").in("application_id", appIds).order("starts_at", { ascending: true }),
    c.from("application_emails").select("id, application_id, subject, body, sent_by_name, sent_at").in("application_id", appIds).eq("delivered", true).order("sent_at", { ascending: false }),
    assessmentIds.length ? c.from("assessments").select("id, status, task_status").in("id", assessmentIds) : Promise.resolve({ data: [] as any[] }),
    assessmentIds.length
      ? c.from("sessions").select("id, assessment_id, started_at").in("assessment_id", assessmentIds).order("started_at", { ascending: false })
      : Promise.resolve({ data: [] as any[] }),
    applicantCounts(roleIds),
  ]);

  const byApp = <T,>(list: any[] | null, map: (r: any) => T) => {
    const out = new Map<string, T[]>();
    for (const r of list ?? []) {
      const k = r.application_id as string;
      out.set(k, [...(out.get(k) ?? []), map(r)]);
    }
    return out;
  };
  const interviewsBy = byApp<Interview>(interviews.data, (r) => ({
    id: r.id,
    title: r.title,
    startsAt: r.starts_at,
    durationMin: r.duration_min ?? 45,
    location: r.location ?? null,
    notes: r.notes ?? null,
    status: r.status as InterviewStatus,
    candidateMessage: r.candidate_message ?? null,
    updatedAt: r.updated_at,
  }));
  const emailsBy = byApp<ReceivedEmail>(emails.data, (r) => ({ id: r.id, subject: r.subject, body: r.body, fromName: r.sent_by_name ?? null, sentAt: r.sent_at }));
  const assessmentBy = new Map(((assessments.data ?? []) as any[]).map((a) => [a.id as string, a]));
  const sessionBy = new Map<string, any>();
  for (const s of (sessions.data ?? []) as any[]) if (!sessionBy.has(s.assessment_id)) sessionBy.set(s.assessment_id, s);

  return {
    available: true,
    value: rows.map((r): MyApplication => {
      const role = r.job_roles ?? {};
      const a = r.assessment_id ? assessmentBy.get(r.assessment_id) : null;
      const s = r.assessment_id ? sessionBy.get(r.assessment_id) : null;
      return {
        id: r.id,
        job: {
          id: role.id ?? r.job_role_id,
          title: role.title ?? "A role",
          companyName: role.companies?.name ?? "A company on Mindfries",
          location: role.location ?? "",
          workMode: WORK_MODES.includes(role.work_mode) ? role.work_mode : null,
          employmentType: EMPLOYMENT.includes(role.employment_type) ? role.employment_type : null,
          publishedAt: role.published_at ?? null,
          applicants: counts.get(r.job_role_id) ?? 0,
          selfAssessment: role.self_assessment === true,
        },
        jobListed: role.listed === true && role.status === "open",
        stage: STAGES.includes(r.stage) ? r.stage : "applied",
        source: r.source === "applied" ? "applied" : "invited",
        coverNote: r.cover_note ?? null,
        createdAt: r.created_at,
        stageChangedAt: r.stage_changed_at ?? null,
        completedAt: r.completed_at ?? null,
        assessment: a
          ? { id: a.id, status: a.status, taskStatus: a.task_status ?? "ready", sessionId: s?.id ?? null, startedAt: s?.started_at ?? null }
          : null,
        interviews: interviewsBy.get(r.id) ?? [],
        emails: emailsBy.get(r.id) ?? [],
      };
    }),
  };
}

/** The ids of jobs this candidate already has an application for, so the board can say "Applied". */
export async function myAppliedJobIds(candidate: { id: string; email: string }): Promise<Set<string>> {
  const c = db();
  if (!c) return new Set();
  const { data, error } = await c
    .from("candidate_applications")
    .select("job_role_id")
    .or(ownedBy(candidate));
  if (error) return new Set();
  return new Set((data ?? []).map((r) => r.job_role_id as string));
}
