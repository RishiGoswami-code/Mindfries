import "server-only";

import { currentCandidate } from "@/lib/auth/users";
import { db } from "@/lib/supabase";
import { v2Enabled } from "@/lib/v2/flag";
import { portalReady } from "@/lib/v2/api";
import { roleLabel } from "@/lib/v2/roles";

export type Failure = { ok: false; error: string };

export type Owned = { assessmentId: string; candidateId: string; companyId: string | null; role: string; jd: string; title: string };

/**
 * The assessment, but only if it's this candidate's own V2 assessment: the
 * ownership check (candidate_email must match the signed-in candidate) is
 * what keeps a code, a consent or a sign-in from being made for someone
 * else's assessment.
 */
export async function ownedAssessment(assessmentId: string): Promise<{ ok: true; a: Owned } | Failure> {
  if (!v2Enabled()) return { ok: false, error: "This isn't available yet." };
  const candidate = await currentCandidate();
  if (!candidate) return { ok: false, error: "Please sign in again." };
  if (!portalReady()) return { ok: false, error: "The workspace service isn't available right now. Please try again shortly." };
  const c = db();
  if (!c) return { ok: false, error: "Can't reach your assessment right now." };
  const { data, error } = await c
    .from("assessments")
    .select("role, jd, engine, candidate_email, company_id")
    .eq("id", assessmentId)
    .maybeSingle();
  if (error || !data) return { ok: false, error: "This assessment couldn't be found." };
  if ((data.candidate_email as string)?.trim().toLowerCase() !== candidate.email.trim().toLowerCase()) {
    return { ok: false, error: "This assessment isn't yours." };
  }
  if (data.engine !== "v2") return { ok: false, error: "This assessment doesn't use the local workspace." };
  return {
    ok: true,
    a: {
      assessmentId,
      candidateId: candidate.id,
      companyId: (data.company_id as string | null) ?? null,
      role: data.role as string,
      jd: (data.jd as string) || "",
      title: roleLabel(data.role as string),
    },
  };
}

