"use client";

import { useEffect, useRef } from "react";
import clsx from "clsx";
import { Camera, Mic, Monitor, ScreenShare, Video } from "lucide-react";
import type { Monitoring } from "./useMonitoring";

/**
 * The camera tile when the panel is live: the candidate sees themselves, and
 * under it what is being shared (camera, microphone, entire screen) with a
 * button for whatever isn't on yet. Sharing is the candidate's to start, since
 * the browser only asks on a click; the clock waits for it.
 */
export function MonitorCard({ m, requires }: { m: Monitoring; requires: { camera: boolean; screen: boolean; audio: boolean } }) {
  const video = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    if (video.current) video.current.srcObject = m.cameraStream;
  }, [m.cameraStream]);

  const rows = [
    { key: "camera", label: "Camera", icon: Camera, on: m.flags.camera, needed: requires.camera, action: m.enableCameraAndMic, verb: "Turn on" },
    { key: "audio", label: "Microphone", icon: Mic, on: m.flags.audio, needed: requires.audio, action: m.enableCameraAndMic, verb: "Turn on" },
    { key: "screen", label: "Entire screen", icon: ScreenShare, on: m.flags.screen, needed: requires.screen, action: m.enableScreen, verb: "Share" },
  ] as const;
  const recording = m.sessionStatus === "active" && (m.flags.camera || m.flags.screen || m.flags.audio);

  return (
    <section className="rounded-2xl border border-[#E8EDF7] bg-white p-3" aria-label="What is shared">
      <div className="mb-2 flex items-center justify-between px-1">
        <div className="flex items-center gap-2 text-sm font-medium text-[#2A3355]">
          <Video className="h-4 w-4" aria-hidden />
          Monitoring
        </div>
        <span
          className={clsx(
            "flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
            recording ? "bg-[#EAF7EE] text-[#1F7A3F]" : "bg-[#F1F3F8] text-[#6B7393]",
          )}
        >
          <span className={clsx("h-2 w-2 rounded-full", recording ? "animate-pulse bg-[#E5484D]" : "bg-[#A3A9BF]")} aria-hidden />
          {recording ? "Recording" : m.sessionStatus === "active" ? "Paused" : "Not started"}
        </span>
      </div>
      <div className="relative aspect-[4/3] overflow-hidden rounded-xl bg-[#1A1F2E]">
        {m.cameraStream ? (
          <video ref={video} autoPlay muted playsInline className="h-full w-full -scale-x-100 object-cover" />
        ) : (
          <div className="flex h-full items-center justify-center p-4 text-center text-sm text-[#C9CEDD]">
            <Monitor className="mr-2 h-4 w-4 shrink-0" aria-hidden />
            Your camera preview shows here.
          </div>
        )}
      </div>
      <ul className="mt-3 space-y-1.5">
        {rows
          .filter((r) => r.needed)
          .map((r) => (
            <li key={r.key} className="flex items-center gap-2 text-sm">
              <r.icon className={clsx("h-4 w-4", r.on ? "text-[#1F7A3F]" : "text-[#A3A9BF]")} aria-hidden />
              <span className="flex-1 text-[#2A3355]">{r.label}</span>
              {r.on ? (
                <span className="text-xs font-medium text-[#1F7A3F]">On</span>
              ) : (
                <button type="button" onClick={() => void r.action()} className="rounded-lg bg-[#2F5BD3] px-2.5 py-1 text-xs font-medium text-white hover:bg-[#274DB5]">
                  {r.verb}
                </button>
              )}
            </li>
          ))}
      </ul>
      {m.error && (
        <p role="alert" className="mt-2 text-xs text-[#D23B3B]">
          {m.error}
        </p>
      )}
    </section>
  );
}
