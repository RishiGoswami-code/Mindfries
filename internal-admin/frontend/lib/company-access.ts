import "server-only";
import { db } from "./supabase";
import type { Plan } from "./types";

// Companies that signed themselves up on the company portal, and the
// credits Mindfries grants them (supabase/migrations/0027). A self-signed-up
// company starts 'pending' with 0 credits; granting access here switches
// hiring on and adds credits (1 credit = 1 candidate invited).

export type AccessStatus = "pending" | "active" | "declined";

export interface AccessCompany {
  id: string;
  name: string;
  website: string | null;
  plan: Plan;
  status: AccessStatus;
  credits: number | null;
  requestedAt: string | null;
  createdAt: string;
  /** The company's own admin — whoever signed it up. */
  contact: { name: string; email: string } | null;
  grants: { amount: number; plan: string | null; note: string | null; by: string; at: string }[];
}

export async function listSelfSignupCompanies(): Promise<AccessCompany[]> {
  const c = db();
  if (!c) return [];
  const { data: companies, error } = await c
    .from("companies")
    .select("id, name, website, plan, access_status, credits, access_requested_at, created_at")
    .eq("signup_source", "self")
    .order("created_at", { ascending: false });
  if (error || !companies?.length) return [];

  const ids = companies.map((r) => r.id as string);
  const [{ data: users }, { data: grants }] = await Promise.all([
    c.from("company_users").select("company_id, name, email, role, created_at").in("company_id", ids).order("created_at", { ascending: true }),
    c.from("company_credit_grants").select("company_id, amount, plan, note, granted_by, created_at").in("company_id", ids).order("created_at", { ascending: false }),
  ]);

  return companies.map((r) => {
    const own = (users ?? []).filter((u) => u.company_id === r.id);
    const contact = own.find((u) => u.role === "admin") ?? own[0] ?? null;
    return {
      id: r.id,
      name: r.name,
      website: r.website ?? null,
      plan: r.plan as Plan,
      status: (r.access_status ?? "pending") as AccessStatus,
      credits: r.credits ?? null,
      requestedAt: r.access_requested_at ?? null,
      createdAt: r.created_at,
      contact: contact ? { name: contact.name, email: contact.email } : null,
      grants: (grants ?? [])
        .filter((g) => g.company_id === r.id)
        .map((g) => ({ amount: g.amount, plan: g.plan ?? null, note: g.note ?? null, by: g.granted_by, at: g.created_at })),
    };
  });
}

/** Grants access and adds credits, logged with who did it. Returns the new balance. */
export async function grantCompanyCredits(input: { companyId: string; amount: number; plan: Plan | ""; note: string; by: string }): Promise<number> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { data, error } = await c.rpc("grant_company_credits", {
    p_company: input.companyId,
    p_amount: input.amount,
    p_plan: input.plan,
    p_note: input.note,
    p_by: input.by,
  });
  if (error) throw error;
  return Number(data);
}

export async function declineCompanyAccess(companyId: string): Promise<void> {
  const c = db();
  if (!c) throw new Error("Supabase not configured");
  const { error } = await c.from("companies").update({ access_status: "declined" }).eq("id", companyId).eq("signup_source", "self");
  if (error) throw error;
}

/** The company's admin, to tell them access is on. */
export async function companyContact(companyId: string): Promise<{ name: string; email: string; company: string } | null> {
  const c = db();
  if (!c) return null;
  const [{ data: company }, { data: user }] = await Promise.all([
    c.from("companies").select("name").eq("id", companyId).maybeSingle(),
    c.from("company_users").select("name, email").eq("company_id", companyId).eq("role", "admin").order("created_at", { ascending: true }).limit(1).maybeSingle(),
  ]);
  if (!company || !user) return null;
  return { name: user.name, email: user.email, company: company.name };
}
