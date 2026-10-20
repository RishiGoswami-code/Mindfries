"use server";

import { currentCandidate } from "@/lib/auth/users";
import { db } from "@/lib/supabase";
import { mintV2SetupCode, v2EngineUrl } from "@/lib/v2";

/**
 * Called when a candidate clicks "Generate setup code" for a V2 assessment.
 * It mints a one-time code from the engine for THIS candidate's assessment —
 * the ownership check (candidate_email must match the signed-in candidate) is
 * what keeps a code from being minted for someone else's assessment. The code
 * is single-use and short-lived; the candidate then runs `mindfries setup
 * <code>` and `mindfries start`.
 */
export async function startV2Session(
  assessmentId: string,
): Promise<{ ok: true; setupCode: string; engineUrl: string } | { ok: false; error: string }> {
  try {
    const candidate = await currentCandidate();
    if (!candidate) return { ok: false, error: "Please sign in again." };

    const engineUrl = v2EngineUrl();
    if (!engineUrl) return { ok: false, error: "The workspace service isn't available right now. Please try again shortly." };

    const c = db();
    if (!c) return { ok: false, error: "Can't reach your assessment right now." };

    const { data, error } = await c
      .from("assessments")
      .select("role, jd, engine, candidate_email")
      .eq("id", assessmentId)
      .maybeSingle();
    if (error || !data) return { ok: false, error: "This assessment couldn't be found." };
    if ((data.candidate_email as string)?.trim().toLowerCase() !== candidate.email.trim().toLowerCase()) {
      return { ok: false, error: "This assessment isn't yours." };
    }
    if (data.engine !== "v2") return { ok: false, error: "This assessment doesn't use the local workspace." };

    const minted = await mintV2SetupCode({ role: data.role as string, jd: (data.jd as string) || "", assessmentId });
    if (!minted) return { ok: false, error: "Couldn't create a setup code. Please try again." };

    return { ok: true, setupCode: minted.code, engineUrl };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : "Something went wrong. Please try again." };
  }
}
