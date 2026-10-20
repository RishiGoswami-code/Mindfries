"use client";

import { useEffect, useRef, useState } from "react";
import clsx from "clsx";
import { Mic, MicOff, Square, Video, VideoOff } from "lucide-react";
import { useProctorCamera } from "@/lib/ide/proctor-camera";

/**
 * The camera tile on the right of the panel: the candidate sees themselves
 * for the whole session, with the Recording dot and mic / camera / stop
 * controls, as in the mock-up.
 *
 * Step 1 shows the local preview only; nothing is sent anywhere yet
 * (v2/PANEL.md §5 moves the stills and audio here in step 4).
 */
export function CameraTile({ required }: { required: boolean }) {
  const cam = useProctorCamera(true);
  const video = useRef<HTMLVideoElement>(null);
  const [micOn, setMicOn] = useState(true);
  const [camOn, setCamOn] = useState(true);

  useEffect(() => {
    if (video.current) video.current.srcObject = cam.stream;
  }, [cam.stream]);

  const toggle = (kind: "audio" | "video", on: boolean) => {
    for (const t of cam.stream?.getTracks() ?? []) if (t.kind === kind) t.enabled = on;
  };

  const live = cam.status === "live";
  return (
    <section className="rounded-2xl border border-[#E8EDF7] bg-white p-3" aria-label="Camera">
      <div className="mb-2 flex items-center justify-between px-1">
        <div className="flex items-center gap-2 text-sm font-medium text-[#2A3355]">
          <Video className="h-4 w-4" aria-hidden />
          Camera
        </div>
        <span
          className={clsx(
            "flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
            live ? "bg-[#EAF7EE] text-[#1F7A3F]" : "bg-[#F1F3F8] text-[#6B7393]",
          )}
        >
          <span className={clsx("h-2 w-2 rounded-full", live ? "animate-pulse bg-[#E5484D]" : "bg-[#A3A9BF]")} aria-hidden />
          {live ? "Recording" : "Off"}
        </span>
      </div>
      <div className="relative aspect-[4/3] overflow-hidden rounded-xl bg-[#1A1F2E]">
        {live ? (
          <video ref={video} autoPlay muted playsInline className={clsx("h-full w-full -scale-x-100 object-cover", !camOn && "opacity-0")} />
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center text-sm text-[#C9CEDD]">
            <p>{cam.message ?? (required ? "This assessment needs your camera." : "Your camera is off.")}</p>
            <button
              type="button"
              onClick={() => void cam.request()}
              disabled={cam.status === "requesting"}
              className="rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-[#10183A] disabled:opacity-60"
            >
              {cam.status === "requesting" ? "Asking…" : "Turn on camera"}
            </button>
          </div>
        )}
        {live && (
          <div className="absolute inset-x-0 bottom-3 flex justify-center gap-3">
            <RoundButton
              label={micOn ? "Mute microphone" : "Unmute microphone"}
              onClick={() => {
                toggle("audio", !micOn);
                setMicOn(!micOn);
              }}
            >
              {micOn ? <Mic className="h-4 w-4" /> : <MicOff className="h-4 w-4" />}
            </RoundButton>
            <RoundButton
              label={camOn ? "Turn camera off" : "Turn camera on"}
              onClick={() => {
                toggle("video", !camOn);
                setCamOn(!camOn);
              }}
            >
              {camOn ? <Video className="h-4 w-4" /> : <VideoOff className="h-4 w-4" />}
            </RoundButton>
            <RoundButton label="Stop the camera" danger onClick={cam.stop}>
              <Square className="h-3.5 w-3.5 fill-current" />
            </RoundButton>
          </div>
        )}
      </div>
    </section>
  );
}

function RoundButton({ label, onClick, danger, children }: { label: string; onClick: () => void; danger?: boolean; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className={clsx(
        "flex h-10 w-10 items-center justify-center rounded-full border text-white backdrop-blur",
        danger ? "border-transparent bg-[#E5484D] hover:bg-[#D13B40]" : "border-white/60 bg-black/30 hover:bg-black/45",
      )}
    >
      {children}
    </button>
  );
}
