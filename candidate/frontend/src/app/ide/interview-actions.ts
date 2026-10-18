"use server";

import {
  backendReady,
  BackendAuthError,
  getSession,
  interviewStep as interviewStepOnBackend,
  liveInterviewUrl,
  startLiveInterview,
  postSessionEvents,
  type InterviewInput,
  type InterviewState,
} from "@/lib/backend/client";
import { db } from "@/lib/supabase";

const AI_OFFLINE = "The assessment service isn't connected, so the AI can't be reached right now.";

function failure(err: unknown, fallback: string): { error: string } {
  if (err instanceof BackendAuthError) return { error: "Your sign-in has expired — sign in again to continue." };
  return { error: err instanceof Error ? err.message : fallback };
}

/** One step of the follow-up interview — see InterviewDialog. */
export async function interviewStep(sessionId: string, input: InterviewInput = {}): Promise<InterviewState | { error: string }> {
  if (!backendReady()) return { error: AI_OFFLINE };
  try {
    return await interviewStepOnBackend(sessionId, input);
  } catch (err) {
    return failure(err, "The AI service didn't answer — try again.");
  }
}

export interface LiveInterviewCall {
  /** The WebSocket the page opens — straight to the candidate backend. */
  url: string;
  ticket: string;
  language: string;
  inputRate: number;
  outputRate: number;
}

/**
 * Sets up the interview as a live voice call. `unavailable` covers every
 * reason there isn't one to be had — no voice line configured, the backend
 * out of reach — and means the interview is held turn by turn instead, which
 * needs nothing this does.
 *
 * The workspace files go with it for the same reason they go with the first
 * turn-based question: the interviewer asks about what was actually written.
 */
export async function openLiveInterview(
  sessionId: string,
  files: Record<string, string>,
): Promise<LiveInterviewCall | { unavailable: true }> {
  if (!backendReady()) return { unavailable: true };
  try {
    const start = await startLiveInterview(sessionId, files);
    return { url: liveInterviewUrl(), ticket: start.ticket, language: start.language, inputRate: start.inputRate, outputRate: start.outputRate };
  } catch {
    return { unavailable: true };
  }
}

/**
 * Where interview recordings live in Supabase Storage. Private: nothing in
 * it is reachable without a signed URL minted server-side, the same way the
 * `resumes` bucket works.
 */
const RECORDINGS_BUCKET = "interview-recordings";

// A two-minute answer at the recorder's bitrates is 2–5 MB; this leaves room
// for the longest answer limit a company can set and nothing far beyond it.
const MAX_CLIP_BYTES = 40 * 1024 * 1024;

const EXTENSIONS: Record<string, string> = {
  "video/webm": "webm",
  "audio/webm": "webm",
  "video/mp4": "mp4",
  "audio/mp4": "m4a",
};

export interface RecordingUpload {
  /** Where the browser PUTs the clip — straight to storage, not through this app. */
  uploadUrl: string;
  path: string;
}

// A bucket is made on first use and exists forever after. Without this guard,
// createBucket fired on every single upload — every snapshot, every ~15s —
// and each one came back "already exists" (a 400, unique-constraint on
// buckets_pkey), spamming the storage logs with a failing call per still.
// Attempt it at most once per bucket per server process instead; the uploads
// themselves were never affected either way.
const ensuredBuckets = new Set<string>();
async function ensureBucketOnce(
  storage: NonNullable<ReturnType<typeof db>>["storage"],
  bucket: string,
  fileSizeLimit: number,
): Promise<void> {
  if (ensuredBuckets.has(bucket)) return;
  ensuredBuckets.add(bucket);
  // Result ignored on purpose: "already exists" is the normal answer, and the
  // signed-URL request that follows is what actually decides success.
  await storage.createBucket(bucket, { public: false, fileSizeLimit });
}

/**
 * Hands the browser a one-off signed URL to upload one answer's recording
 * to. The clip goes directly from the browser to storage: routing video
 * through a server action would hit its request-size limit on the first
 * long answer.
 *
 * Ownership is checked the same way as everything else about a session —
 * by asking the backend for it with the candidate's own cookie, which only
 * succeeds for the candidate it belongs to.
 */
