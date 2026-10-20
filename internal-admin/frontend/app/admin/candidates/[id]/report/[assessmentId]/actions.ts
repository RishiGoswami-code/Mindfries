"use server";

import { revalidatePath } from "next/cache";
import { requireAdminRole } from "@/lib/auth/admins";
import { db } from "@/lib/supabase";

export type DecisionState = { error: string | null; success: boolean };

const DECISIONS = ["strong_hire", "hire", "lean_no", "no_hire"];

/**
 * Records a Mindfries admin's hiring decision on an assessment's report — the
 * same reviewer_recommendation a company's reviewer sets (migration 0016), so
 * there is one decision whoever made it, and the AI's recommendation is never
 * it. Saving "Not decided" with no note clears it.
 */
export async function saveAdminDecision(candidateId: string, assessmentId: string, _prev: DecisionState, form: FormData): Promise<DecisionState> {
  try {
    const admin = await requireAdminRole();
    const raw = String(form.get("recommendation") ?? "");
    const recommendation = DECISIONS.includes(raw) ? raw : null;
    const note = String(form.get("note") ?? "").slice(0, 4000).trim() || null;
    const c = db();
    if (!c) return { error: "Supabase isn't configured here.", success: false };
    const { data: session } = await c
      .from("sessions")
      .select("id")
      .eq("assessment_id", assessmentId)
      .order("started_at", { ascending: false })
      .limit(1)
      .maybeSingle();
    if (!session) return { error: "This assessment hasn't been started, so there's no report to decide on.", success: false };
    const cleared = !recommendation && !note;
    const { data, error } = await c
      .from("assessment_reports")
      .update({
        reviewer_recommendation: recommendation,
        reviewer_note: note,
        reviewed_by: cleared ? null : `Mindfries (${admin.email})`,
        reviewed_at: cleared ? null : new Date().toISOString(),
      })
      .eq("session_id", session.id)
      .select("id");
    if (error) throw error;
    if (!data || data.length === 0) return { error: "There's no report for this session yet.", success: false };
    revalidatePath(`/admin/candidates/${candidateId}`);
    revalidatePath(`/admin/candidates/${candidateId}/report/${assessmentId}`);
    return { error: null, success: true };
  } catch (e) {
    return { error: e instanceof Error ? e.message : String(e), success: false };
  }
}
