"use server";

import { revalidatePath } from "next/cache";
import { ForbiddenError, requireCompanyPermission } from "@/lib/auth/company-users";
import { bulkInviteCandidatesToRole, bulkSetApplicationStage, getApplicationForCompany, getJobRole, inviteCandidateToRole, markInvitationEmailed, retryInvitationTask, setApplicationStage, setJobRoleAssistant, setJobRoleBrief, setJobRoleInterview, setJobRoleStatus, TASK_VARIANTS } from "@/lib/db";
import type { ApplicationStage, EmploymentType, TaskVariant, WorkMode } from "@/lib/types";
import { EMPLOYMENT_TYPES, WORK_MODES, inviteApplicant, setJobPosting } from "@/lib/hiring";
import { inviteLink } from "@/lib/invite-link";
import { invitationMail } from "@/lib/invitation-mail";
import { mailerReady, sendMail } from "@/lib/mailer";
import { candidateWantsEmail } from "@/lib/candidate-settings";
import { refundCredit, spendCredit, type Spend } from "@/lib/access";

/** What a successful invitation says about credits: nothing for an unmetered company. */
const creditNote = (s: Spend) => (s.metered ? ` 1 credit used — ${s.left} left.` : "");

const text = (v: unknown, max: number) => (typeof v === "string" ? v.slice(0, max).trim() : "");
const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

export type InviteCandidateState = {
  error: string | null;
  success: boolean;
  /** What happened with the email, for the form to say. */
  notice?: string;
  /** The invitation's own link, to copy and send another way. Absent when CANDIDATE_PORTAL_URL isn't set. */
  link?: string;
};

/**
 * Bound to a specific roleId in the client form (`inviteCandidate.bind(null, roleId)`)
 * so useActionState's (prevState, formData) signature still applies — the
 * roleId never comes from the form body, which keeps a crafted request from
 * inviting into a role this session doesn't own (getJobRole inside
 * inviteCandidateToRole checks that against companyId regardless).
 */
export async function inviteCandidate(roleId: string, _prev: InviteCandidateState, form: FormData): Promise<InviteCandidateState> {
  let companyId: string;
  let companyName: string;
  try {
    const session = await requireCompanyPermission("candidate:invite");
    companyId = session.companyId;
    companyName = session.companyName;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", success: false };
  }

  const email = text(form.get("candidateEmail"), 254).toLowerCase();
  if (!EMAIL_RE.test(email)) return { error: "That doesn't look like an email address.", success: false };

  const candidateName = text(form.get("candidateName"), 200) || undefined;
  const dueDate = text(form.get("dueDate"), 10) || undefined;

  // 1 credit per candidate invited, taken before the invitation is made and
  // given back if it can't be.
  let spent: Spend;
  try {
    spent = await spendCredit(companyId);
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't check your credits — try again.", success: false };
  }

  let notice: string;
  let link: string | undefined;
  let invited = false;
  try {
    const application = await inviteCandidateToRole({ companyId, jobRoleId: roleId, candidateEmail: email, candidateName, dueDate });
    invited = true;
    link = inviteLink(application.invite?.token) ?? undefined;

    // The invitation exists from here on, and shows on the candidate's
    // dashboard whether or not a message reaches them. The email is what
    // tells them it's there — so what the form reports is what actually
    // happened to the email, not just "Invited". The candidate's own task
    // is written for them over the next minute or two.
    const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
    if (!mailerReady() || !portal || !link) {
      notice = `Invited — their own task is being prepared. No email was sent (${!mailerReady() ? "email isn't set up on this portal" : "CANDIDATE_PORTAL_URL isn't set, so there is no link"})${link ? "; send them the link below" : `; tell ${email} to sign in to the candidate portal with that address`}.`;
    } else if (!(await candidateWantsEmail(email, "invitations"))) {
      notice = "Invited — their own task is being prepared. No email was sent: they've turned invitation emails off, and it shows on their dashboard.";
    } else {
      const role = await getJobRole(companyId, roleId);
      try {
        await sendMail({ to: email, ...invitationMail({ link, companyName, roleTitle: role?.title ?? "a role", candidateName, dueDate }) });
        if (application.assessmentId) await markInvitationEmailed(companyId, application.assessmentId);
        notice = `Invited, and emailed ${email} their link. Their own task is being prepared.`;
      } catch (e) {
        notice = `Invited, but the email to ${email} couldn't be sent (${e instanceof Error ? e.message : "unknown error"}). Send them the link below yourself.`;
      }
    }
  } catch (e) {
    if (!invited && spent.metered) await refundCredit(companyId);
    return { error: e instanceof Error ? e.message : "Couldn't invite that candidate — try again.", success: false };
  }

  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  revalidatePath("/candidates");
  revalidatePath("/dashboard");
  return { error: null, success: true, notice: notice + creditNote(spent), link };
}