export async function startRecordingUpload(
  sessionId: string,
  question: number,
  contentType: string,
): Promise<RecordingUpload | { error: string }> {
  const extension = EXTENSIONS[contentType];
  if (!extension) return { error: "Unsupported recording format." };
  if (!Number.isInteger(question) || question < 1 || question > 20) return { error: "Invalid question number." };
  if (!backendReady()) return { error: AI_OFFLINE };

  try {
    const session = await getSession(sessionId);
    if (session.status !== "live") return { error: "This session is no longer active." };
  } catch (err) {
    return failure(err, "Could not verify the session.");
  }

  const storage = db()?.storage;
  if (!storage) return { error: "Recording storage isn't configured." };

  await ensureBucketOnce(storage, RECORDINGS_BUCKET, MAX_CLIP_BYTES);

  const path = `${safeSegment(sessionId)}/q${question}-${Date.now()}.${extension}`;
  const { data, error } = await storage.from(RECORDINGS_BUCKET).createSignedUploadUrl(path);
  if (error || !data) return { error: error?.message ?? "Could not prepare the upload." };
  return { uploadUrl: data.signedUrl, path };
}

/** Session ids are UUIDs; this keeps anything else from becoming a path segment. */
function safeSegment(sessionId: string): string {
  return sessionId.replace(/[^a-zA-Z0-9-]/g, "");
}

/**
 * Records that an answer's clip was uploaded, as an event in the session's
 * evidence trail — which is how the hiring team's view finds it. Called
 * only after the upload itself succeeded, so the trail never points at a
 * recording that isn't there.
 */
export async function confirmRecording(
  sessionId: string,
  clip: { path: string; question: number; seconds: number; bytes: number; kind: "audio" | "video" },
): Promise<{ ok: true } | { error: string }> {
  if (!backendReady()) return { error: AI_OFFLINE };
  if (!clip.path.startsWith(`${safeSegment(sessionId)}/`)) return { error: "That recording doesn't belong to this session." };
  try {
    await postSessionEvents(sessionId, [{ type: "interview_recording", payload: clip }]);
    return { ok: true };
  } catch (err) {
    return failure(err, "Could not record the upload.");
  }
}

/** Where the proctoring camera's stills live. Private, like the recordings. */
const SNAPSHOTS_BUCKET = "proctor-snapshots";
const MAX_SNAPSHOT_BYTES = 200 * 1024;

/**
 * Hands the browser a one-off signed URL to upload one still of the
 * proctoring camera to (lib/ide/camera-snapshots.ts). Ownership is checked
 * the same way as for an interview recording: by asking the backend for the
 * session with the candidate's own cookie.
 */
export async function startSnapshotUpload(sessionId: string, kind: "camera" | "screen" | "audio" = "camera"): Promise<RecordingUpload | { error: string }> {
  if (!backendReady()) return { error: AI_OFFLINE };
  try {
    const session = await getSession(sessionId);
    if (session.status !== "live") return { error: "This session is no longer active." };
  } catch (err) {
    return failure(err, "Could not verify the session.");
  }
  const storage = db()?.storage;
  if (!storage) return { error: "Snapshot storage isn't configured." };

  await ensureBucketOnce(storage, SNAPSHOTS_BUCKET, MAX_SNAPSHOT_BYTES);
  // Screen stills are kept beside the camera's, named so either can be told
  // apart at a glance (issue #120).
  const path = `${safeSegment(sessionId)}/${kind === "screen" ? "screen-" : kind === "audio" ? "audio-" : ""}${Date.now()}.${kind === "audio" ? "webm" : "jpg"}`;
  const { data, error } = await storage.from(SNAPSHOTS_BUCKET).createSignedUploadUrl(path);
  if (error || !data) return { error: error?.message ?? "Could not prepare the upload." };
  return { uploadUrl: data.signedUrl, path };
}
