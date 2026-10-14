"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { keepStill, stillTaker, type StillSink } from "./stills";

/**
 * The candidate's screen, for a proctored session (issue #120).
 *
 * The whole screen is shared — not a tab or a window — so what happens
 * outside the workspace is in view; the browser's own picker is used, and a
 * choice of anything less than the entire screen is refused with a reason.
 * Stopping the share locks the workspace, exactly as stopping the camera
 * does (ProctorGate).
 *
 * What is kept is not video. Following the approach of capturing a frame
 * only when the screen changes (Migut et al., cited in the EDM 2025 work on
 * online programming exams) rather than recording continuously, a frame is
 * kept:
 *
 *   - when the screen has visibly changed since the last kept frame, at most
 *     once every KEYFRAME_MIN_GAP_MS;
 *   - every BASELINE_MS regardless, so a still screen still has a record;
 *   - at once, at every flagged moment (activity-watch.ts).
 *
 * Change is measured on a tiny greyscale thumbnail, so the comparison itself
 * costs next to nothing. A session of an hour keeps tens of stills of about
 * 100 KB, where an hour of screen video is hundreds of megabytes that nobody
 * would watch.
 */

export type ScreenStatus = "idle" | "requesting" | "live" | "refused" | "not-whole-screen" | "unsupported" | "ended";

export interface ProctorScreen {
  status: ScreenStatus;
  stream: MediaStream | null;
  message: string | null;
  request: () => Promise<void>;
  stop: () => void;
}

export function useProctorScreen(): ProctorScreen {
  const [status, setStatus] = useState<ScreenStatus>("idle");
  const [message, setMessage] = useState<string | null>(null);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const streamRef = useRef<MediaStream | null>(null);

  useEffect(
    () => () => {
      for (const track of streamRef.current?.getTracks() ?? []) track.stop();
      streamRef.current = null;
    },
    [],
  );

  const request = useCallback(async () => {
    if (!navigator.mediaDevices?.getDisplayMedia) {
      setStatus("unsupported");
      setMessage("This browser can't share its screen. Use a current Chrome, Edge or Firefox on a computer.");
      return;
    }
    setStatus("requesting");
    setMessage(null);
    try {
      const next = await navigator.mediaDevices.getDisplayMedia({
        // A low frame rate: nothing here needs more than a frame every few seconds.
        video: { displaySurface: "monitor", frameRate: { ideal: 1, max: 5 } },
        audio: false,
        // Chrome's options for the picker: offer the entire screen first, and
        // don't let a tab be chosen in place of it later.
        ...({ monitorTypeSurfaces: "include", surfaceSwitching: "exclude", selfBrowserSurface: "include" } as object),
      } as DisplayMediaStreamOptions);
      const track = next.getVideoTracks()[0];
      const surface = (track?.getSettings() as MediaTrackSettings & { displaySurface?: string }).displaySurface;
      if (surface && surface !== "monitor") {
        for (const t of next.getTracks()) t.stop();
        setStatus("not-whole-screen");
        setMessage("Share your entire screen, not a single window or tab — choose “Entire screen” in the picker.");
        return;
      }
      streamRef.current = next;
      setStream(next);
      setStatus("live");
      track?.addEventListener("ended", () => {
        streamRef.current = null;
        setStream(null);
        setStatus("ended");
        setMessage("Screen sharing stopped. Share your entire screen again to carry on — this is recorded.");
      });
    } catch (err) {
      const name = err instanceof DOMException ? err.name : "";
      setStatus("refused");
      setMessage(
        name === "NotAllowedError"
          ? "Screen sharing was cancelled or blocked. The session needs your entire screen shared to continue."
          : "Screen sharing couldn't start. Try again, or use a current Chrome or Edge.",
      );
    }
  }, []);

  const stop = useCallback(() => {
    for (const track of streamRef.current?.getTracks() ?? []) track.stop();
    streamRef.current = null;
    setStream(null);
    setStatus("idle");
  }, []);

  return { status, stream, message, request, stop };
}

