/**
 * Checks audio-decide.ts, in Node:
 *
 *   node --experimental-strip-types src/lib/ide/audio-decide.check.ts
 */
import { AudioDecider, CALIBRATE_TICKS, SAME_KIND_GAP_TICKS, VOICE_WINDOW_TICKS } from "./audio-decide.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}

const ROOM = -55;
const VOICE = { db: ROOM + 25, share: 0.8 };
const QUIET = { db: ROOM, share: 0.3 };

/** A decider past calibration in a quiet room. */
function calibrated(): AudioDecider {
  const d = new AudioDecider();
  for (let i = 0; i < CALIBRATE_TICKS + 40; i++) d.tick(QUIET.db, QUIET.share);
  return d;
}

/** Feeds a repeating pattern: `voiceTicks` of voice in every `period` ticks. Returns the flags raised. */
function feed(d: AudioDecider, ticks: number, voiceTicks: number, period: number): string[] {
  const flags: string[] = [];
  for (let i = 0; i < ticks; i++) {
    const s = i % period < voiceTicks ? VOICE : QUIET;
    const { flag } = d.tick(s.db, s.share);
    if (flag) flags.push(flag);
  }
  return flags;
}

check("a quiet room raises nothing", feed(calibrated(), 400, 0, 20).length === 0);

// 15% of the time voices — background, allowed.
let flags = feed(calibrated(), 2 * VOICE_WINDOW_TICKS, 3, 20);
check("background voices for 15% of the time aren't flagged", flags.length === 0, flags.join(","));

// 20% exactly is still within what's allowed.
flags = feed(calibrated(), 2 * VOICE_WINDOW_TICKS, 4, 20);
check("background voices for 20% of the time aren't flagged", flags.length === 0, flags.join(","));

// A few words, once: 3 s of voices, then quiet.
const d = calibrated();
flags = feed(d, 12, 12, 12).concat(feed(d, 200, 0, 20));
check("three seconds of talk, once, isn't flagged", flags.length === 0, flags.join(","));

// Sustained conversation: half the time.
flags = feed(calibrated(), VOICE_WINDOW_TICKS, 10, 20);
check("a conversation (half the time) is flagged", flags[0] === "voices_detected", flags.join(","));

// It doesn't repeat every few seconds while the conversation goes on.
flags = feed(calibrated(), SAME_KIND_GAP_TICKS, 10, 20);
check("a long conversation is flagged once per 30 s, not every few seconds", flags.length === 1, `${flags.length} flags`);

// A steady background (a television) becomes the room's level.
const tv = calibrated();
for (let i = 0; i < 400; i++) tv.tick(ROOM + 20, 0.7);
check("a steady background becomes the room's level", tv.floor > ROOM + 15, `floor ${tv.floor.toFixed(1)}`);
flags = [];
for (let i = 0; i < 400; i++) {
  const { flag } = tv.tick(ROOM + 20, 0.7);
  if (flag) flags.push(flag);
}
check("and isn't flagged once learned", flags.length === 0, flags.join(","));

// Loud non-speech noise is still caught.
const n = calibrated();
flags = [];
for (let i = 0; i < 10; i++) {
  const { flag } = n.tick(ROOM + 30, 0.2);
  if (flag) flags.push(flag);
}
check("a loud noise lasting a couple of seconds is flagged", flags[0] === "loud_noise", flags.join(","));

if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall passed");
