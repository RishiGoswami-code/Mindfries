/**
 * What the evidence report shows beside the agents' observations, worked
 * out from the session's own activity trail (activity_events): how long the
 * work took, the test runs, the commands, the moments that were flagged,
 * and a timeline of what happened when.
 */

export interface TrailEvent {
  id: number;
  type: string;
  payload: Record<string, unknown>;
  /** ISO time it happened. */
  at: string;
}

export type FlagSeverity = "high" | "medium" | "low";

export interface ReportFlag {
  id: number;
  kind: string;
  title: string;
  detail: string;
  severity: FlagSeverity;
  /** Seconds into the session. */
  offset: number;
}

export type TimelineTone = "neutral" | "good" | "bad" | "flag";

export interface TimelineItem {
  id: number;
  offset: number;
  text: string;
  tone: TimelineTone;
  /** Shown on the Summary's short timeline, not only the full one. */
  key: boolean;
}

export interface TestRun {
  id: number;
  offset: number;
  command: string;
  /** Null when the output couldn't be read as a test report. */
  passed: number | null;
  failed: number | null;
  skipped: number;
  failing: string[];
  exitCode: number;
}

export interface Paste {
  id: number;
  offset: number;
  target: string;
  chars: number;
  lines: number;
  /** True when it came from outside the workspace. */
  external: boolean;
}

export interface Turn {
  id: number;
  offset: number;
  role: "candidate" | "other";
  text: string;
}

export interface Trail {
  /** From the start to the last thing done before the interview, in minutes. */
  workMinutes: number;
  /** ISO time of the last recorded event, or null when there are none. */
  lastAt: string | null;
  terminalCommands: number;
  testRuns: TestRun[];
  /** The last run that could be read, or null. */
  lastTests: TestRun | null;
  pastes: Paste[];
  flags: ReportFlag[];
  timeline: TimelineItem[];
  assistant: Turn[];
  interview: Turn[];
  /** Paths the candidate edited, in the order first edited. */
  editedPaths: string[];
  media: { cameraStills: number; screenStills: number; audioClips: number; interviewRecordings: number };
  /** Minutes away from the workspace, summed from tab_hidden. */
  awayMinutes: number;
}

const str = (v: unknown) => (typeof v === "string" ? v : "");
const num = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : 0);
const clip = (text: string, max = 90) => {
  const flat = text.replace(/\s+/g, " ").trim();
  return flat.length > max ? `${flat.slice(0, max - 1).trimEnd()}…` : flat;
};

/** What each flag kind is called, and how closely it deserves looking at. */
export const FLAG_INFO: Record<string, { title: string; severity: FlagSeverity }> = {
  external_paste: { title: "Large paste from outside the workspace", severity: "high" },
  second_window: { title: "Session open in a second window", severity: "high" },
  multiple_faces: { title: "More than one person on camera", severity: "high" },
  phone_visible: { title: "A phone in view of the camera", severity: "medium" },
  voices_detected: { title: "Voices picked up by the microphone", severity: "medium" },
  command_record_altered: { title: "Terminal record tampered with", severity: "high" },
  screen_share_stopped: { title: "Screen share stopped", severity: "medium" },
  extra_display: { title: "More than one display connected", severity: "medium" },
  left_workspace: { title: "Left the workspace", severity: "medium" },
  devtools_attempt: { title: "Tried to open developer tools", severity: "medium" },
  screenshot_attempt: { title: "Tried to take a screenshot", severity: "low" },
  fullscreen_exit: { title: "Left fullscreen", severity: "low" },
  face_missing: { title: "Face not visible on camera", severity: "low" },
  loud_noise: { title: "Loud noise picked up", severity: "low" },
};

function flagDetail(kind: string, detail: Record<string, unknown>, payload: Record<string, unknown>): string {
  if (kind === "external_paste") {
    const lines = num(detail.lines);
    const chars = num(detail.chars);
    return lines ? `${lines} lines (${chars.toLocaleString("en")} characters)` : `${chars.toLocaleString("en")} characters`;
  }
  if (kind === "left_workspace" && num(detail.seconds)) return `Away for ${num(detail.seconds)} seconds`;
  if (kind === "command_record_altered") return clip(str(payload.problem), 120);
  if (kind === "face_missing") return "The camera check is the least reliable signal";
  return "";
}

