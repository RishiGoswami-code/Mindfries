import "server-only";
/* eslint-disable @typescript-eslint/no-explicit-any -- untyped Supabase rows, as in company/frontend/lib/db.ts */
// The evidence report for one assessment, as the company portal reads it
// (company/frontend/lib/db.ts, getCandidateReport and its helpers), copied
// so the Mindfries admin sees the same report. Keep the two in step.
import { db } from "@/lib/supabase";
import { citedIds, describeEvent, type Moment } from "./moments";
import type { AssessmentReport, CameraRecord, CandidateReport, FlaggedMoment, InterviewExchange, SessionSummary } from "./types";

/**
 * Reads the session and evidence-based report for a candidate's assessment
 * — direct Supabase reads of the shared `sessions` / `assessment_reports` /
 * `evidence_items` tables, not a call to candidate/backend's Go API (see
 * lib/types.ts's CandidateReport for why: that API authenticates by
 * forwarding the *candidate's own* session cookie, which this app never
 * has). Returns nulls rather than throwing when nothing exists yet — a
 * candidate who hasn't started, or whose report hasn't been generated, is
 * an honest empty state, not an error.
 */
export async function getCandidateReport(assessmentId: string | null): Promise<CandidateReport> {
  const none = { total: 0, expired: 0, stills: [] };
  const empty: CandidateReport = { session: null, report: null, interview: [], camera: none, screen: none, flags: [] };
  if (!assessmentId) return empty;
  const c = db();
  if (!c) return empty;

  const { data: sessionRow } = await c
    .from("sessions")
    .select("*")
    .eq("assessment_id", assessmentId)
    .order("started_at", { ascending: false })
    .limit(1)
    .maybeSingle();
  if (!sessionRow) return empty;

  // A sandbox session is marked by its sandbox_ready event (candidate/backend
  // writes it when the machine holds the task); the sandbox id itself is
  // cleared once the session is submitted.
  const { count: sandboxReady } = await c
    .from("activity_events")
    .select("id", { count: "exact", head: true })
    .eq("session_id", sessionRow.id)
    .eq("event_type", "sandbox_ready");

  const session: SessionSummary = {
    id: sessionRow.id,
    status: sessionRow.status,
    sandboxHealth: sessionRow.sandbox_health,
    workspace: (sandboxReady ?? 0) > 0 ? "sandbox" : "browser",
    progressPct: sessionRow.progress_pct ?? 0,
    durationMin: sessionRow.duration_min ?? 60,
    elapsedMin: sessionRow.elapsed_min ?? 0,
    startedAt: sessionRow.started_at,
  };

  const { data: reportRow } = await c.from("assessment_reports").select("*").eq("session_id", session.id).maybeSingle();
  const [interview, camera, screen, flags] = await Promise.all([
    getInterviewExchanges(session.id),
    getCameraRecord(session.id, session.startedAt),
    getCameraRecord(session.id, session.startedAt, "screen_snapshot"),
    getFlaggedMoments(session.id, session.startedAt),
  ]);
  if (!reportRow) return { session, report: null, interview, camera, screen, flags };

  const { data: evidenceRows } = await c
    .from("evidence_items")
    .select("*")
    .eq("report_id", reportRow.id)
    .order("created_at", { ascending: true });

  const report: AssessmentReport = {
    id: reportRow.id,
    status: reportRow.status,
    recommendation: reportRow.recommendation ?? null,
    summary: reportRow.summary ?? null,
    error: reportRow.error ?? null,
    evidence: (evidenceRows ?? []).map((e: any) => ({ id: e.id, category: e.category, observation: e.observation })),
    moments: [],
    annotations: [],
    review:
      reportRow.reviewer_recommendation || reportRow.reviewer_note
        ? {
            recommendation: reportRow.reviewer_recommendation ?? null,
            note: reportRow.reviewer_note ?? null,
            by: reportRow.reviewed_by ?? null,
            at: reportRow.reviewed_at ?? null,
          }
        : null,
    sharedWithCandidate: !!reportRow.shared_with_candidate,
    sharedAt: reportRow.shared_at ?? null,
    sharedBy: reportRow.shared_by ?? null,
  };
  report.moments = await getMoments(session.id, session.startedAt, citedIds(report.evidence.map((e) => e.observation)));

  // Absent before migration 0016 has run — then there are simply no notes yet.
  const { data: noteRows } = await c
    .from("report_annotations")
    .select("id, category, kind, body, author_name, created_at")
    .eq("report_id", reportRow.id)
    .order("created_at", { ascending: true });
  report.annotations = (noteRows ?? []).map((n: any) => ({
    id: n.id,
    category: n.category ?? null,
    kind: n.kind === "correction" ? "correction" : "note",
    body: n.body,
    authorName: n.author_name,
    createdAt: n.created_at,
  }));

  return { session, report, interview, camera, screen, flags };
}