/** Splits a pasted list into candidates: one per line or comma, each an email or "Name <email>". */
function parseCandidateList(raw: string): { email: string; name?: string }[] {
  const out: { email: string; name?: string }[] = [];
  for (const piece of raw.split(/[\n,;]+/)) {
    const token = piece.trim();
    if (!token) continue;
    // "Name <email@x.com>" or "email@x.com".
    const angle = token.match(/^(.*?)<([^>]+)>$/);
    if (angle) {
      out.push({ email: angle[2].trim().toLowerCase(), name: angle[1].trim() || undefined });
    } else {
      out.push({ email: token.toLowerCase() });
    }
  }
  return out;
}

export type BulkInviteState = {
  error: string | null;
  success: boolean;
  /** A human summary of what happened: invited, emailed, skipped. */
  summary?: string;
};

/**
 * Invites a whole list of candidates to one role at once — the point of
 * per-role tasks: give a list instead of filling the form per person, and they
 * all share the role's one task (no per-candidate generation, no "preparing").
 * Bound to roleId in the form, like inviteCandidate. One credit per candidate
 * actually invited; credits are refunded for anyone skipped.
 */
export async function bulkInviteCandidates(roleId: string, _prev: BulkInviteState, form: FormData): Promise<BulkInviteState> {
  let companyId: string;
  let companyName: string;
  try {
    const session = await requireCompanyPermission("candidate:invite");
    companyId = session.companyId;
    companyName = session.companyName;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", success: false };
  }

  const dueDate = text(form.get("dueDate"), 10) || undefined;
  const parsed = parseCandidateList(text(form.get("candidates"), 20000));
  if (parsed.length === 0) return { error: "Add at least one candidate email.", success: false };

  // Validate and de-duplicate the list before spending anything.
  const seen = new Set<string>();
  const valid: { email: string; name?: string }[] = [];
  const skips: string[] = [];
  for (const cand of parsed) {
    if (!EMAIL_RE.test(cand.email)) {
      skips.push(`${cand.email} (not a valid email)`);
      continue;
    }
    if (seen.has(cand.email)) continue;
    seen.add(cand.email);
    valid.push(cand);
  }
  if (valid.length === 0) return { error: "None of those were valid email addresses.", success: false };

  // One credit per candidate, spent up front; stop when the company runs out so
  // we never invite more than it has paid for.
  let metered = false;
  let spent = 0;
  const affordable: { email: string; name?: string }[] = [];
  for (const cand of valid) {
    try {
      const s = await spendCredit(companyId);
      if (s.metered) {
        metered = true;
        spent++;
      }
      affordable.push(cand);
    } catch (e) {
      skips.push(`${valid.length - affordable.length} more (${e instanceof Error ? e.message : "out of credits"})`);
      break;
    }
  }
  if (affordable.length === 0) {
    return { error: "Couldn't invite anyone. " + skips.join("; "), success: false };
  }

  let result: Awaited<ReturnType<typeof bulkInviteCandidatesToRole>>;
  try {
    result = await bulkInviteCandidatesToRole({ companyId, jobRoleId: roleId, candidates: affordable, dueDate });
  } catch (e) {
    if (metered) for (let i = 0; i < spent; i++) await refundCredit(companyId);
    return { error: e instanceof Error ? e.message : "Couldn't send the invitations — try again.", success: false };
  }

  // Refund credits for anyone the invite itself skipped (already invited).
  if (metered) for (let i = 0; i < result.skipped.length; i++) await refundCredit(companyId);

  // Email everyone invited — best effort; the invitation shows on their
  // dashboard whether or not the mail reaches them.
  let emailed = 0;
  const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
  if (mailerReady() && portal) {
    const role = await getJobRole(companyId, roleId);
    for (const app of result.invited) {
      const link = inviteLink(app.invite?.token);
      if (!link) continue;
      if (!(await candidateWantsEmail(app.candidateEmail, "invitations"))) continue;
      try {
        await sendMail({ to: app.candidateEmail, ...invitationMail({ link, companyName, roleTitle: role?.title ?? "a role", candidateName: app.candidateName ?? undefined, dueDate }) });
        if (app.assessmentId) await markInvitationEmailed(companyId, app.assessmentId);
        emailed++;
      } catch {
        // shows on their dashboard regardless
      }
    }
  }

  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  revalidatePath("/candidates");
  revalidatePath("/dashboard");

  const allSkips = [...result.skipped.map((s) => `${s.email} (${s.reason})`), ...skips];
  const parts = [`Invited ${result.invited.length} candidate${result.invited.length === 1 ? "" : "s"} — they share this role's task, ready now`];
  if (emailed) parts.push(`emailed ${emailed}`);
  if (allSkips.length) parts.push(`skipped ${allSkips.length}: ${allSkips.join(", ")}`);
  if (metered) {
    const used = spent - result.skipped.length;
    parts.push(`${used} credit${used === 1 ? "" : "s"} used`);
  }
  return { error: null, success: true, summary: parts.join(". ") + "." };
}

