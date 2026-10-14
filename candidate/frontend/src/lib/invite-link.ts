import "server-only";
import { db } from "./supabase";

// An invitation's own link: /invite/<token> (migration 0022).
//
// The link opens one assessment for one person. It is not a way around
// signing in: the candidate still signs in (or creates an account) as the
// email the invitation was sent to, and the link does nothing for anyone
// else — a forwarded link shows the other person whose invitation it is,
// with the address masked, and nothing more.

export type InviteTaskStatus = "pending" | "generating" | "ready" | "failed";

export interface InviteByLink {
  id: string;
  candidateEmail: string;
  candidateName: string | null;
  company: string;
  role: string;
  status: string;
  taskStatus: InviteTaskStatus;
  dueDate: string | null;
  accepted: boolean;
  declined: boolean;
}

const TOKEN = /^[0-9a-f]{64}$/;

export async function findInviteByToken(token: string): Promise<InviteByLink | null> {
  if (!TOKEN.test(token)) return null;
  const c = db();
  if (!c) return null;
  const { data, error } = await c
    .from("assessments")
    .select("id, candidate_email, candidate_name, company_id, role, status, task_status, due_date, accepted_at, declined_at, job_role_id")
    .eq("invite_token", token)
    .maybeSingle();
  if (error || !data) return null;

  let company = "Mindfries";
  let role = (data.role as string | null) || "an engineering role";
  const [companyRow, roleRow] = await Promise.all([
    data.company_id ? c.from("companies").select("name").eq("id", data.company_id).maybeSingle() : null,
    data.job_role_id ? c.from("job_roles").select("title").eq("id", data.job_role_id).maybeSingle() : null,
  ]);
  if (companyRow?.data?.name) company = companyRow.data.name as string;
  if (roleRow?.data?.title) role = roleRow.data.title as string;

  return {
    id: data.id as string,
    candidateEmail: (data.candidate_email as string).toLowerCase(),
    candidateName: (data.candidate_name as string | null) ?? null,
    company,
    role,
    status: data.status as string,
    taskStatus: ((data.task_status as string) ?? "ready") as InviteTaskStatus,
    dueDate: (data.due_date as string | null) ?? null,
    accepted: !!data.accepted_at,
    declined: !!data.declined_at,
  };
}

/** "jane.doe@acme.com" → "j•••e@acme.com" — enough to recognise, not to read. */
export function maskEmail(email: string): string {
  const [local, domain] = email.split("@");
  if (!domain) return "•••";
  const shown = local.length <= 2 ? local[0] ?? "" : `${local[0]}•••${local[local.length - 1]}`;
  return `${shown}@${domain}`;
}

/** The candidate's session on this invitation, newest first, if they have started it. */
export async function sessionForInvite(assessmentId: string, candidateId: string): Promise<string | null> {
  const c = db();
  if (!c) return null;
  const { data } = await c
    .from("sessions")
    .select("id")
    .eq("assessment_id", assessmentId)
    .eq("candidate_id", candidateId)
    .order("started_at", { ascending: false })
    .limit(1)
    .maybeSingle();
  return (data?.id as string | undefined) ?? null;
}
