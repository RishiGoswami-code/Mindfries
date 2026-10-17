"use server";

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { db } from "@/lib/supabase";
import { currentCandidate } from "@/lib/auth/users";
import { hashPassword, verifyPassword } from "@/lib/auth/password";
import { requestPasswordReset } from "@/lib/auth/email-links";
import { attemptsExceeded, noteAttempt, waitMessage } from "@/lib/auth/throttle";
import { SESSION_COOKIE, SESSION_MAX_AGE, sessionSecret, signSession } from "@/lib/auth/session";
import { endConnection } from "@/lib/composio";
import { DOCUMENTS_BUCKET, isMissingSchema } from "@/lib/profile/extras";
import { PRIVACY_CONTACT } from "@/lib/policies";
import { DELETE_PHRASE, EMAIL_KINDS, HIDEABLE_FIELDS, readEmailPrefs, readHiddenFields, readHiddenProjects, type EmailKind, type HideableField } from "./shape";

/**
 * The Settings page's writes. Each one is a public endpoint: it acts only on
 * the signed-in candidate's own row, and takes nothing else from the caller
 * but the value being set — checked against the fixed lists in shape.ts.
 */

export type SettingResult = { ok: true } | { ok: false; error: string };

const NOT_SET_UP = "Settings aren't set up on this database yet (migration 0026).";

async function readRow(candidateId: string) {
  const c = db();
  if (!c) return { c: null, row: null, error: "The database isn't connected." } as const;
  const { data, error } = await c.from("candidate_users").select("email_prefs, hidden_fields, hidden_projects").eq("id", candidateId).maybeSingle();
  if (error) return { c, row: null, error: isMissingSchema(error) ? NOT_SET_UP : "Couldn't read your settings — try again." } as const;
  return { c, row: data, error: null } as const;
}

function refresh() {
  revalidatePath("/settings");
  revalidatePath("/profile");
}

export async function setEmailPreference(kind: EmailKind, on: boolean): Promise<SettingResult> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  if (!EMAIL_KINDS.includes(kind) || typeof on !== "boolean") return { ok: false, error: "Unknown setting." };
  const { c, row, error } = await readRow(session.id);
  if (!c || error) return { ok: false, error: error ?? "Couldn't save." };
  const email_prefs = { ...readEmailPrefs(row?.email_prefs), [kind]: on };
  const { error: saveError } = await c.from("candidate_users").update({ email_prefs }).eq("id", session.id);
  if (saveError) return { ok: false, error: "Couldn't save that — try again." };
  refresh();
  return { ok: true };
}

export async function setFieldHidden(field: HideableField, hidden: boolean): Promise<SettingResult> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  if (!HIDEABLE_FIELDS.includes(field) || typeof hidden !== "boolean") return { ok: false, error: "Unknown setting." };
  const { c, row, error } = await readRow(session.id);
  if (!c || error) return { ok: false, error: error ?? "Couldn't save." };
  const current = new Set(readHiddenFields(row?.hidden_fields));
  if (hidden) current.add(field);
  else current.delete(field);
  const { error: saveError } = await c
    .from("candidate_users")
    .update({ hidden_fields: HIDEABLE_FIELDS.filter((f) => current.has(f)) })
    .eq("id", session.id);
  if (saveError) return { ok: false, error: "Couldn't save that — try again." };
  refresh();
  return { ok: true };
}

/**
 * Shows or hides one project from a linked account. Only an address that is
 * actually among the candidate's stored projects can be hidden — anything
 * else is refused, so the list can't be filled with arbitrary strings.
 */
export async function setProjectHidden(url: string, hidden: boolean): Promise<SettingResult> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  if (typeof url !== "string" || typeof hidden !== "boolean") return { ok: false, error: "Unknown project." };
  const { c, row, error } = await readRow(session.id);
  if (!c || error) return { ok: false, error: error ?? "Couldn't save." };

  const current = new Set(readHiddenProjects(row?.hidden_projects));
  if (hidden) {
    const { data: knowledge } = await c.from("candidate_knowledge").select("payload").eq("candidate_id", session.id).in("source", ["github", "gitlab"]);
    const known = (knowledge ?? []).some((k) => Array.isArray(k.payload?.projects) && k.payload.projects.some((p: { url?: unknown }) => p?.url === url));
    if (!known) return { ok: false, error: "That project isn't on your linked accounts." };
    current.add(url);
  } else {
    current.delete(url);
  }
  const { error: saveError } = await c.from("candidate_users").update({ hidden_projects: [...current] }).eq("id", session.id);
  if (saveError) return { ok: false, error: "Couldn't save that — try again." };
  refresh();
  return { ok: true };
}

export type PasswordState = { error: string | null; notice: string | null };