/** How often the screen is looked at for change. */
const SAMPLE_MS = 4_000;
/** The least time between two frames kept for change. */
const KEYFRAME_MIN_GAP_MS = 15_000;
/** A frame is kept at least this often, changed or not. */
const BASELINE_MS = 180_000;
/** Share of the thumbnail's pixels that must differ for the screen to count as changed. */
const CHANGE_THRESHOLD = 0.06;
/** How different one thumbnail pixel must be (0–255) to count. */
const PIXEL_DELTA = 24;
const THUMB_W = 48;
const THUMB_H = 27;

export interface ScreenKeyframes {
  /** Keeps a frame now — for a flagged moment. Resolves to its path, or null. */
  captureNow(): Promise<string | null>;
  stop(): void;
}

/**
 * Starts keeping keyframes of `stream`. `stored` is told about each one kept,
 * with why: "change", "baseline" or "flag".
 */
export function startScreenKeyframes(
  stream: MediaStream,
  sink: StillSink,
  stored: (still: { path: string; bytes: number; reason: "change" | "baseline" | "flag" }) => void,
): ScreenKeyframes {
  const taker = stillTaker(stream, { width: 1024, quality: 0.55, maxBytes: 190 * 1024 });
  const thumb = document.createElement("canvas");
  thumb.width = THUMB_W;
  thumb.height = THUMB_H;
  const ctx = thumb.getContext("2d", { willReadFrequently: true });
  let lastKeptThumb: Uint8ClampedArray | null = null;
  let lastKeptAt = 0;
  let busy = false;
  let stopped = false;

  const greyThumb = (): Uint8ClampedArray | null => {
    const v = taker.video;
    if (!ctx || v.readyState < 2 || v.videoWidth === 0) return null;
    ctx.drawImage(v, 0, 0, THUMB_W, THUMB_H);
    const rgba = ctx.getImageData(0, 0, THUMB_W, THUMB_H).data;
    const grey = new Uint8ClampedArray(THUMB_W * THUMB_H);
    for (let i = 0; i < grey.length; i++) grey[i] = (rgba[i * 4] * 3 + rgba[i * 4 + 1] * 6 + rgba[i * 4 + 2]) / 10;
    return grey;
  };

  const changedShare = (a: Uint8ClampedArray, b: Uint8ClampedArray) => {
    let changed = 0;
    for (let i = 0; i < a.length; i++) if (Math.abs(a[i] - b[i]) > PIXEL_DELTA) changed++;
    return changed / a.length;
  };

  const keep = async (reason: "change" | "baseline" | "flag"): Promise<string | null> => {
    const blob = await taker.take();
    if (!blob || stopped) return null;
    const path = await keepStill(blob, "screen", sink);
    if (path) {
      lastKeptThumb = greyThumb();
      lastKeptAt = Date.now();
      stored({ path, bytes: blob.size, reason });
    }
    return path;
  };

  const sample = async () => {
    if (stopped || busy) return;
    busy = true;
    try {
      const now = Date.now();
      if (!lastKeptThumb || now - lastKeptAt >= BASELINE_MS) {
        await keep(lastKeptThumb ? "baseline" : "change");
        return;
      }
      if (now - lastKeptAt < KEYFRAME_MIN_GAP_MS) return;
      const current = greyThumb();
      if (current && changedShare(current, lastKeptThumb) >= CHANGE_THRESHOLD) await keep("change");
    } finally {
      busy = false;
    }
  };

  const first = setTimeout(() => void sample(), 3_000);
  const interval = setInterval(() => void sample(), SAMPLE_MS);
  return {
    captureNow: () => keep("flag"),
    stop: () => {
      stopped = true;
      clearTimeout(first);
      clearInterval(interval);
      taker.stop();
    },
  };
}
