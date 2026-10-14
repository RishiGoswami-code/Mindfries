"use server";

import { revalidatePath } from "next/cache";
import {
  addOnboarded, addWaitlist, waitlistActivity, createCompany, listCompanyRoles, markInvitationEmailedByToken, type CompanyRole, createInvitation, createTemplate, recordEmailEvent, setCompanyStatus,
  setLeadStage, setSessionState, setTemplateStatus,
} from "@/lib/db";
import { mailerReady, sendMail, NOTIFY_EMAIL } from "@/lib/mailer";
import { targetsStore } from "@/lib/targets-store";
import { requireAdminRole } from "@/lib/auth/admins";
import { backendReady, generateTaskViaBackend, resetSessionViaBackend, retriggerEvaluationViaBackend, type GeneratedTask } from "@/lib/backend/client";
import type { EmailTemplate } from "@/lib/email-templates";
import type { CompanyStatus, LeadStage, MemberRole, Plan, RubricCriterion, TaskVariant, TemplateStatus, TemplateVariant } from "@/lib/types";

type Result = { ok: true } | { ok: false; error: string };
const fail = (e: unknown): Result => ({ ok: false, error: e instanceof Error ? e.message : String(e) });

// Send a demo/POC email to a lead, log it, advance the pipeline.
export async function sendLeadEmail(input: {
  leadId: string; template: EmailTemplate; to: string; subject: string; body: string;
}): Promise<Result> {
  try {
    await requireAdminRole();
    if (!input.to.trim()) throw new Error("No recipient email — fill in a contact address first");
    const id = await sendMail({ to: input.to, subject: input.subject, text: input.body, replyTo: NOTIFY_EMAIL });
    await recordEmailEvent({ leadId: input.leadId, type: "sent", template: input.template, resendId: id });
    revalidatePath("/admin/tracker");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

export async function changeLeadStage(leadId: string, stage: LeadStage): Promise<Result> {
  try {
    await requireAdminRole();
    await setLeadStage(leadId, stage);
    // A manual "mark replied" is also a tracked reply event.
    if (stage === "replied") await recordEmailEvent({ leadId, type: "replied" });
    revalidatePath("/admin/tracker");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

// Onboard a company: create the real record, mark the source lead onboarded,
// and best-effort notify the company contact.
//
// Now sends a real sign-in link when COMPANY_PORTAL_URL and
// COMPANY_INVITE_SECRET are both configured (IMPLEMENTATION.md §3.6, §6) —
// addOnboarded creates the company_users row and mints the token; this is
// the layer that decides whether an email claiming a real link is honest to
// send. Falls back to the old "we'll be in touch" message otherwise, rather
// than claiming a sign-in link that can't be verified anywhere — this used
// to generate one unconditionally, pointing at a URL
// (https://app.mindfries.com) that didn't exist. Not before now.
export async function onboardCompany(input: {
  leadId?: string; company: string; adminEmail: string; plan: Plan; monthlyCost: number; targetId?: string;
}): Promise<Result> {
  try {
    await requireAdminRole();
    if (!input.company.trim() || !input.adminEmail.trim()) throw new Error("Company and admin email are required");

    const { inviteToken } = await addOnboarded({
      leadId: input.leadId, company: input.company, adminEmail: input.adminEmail,
      plan: input.plan, monthlyCost: input.monthlyCost,
    });

    const portalUrl = process.env.COMPANY_PORTAL_URL;
    try {
      if (inviteToken && portalUrl) {
        await sendMail({
          to: input.adminEmail,
          subject: `You're onboarded with Mindfries, ${input.company}!`,
          text:
`Welcome to Mindfries, ${input.company}!

Your Company Portal workspace is ready. Set your password to sign in:

${portalUrl}/set-password?token=${inviteToken}

This link expires in 7 days. Reply to this email if you need a hand.

— The Mindfries team`,
        });
      } else {
        await sendMail({
          to: input.adminEmail,
          subject: `You're onboarded with Mindfries, ${input.company}!`,
          text:
`Welcome to Mindfries, ${input.company}!

Your account is set up on our end — we'll be in touch shortly with next steps to get your team started.

Reply to this email if you need a hand in the meantime.

— The Mindfries team`,
        });
      }
    } catch {
      // Best-effort, same as the waitlist form's own notification (below).
      // This used to be unguarded: a missing Resend key made the whole
      // action fail here, before the real company record above was ever
      // created — with RESEND_API_KEY unset (true everywhere today), that
      // meant onboarding a company via the Tracker failed outright.
    }
    // Came from a target: close the loop so it doesn't sit in "Pilot" forever.
    // Only after the record was created — a failed onboarding must not mark
    // anything won. A note, not a touch: it isn't contact with them.
    if (typeof input.targetId === "string" && input.targetId) {
      const store = targetsStore();
      if (await store.getTarget(input.targetId)) {
        await store.updateTarget(input.targetId, { stage: "won", nextAction: null, nextActionDue: null });
        await store.insertActivity({
          targetId: input.targetId, contactId: null, channel: "note", direction: null, outcome: null,
          summary: `Onboarded on the ${input.plan} plan — ${input.adminEmail} notified.`,
          happenedAt: new Date().toISOString(), by: null,
        });
        revalidatePath("/admin/targets");
        revalidatePath(`/admin/targets/${input.targetId}`);
      }
    }
    revalidatePath("/admin/onboarding");
    revalidatePath("/admin/costs");
    revalidatePath("/admin/tracker");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

// Author a game into the shared library (candidate app reads published ones).
export async function createGameTemplate(input: {
  name: string; taskVariant: TaskVariant; repoTemplate: string; techStack: string[];
  durationMin: number; interviewerPrompt: string; rubric: RubricCriterion[]; status: TemplateStatus;
  taskBrief?: string; starterFiles?: Record<string, string>;
  solutionFiles?: Record<string, string>; verification?: unknown; variants?: TemplateVariant[];
}): Promise<Result> {
  try {
    await requireAdminRole();
    if (!input.name.trim()) throw new Error("Game name is required");
    await createTemplate(input);
    revalidatePath("/admin/library");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

/**
 * Drafts a task brief and starter codebase with the Task Generation agent,
 * from what the author has already filled in plus free-text notes about the
 * company and role. Returns the draft for the form to show — the author
 * reads and edits it, then saves it like any hand-written game. There is no
 * fallback: without the backend and a model key there is nothing to draft
 * with, and the form says so.
 */
export async function generateTaskDraft(input: {
  name: string; taskVariant: TaskVariant; techStack: string[]; durationMin: number; notes: string;
  jobDescription?: string; differentFrom?: string;
}): Promise<{ ok: true; task: GeneratedTask } | { ok: false; error: string }> {
  try {
    await requireAdminRole();
    if (!input.name.trim()) throw new Error("Give the game a name first — the task is generated from it");
    if (!backendReady()) throw new Error("Task generation isn't connected — set ADMIN_BACKEND_URL to the candidate backend");
    return { ok: true, task: await generateTaskViaBackend(input) };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
}

export async function toggleTemplateStatus(id: string, status: TemplateStatus): Promise<Result> {
  try {
    await requireAdminRole();
    await setTemplateStatus(id, status);
    revalidatePath("/admin/library");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

// Company Onboarding, wired for real (ADMIN_BACKEND_PLAN.md §5.1) — this
// used to be a client component's local useState, persisting nothing.
export async function onboardCompanyAccount(input: {
  name: string; website: string; plan: Plan; status: CompanyStatus; seats: number;
  team: { email: string; role: MemberRole }[]; defaultTemplateIds: string[];
}): Promise<Result> {
  try {
    await requireAdminRole();
    if (!input.name.trim()) throw new Error("Company name is required");
    await createCompany(input);
    revalidatePath("/admin/companies");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

export async function setCompanyStatusAction(id: string, status: CompanyStatus): Promise<Result> {
  try {
    await requireAdminRole();
    await setCompanyStatus(id, status);
    revalidatePath("/admin/companies");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

// Invite a candidate on a company's behalf. The invitation is for one of
// the company's roles (or a new one described here); the candidate gets
// their own task, written for them from the role's brief by
// candidate/backend, and their own link (lib/db.ts's createInvitation). The
// link is returned so it can be sent another way when mail isn't set up.
export async function inviteCandidate(input: {
  companyId: string;
  roleId?: string;
  newRole?: { title: string; assessmentBrief: string; taskVariant: string };
  candidateEmail: string;
  candidateName?: string;
  dueDate?: string;
}): Promise<{ ok: true; link: string | null; emailed: boolean; note: string } | { ok: false; error: string }> {
  try {
    await requireAdminRole();
    if (!input.companyId) throw new Error("Pick a company");
    const email = input.candidateEmail.trim().toLowerCase().slice(0, 254);
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) throw new Error("That doesn't look like an email address");
    const variants = ["bug_fix", "feature", "refactor", "debug"] as const;
    const newRole = input.newRole
      ? {
          title: input.newRole.title.trim().slice(0, 200),
          assessmentBrief: input.newRole.assessmentBrief.trim().slice(0, 4000),
          taskVariant: (variants as readonly string[]).includes(input.newRole.taskVariant) ? (input.newRole.taskVariant as TaskVariant) : "bug_fix",
        }
      : undefined;
    if (!input.roleId && !newRole?.title) throw new Error("Pick a role, or give the new one a title");

    const { inviteToken, roleTitle } = await createInvitation({
      companyId: input.companyId,
      roleId: input.roleId || undefined,
      newRole: input.roleId ? undefined : newRole,
      candidateEmail: email,
      candidateName: input.candidateName?.trim().slice(0, 200) || undefined,
      dueDate: input.dueDate || undefined,
    });

    const portal = process.env.CANDIDATE_PORTAL_URL?.replace(/\/+$/, "");
    const link = portal && inviteToken ? `${portal}/invite/${inviteToken}` : null;
    let emailed = false;
    let note = "Their own task is being prepared — usually a minute or two.";
    if (link && mailerReady()) {
      try {
        await sendMail({
          to: email,
          subject: `You're invited to a technical assessment for ${roleTitle}`,
          text:
            `Hi${input.candidateName?.trim() ? ` ${input.candidateName.trim().split(/\s+/)[0]}` : ""},\n\n` +
            `You've been invited to take a technical assessment for ${roleTitle} on Mindfries.\n\n` +
            `Your invitation, and the assessment itself, are here:\n\n${link}\n\n` +
            `Sign in — or create an account — with this email address; the invitation is tied to it, and the link works for no one else.\n` +
            (input.dueDate ? `\nPlease complete it by ${input.dueDate}.\n` : "") +
            `\nNothing starts, and no clock runs, until you choose to begin.\n`,
          replyTo: NOTIFY_EMAIL,
        });
        emailed = true;
        await markInvitationEmailedByToken(inviteToken);
      } catch (e) {
        note = `The email couldn't be sent (${e instanceof Error ? e.message : "unknown error"}) — send them the link yourself. ${note}`;
      }
    } else {
      note = `No email was sent (${!link ? "CANDIDATE_PORTAL_URL isn't set, so there is no link" : "email isn't set up"})${link ? " — send them the link" : ""}. ${note}`;
    }
    revalidatePath("/admin/companies");
    return { ok: true, link, emailed, note };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
}

/** The company's open roles, for the invite form. */
export async function companyRoles(companyId: string): Promise<CompanyRole[]> {
  await requireAdminRole();
  return listCompanyRoles(companyId);
}

// Session Monitor support overrides — go through the real Go Admin API
// (candidate/backend/internal/httpapi/admin.go) when it's reachable, rather
// than internal-admin's own direct-to-Supabase write. This is the
// consolidation task.md's "Gaps worth naming" #3 asked for: two separate
// implementations of "reset a session" is a real maintenance hazard, and
// for retrigger specifically the two aren't even equivalent today — the
// direct-Supabase version below only ever flips sessions.status to
// "evaluating"; it never actually calls an evaluation agent. The backend's
// own /retrigger-evaluation runs the real pipeline. ADMIN_BACKEND_URL is
// unset in every deployed environment right now (the Go backend isn't
// deployed anywhere — see task.md), so the fallback below is what actually
// runs today; it's kept, not deleted, so this feature doesn't regress the
// moment nobody's set that variable yet. Once the backend has a real home,
// removing the fallback finishes what this pass started.
export async function resetSession(id: string): Promise<Result> {
  try {
    await requireAdminRole();
    if (backendReady()) {
      await resetSessionViaBackend(id);
    } else {
      await setSessionState(id, { status: "live", sandboxHealth: "healthy", progressPct: 0, elapsedMin: 0 });
    }
    revalidatePath("/admin/sessions");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

export async function retriggerEval(id: string): Promise<Result> {
  try {
    await requireAdminRole();
    if (backendReady()) {
      await retriggerEvaluationViaBackend(id);
    } else {
      // Honest about what this fallback actually does: it flips the status
      // flag the Sessions page reads, but doesn't call an evaluation agent
      // the way the real backend path (above) does. Configuring
      // ADMIN_BACKEND_URL is what makes "retrigger" genuinely retrigger.
      await setSessionState(id, { status: "evaluating" });
    }
    revalidatePath("/admin/sessions");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}

// Public waitlist form → store + ping the team inbox.
//
// The one action here anyone on the internet can call, so nothing about its
// input is taken on trust: every field is capped, and how much it will
// accept is bounded.
//
//   - `website` is a field no person sees or fills in (WaitlistForm hides
//     it). A form-filling script fills it; that submission is answered as
//     if it worked and dropped.
//   - An address that signed up in the last day is answered the same way,
//     without a second row or a second email to the team.
//   - Past WAITLIST_PER_HOUR sign-ups in an hour, across everyone, the form
//     asks to be tried later. A real launch day could reach that; a few
//     people waiting an hour is the cheaper failure than a full table and a
//     flooded inbox.
const WAITLIST_PER_HOUR = 60;
const cap = (value: unknown, max: number) => (typeof value === "string" ? value.trim().slice(0, max) : "");

export async function joinWaitlist(raw: {
  name?: string; email: string; company?: string; message?: string; website?: string;
}): Promise<Result> {
  try {
    if (cap(raw?.website, 200)) return { ok: true };
    const input = {
      email: cap(raw?.email, 254).toLowerCase(),
      name: cap(raw?.name, 120) || undefined,
      company: cap(raw?.company, 160) || undefined,
      message: cap(raw?.message, 2000) || undefined,
    };
    if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(input.email)) throw new Error("Enter a valid email");
    const activity = await waitlistActivity(input.email);
    if (activity.sameEmailToday) return { ok: true };
    if (activity.lastHour >= WAITLIST_PER_HOUR) throw new Error("We're getting a lot of sign-ups right now — please try again in an hour.");
    await addWaitlist(input);
    try {
      await sendMail({
        to: NOTIFY_EMAIL,
        subject: `New Mindfries waitlist signup${input.company ? ` — ${input.company}` : ""}`.replace(/[\r\n]+/g, " "),
        text: `${input.name ?? "Someone"} (${input.email})${input.company ? ` from ${input.company}` : ""} joined the waitlist.\n\n${input.message ?? ""}`,
      });
    } catch {
      // Notification is best-effort; the signup is already saved.
    }
    revalidatePath("/admin/waitlist");
    return { ok: true };
  } catch (e) {
    return fail(e);
  }
}
