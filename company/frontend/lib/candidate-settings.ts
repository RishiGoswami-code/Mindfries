import "server-only";
import { db } from "./supabase";

// What a candidate has chosen in their Settings (candidate/frontend's
// /settings, migration 0026): which emails they want, and which parts of
// their profile hiring teams may see. This portal honours both — it doesn't
// email a kind the candidate turned off, and leaves hidden parts out of the
// profile it shows.
//
// The lists match candidate/frontend/src/lib/settings/shape.ts and the check
// constraint in 0026. Before 0026 is applied every column is missing, and
// the answer is "nothing hidden, every email on" — what was true before.

export type CandidateEmailKind = "invitations" | "interviews" | "messages";

export type HiddenField = "summary" | "location" | "photo" | "code" | "linkedin" | "portfolio" | "resume" | "preferences" | "documents";
const HIDEABLE: HiddenField[] = ["summary", "location", "photo", "code", "linkedin", "portfolio", "resume", "preferences", "documents"];

export interface CandidateSettings {
  email: Record<CandidateEmailKind, boolean>;
  hidden: Set<HiddenField>;
  hiddenProjects: Set<string>;
}

const OPEN: CandidateSettings = { email: { invitations: true, interviews: true, messages: true }, hidden: new Set(), hiddenProjects: new Set() };

/* eslint-disable @typescript-eslint/no-explicit-any */
function shape(row: any): CandidateSettings {
  const prefs = row?.email_prefs && typeof row.email_prefs === "object" ? row.email_prefs : {};
  return {
    email: { invitations: prefs.invitations !== false, interviews: prefs.interviews !== false, messages: prefs.messages !== false },
    hidden: new Set(Array.isArray(row?.hidden_fields) ? HIDEABLE.filter((f) => row.hidden_fields.includes(f)) : []),
    hiddenProjects: new Set(Array.isArray(row?.hidden_projects) ? row.hidden_projects.filter((u: unknown) => typeof u === "string") : []),
  };
}

/** By candidate account id. */
export async function settingsFor(candidateId: string): Promise<CandidateSettings> {
  const c = db();
  if (!c) return OPEN;
  const { data, error } = await c.from("candidate_users").select("email_prefs, hidden_fields, hidden_projects").eq("id", candidateId).maybeSingle();
  return error || !data ? OPEN : shape(data);
}

/** By several account ids at once. An id with no row gets the open defaults. */
export async function settingsForMany(candidateIds: string[]): Promise<Map<string, CandidateSettings>> {
  const out = new Map<string, CandidateSettings>();
  const c = db();
  if (!c || candidateIds.length === 0) return out;
  const { data, error } = await c.from("candidate_users").select("id, email_prefs, hidden_fields, hidden_projects").in("id", candidateIds);
  if (error) return out;
  for (const row of data ?? []) out.set(row.id as string, shape(row));
  return out;
}

/**
 * Whether this kind of email may go to this address. An address with no
 * candidate account (invited, never signed up) has said nothing, so yes.
 */
export async function candidateWantsEmail(email: string, kind: CandidateEmailKind): Promise<boolean> {
  const c = db();
  if (!c) return true;
  const { data, error } = await c.from("candidate_users").select("email_prefs").eq("email", email.trim().toLowerCase()).maybeSingle();
  if (error || !data) return true;
  return shape(data).email[kind];
}
