"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { PanelApi } from "@/lib/v2/panel/api";
import {
  CLIP_MS,
  fitWidth,
  liveFlags,
  missingWords,
  pickAudioMime,
  STATUS_EVERY_MS,
  stillsDue,
  type Source,
} from "@/lib/v2/panel/monitoring";

export interface Monitoring {
  /** What is being shared right now. */
  flags: Record<Source, boolean>;
  /** What the server still needs before the clock can start (its words). */
  missing: string[];
  /** The session's status as the server last said (ready, active, ended). */
  sessionStatus: string;
  /** Plain words about the last problem, if any. */
  error: string | null;
  /** Local steps still to do, from what this assessment requires. */
  todo: string[];
  cameraStream: MediaStream | null;
  /** A JPEG (base64) of the camera or the screen right now, or null when that isn't shared. */
  frame: (what: "camera" | "screen", maxWidth: number) => Promise<string | null>;
  enableCameraAndMic: () => Promise<void>;
  enableScreen: () => Promise<void>;
  stopAll: () => void;
}

type Requires = { camera: boolean; screen: boolean; audio: boolean };

const tracksOf = (s: MediaStream | null, kind: "audio" | "video") => (s ? (kind === "audio" ? s.getAudioTracks() : s.getVideoTracks()) : []);

/**
 * The panel's monitoring, in the candidate's browser: asks for the camera,
 * microphone and the entire screen, tells the server what is on every few
 * seconds (the clock waits for it), and while the clock runs sends a camera
 * still and a screenshot about every 30 seconds and the microphone in 30-second
 * clips. A share that stops is reported and shows here; it is a gap in the
 * record, never a verdict.
 *
 * It lives in the page the CLI opened (not in the pop-out window), so popping
 * the panel out never stops it.
 */
