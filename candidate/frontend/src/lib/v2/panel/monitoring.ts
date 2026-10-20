/**
 * The monitoring panel's rules, apart from the browser, so they can be checked
 * in Node. The assessment needs the camera, the screen and the microphone
 * live before the clock starts; once it runs, a still and a screenshot go up
 * about every 30 seconds and the microphone in 30-second clips. A share that
 * stops is reported, and is a gap in the record, never a verdict.
 */

export const UPLOAD_EVERY_MS = 30_000;
export const STATUS_EVERY_MS = 5_000;
export const CLIP_MS = 30_000;

export type Source = "camera" | "screen" | "audio";

/** What's being shared, from the tracks' own state. */
export function liveFlags(t: {
  camera?: { readyState: string; enabled: boolean }[];
  mic?: { readyState: string; enabled: boolean }[];
  screen?: { readyState: string; enabled: boolean; displaySurface?: string }[];
}): Record<Source, boolean> {
  const live = (xs?: { readyState: string; enabled: boolean }[]) => !!xs?.some((x) => x.readyState === "live" && x.enabled);
  return {
    camera: live(t.camera),
    audio: live(t.mic),
    // Only the entire screen counts: a single window or tab would hide the rest.
    screen: !!t.screen?.some((x) => x.readyState === "live" && x.enabled && isEntireScreen(x.displaySurface)),
  };
}

/** displaySurface is "monitor" for the entire screen; a browser that doesn't say is given the benefit of the doubt. */
export function isEntireScreen(displaySurface?: string): boolean {
  return displaySurface === undefined || displaySurface === "monitor";
}

/** Which still is due: the clock must be running, and 30 seconds since the last. */
export function stillsDue(now: number, last: { camera: number; screen: number }, running: boolean): ("camera" | "screen")[] {
  if (!running) return [];
  const due: ("camera" | "screen")[] = [];
  if (now - last.camera >= UPLOAD_EVERY_MS) due.push("camera");
  if (now - last.screen >= UPLOAD_EVERY_MS) due.push("screen");
  return due;
}

/** Fits a frame inside a maximum width, keeping its shape. */
export function fitWidth(w: number, h: number, maxW: number): { w: number; h: number } {
  if (w <= maxW || w <= 0) return { w, h };
  const k = maxW / w;
  return { w: maxW, h: Math.max(1, Math.round(h * k)) };
}

/** The best recording format this browser can make, in order of preference. */
export function pickAudioMime(supported: (t: string) => boolean): string | null {
  for (const t of ["audio/webm;codecs=opus", "audio/webm", "audio/ogg;codecs=opus", "audio/mp4"]) if (supported(t)) return t;
  return null;
}

/** Words for what is missing, as the API sent it, or from what we know locally. */
export function missingWords(flags: Record<Source, boolean>, requires: Record<Source, boolean>): string[] {
  const out: string[] = [];
  if (requires.camera && !flags.camera) out.push("Turn on your camera");
  if (requires.audio && !flags.audio) out.push("Turn on your microphone");
  if (requires.screen && !flags.screen) out.push("Share your entire screen");
  return out;
}
