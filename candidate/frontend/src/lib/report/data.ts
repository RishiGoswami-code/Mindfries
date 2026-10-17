import "server-only";
import { db } from "@/lib/supabase";
import { getSessionAssessmentOrUndefined } from "@/lib/backend/client";
import { diffFiles, type FileDiff } from "./diff";
import { buildTrail, type Trail, type TrailEvent } from "./trail";

/**
 * The activity behind a report: the session's recorded events, and its
 * files as the task gave them and as the candidate left them.
 *
 * Read only for the signed-in candidate's own session — the session row
 * must name them — so a session id alone never opens someone else's trail.
 * Without a database, or before any event was recorded, the trail is empty
 * and the page says so; nothing is filled in.
 */

export interface ReportActivity {
  trail: Trail;
  diffs: FileDiff[];
  /** The files as the candidate left them, from the last saved copy. Null when none was saved. */
  finalFiles: Record<string, string> | null;
  /** When that copy was taken. */
  finalAt: string | null;
  /** False when the trail couldn't be read at all (no database). */
  available: boolean;
}

/** Events that are bookkeeping, or too big to fetch in bulk — the last snapshot is read on its own. */
const SKIPPED = ["workspace_snapshot", "ai_cost", "variant_assigned", "sandbox_ready"];

export async function loadReportActivity(sessionId: string, candidateId: string, startedAt: string): Promise<ReportActivity> {
  const empty: ReportActivity = { trail: buildTrail([], startedAt), diffs: [], finalFiles: null, finalAt: null, available: false };
  const c = db();
  if (!c || !/^[0-9a-f-]{36}$/i.test(sessionId)) return empty;

  const { data: owned } = await c.from("sessions").select("id").eq("id", sessionId).eq("candidate_id", candidateId).maybeSingle();
  if (!owned) return empty;

  const [events, snapshot, assessment] = await Promise.all([
    c
      .from("activity_events")
      .select("id, event_type, payload, occurred_at")
      .eq("session_id", sessionId)
      .not("event_type", "in", `(${SKIPPED.join(",")})`)
      .order("occurred_at", { ascending: true })
      .limit(5000),
    c
      .from("activity_events")
      .select("payload, occurred_at")
      .eq("session_id", sessionId)
      .eq("event_type", "workspace_snapshot")
      .order("occurred_at", { ascending: false })
      .limit(1)
      .maybeSingle(),
    getSessionAssessmentOrUndefined(sessionId),
  ]);
  if (events.error) {
    console.error("report: couldn't read the activity trail:", events.error.message);
    return empty;
  }

  const trailEvents: TrailEvent[] = (events.data ?? []).map((r) => ({
    id: Number(r.id),
    type: r.event_type as string,
    payload: (r.payload && typeof r.payload === "object" ? r.payload : {}) as Record<string, unknown>,
    at: r.occurred_at as string,
  }));

  const snapFiles = snapshot.data?.payload?.files;
  const finalFiles = snapFiles && typeof snapFiles === "object" ? (snapFiles as Record<string, string>) : null;
  const starter = assessment?.starterFiles ?? null;

  return {
    trail: buildTrail(trailEvents, startedAt),
    // Without both versions there's nothing honest to compare.
    diffs: finalFiles && starter ? diffFiles(starter, finalFiles) : [],
    finalFiles,
    finalAt: (snapshot.data?.occurred_at as string) ?? null,
    available: true,
  };
}
