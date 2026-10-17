"use client";

import { useEffect, useRef, type ReactNode } from "react";
import clsx from "clsx";
import { Video } from "lucide-react";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

/**
 * The proctoring camera preview.
 *
 * It has no start, stop or close control by design: the session requires the
 * camera to stay on, so offering an off switch here would only ever be a
 * switch that also locks the workspace. Stopping means ending the session
 * (or revoking permission in the browser, which the gate then catches).
 *
 * The stream is owned by `useProctorCamera` above this component — this only
 * displays it, so the feed survives layout changes and can't be detached by
 * re-rendering.
 */
export function CameraPanel({ theme, stream, compact = false, overlay }: { theme: IdeTheme; stream: MediaStream | null; compact?: boolean; overlay?: ReactNode }) {
  const palette = idePalette(theme);
  const videoRef = useRef<HTMLVideoElement>(null);
  const dark = theme === "dark";

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    video.srcObject = stream;
    if (stream) void video.play().catch(() => undefined);
  }, [stream]);

  // `compact` (the task drawer is closed) lays the same elements out in a
  // row — label, indicator, a small live preview — instead of a stack. The
  // <video> is the same element either way, so the feed never restarts.
  return (
    <div className={clsx("flex h-full min-h-0", compact ? "flex-row items-center gap-3 px-3" : "flex-col")}>
      <div className={clsx("flex shrink-0 items-center gap-2 text-[13px]", compact ? "flex-1" : "h-12 px-4")}>
        <Video size={17} className={dark ? "text-[#60A5FA]" : palette.accent} />
        <span className={clsx("text-[14.5px] font-semibold", palette.text)}>Camera</span>
        {/* A steady, always-present indicator: no ambiguity about whether
            the session is being watched. */}
        <span className={clsx("ml-auto flex items-center gap-1.5", dark ? "text-[#F87182]" : "text-[#c0304c]")}>
          <span className="h-2 w-2 rounded-full bg-[#F0566A] shadow-[0_0_8px_rgba(240,86,106,0.8)]" />
          Recording
        </span>
      </div>

      <div
        className={clsx(
          "relative overflow-hidden bg-black",
          compact ? "h-8 w-14 shrink-0 rounded-md" : "mx-3 mb-3 min-h-0 flex-1 rounded-xl"
        )}
      >
        {/* Landscape, letterboxed rather than cropped. */}
        <video ref={videoRef} muted playsInline className="h-full w-full object-contain" />
        {/* Floats over the picture: the chips that reopen closed cards. */}
        {overlay && !compact && <div className="absolute inset-x-2 bottom-2 flex flex-wrap justify-end gap-2">{overlay}</div>}
      </div>
    </div>
  );
}
