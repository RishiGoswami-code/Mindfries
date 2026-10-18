"use client";

import clsx from "clsx";
import { AlertTriangle, Clock, Send, Video, VideoOff } from "lucide-react";
import { idePalette, PRIMARY_BUTTON, STATUS_BAR_BG } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

const pad = (n: number) => String(n).padStart(2, "0");
const formatClock = (s: number) => `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;

/**
 * The workspace's top bar: the logo and what's being worked on, then the
 * time left, the camera's state and Submit.
 *
 * The clock is told the time left (useSessionClock) rather than counting
 * on its own, so it's always the deadline's time. At five minutes and at
 * one, a screen reader hears it too.
 */
export function WorkspaceHeader({
  theme,
  title,
  remaining,
  cameraLive,
  onSubmit,
  submitLabel = "Submit",
}: {
  theme: IdeTheme;
  title: string;
  /** Seconds left; undefined when nothing is timed (practice, scratch) — no clock is shown. */
  remaining?: number;
  /** Undefined: no camera in this workspace. */
  cameraLive?: boolean;
  onSubmit: () => void;
  submitLabel?: string;
}) {
  const palette = idePalette(theme);
  const dark = theme === "dark";
  const low = remaining !== undefined && remaining <= 300;
  const critical = remaining !== undefined && remaining <= 60;
  const announcement = remaining === undefined ? "" : remaining <= 60 && remaining > 0 ? "One minute left." : remaining <= 300 && remaining > 60 ? "Five minutes left." : "";

  return (
    <div className={clsx("-mx-2 -mt-2 flex h-[64px] shrink-0 items-center justify-between gap-4 border-b px-5", palette.border, dark ? "bg-[#0B1222]" : palette.panelBg, palette.text)}>
      <div className="flex min-w-0 items-center gap-3">
        {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG */}
        <img src="/mindfries-logo.svg" alt="" width={34} height={34} />
        <span className="text-[20px] font-bold tracking-tight">Mindfries</span>
        <span className={clsx("mx-1 h-6 w-px", dark ? "bg-[#1E2A44]" : "bg-[#B3CFE5]")} />
        <span className={clsx("truncate text-[15px]", palette.textMuted)}>{title}</span>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {remaining !== undefined && (
          <div
            title="Time left in this session"
            className={clsx("flex h-10 items-center gap-2 rounded-xl border px-3.5 font-mono text-sm tabular-nums", palette.border, {
              "bg-[#2F6BFF]/10 text-[#E2E8F0]": !low && dark,
              "bg-[#4A7FA7]/10 text-[#0A1931]": !low && !dark,
              "border-[#E06C75]/40 bg-[#E06C75]/15 text-[#E06C75]": low && !critical,
              "animate-pulse border-[#E06C75]/60 bg-[#AD2934]/20 text-[#E06C75]": critical,
            })}
          >
            {low ? <AlertTriangle size={15} /> : <Clock size={15} className={dark ? "text-[#60A5FA]" : "text-[#4A7FA7]"} />}
            <span>{formatClock(remaining)}</span>
          </div>
        )}
        {/* Spoken once as each threshold is crossed: the text only changes then. */}
        <span className="sr-only" role="status" aria-live="polite">
          {announcement}
        </span>

        {cameraLive !== undefined && (
          <span
            role="status"
            title={cameraLive ? "Camera on — recording for this session" : "Camera off"}
            className={clsx("flex h-10 w-10 items-center justify-center rounded-xl border", palette.border, cameraLive ? palette.textMuted : "text-[#E06C75]")}
          >
            {cameraLive ? <Video size={16} /> : <VideoOff size={16} />}
          </span>
        )}

        <button
          type="button"
          onClick={onSubmit}
          className={clsx("flex h-10 items-center gap-2 rounded-xl px-5 text-[14.5px] font-semibold transition", dark ? PRIMARY_BUTTON : "text-white hover:opacity-90")}
          style={dark ? undefined : { backgroundColor: STATUS_BAR_BG }}
        >
          <Send size={15} />
          {submitLabel}
        </button>
      </div>
    </div>
  );
}
