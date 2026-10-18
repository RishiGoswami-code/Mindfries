import "server-only";
import { db } from "./supabase";

// A company's access to hiring, and its credits (supabase/migrations/0027).
//
// A company that signed itself up starts 'pending' with 0 credits: it can
// sign in and look around, but creating roles and inviting candidates stay
// locked until a Mindfries admin grants access and credits from
// internal-admin. Inviting a candidate spends 1 credit.
//
// `credits: null` means "not metered" — every company Mindfries onboarded
// itself — and for those nothing is counted or blocked.

export type AccessStatus = "pending" | "active" | "declined";

export interface CompanyAccess {
  status: AccessStatus;
  /** The balance; null when the company isn't metered. */
  credits: number | null;
  /** The company's latest "remind the team" nudge, if any. */
  requestedAt: string | null;
}

/** What a company's access is when it can't be read — a database that isn't set up, or isn't migrated yet: unrestricted, as before credits existed. */
const UNMETERED: CompanyAccess = { status: "active", credits: null, requestedAt: null };

export const SUPPORT_EMAIL = "officemindfries@gmail.com";

export async function getCompanyAccess(companyId: string): Promise<CompanyAccess> {
  const c = db();
  if (!c) return UNMETERED;
  const { data, error } = await c
    .from("companies")
    .select("access_status, credits, access_requested_at")
    .eq("id", companyId)
    .maybeSingle<{ access_status: AccessStatus | null; credits: number | null; access_requested_at: string | null }>();
  if (error || !data) return UNMETERED;
  return { status: data.access_status ?? "active", credits: data.credits ?? null, requestedAt: data.access_requested_at ?? null };
}

/** True when the company may create roles and invite (it may still have no credits left to invite with). */
export const hasHiringAccess = (a: CompanyAccess) => a.status === "active";

/** True when the company could invite someone right now. */
export const canInvite = (a: CompanyAccess) => a.status === "active" && (a.credits === null || a.credits > 0);

/** Plain words for why hiring is locked, for an action to return as its error. */
export function lockedReason(a: CompanyAccess): string | null {
  if (a.status === "pending") return "Your account is waiting for access. The Mindfries team adds credits once it's reviewed.";
  if (a.status === "declined") return `Your account doesn't have access. Contact ${SUPPORT_EMAIL} if you think that's a mistake.`;
  if (a.credits !== null && a.credits < 1) return "You're out of credits. Ask the Mindfries team for more.";
  return null;
}

/** Throws with lockedReason when the company can't create roles. */
export async function requireHiringAccess(companyId: string): Promise<CompanyAccess> {
  const access = await getCompanyAccess(companyId);
  if (!hasHiringAccess(access)) throw new Error(lockedReason(access) ?? "Your account doesn't have access yet.");
  return access;
}

export type Spend = { metered: false } | { metered: true; left: number };

/**
 * Spends one credit for an invitation, atomically in the database
 * (spend_company_credit locks the company's row), so two invitations racing
 * for a last credit can't both get it. Throws with a plain reason when the
 * company has no access or no credits. Pair with refundCredit if the
 * invitation then fails.
 */
export async function spendCredit(companyId: string): Promise<Spend> {
  const c = db();
  if (!c) return { metered: false };
  const { data, error } = await c.rpc("spend_company_credit", { p_company: companyId });
  if (error) {
    const msg = error.message ?? "";
    // Not migrated yet: behave as before credits existed.
    if ((error as { code?: string }).code === "PGRST202" || msg.includes("Could not find the function")) return { metered: false };
    if (msg.includes("NO_ACCESS")) throw new Error(lockedReason(await getCompanyAccess(companyId)) ?? "Your account doesn't have access yet.");
    if (msg.includes("NO_CREDITS")) throw new Error("You're out of credits. Ask the Mindfries team for more.");
    throw new Error("Couldn't check your credits — try again.");
  }
  return data === null || data === undefined ? { metered: false } : { metered: true, left: Number(data) };
}

/** Gives back a credit spent on an invitation that then couldn't be made. Best effort. */
export async function refundCredit(companyId: string): Promise<void> {
  const c = db();
  if (!c) return;
  await c.rpc("refund_company_credit", { p_company: companyId });
}

/** Records the company's "remind the team" nudge. */
export async function markAccessRequested(companyId: string): Promise<void> {
  const c = db();
  if (!c) return;
  await c.from("companies").update({ access_requested_at: new Date().toISOString() }).eq("id", companyId);
}