export type RoleBriefState = { error: string | null; saved: boolean };

/** What the role's assessment tests — the brief each invited candidate's own task is generated from. */
export async function saveRoleBrief(roleId: string, _prev: RoleBriefState, form: FormData): Promise<RoleBriefState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", saved: false };
  }
  const brief = text(form.get("assessmentBrief"), 4000);
  const variant = String(form.get("taskVariant") ?? "");
  const taskVariant: TaskVariant = (TASK_VARIANTS as string[]).includes(variant) ? (variant as TaskVariant) : "bug_fix";
  try {
    await setJobRoleBrief(companyId, roleId, brief, taskVariant);
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't save that — try again.", saved: false };
  }
  revalidatePath(`/roles/${roleId}`);
  return { error: null, saved: true };
}

/** Tries again to make a candidate's task, after it failed. */
export async function retryTask(roleId: string, assessmentId: string): Promise<void> {
  const { companyId } = await requireCompanyPermission("candidate:invite");
  await retryInvitationTask(companyId, assessmentId);
  revalidatePath(`/roles/${roleId}`);
}

/**
 * Bound to a specific roleId/applicationId/stage per button
 * (`changeStage.bind(null, roleId, application.id, "shortlisted")`) and
 * used directly as a `<form action={...}>` — no client component needed,
 * since there's no per-field input to validate or pending state beyond
 * what the browser's own form submission already shows. The bound args
 * come from this app's own JSX, not form input, so there's nothing here to
 * sanitize; requireCompanyPermission + setApplicationStage's
 * company-ownership check are what actually keep this safe.
 */
export async function changeStage(roleId: string, applicationId: string, stage: ApplicationStage): Promise<void> {
  const { companyId } = await requireCompanyPermission("candidate:stage");
  await setApplicationStage(companyId, applicationId, stage);
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  revalidatePath("/candidates");
  revalidatePath(`/candidates/${applicationId}`);
  revalidatePath("/dashboard");
}

/**
 * Bound to a specific roleId/stage via the bulk-action bar's own
 * `formAction={bulkChangeStage.bind(null, roleId, "shortlisted")}` — one
 * submit button per stage, same as changeStage. Unlike changeStage, the ids
 * to move aren't bound ahead of time: they're whichever checkboxes were
 * checked, read from the real submitted FormData (name="applicationIds",
 * repeated once per checked row).
 */
export async function bulkChangeStage(roleId: string, stage: ApplicationStage, formData: FormData): Promise<void> {
  const { companyId } = await requireCompanyPermission("candidate:stage");
  const applicationIds = formData.getAll("applicationIds").map(String).filter(Boolean);
  if (applicationIds.length === 0) return;
  await bulkSetApplicationStage(companyId, roleId, applicationIds, stage);
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  revalidatePath("/candidates");
  revalidatePath("/dashboard");
}


export type InterviewSettingsState = { error: string | null; success: boolean };

/**
 * Saves the role's AI interview settings. The form's values arrive as
 * strings; setJobRoleInterview normalizes them against the allowed ranges,
 * so a tampered form can't store an interview with a thousand questions.
 */
export async function saveInterviewSettings(roleId: string, _prev: InterviewSettingsState, form: FormData): Promise<InterviewSettingsState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", success: false };
  }
  try {
    await setJobRoleInterview(companyId, roleId, {
      questions: Number(form.get("questions")),
      answerSeconds: Number(form.get("answerSeconds")),
      tone: String(form.get("tone") ?? ""),
      language: String(form.get("language") ?? ""),
    });
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't save the interview settings — try again.", success: false };
  }
  revalidatePath(`/roles/${roleId}`);
  return { error: null, success: true };
}

export type AssistantSettingsState = { error: string | null; success: boolean };

/**
 * Saves the role's AI assistant settings. setJobRoleAssistant normalizes the
 * values against the allowed range, so a tampered form can't raise the
 * message limit past the platform's own.
 */
export async function saveAssistantSettings(roleId: string, _prev: AssistantSettingsState, form: FormData): Promise<AssistantSettingsState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", success: false };
  }
  try {
    await setJobRoleAssistant(companyId, roleId, {
      enabled: form.get("enabled") !== "off",
      maxMessages: Number(form.get("maxMessages")),
    });
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't save the assistant settings — try again.", success: false };
  }
  revalidatePath(`/roles/${roleId}`);
  return { error: null, success: true };
}

export type SetRoleStatusState = { error: string | null; success: boolean };

