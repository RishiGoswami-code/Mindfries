import "server-only";
import { db } from "../supabase";
import { hashPassword } from "./password";
import type { Session } from "./session";

// A company signing itself up (company/frontend /signup). It gets a
// company row — access 'pending', 0 credits, signup_source 'self' — and the
// person signing up becomes its admin, active straight away so they can
// sign in and look around. Hiring stays locked until Mindfries grants
// access (lib/access.ts, internal-admin /admin/access).

export const MIN_PASSWORD = 8;

export type SignUpInput = { companyName: string; website: string; name: string; email: string; password: string };
export type SignUpResult = { ok: true; session: Omit<Session, "exp"> } | { ok: false; error: string };

export async function createCompanyAccount(input: SignUpInput): Promise<SignUpResult> {
  const c = db();
  if (!c) return { ok: false, error: "Sign-up isn't available yet — the database isn't connected." };

  const email = input.email.trim().toLowerCase();

  // Emails are stored lower-cased (checkCredentials matches the same way).
  const { data: existing } = await c.from("company_users").select("id").eq("email", email).maybeSingle();
  if (existing) return { ok: false, error: "There's already an account with that email. Sign in instead." };

  const { data: company, error: companyError } = await c
    .from("companies")
    .insert({
      name: input.companyName,
      website: input.website || null,
      plan: "trial",
      status: "onboarding",
      access_status: "pending",
      credits: 0,
      signup_source: "self",
    })
    .select("id, name")
    .single<{ id: string; name: string }>();
  if (companyError || !company) {
    if ((companyError as { code?: string } | null)?.code === "PGRST204") {
      return { ok: false, error: "Sign-up needs the latest database migration (0027). Ask the Mindfries team." };
    }
    return { ok: false, error: "Couldn't create your account — try again." };
  }

  const password_hash = await hashPassword(input.password);
  const { error: userError } = await c.from("company_users").insert({
    company_id: company.id,
    email,
    name: input.name,
    password_hash,
    role: "admin",
    status: "active",
  });
  if (userError) {
    // Don't leave a company with nobody able to sign in to it.
    await c.from("companies").delete().eq("id", company.id);
    return {
      ok: false,
      error: (userError as { code?: string }).code === "23505" ? "There's already an account with that email. Sign in instead." : "Couldn't create your account — try again.",
    };
  }

  return { ok: true, session: { email, name: input.name, role: "admin", companyId: company.id, companyName: company.name } };
}