const SNAPSHOTS_BUCKET = "proctor-snapshots";
/** How many stills the report page shows. A session has one every half minute; a reviewer wants a sense of it, not all 120. */
const CAMERA_SAMPLE = 24;

/**
 * The proctoring camera's stills for a session (candidate/frontend's
 * lib/ide/camera-snapshots.ts): an even sample of them, each with a signed
 * link valid for an hour. Only paths inside the session's own folder are
 * used — the events come from the candidate's browser.
 */
async function getCameraRecord(sessionId: string, startedAt: string, event: "camera_snapshot" | "screen_snapshot" = "camera_snapshot"): Promise<CameraRecord> {
  const c = db();
  const none: CameraRecord = { total: 0, expired: 0, stills: [] };
  if (!c) return none;
  const { data } = await c
    .from("activity_events")
    .select("event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("event_type", [event, `${event}_expired`])
    .order("occurred_at", { ascending: true });
  if (!data || data.length === 0) return none;

  const expired = data.filter((r: any) => r.event_type === `${event}_expired`).length;
  const live = data
    .filter((r: any) => r.event_type === event)
    .map((r: any) => ({ path: (r.payload ?? {}).path as unknown, at: r.occurred_at as string }))
    .filter((s): s is { path: string; at: string } => typeof s.path === "string" && s.path.startsWith(`${sessionId}/`) && !s.path.includes(".."));

  const step = Math.max(1, Math.ceil(live.length / CAMERA_SAMPLE));
  const sample = live.filter((_, i) => i % step === 0);
  const start = new Date(startedAt).getTime();
  const stills = (
    await Promise.all(
      sample.map(async (s) => {
        const { data: signed } = await c.storage.from(SNAPSHOTS_BUCKET).createSignedUrl(s.path, RECORDING_LINK_SECONDS);
        return signed?.signedUrl ? { url: signed.signedUrl, offsetSeconds: Math.max(0, (new Date(s.at).getTime() - start) / 1000) } : null;
      }),
    )
  ).filter((s): s is { url: string; offsetSeconds: number } => s !== null);
  return { total: live.length + expired, expired, stills };
}

const FLAG_LABEL: Record<string, string> = {
  left_workspace: "Left the workspace",
  fullscreen_exit: "Left fullscreen",
  screen_share_stopped: "Stopped sharing the screen",
  extra_display: "More than one display connected",
  external_paste: "Large paste from outside the workspace",
  face_missing: "Nobody in front of the camera",
  multiple_faces: "More than one face on camera",
  voices_detected: "Voices on the microphone",
  loud_noise: "Loud noise on the microphone",
  devtools_attempt: "Tried to open developer tools",
  screenshot_attempt: "Pressed PrintScreen",
  second_window: "Assessment open in a second window",
};

/**
 * The moments flagged live during a session (issue #120), each with signed
 * links to its camera still, screen still and audio clip. The paths come
 * from the candidate's browser, so only ones inside the session's own
 * folder are used. A file deleted at the end of its 90 days simply has no
 * link.
 */
async function getFlaggedMoments(sessionId: string, startedAt: string): Promise<FlaggedMoment[]> {
  const c = db();
  if (!c) return [];
  const { data } = await c
    .from("activity_events")
    .select("payload, occurred_at")
    .eq("session_id", sessionId)
    .eq("event_type", "activity_flag")
    .order("occurred_at", { ascending: true })
    .limit(200);
  if (!data || data.length === 0) return [];
  const start = new Date(startedAt).getTime();
  const ownPath = (p: unknown): p is string => typeof p === "string" && p.startsWith(`${sessionId}/`) && !p.includes("..");
  const link = async (p: unknown) => {
    if (!ownPath(p)) return null;
    const { data: signed } = await c.storage.from(SNAPSHOTS_BUCKET).createSignedUrl(p, RECORDING_LINK_SECONDS);
    return signed?.signedUrl ?? null;
  };
  return Promise.all(
    data
      .filter((r: any) => FLAG_LABEL[(r.payload ?? {}).kind])
      .map(async (r: any) => {
        const p = r.payload ?? {};
        const at = typeof p.at === "string" && !Number.isNaN(Date.parse(p.at)) ? Date.parse(p.at) : new Date(r.occurred_at).getTime();
        const detail =
          p.kind === "external_paste" && typeof p.detail?.chars === "number"
            ? `${p.detail.chars} characters`
            : p.kind === "multiple_faces" && typeof p.detail?.faces === "number"
              ? `${p.detail.faces} faces`
              : p.kind === "devtools_attempt" && typeof p.detail?.keys === "string"
                ? p.detail.keys
                : null;
        const [cameraUrl, screenUrl, audioUrl] = await Promise.all([link(p.cameraPath), link(p.screenPath), link(p.audioPath)]);
        return {
          kind: p.kind,
          label: FLAG_LABEL[p.kind],
          offsetSeconds: Math.max(0, (at - start) / 1000),
          detail,
          cameraUrl,
          screenUrl,
          audioUrl,
          lowConfidence: p.kind === "face_missing" || p.kind === "multiple_faces",
        };
      }),
  );
}

/**
 * The session events a report's evidence cites, as moments: how far into
 * the session, and what happened. Restricted to the session's own events, so
 * a citation can only ever resolve to something that session recorded.
 */
async function getMoments(sessionId: string, startedAt: string, ids: number[]): Promise<Moment[]> {
  const c = db();
  if (!c || ids.length === 0) return [];
  const { data } = await c
    .from("activity_events")
    .select("id, event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("id", ids.slice(0, 200));
  const start = new Date(startedAt).getTime();
  return (data ?? []).map((row: any) => ({
    id: Number(row.id),
    offsetSeconds: Math.max(0, (new Date(row.occurred_at).getTime() - start) / 1000),
    what: describeEvent(row.event_type, (row.payload ?? {}) as Record<string, unknown>),
  }));
}

const RECORDINGS_BUCKET = "interview-recordings";
const RECORDING_LINK_SECONDS = 3600;
const NO_ANSWER_TEXT = "(no answer given in the time allowed)";

/**
 * Rebuilds the interview from the session's evidence trail: "interview"
 * events are the questions and answers, in order (written by
 * candidate/backend's orchestrator), and "interview_recording" events point
 * at the clip uploaded for each answer (written by candidate/frontend once
 * an upload succeeds). Recordings sit in a private bucket; each gets a
 * signed link valid for an hour.
 *
 * Returns [] on any failure — a missing transcript must not take the whole
 * report page down with it.
 */
async function getInterviewExchanges(sessionId: string): Promise<InterviewExchange[]> {
  const c = db();
  if (!c) return [];
  const { data, error } = await c
    .from("activity_events")
    .select("event_type, payload, occurred_at")
    .eq("session_id", sessionId)
    .in("event_type", ["interview", "interview_recording", "interview_recording_expired"])
    .order("occurred_at", { ascending: true });
  if (error || !data) return [];

  const exchanges: InterviewExchange[] = [];
  const clips = new Map<number, { path: string; kind: "audio" | "video"; seconds: number }>();
  // Recordings are deleted after 90 days (internal-admin's lib/retention.ts),
  // which leaves this event behind in place of the one pointing at the file.
  const expired = new Set<number>();
  for (const row of data) {
    const p = (row.payload ?? {}) as Record<string, unknown>;
    if (row.event_type === "interview_recording_expired") {
      if (typeof p.question === "number") expired.add(p.question);
      continue;
    }
    if (row.event_type === "interview_recording") {
      if (typeof p.path === "string" && typeof p.question === "number" && p.path.startsWith(`${sessionId}/`)) {
        clips.set(p.question, { path: p.path, kind: p.kind === "video" ? "video" : "audio", seconds: Number(p.seconds) || 0 });
      }
      continue;
    }
    if (typeof p.text !== "string") continue;
    if (p.role === "interviewer") {
      exchanges.push({ number: exchanges.length + 1, question: p.text, answer: null, seconds: null, timedOut: false, unanswered: false, recording: null, recordingExpired: false });
    } else if (p.role === "candidate" && exchanges.length > 0) {
      const last = exchanges[exchanges.length - 1];
      last.answer = p.text;
      last.seconds = typeof p.seconds === "number" ? p.seconds : null;
      last.timedOut = p.timedOut === true;
      last.unanswered = p.text === NO_ANSWER_TEXT;
    }
  }

  await Promise.all(
    exchanges.map(async (x) => {
      const clip = clips.get(x.number);
      if (!clip) {
        x.recordingExpired = expired.has(x.number);
        return;
      }
      const { data: signed } = await c.storage.from(RECORDINGS_BUCKET).createSignedUrl(clip.path, RECORDING_LINK_SECONDS);
      if (signed?.signedUrl) x.recording = { url: signed.signedUrl, kind: clip.kind, seconds: clip.seconds };
    }),
  );
  return exchanges;
}