/**
 * Bound to a specific roleId + target status (`setRoleStatus.bind(null,
 * roleId, "closed")`), same pattern as attachTemplate — no form fields of
 * its own, the target status is already decided by which button was
 * clicked (open → "Close role", closed → "Reopen role").
 */
export async function setRoleStatus(
  roleId: string,
  status: "open" | "closed",
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  _prev: SetRoleStatusState,
): Promise<SetRoleStatusState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", success: false };
  }
  try {
    await setJobRoleStatus(companyId, roleId, status);
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't update that role — try again.", success: false };
  }
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  return { error: null, success: true };
}

export type JobPostingState = { error: string | null; saved: boolean };

/**
 * How the role appears on the candidates' job board: whether it's listed,
 * whether every applicant can take the assessment straight away, and what
 * the posting says. Listing needs a description, so a candidate never finds
 * a job that says nothing.
 */
export async function saveJobPosting(roleId: string, _prev: JobPostingState, form: FormData): Promise<JobPostingState> {
  let companyId: string;
  try {
    companyId = (await requireCompanyPermission("role:write")).companyId;
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", saved: false };
  }
  const listed = form.get("listed") === "on";
  const description = text(form.get("description"), 8000);
  const workMode = String(form.get("workMode") ?? "");
  const employmentType = String(form.get("employmentType") ?? "");
  if (listed && description.length < 40) {
    return { error: "Write a description of at least a few sentences before listing the job — it's what candidates read first.", saved: false };
  }
  try {
    await setJobPosting(companyId, roleId, {
      listed,
      selfAssessment: form.get("selfAssessment") === "on",
      description,
      location: text(form.get("location"), 120),
      workMode: (WORK_MODES as string[]).includes(workMode) ? (workMode as WorkMode) : null,
      employmentType: (EMPLOYMENT_TYPES as string[]).includes(employmentType) ? (employmentType as EmploymentType) : null,
    });
  } catch (e) {
    return { error: e instanceof Error ? e.message : "Couldn't save that — try again.", saved: false };
  }
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  return { error: null, saved: true };
}

export type InviteApplicantState = { error: string | null; notice: string | null };

/**
 * Invites someone who applied from the job board to the role's assessment,
 * and emails them their link when email is set up — reporting what actually
 * happened to the email, like a direct invitation does.
 */
export async function inviteApplicantToAssessment(roleId: string, applicationId: string, _prev: InviteApplicantState, form: FormData): Promise<InviteApplicantState> {
  let session: Awaited<ReturnType<typeof requireCompanyPermission>>;
  try {
    session = await requireCompanyPermission("candidate:invite");
  } catch (e) {
    return { error: e instanceof ForbiddenError ? e.message : "Not signed in.", notice: null };
  }
  const dueDate = text(form.get("dueDate"), 10) || undefined;
  let notice: string;
  let spent: Spend = { metered: false };
  let invited = false;
  try {
    const application = await getApplicationForCompany(session.companyId, applicationId);
    if (!application) return { error: "Candidate not found.", notice: null };
    // 1 credit per candidate invited, given back if the invitation can't be made.
    spent = await spendCredit(session.companyId);
    const { assessmentId, inviteToken } = await inviteApplicant(session.companyId, applicationId, dueDate);
    invited = true;
    const link = inviteLink(inviteToken);
    const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
    if (!mailerReady() || !portal || !link) {
      notice = `Invited — their own task is being prepared. No email was sent (${!mailerReady() ? "email isn't set up on this portal" : "CANDIDATE_PORTAL_URL isn't set"}); it shows on their dashboard.`;
    } else if (!(await candidateWantsEmail(application.candidateEmail, "invitations"))) {
      notice = "Invited — their own task is being prepared. No email was sent: they've turned invitation emails off, and it shows on their dashboard.";
    } else {
      try {
        await sendMail({
          to: application.candidateEmail,
          ...invitationMail({ link, companyName: session.companyName, roleTitle: application.roleTitle, candidateName: application.candidateName ?? undefined, dueDate }),
        });
        await markInvitationEmailed(session.companyId, assessmentId);
        notice = `Invited, and emailed ${application.candidateEmail} their link. Their own task is being prepared.`;
      } catch (e) {
        notice = `Invited, but the email couldn't be sent (${e instanceof Error ? e.message : "unknown error"}). It shows on their dashboard.`;
      }
    }
  } catch (e) {
    if (!invited && spent.metered) await refundCredit(session.companyId);
    return { error: e instanceof Error ? e.message : "Couldn't invite them — try again.", notice: null };
  }
  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/candidates");
  revalidatePath(`/candidates/${applicationId}`);
  revalidatePath("/dashboard");
  return { error: null, notice: notice + creditNote(spent) };
}