const MIN_PASSWORD_LENGTH = 8;
const text = (v: unknown, max: number) => (typeof v === "string" ? v.slice(0, max) : "");

/**
 * Changes the password while signed in, given the current one. Every other
 * browser and device is signed out — the reason someone changes a password
 * is usually that someone else might know it — and this one is issued a
 * fresh session so it stays signed in.
 */
export async function changePassword(_prev: PasswordState, form: FormData): Promise<PasswordState> {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in.", notice: null };
  const secret = sessionSecret();
  if (!secret) return { error: "SESSION_SECRET isn't set on this site.", notice: null };
  const c = db();
  if (!c) return { error: "The database isn't connected.", notice: null };

  const current = text(form.get("current"), 512);
  const next = text(form.get("next"), 512);
  const confirm = text(form.get("confirm"), 512);
  if (next.length < MIN_PASSWORD_LENGTH) return { error: `Use at least ${MIN_PASSWORD_LENGTH} characters for the new password.`, notice: null };
  if (next !== confirm) return { error: "The new passwords don't match.", notice: null };
  if (next === current) return { error: "That's the password you already have.", notice: null };

  // A guess at the current password counts like a guess at sign-in.
  const wait = await attemptsExceeded("login");
  if (wait > 0) return { error: waitMessage(wait), notice: null };

  const { data: row } = await c.from("candidate_users").select("password_hash").eq("id", session.id).maybeSingle();
  if (!row?.password_hash) return { error: "Your account doesn't have a password yet — send yourself a link to set one.", notice: null };
  if (!(await verifyPassword(current, row.password_hash as string))) {
    await noteAttempt("login");
    return { error: "Your current password isn't right.", notice: null };
  }

  const password_hash = await hashPassword(next);
  const now = new Date();
  const { error } = await c
    .from("candidate_users")
    .update({ password_hash, failed_attempts: 0, locked_until: null, sessions_valid_from: now.toISOString() })
    .eq("id", session.id);
  if (error) return { error: "Couldn't save the new password — try again.", notice: null };

  // Issued after sessions_valid_from, so this browser's session still counts.
  const jar = await cookies();
  jar.set(SESSION_COOKIE, await signSession({ id: session.id, email: session.email, name: session.name }, secret), {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: SESSION_MAX_AGE,
  });
  return { error: null, notice: "Password changed. Every other browser and device has been signed out." };
}

/** For an account with no password (made through Google, GitHub…): mails the link that sets one. */
export async function sendPasswordLink(): Promise<SettingResult> {
  const session = await currentCandidate();
  if (!session) return { ok: false, error: "Not signed in." };
  const result = await requestPasswordReset(session.email);
  if (result === "not_configured") return { ok: false, error: "Email isn't set up on this site, so the link can't be sent." };
  return { ok: true };
}

export type DeleteState = { error: string | null };

/**
 * Deletes the account, after the candidate types the phrase. Removed: the
 * account row and everything that hangs off it (sign-in identities, linked
 * account reads, documents, preferences, notification state, practice runs
 * — all `on delete cascade`), the resume and document files, and the
 * connections to GitHub, GitLab or LinkedIn. Applications and assessments
 * belong to the companies that ran them: they stay, with the link to this
 * account cleared (`on delete set null`), so those companies keep their
 * records but no longer see a profile behind them.
 */
export async function deleteAccount(_prev: DeleteState, form: FormData): Promise<DeleteState> {
  const session = await currentCandidate();
  if (!session) return { error: "Not signed in." };
  if (text(form.get("confirm"), 100).trim().toLowerCase() !== DELETE_PHRASE) return { error: `Type “${DELETE_PHRASE}” to confirm.` };
  const c = db();
  if (!c) return { error: "The database isn't connected." };

  const { data: row } = await c.from("candidate_users").select("resume_path, links").eq("id", session.id).maybeSingle();
  const { data: documents } = await c.from("candidate_documents").select("path").eq("candidate_id", session.id);

  // Files first: once the row is gone, nothing points at them.
  if (row?.resume_path) await c.storage.from("resumes").remove([row.resume_path as string]);
  const paths = (documents ?? []).map((d) => d.path as string).filter(Boolean);
  if (paths.length) await c.storage.from(DOCUMENTS_BUCKET).remove(paths);
  for (const link of Object.values((row?.links ?? {}) as Record<string, { connectionId?: unknown }>)) {
    if (typeof link?.connectionId === "string" && link.connectionId) await endConnection(link.connectionId);
  }

  const { error } = await c.from("candidate_users").delete().eq("id", session.id);
  if (error) return { error: `Couldn't delete the account — try again, or email ${PRIVACY_CONTACT}.` };

  const jar = await cookies();
  jar.delete(SESSION_COOKIE);
  redirect("/login?deleted=1");
}