const SEVERITY_ORDER: Record<FlagSeverity, number> = { high: 0, medium: 1, low: 2 };

/** "05:45" — minutes and seconds into the session; hours when it ran past one. */
export function clock(offsetSeconds: number): string {
  const s = Math.max(0, Math.round(offsetSeconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const mm = String(m).padStart(2, "0");
  const ss = String(s % 60).padStart(2, "0");
  return h ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

/**
 * The Summary's short timeline: the key moments, with a run of the same
 * thing said once with a count, and at most `max` lines.
 */
export function shortTimeline(items: TimelineItem[], max = 9): { items: TimelineItem[]; more: number } {
  const merged: Array<TimelineItem & { times: number }> = [];
  for (const t of items.filter((i) => i.key)) {
    const prev = merged.at(-1);
    if (prev && prev.text === t.text && prev.tone === t.tone) prev.times++;
    else merged.push({ ...t, times: 1 });
  }
  const out = merged.map(({ times, ...t }) => (times > 1 ? { ...t, text: `${t.text} (${times} times)` } : t));
  if (out.length <= max) return { items: out, more: 0 };
  const first = out[0];
  const last = out[out.length - 1];
  const middle = out.slice(1, -1);
  const ranked = [...middle].sort((a, b) => Number(b.tone === "flag") - Number(a.tone === "flag") || a.offset - b.offset).slice(0, max - 2);
  const kept = [first, ...middle.filter((t) => ranked.includes(t)), last];
  return { items: kept, more: out.length - kept.length };
}

export function buildTrail(events: TrailEvent[], startedAt: string): Trail {
  const start = Date.parse(startedAt);
  const sorted = [...events].sort((a, b) => a.at.localeCompare(b.at) || a.id - b.id);
  const offsetOf = (e: TrailEvent) => (Number.isFinite(start) ? Math.max(0, (Date.parse(e.at) - start) / 1000) : 0);

  const testRuns: TestRun[] = [];
  const pastes: Paste[] = [];
  const flags: ReportFlag[] = [];
  const timeline: TimelineItem[] = [];
  const assistant: Turn[] = [];
  const interview: Turn[] = [];
  const edited: string[] = [];
  const opened = new Set<string>();
  const media = { cameraStills: 0, screenStills: 0, audioClips: 0, interviewRecordings: 0 };
  let terminalCommands = 0;
  let awaySeconds = 0;
  let lastWorkOffset = 0;
  let interviewStarted = false;

  const add = (e: TrailEvent, text: string, tone: TimelineTone, key: boolean) => timeline.push({ id: e.id, offset: offsetOf(e), text, tone, key });
  const work = (e: TrailEvent) => {
    if (!interviewStarted) lastWorkOffset = Math.max(lastWorkOffset, offsetOf(e));
  };

  for (const e of sorted) {
    const p = e.payload ?? {};
    const offset = offsetOf(e);
    switch (e.type) {
      case "file_open": {
        const path = str(p.path);
        if (path && !opened.has(path)) {
          opened.add(path);
          add(e, `Opened ${path}`, "neutral", false);
        }
        break;
      }
      case "file_edit": {
        const paths = Array.isArray(p.paths) ? p.paths.filter((x): x is string => typeof x === "string") : [];
        for (const path of paths) if (!edited.includes(path)) edited.push(path);
        if (paths.length) add(e, `Edited ${paths.join(", ")}`, "neutral", false);
        work(e);
        break;
      }
      case "terminal_command":
        terminalCommands++;
        add(e, `Ran ${clip(str(p.command), 60)}${num(p.exitCode) ? ` (exit ${num(p.exitCode)})` : ""}`, num(p.exitCode) ? "bad" : "neutral", false);
        work(e);
        break;
      case "test_run": {
        const parsed = p.parsed !== false;
        const run: TestRun = {
          id: e.id,
          offset,
          command: str(p.command),
          passed: parsed ? num(p.passed) : null,
          failed: parsed ? num(p.failed) : null,
          skipped: num(p.skipped),
          failing: Array.isArray(p.failing) ? p.failing.filter((x): x is string => typeof x === "string").slice(0, 20) : [],
          exitCode: num(p.exitCode),
        };
        testRuns.push(run);
        const total = (run.passed ?? 0) + (run.failed ?? 0);
        const nth = testRuns.length > 1 ? "Re-ran tests" : "Ran tests";
        add(e, parsed ? `${nth} (${run.passed}/${total} passed)` : `${nth} (exit ${run.exitCode})`, parsed ? (run.failed ? "bad" : "good") : run.exitCode ? "bad" : "neutral", true);
        work(e);
        break;
      }
      case "paste": {
        const paste: Paste = { id: e.id, offset, target: str(p.target) || "the workspace", chars: num(p.chars), lines: num(p.lines), external: p.internal === false };
        pastes.push(paste);
        const big = paste.external && paste.chars >= 200;
        add(e, `Pasted ${paste.lines ? `${paste.lines} lines` : `${paste.chars} characters`} into ${paste.target}${paste.external ? "" : " (from inside the workspace)"}`, big ? "flag" : "neutral", big);
        work(e);
        break;
      }
      case "activity_flag":
      case "command_record_altered": {
        const kind = e.type === "command_record_altered" ? "command_record_altered" : str(p.kind);
        const info = FLAG_INFO[kind] ?? { title: kind.replace(/_/g, " "), severity: "medium" as const };
        const detail = p.detail && typeof p.detail === "object" ? (p.detail as Record<string, unknown>) : {};
        flags.push({ id: e.id, kind, title: info.title, detail: flagDetail(kind, detail, p), severity: info.severity, offset });
        if (kind !== "external_paste") add(e, info.title, "flag", info.severity !== "low");
        break;
      }
      case "tab_hidden":
        awaySeconds += num(p.seconds);
        add(e, `Back after ${num(p.seconds)} seconds away`, "neutral", false);
        break;
      case "ai_usage": {
        const role = p.role === "candidate" ? "candidate" : "other";
        assistant.push({ id: e.id, offset, role, text: str(p.text) });
        if (role === "candidate") add(e, `Asked the assistant: ${clip(str(p.text), 60)}`, "neutral", true);
        work(e);
        break;
      }
      case "assistant_copy":
        add(e, `Copied ${num(p.chars)} characters from the assistant`, "neutral", false);
        break;
      case "interview": {
        if (!interviewStarted) {
          interviewStarted = true;
          add(e, "Interview started", "neutral", true);
        }
        interview.push({ id: e.id, offset, role: p.role === "candidate" ? "candidate" : "other", text: str(p.text) });
        break;
      }
      case "auto_submitted":
        add(e, "Time ran out — the server submitted the work", "bad", true);
        break;
      case "camera_snapshot":
        media.cameraStills++;
        break;
      case "screen_snapshot":
        media.screenStills++;
        break;
      case "audio_clip":
        media.audioClips++;
        break;
      case "interview_recording":
        media.interviewRecordings++;
        break;
    }
  }

  if (sorted.length) timeline.unshift({ id: 0, offset: 0, text: "Started session", tone: "neutral", key: true });
  const last = sorted.at(-1);
  if (last) timeline.push({ id: -1, offset: offsetOf(last), text: "Last recorded activity", tone: "neutral", key: true });

  const lastTests = [...testRuns].reverse().find((r) => r.passed !== null) ?? null;

  return {
    workMinutes: Math.max(0, Math.round(lastWorkOffset / 60)),
    lastAt: last?.at ?? null,
    terminalCommands,
    testRuns,
    lastTests,
    pastes,
    flags: flags.sort((a, b) => SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity] || a.offset - b.offset),
    timeline,
    assistant,
    interview,
    editedPaths: edited,
    media,
    awayMinutes: Math.round(awaySeconds / 60),
  };
}
