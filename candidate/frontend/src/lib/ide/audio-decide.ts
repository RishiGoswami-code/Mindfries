/**
 * The decision behind audio-watch.ts, with no browser in it, so it can be
 * checked in Node (audio-decide.check.ts). It is fed one measurement per tick
 * — the level in dB and the share of energy in the speech band — and says
 * when a moment is worth flagging.
 *
 * Some background sound is allowed. A room is rarely silent: a voice from
 * the next room, a television, someone passing by. Candidates were flagged
 * "high" for a few words of that, so:
 *   - the room's normal level keeps being learned (the quieter end of the
 *     last minute), not fixed from the first six seconds — a steady
 *     background becomes the room's level, not an event;
 *   - a sound has to be clearly above it (VOICE_ABOVE_DB) to count;
 *   - voices are flagged only when they fill more than VOICE_SHARE_ALLOWED of
 *     the last VOICE_WINDOW_TICKS — up to a fifth of the time is background;
 *   - after a flag, the same kind isn't raised again for SAME_KIND_GAP_TICKS.
 */

/** How often the level is measured, in ms. */
export const TICK_MS = 250;
/** Ticks before anything is judged, while the room's level is first learned. */
export const CALIBRATE_TICKS = 24; // 6 s
/** How much history the room's level is learned from, and which part of it. */
export const FLOOR_HISTORY_TICKS = 240; // 60 s
export const FLOOR_PERCENTILE = 0.2;
/** How far above the room's level counts as a sound worth noticing, in dB. */
export const VOICE_ABOVE_DB = 18;
export const NOISE_ABOVE_DB = 22;
/** Share of energy in the speech band for a sound to count as voices. */
export const SPEECH_SHARE = 0.55;
/** Voices: the window, and how much of it background voices may fill. */
export const VOICE_WINDOW_TICKS = 80; // 20 s
export const VOICE_SHARE_ALLOWED = 0.2;
/** Noise: this many loud ticks within the noise window. */
export const NOISE_TICKS = 8; // 2 s
export const NOISE_WINDOW_TICKS = 20; // 5 s
/** After a flag, how long before the same kind is raised again. */
export const SAME_KIND_GAP_TICKS = 120; // 30 s

export type AudioKind = "voices_detected" | "loud_noise";
export type TickKind = "voice" | "noise" | "quiet";

export class AudioDecider {
  private levels: number[] = [];
  private floorDb = -60;
  private ticks = 0;
  private recent: TickKind[] = [];
  private lastFlag: Record<AudioKind, number> = { voices_detected: -Infinity, loud_noise: -Infinity };

  /** The room's level as currently learned, in dB. */
  get floor(): number {
    return this.floorDb;
  }

  /**
   * Takes one measurement. Returns what this tick was (null while the room's
   * level is first learned) and the flag to raise now, if any.
   */
  tick(db: number, speechShare: number): { kind: TickKind | null; flag: AudioKind | null } {
    this.ticks++;
    this.levels.push(db);
    if (this.levels.length > FLOOR_HISTORY_TICKS) this.levels.shift();
    if (this.ticks < CALIBRATE_TICKS) return { kind: null, flag: null };
    if (this.ticks === CALIBRATE_TICKS || this.ticks % 4 === 0) this.floorDb = percentile(this.levels, FLOOR_PERCENTILE);
    if (this.ticks === CALIBRATE_TICKS) return { kind: null, flag: null };

    const kind: TickKind =
      db > this.floorDb + NOISE_ABOVE_DB && speechShare < SPEECH_SHARE
        ? "noise"
        : db > this.floorDb + VOICE_ABOVE_DB && speechShare >= SPEECH_SHARE
          ? "voice"
          : "quiet";
    this.recent.push(kind);
    if (this.recent.length > VOICE_WINDOW_TICKS) this.recent.shift();

    const voices = this.recent.filter((k) => k === "voice").length;
    const noise = this.recent.slice(-NOISE_WINDOW_TICKS).filter((k) => k === "noise").length;
    let flag: AudioKind | null = null;
    if (voices > VOICE_SHARE_ALLOWED * VOICE_WINDOW_TICKS) flag = "voices_detected";
    else if (noise >= NOISE_TICKS) flag = "loud_noise";
    if (flag) {
      this.recent.length = 0; // one flag per stretch of sound
      if (this.ticks - this.lastFlag[flag] < SAME_KIND_GAP_TICKS) flag = null;
      else this.lastFlag[flag] = this.ticks;
    }
    return { kind, flag };
  }
}

function percentile(values: number[], p: number): number {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * p))];
}