export function useMonitoring(api: PanelApi, requires: Requires, initialStatus: string): Monitoring {
  const [userStream, setUserStream] = useState<MediaStream | null>(null);
  const [screenStream, setScreenStream] = useState<MediaStream | null>(null);
  const [tick, setTick] = useState(0); // bumps when a track ends, so flags recompute
  const [missing, setMissing] = useState<string[]>([]);
  const [sessionStatus, setSessionStatus] = useState(initialStatus);
  const [error, setError] = useState<string | null>(null);

  const userRef = useRef<MediaStream | null>(null);
  const screenRef = useRef<MediaStream | null>(null);
  const statusRef = useRef(initialStatus);
  const flagsRef = useRef<Record<Source, boolean>>({ camera: false, screen: false, audio: false });
  const last = useRef({ camera: 0, screen: 0 });
  const videos = useRef<{ camera?: HTMLVideoElement; screen?: HTMLVideoElement }>({});

  // tick is how a track ending forces a recompute: a track's own state isn't React state.
  const flags = useMemo(
    () =>
      liveFlags({
        camera: tracksOf(userStream, "video"),
        mic: tracksOf(userStream, "audio"),
        screen: tracksOf(screenStream, "video").map((t) => ({
          readyState: t.readyState,
          enabled: t.enabled,
          displaySurface: (t.getSettings() as { displaySurface?: string }).displaySurface,
        })),
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [userStream, screenStream, tick],
  );
  useEffect(() => {
    flagsRef.current = flags;
  }, [flags]);

  const watch = useCallback((s: MediaStream) => {
    for (const t of s.getTracks()) t.addEventListener("ended", () => setTick((n) => n + 1));
  }, []);

  const enableCameraAndMic = useCallback(async () => {
    setError(null);
    try {
      const s = await navigator.mediaDevices.getUserMedia({ video: { width: 640, height: 480 }, audio: requires.audio });
      userRef.current?.getTracks().forEach((t) => t.stop());
      userRef.current = s;
      setUserStream(s);
      watch(s);
    } catch {
      setError("Your browser didn't give access to the camera or microphone. Allow it in the address bar and try again.");
    }
  }, [requires.audio, watch]);

  const enableScreen = useCallback(async () => {
    setError(null);
    try {
      const s = await navigator.mediaDevices.getDisplayMedia({ video: { displaySurface: "monitor" } as MediaTrackConstraints, audio: false });
      const surface = (s.getVideoTracks()[0]?.getSettings() as { displaySurface?: string } | undefined)?.displaySurface;
      if (surface && surface !== "monitor") {
        s.getTracks().forEach((t) => t.stop());
        setError("Please share your entire screen, not a window or a tab.");
        return;
      }
      screenRef.current?.getTracks().forEach((t) => t.stop());
      screenRef.current = s;
      setScreenStream(s);
      watch(s);
    } catch {
      setError("Screen sharing wasn't allowed. Choose your entire screen when your browser asks.");
    }
  }, [watch]);

  const frame = useCallback(async (what: "camera" | "screen", maxWidth: number): Promise<string | null> => {
    const v = videos.current[what];
    if (!v || !v.videoWidth || !(what === "camera" ? flagsRef.current.camera : flagsRef.current.screen)) return null;
    const { w, h } = fitWidth(v.videoWidth, v.videoHeight, maxWidth);
    const c = document.createElement("canvas");
    c.width = w;
    c.height = h;
    c.getContext("2d")?.drawImage(v, 0, 0, w, h);
    const url = c.toDataURL("image/jpeg", 0.6);
    return url.slice(url.indexOf(",") + 1);
  }, []);

  const stopAll = useCallback(() => {
    for (const r of [userRef, screenRef]) r.current?.getTracks().forEach((t) => t.stop());
    userRef.current = screenRef.current = null;
    setUserStream(null);
    setScreenStream(null);
  }, []);

  // Hidden video elements so a frame can be drawn from each stream.
  useEffect(() => {
    const make = (s: MediaStream | null) => {
      if (!s) return undefined;
      const v = document.createElement("video");
      v.muted = true;
      v.playsInline = true;
      v.srcObject = s;
      void v.play().catch(() => {});
      return v;
    };
    videos.current = { camera: make(userStream), screen: make(screenStream) };
    return () => {
      for (const v of Object.values(videos.current)) if (v) v.srcObject = null;
    };
  }, [userStream, screenStream]);

  // What's on, to the server, every few seconds. The clock waits for it.
  useEffect(() => {
    let live = true;
    const send = async () => {
      try {
        const r = await api.status(flagsRef.current);
        if (!live) return;
        setMissing(r.missing ?? []);
        statusRef.current = r.session_status;
        setSessionStatus(r.session_status);
        setError((e) => (e && e.startsWith("Couldn't reach") ? null : e));
      } catch (e) {
        if (live && e instanceof Error) setError(e.message);
      }
    };
    void send();
    const id = setInterval(send, STATUS_EVERY_MS);
    return () => {
      live = false;
      clearInterval(id);
    };
  }, [api]);

  // Stills and screenshots, while the clock runs.
  useEffect(() => {
    const frame = (v: HTMLVideoElement | undefined, maxW: number): Promise<Blob | null> =>
      new Promise((resolve) => {
        if (!v || !v.videoWidth) return resolve(null);
        const { w, h } = fitWidth(v.videoWidth, v.videoHeight, maxW);
        const c = document.createElement("canvas");
        c.width = w;
        c.height = h;
        c.getContext("2d")?.drawImage(v, 0, 0, w, h);
        c.toBlob((b) => resolve(b), "image/jpeg", 0.7);
      });
    const id = setInterval(async () => {
      const due = stillsDue(Date.now(), last.current, statusRef.current === "active");
      for (const what of due) {
        const ok = what === "camera" ? flagsRef.current.camera : flagsRef.current.screen;
        if (!ok) continue;
        last.current[what] = Date.now();
        const blob = await frame(videos.current[what], what === "camera" ? 640 : 1600);
        if (blob) void api.upload(what, blob).catch(() => {});
      }
    }, 1000);
    return () => clearInterval(id);
  }, [api]);

  // The microphone, in clips: each recorder runs 30 seconds, then its clip goes up.
  useEffect(() => {
    if (!userStream || !flags.audio || sessionStatus !== "active") return;
    const mime = pickAudioMime((t) => typeof MediaRecorder !== "undefined" && MediaRecorder.isTypeSupported(t));
    const audio = new MediaStream(tracksOf(userStream, "audio"));
    if (!mime || !audio.getAudioTracks().length) return;
    let stopped = false;
    let rec: MediaRecorder | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const begin = () => {
      if (stopped) return;
      const chunks: Blob[] = [];
      rec = new MediaRecorder(audio, { mimeType: mime });
      rec.ondataavailable = (e) => e.data.size && chunks.push(e.data);
      rec.onstop = () => {
        const blob = new Blob(chunks, { type: mime.split(";")[0] });
        if (blob.size > 1000) void api.upload("audio", blob).catch(() => {});
        begin();
      };
      rec.start();
      timer = setTimeout(() => rec?.state === "recording" && rec.stop(), CLIP_MS);
    };
    begin();
    return () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      if (rec && rec.state === "recording") rec.stop();
    };
  }, [api, userStream, flags.audio, sessionStatus]);

  useEffect(
    () => () => {
      for (const r of [userRef, screenRef]) r.current?.getTracks().forEach((t) => t.stop());
    },
    [],
  );

  return {
    flags,
    missing,
    sessionStatus,
    error,
    todo: missingWords(flags, requires),
    cameraStream: userStream,
    frame,
    enableCameraAndMic,
    enableScreen,
    stopAll,
  };
}
