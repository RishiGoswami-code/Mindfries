"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useProctorCamera } from "@/lib/ide/proctor-camera";
import { startScreenKeyframes, useProctorScreen, type ScreenKeyframes } from "@/lib/ide/screen-capture";
import { keepStill, stillTaker, type StillSink, type StillTaker } from "@/lib/ide/stills";
import { FLAG_NOTICE, watchActivity, type Flag } from "@/lib/ide/activity-watch";
import { watchFaces } from "@/lib/ide/face-watch";
import { watchAudio } from "@/lib/ide/audio-watch";
import { startCameraSnapshots } from "@/lib/ide/camera-snapshots";
import { startSnapshotUpload } from "@/app/ide/interview-actions";

/**
 * The proctored session: the camera (and, in a real session, the
 * microphone and the whole screen), stills of both, and the moments that
 * are flagged as they happen — each kept with a camera still and a screen
 * still of that moment. A practice run asks for the camera too, so the
 * candidate sees what a session is like, but records nothing.
 *
 * `notice` is what the candidate is told when a moment is flagged; it can be
 * dismissed, and goes by itself after a few seconds.
 */
export function useProctoring({ sessionId, practice, record }: { sessionId?: string; practice: boolean; record: (type: string, payload: unknown) => void }) {
  const proctored = !!sessionId && !practice;
  const camera = useProctorCamera(proctored);
  const screen = useProctorScreen();

  // A camera still every half minute, kept with the session.
  useEffect(() => {
    if (!sessionId || !camera.stream) return;
    return startCameraSnapshots(camera.stream, {
      uploadTarget: async () => {
        const target = await startSnapshotUpload(sessionId, "camera");
        return "error" in target ? null : target;
      },
      stored: (snapshot) => record("camera_snapshot", snapshot),
    });
  }, [sessionId, camera.stream, record]);

  const sink = useMemo<StillSink | null>(
    () =>
      sessionId
        ? {
            uploadTarget: async (kind) => {
              const target = await startSnapshotUpload(sessionId, kind);
              return "error" in target ? null : target;
            },
          }
        : null,
    [sessionId]
  );

  // Screen keyframes, and a camera still-taker for flagged moments.
  const keyframesRef = useRef<ScreenKeyframes | null>(null);
  const stillsRef = useRef<StillTaker | null>(null);
  useEffect(() => {
    if (!proctored || !screen.stream || !sink) return;
    const keyframes = startScreenKeyframes(screen.stream, sink, (still) => record("screen_snapshot", still));
    keyframesRef.current = keyframes;
    return () => {
      keyframes.stop();
      keyframesRef.current = null;
    };
  }, [proctored, screen.stream, sink, record]);
  useEffect(() => {
    if (!proctored || !camera.stream) return;
    const taker = stillTaker(camera.stream, { width: 640, quality: 0.6 });
    stillsRef.current = taker;
    return () => {
      taker.stop();
      stillsRef.current = null;
    };
  }, [proctored, camera.stream]);

  const [notice, setNotice] = useState<string | null>(null);
  const raiseFlag = useCallback(
    async (flag: Flag, audioClip?: Promise<Blob | null>) => {
      const at = new Date().toISOString();
      const text = FLAG_NOTICE[flag.kind];
      if (text) setNotice(text);
      const cameraStill = async () => {
        const blob = await stillsRef.current?.take();
        if (!blob || !sink) return null;
        const path = await keepStill(blob, "camera", sink);
        if (path) record("camera_snapshot", { path, bytes: blob.size, reason: "flag" });
        return path;
      };
      const audio = async () => {
        const blob = audioClip ? await audioClip : null;
        if (!blob || !sink) return null;
        const path = await keepStill(blob, "audio", sink);
        if (path) record("audio_clip", { path, bytes: blob.size });
        return path;
      };
      const [cameraPath, screenPath, audioPath] = await Promise.all([cameraStill(), keyframesRef.current?.captureNow() ?? Promise.resolve(null), audio()]);
      record("activity_flag", { kind: flag.kind, at, ...(flag.detail ? { detail: flag.detail } : {}), cameraPath, screenPath, ...(audioClip ? { audioPath } : {}) });
    },
    [sink, record]
  );
  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(null), 8000);
    return () => clearTimeout(timer);
  }, [notice]);

  // Watching starts once both shares are live, so the permission prompts themselves aren't flagged.
  const watching = proctored && camera.status === "live" && screen.status === "live";
  useEffect(() => {
    if (!watching || !sessionId) return;
    return watchActivity((flag) => void raiseFlag(flag), sessionId);
  }, [watching, sessionId, raiseFlag]);
  useEffect(() => {
    if (!watching || !camera.stream) return;
    return watchAudio(camera.stream, (event) => void raiseFlag({ kind: event.kind }, event.clip));
  }, [watching, camera.stream, raiseFlag]);
  useEffect(() => {
    if (!watching || !camera.stream) return;
    return watchFaces(
      camera.stream,
      (event) => void raiseFlag({ kind: event.kind, ...(event.kind === "multiple_faces" ? { detail: { faces: event.faces } } : {}) }),
      (status) => record("proctor_ml", status)
    );
  }, [watching, camera.stream, raiseFlag, record]);

  // The share stopping is a moment too; the gate locks the workspace.
  const screenStatus = screen.status;
  useEffect(() => {
    if (!proctored || screenStatus !== "ended") return;
    const timer = setTimeout(() => void raiseFlag({ kind: "screen_share_stopped" }), 0);
    return () => clearTimeout(timer);
  }, [proctored, screenStatus, raiseFlag]);

  return { camera, screen, proctored, notice, dismissNotice: () => setNotice(null) };
}
