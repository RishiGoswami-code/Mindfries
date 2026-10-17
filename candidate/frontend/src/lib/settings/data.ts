import "server-only";
import { db } from "@/lib/supabase";
import { isMissingSchema } from "@/lib/profile/extras";
import { readEmailPrefs, readHiddenFields, readHiddenProjects, type AccountSettings } from "./shape";

/**
 * Reads for the Settings page. The settings columns come from migration
 * 0026 and are read apart from everything else, so a database without it
 * still loads the page — which then says settings aren't set up instead of
 * showing switches that would save nowhere.
 */

const DEFAULTS: AccountSettings = { available: false, email: readEmailPrefs(null), hiddenFields: [], hiddenProjects: [] };

export async function getAccountSettings(candidateId: string): Promise<AccountSettings> {
  const c = db();
  if (!c) return DEFAULTS;
  const { data, error } = await c.from("candidate_users").select("email_prefs, hidden_fields, hidden_projects").eq("id", candidateId).maybeSingle();
  if (error) {
    if (!isMissingSchema(error)) console.error("settings: couldn't read settings:", error.message);
    return DEFAULTS;
  }
  return {
    available: true,
    email: readEmailPrefs(data?.email_prefs),
    hiddenFields: readHiddenFields(data?.hidden_fields),
    hiddenProjects: readHiddenProjects(data?.hidden_projects),
  };
}

export interface SignInMethods {
  email: string;
  emailVerified: boolean;
  /** False for an account made through Google/GitHub/GitLab/LinkedIn that never set a password. */
  hasPassword: boolean;
  /** Providers this account signs in with, and when each was linked. */
  providers: Array<{ provider: string; linkedAt: string }>;
  createdAt: string | null;
  lastLoginAt: string | null;
}

export async function getSignInMethods(candidateId: string): Promise<SignInMethods | null> {
  const c = db();
  if (!c) return null;
  const [{ data: user }, { data: identities }] = await Promise.all([
    c.from("candidate_users").select("email, password_hash, email_verified_at, created_at, last_login_at").eq("id", candidateId).maybeSingle(),
    c.from("candidate_oauth_identities").select("provider, created_at").eq("candidate_id", candidateId).order("created_at"),
  ]);
  if (!user) return null;
  return {
    email: user.email as string,
    emailVerified: !!user.email_verified_at,
    hasPassword: !!user.password_hash,
    providers: (identities ?? []).map((i) => ({ provider: i.provider as string, linkedAt: i.created_at as string })),
    createdAt: (user.created_at as string) ?? null,
    lastLoginAt: (user.last_login_at as string) ?? null,
  };
}
