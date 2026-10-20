import "server-only";
// Copied from company/frontend/lib/report/data.ts so the Mindfries admin sees the same
// evidence report a company does. Keep the two in step.
import { db } from "@/lib/supabase";
import { diffFiles, type FileDiff } from "./diff";
import { buildTrail, type Trail, type TrailEvent } from "./trail";

export interface ReportActivity {
  trail: Trail;
  diffs: FileDiff[];
  finalFiles: Record<string, string> | null;
  finalAt: string | null;
  starterFiles: Record<string, string> | null;
  available: boolean;
}

const SKIPPED = ["workspace_snapshot", "ai_cost", "variant_assigned", "sandbox_ready"];

export async function loadReportActivity(
  sessionId: string,
  startedAt: string,
  templateId?: string | null,
  assessmentId?: string | null,
): Promise<ReportActivity> {
  const empty: ReportActivity = {
    trail: buildTrail([], startedAt),
    diffs: [],
    finalFiles: null,
    finalAt: null,
    starterFiles: null,
    available: false,
  };

  const c = db();
  if (!c || !sessionId || !/^[0-9a-f-]{36}$/i.test(sessionId)) return empty;

  try {
    // 1. Fetch non-skipped activity events
    const eventsPromise = c
      .from("activity_events")
      .select("id, event_type, payload, occurred_at")
      .eq("session_id", sessionId)
      .not("event_type", "in", `(${SKIPPED.join(",")})`)
      .order("occurred_at", { ascending: true })
      .limit(5000);

    // 2. Fetch latest workspace snapshot
    const snapshotPromise = c
      .from("activity_events")
      .select("payload, occurred_at")
      .eq("session_id", sessionId)
      .eq("event_type", "workspace_snapshot")
      .order("occurred_at", { ascending: false })
      .limit(1)
      .maybeSingle();

    // 3. Resolve templateId if not passed directly
    let tid = templateId;
    if (!tid && assessmentId) {
      const { data: assess } = await c.from("assessments").select("template_id").eq("id", assessmentId).maybeSingle();
      tid = assess?.template_id;
    }
    if (!tid) {
      const { data: sess } = await c.from("sessions").select("template_id").eq("id", sessionId).maybeSingle();
      tid = sess?.template_id;
    }

    // 4. Fetch starter files from game_templates
    type StarterResult = { data: { starter_files: unknown } | null; error: { message: string } | null };
    let starterPromise: PromiseLike<StarterResult> = Promise.resolve({ data: null, error: null });
    if (tid) {
      starterPromise = c.from("game_templates").select("starter_files").eq("id", tid).maybeSingle();
    }

    const [eventsRes, snapshotRes, starterRes] = await Promise.all([eventsPromise, snapshotPromise, starterPromise]);

    if (eventsRes.error) {
      console.error("company report: failed to read activity events:", eventsRes.error.message);
      return empty;
    }

    const trailEvents: TrailEvent[] = (eventsRes.data ?? []).map((r) => ({
      id: Number(r.id),
      type: r.event_type as string,
      payload: (r.payload && typeof r.payload === "object" ? r.payload : {}) as Record<string, unknown>,
      at: r.occurred_at as string,
    }));

    const snapFiles = snapshotRes.data?.payload?.files;
    const finalFiles = snapFiles && typeof snapFiles === "object" ? (snapFiles as Record<string, string>) : null;

    const starterRaw = starterRes.data?.starter_files;
    const starterFiles = starterRaw && typeof starterRaw === "object" ? (starterRaw as Record<string, string>) : null;

    const trail = buildTrail(trailEvents, startedAt);
    const diffs = finalFiles && starterFiles ? diffFiles(starterFiles, finalFiles) : [];

    return {
      trail,
      diffs,
      finalFiles,
      finalAt: (snapshotRes.data?.occurred_at as string) ?? null,
      starterFiles,
      available: true,
    };
  } catch (err) {
    console.error("company report: unexpected error loading activity:", err);
    return empty;
  }
}
