/**
 * Checks diff.ts and trail.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/report/trail.check.ts
 */
import { diffFiles, diffText } from "./diff.ts";
import { buildTrail, clock, shortTimeline, type TrailEvent } from "./trail.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}

// ── diff ──────────────────────────────────────────────────────────────────
let d = diffText("a.py", "def f():\n    ...\n    return []\n", "def f():\n    import re\n    return []\n");
check("a changed line is one removed and one added", d.added === 1 && d.removed === 1, `+${d.added} -${d.removed}`);
check("unchanged lines keep both numbers", d.lines[0].kind === "same" && d.lines.at(-1)?.kind === "same");

d = diffText("new.py", null, "a\nb\n");
check("a new file is all added, and says it didn't exist", d.added === 2 && d.removed === 0 && !d.beforeExists);

d = diffText("x", "same\n", "same");
check("a trailing newline alone is not a change", d.added === 0 && d.removed === 0);

const files = diffFiles({ "/README.md": "hi", "parser.py": "a\n", "keep.py": "k" }, { "README.md": "hi", "parser.py": "a\nb\nc\n", "keep.py": "k", "new.py": "n" });
check("only changed files, most changed first, leading slash ignored", files.map((f) => f.path).join(",") === "parser.py,new.py", files.map((f) => f.path).join(","));

const big = diffText("big", Array.from({ length: 2100 }, (_, i) => `l${i}`).join("\n"), Array.from({ length: 2100 }, (_, i) => `l${i + 1}`).join("\n"));
check("a file too big to align says so, with counts", big.tooLarge && big.added === 1 && big.removed === 1, `+${big.added} -${big.removed}`);

// ── trail ─────────────────────────────────────────────────────────────────
const T0 = "2026-10-17T17:00:00.000Z";
const at = (s: number) => new Date(Date.parse(T0) + s * 1000).toISOString();
let n = 0;
const ev = (s: number, type: string, payload: Record<string, unknown> = {}): TrailEvent => ({ id: ++n, type, payload, at: at(s) });

const trail = buildTrail(
  [
    ev(30, "file_open", { path: "parser.py" }),
    ev(130, "test_run", { parsed: true, passed: 2, failed: 5, failing: ["test_a"], command: "python -m unittest", exitCode: 1 }),
    ev(131, "terminal_command", { command: "python -m unittest", exitCode: 1 }),
    ev(345, "paste", { chars: 5410, lines: 198, target: "parser.py", internal: false }),
    ev(345, "activity_flag", { kind: "external_paste", detail: { chars: 5410, lines: 198 } }),
    ev(350, "file_edit", { paths: ["parser.py"] }),
    ev(370, "test_run", { parsed: true, passed: 7, failed: 0, command: "python -m unittest", exitCode: 0 }),
    ev(400, "ai_usage", { role: "candidate", text: "can you confirm the results" }),
    ev(401, "ai_usage", { role: "assistant", text: "All 7 passed." }),
    ev(500, "activity_flag", { kind: "face_missing" }),
    ev(600, "tab_hidden", { seconds: 58 }),
    ev(2600, "interview", { role: "interviewer", text: "Why shlex?" }),
    ev(2610, "interview", { role: "candidate", text: "I don't know." }),
    ev(2620, "camera_snapshot", { path: "x.jpg" }),
  ],
  T0
);

check("work time stops when the interview starts", trail.workMinutes === 7, `${trail.workMinutes} min`);
check("the last readable test run is the one shown", trail.lastTests?.passed === 7 && trail.lastTests.failed === 0);
check("two test runs, one command", trail.testRuns.length === 2 && trail.terminalCommands === 1);
check("flags sorted most serious first", trail.flags.map((f) => f.kind).join(",") === "external_paste,face_missing", trail.flags.map((f) => f.kind).join(","));
check("a paste flag says how big it was", trail.flags[0].detail.startsWith("198 lines"), trail.flags[0].detail);
const key = trail.timeline.filter((t) => t.key).map((t) => `${clock(t.offset)} ${t.text}`);
check("the short timeline has the moments that matter", key.includes("05:45 Pasted 198 lines into parser.py") && key.includes("02:10 Ran tests (2/7 passed)") && key.includes("06:10 Re-ran tests (7/7 passed)"), key.join(" | "));
check("the paste is on the timeline once, flagged", trail.timeline.filter((t) => t.text.includes("Pasted")).length === 1 && trail.timeline.find((t) => t.text.includes("Pasted"))?.tone === "flag");
check("a low-severity flag stays off the short timeline", !key.some((k) => k.includes("Face not visible")));
check("the assistant conversation keeps who said what", trail.assistant.length === 2 && trail.assistant[0].role === "candidate");
check("the interview transcript is kept apart", trail.interview.length === 2 && trail.interview[1].text === "I don't know.");
check("time away is summed", trail.awayMinutes === 1);
check("stills are counted, not listed", trail.media.cameraStills === 1);
check("clock reads minutes and seconds", clock(345) === "05:45" && clock(3725) === "1:02:05", `${clock(345)} ${clock(3725)}`);

const away = buildTrail([ev(10, "activity_flag", { kind: "left_workspace" }), ev(20, "activity_flag", { kind: "left_workspace" }), ev(30, "activity_flag", { kind: "left_workspace" })], T0);
const short = shortTimeline(away.timeline);
check("a run of the same moment is said once, with a count", short.items.some((t) => t.text === "Left the workspace (3 times)"), short.items.map((t) => t.text).join(" | "));

const many = buildTrail(Array.from({ length: 20 }, (_, i) => ev(i * 10 + 5, "test_run", { parsed: true, passed: i, failed: 1 })).concat([ev(400, "paste", { chars: 900, lines: 30, target: "a.py", internal: false })]), T0);
const capped = shortTimeline(many.timeline, 6);
check("a long timeline is capped, keeping the start, the end and the flagged moment", capped.items.length === 6 && capped.items[0].text === "Started session" && capped.items.at(-1)?.text === "Last recorded activity" && capped.items.some((t) => t.tone === "flag") && capped.more > 0, `${capped.items.length} shown, ${capped.more} more`);
check("what's kept stays in time order", capped.items.every((t, i, a) => i === 0 || a[i - 1].offset <= t.offset));

const empty = buildTrail([], T0);
check("no events: an empty trail, not a made-up one", empty.timeline.length === 0 && empty.lastTests === null && empty.lastAt === null);

console.log(failures === 0 ? "\nall passed" : `\n${failures} failed`);
process.exit(failures === 0 ? 0 : 1);
