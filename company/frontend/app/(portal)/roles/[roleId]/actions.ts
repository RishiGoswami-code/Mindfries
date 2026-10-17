"use server";

import { revalidatePath } from "next/cache";
import { ForbiddenError, requireCompanyPermission } from "@/lib/auth/company-users";
import { bulkSetApplicationStage, getJobRole, inviteCandidateToRole, markInvitationEmailed, retryInvitationTask, setApplicationStage, setJobRoleAssistant, setJobRoleBrief, setJobRoleInterview, setJobRoleStatus, TASK_VARIANTS } from "@/lib/db";
import type { ApplicationStage, TaskVariant } from "@/lib/types";
import { inviteLink } from "@/lib/invite-link";
import { invitationMail } from "@/lib/invitation-mail";
import { mailerReady, sendMail } from "@/lib/mailer";

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

  let notice: string;
  let link: string | undefined;
  try {
    const application = await inviteCandidateToRole({ companyId, jobRoleId: roleId, candidateEmail: email, candidateName, dueDate });
    link = inviteLink(application.invite?.token) ?? undefined;

    // The invitation exists from here on, and shows on the candidate's
    // dashboard whether or not a message reaches them. The email is what
    // tells them it's there — so what the form reports is what actually
    // happened to the email, not just "Invited". The candidate's own task
    // is written for them over the next minute or two.
    const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
    if (!mailerReady() || !portal || !link) {
      notice = `Invited — their own task is being prepared. No email was sent (${!mailerReady() ? "email isn't set up on this portal" : "CANDIDATE_PORTAL_URL isn't set, so there is no link"})${link ? "; send them the link below" : `; tell ${email} to sign in to the candidate portal with that address`}.`;
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
    return { error: e instanceof Error ? e.message : "Couldn't invite that candidate — try again.", success: false };
  }

  revalidatePath(`/roles/${roleId}`);
  revalidatePath("/roles");
  revalidatePath("/candidates");
  revalidatePath("/dashboard");
  return { error: null, success: true, notice, link };
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
