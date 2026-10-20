// Copied from company/frontend/lib/types.ts so the Mindfries admin sees the same
// evidence report a company does. Keep the two in step.
import type { Moment } from "./moments";

export interface SessionSummary {
  id: string;
  status: string;
  sandboxHealth: string;
  /**
   * Where the candidate worked: on a sandbox machine, or in their browser
   * (no sandbox could be had — the pool was full, or it didn't come up).
   * The two are different environments, which a hiring team comparing
   * candidates should know.
   */
  workspace: "sandbox" | "browser";
  progressPct: number;
  durationMin: number;
  elapsedMin: number;
  startedAt: string;
}

export interface EvidenceItem {
  id: string;
  category: string;
  observation: string;
}

export type ReportStatus = "pending" | "generating" | "ready" | "failed";

export interface AssessmentReport {
  id: string;
  status: ReportStatus;
  recommendation: string | null;
  summary: string | null;
  error: string | null;
  evidence: EvidenceItem[];
  /** The session moments the evidence cites ("[E<id>]" in an observation). */
  moments: Moment[];
  /** Notes and corrections a reviewer has added, oldest first. */
  annotations: ReportAnnotation[];
  /** The reviewer's own decision, when one has been recorded. */
  review: ReportReview | null;
  /** Whether the report has been shared with the candidate. */
  sharedWithCandidate?: boolean;
  sharedAt?: string | null;
  sharedBy?: string | null;
}

/** A reviewer's note against the report, or one section of it. Never shown to the candidate. */
export interface ReportAnnotation {
  id: string;
  /** The evidence category it is about; null for the report as a whole. */
  category: string | null;
  kind: "note" | "correction";
  body: string;
  authorName: string;
  createdAt: string;
}

/** A person's decision, recorded beside the AI's recommendation rather than over it. */
export interface ReportReview {
  recommendation: string | null;
  note: string | null;
  by: string | null;
  at: string | null;
}

/** One question of the AI follow-up interview, with what came back. */
export interface InterviewExchange {
  /** 1-based position in the interview. */
  number: number;
  question: string;
  /** null when the interview ended with this question still waiting. */
  answer: string | null;
  /** Seconds the candidate took, measured by the backend. null when unknown or unanswered. */
  seconds: number | null;
  /** The answer ran past the role's time limit. */
  timedOut: boolean;
  /** The time ran out with nothing said at all. */
  unanswered: boolean;
  /** The candidate's recorded answer, when one was uploaded. `url` is signed and short-lived. */
  recording: { url: string; kind: "audio" | "video"; seconds: number } | null;
  /** There was a recording, and it has been deleted at the end of its 90-day retention. */
  recordingExpired: boolean;
}

export interface CandidateReport {
  session: SessionSummary | null;
  report: AssessmentReport | null;
  /** The interview as it happened. Empty when none took place. */
  interview: InterviewExchange[];
  /** Stills of the proctoring camera through the session. */
  camera: CameraRecord;
  /** Stills of the candidate's screen: kept when it changed, and every few minutes. */
  screen: CameraRecord;
  /** Moments flagged live during the session, each with its evidence (issue #120). */
  flags: FlaggedMoment[];
}

/**
 * A moment the candidate's browser flagged during the session — leaving the
 * workspace, voices on the microphone, a second face — with the camera, the
 * screen and (for sounds) the audio from that moment. A prompt for a person
 * to look, not a finding.
 */
export interface FlaggedMoment {
  kind: string;
  label: string;
  offsetSeconds: number;
  /** Short facts, e.g. "812 characters". */
  detail: string | null;
  /** Signed, short-lived links; null when there was none or it has been deleted. */
  cameraUrl: string | null;
  screenUrl: string | null;
  audioUrl: string | null;
  /** Lower-confidence signals (face detection) are marked so. */
  lowConfidence: boolean;
}

/** The proctoring camera's record of a session: a sample of its stills. */
export interface CameraRecord {
  /** How many stills were taken in all. */
  total: number;
  /** How many have been deleted at the end of their 90-day retention. */
  expired: number;
  /** An even sample across the session, oldest first. `url` is signed and short-lived. */
  stills: { url: string; offsetSeconds: number }[];
}
