"use client";

import { useEffect, useRef, useState } from "react";
import clsx from "clsx";
import { Clock, Send, AlertTriangle, Video, VideoOff } from "lucide-react";
import { idePalette, PRIMARY_BUTTON, STATUS_BAR_BG } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

interface HeaderPanelProps {
  theme: IdeTheme;
  assessmentName: string;
  /** Seconds left on the session's clock. Undefined when nothing is being timed — the timer is hidden rather than counting down an invented duration. */
  durationSeconds?: number;
  onSubmit: () => void;
  /** Called once, when a timed session reaches zero. */
  onExpire?: () => void;
  /** Whether the proctoring camera is live — shown, not switchable (see CameraPanel). Undefined: no camera in this workspace. */
  cameraLive?: boolean;
}

/**
 * Top-level header bar for the assessment workspace (PRD §1.6).
 *
 * Shows the assessment name on the left, a live countdown timer in the
 * center, and a Submit button on the right. The timer starts from what the
 * backend says is left on the session and ticks locally from there.
 */
export function HeaderPanel({ theme, assessmentName, durationSeconds, onSubmit, onExpire, cameraLive }: HeaderPanelProps) {
  const palette = idePalette(theme);
  const [remaining, setRemaining] = useState(durationSeconds ?? 0);

  useEffect(() => {
    if (durationSeconds === undefined || remaining <= 0) return;
    const id = setInterval(() => {
      setRemaining((prev) => {
        if (prev <= 1) {
          clearInterval(id);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(id);
  }, [remaining, durationSeconds]);

  // Fires once when the clock runs out — including straight away, for a
  // workspace opened after its session's time had already gone.
  const expired = durationSeconds !== undefined && remaining <= 0;
  const onExpireRef = useRef(onExpire);
  useEffect(() => {
    onExpireRef.current = onExpire;
  });
  useEffect(() => {
    if (expired) onExpireRef.current?.();
  }, [expired]);

  const hours = Math.floor(remaining / 3600);
  const minutes = Math.floor((remaining % 3600) / 60);
  const seconds = remaining % 60;
  const formatted = `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;

  const isLow = remaining <= 300; // 5 minutes or less
  const isCritical = remaining <= 60; // 1 minute or less

  const dark = theme === "dark";

  return (
    <div
      className={clsx(
        // Edge to edge across the top, outside the canvas's padding.
        "-mx-2 -mt-2 flex h-[64px] shrink-0 items-center justify-between gap-4 border-b px-5",
        palette.border,
        dark ? "bg-[#0B1222]" : palette.panelBg,
        palette.text
      )}
    >
      {/* Left: the logo, then the assessment's name */}
      <div className="flex min-w-0 items-center gap-3">
        {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG */}
        <img src="/mindfries-logo.svg" alt="" width={34} height={34} />
        <span className="text-[20px] font-bold tracking-tight">Mindfries</span>
        <span className={clsx("mx-1 h-6 w-px", dark ? "bg-[#1E2A44]" : "bg-[#B3CFE5]")} />
        <span className={clsx("truncate text-[15px]", palette.textMuted)}>{assessmentName}</span>
      </div>

      {/* Right: timer, camera, Submit */}
      <div className="flex shrink-0 items-center gap-2">
        {durationSeconds !== undefined && (
          <div
            className={clsx("flex h-10 items-center gap-2 rounded-xl border px-3.5 font-mono text-sm tabular-nums", palette.border, {
              "bg-[#2F6BFF]/10 text-[#E2E8F0]": !isLow && dark,
              "bg-[#4A7FA7]/10 text-[#0A1931]": !isLow && !dark,
              "border-[#E06C75]/40 bg-[#E06C75]/15 text-[#E06C75]": isLow && !isCritical,
              "border-[#E06C75]/60 bg-[#AD2934]/20 text-[#E06C75] animate-pulse": isCritical,
            })}
          >
            {isLow ? <AlertTriangle size={15} /> : <Clock size={15} className={dark ? "text-[#60A5FA]" : "text-[#4A7FA7]"} />}
            <span>{formatted}</span>
          </div>
        )}

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
          className={clsx(
            "flex h-10 items-center gap-2 rounded-xl px-5 text-[14.5px] font-semibold transition",
            dark ? PRIMARY_BUTTON : "text-white hover:opacity-90"
          )}
          style={dark ? undefined : { backgroundColor: STATUS_BAR_BG }}
        >
          <Send size={15} />
          Submit
        </button>
      </div>
    </div>
  );
}

/**
 * Modal confirmation dialog shown when the candidate clicks Submit.
 * Gives them a final chance to go back before their work is submitted.
 */
export function SubmitConfirmDialog({
  theme,
  onCancel,
  onConfirm,
  pending,
  error,
  timeUp,
  beforeInterview,
}: {
  theme: IdeTheme;
  onCancel: () => void;
  onConfirm: () => void;
  /** The work is being submitted as it stands, with no going back to it — the session ran out of time, or the interview is over. */
  timeUp?: boolean;
  /** Confirming leads to the interview rather than straight to submission. */
  beforeInterview?: boolean;
  /** True while a real submit call to the backend is in flight — disables both buttons so a slow request can't be fired twice. */
  pending?: boolean;
  /** Set when a real submit attempt failed — shown instead of silently reopening the workspace as if nothing happened. */
  error?: string;
}) {
  const palette = idePalette(theme);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="submit-dialog-title"
        className={clsx(
          "w-full max-w-md overflow-hidden rounded-xl border shadow-2xl",
          palette.border,
          palette.appBg,
          palette.text
        )}
      >
        <div className={clsx("flex items-center gap-2 border-b px-4 py-3", palette.border)}>
          <Send size={16} className={palette.accent} />
          <h2 id="submit-dialog-title" className="text-sm font-semibold">
            {timeUp ? "Submitting your work" : "Finished with your work?"}
          </h2>
        </div>

        <div className="px-4 py-3 text-sm">
          {timeUp ? (
            <p className={palette.textMuted}>
              Your work is being submitted exactly as it stands — code, activity and interview
              answers.
            </p>
          ) : (
            <>
              <p className={palette.textMuted}>
                Once you submit, you will not be able to make any further changes. Your code, terminal
                history, and all activity will be sent for evaluation.
              </p>
              <p className={clsx("mt-2", palette.textMuted)}>
                Make sure you have saved all your files and are happy with your solution before
                proceeding.
              </p>
              {beforeInterview && (
                <p className={clsx("mt-2 font-medium", palette.text)}>
                  A short AI interview about your work comes next. Your code is locked as soon as it
                  starts — you can&apos;t return to the editor afterwards.
                </p>
              )}
            </>
          )}
          {error && (
            <p className="mt-3 rounded-md border border-red-400/40 bg-red-500/10 px-3 py-2 text-xs font-medium text-red-400">
              {error}
            </p>
          )}
        </div>

        <div className={clsx("flex justify-end gap-2 border-t px-4 py-3", palette.border)}>
          {!timeUp && (
            <button
              type="button"
              onClick={onCancel}
              disabled={pending}
              className={clsx("rounded-md px-3 py-1.5 text-xs disabled:opacity-50", palette.hover, palette.textMuted)}
            >
              Go back
            </button>
          )}
          <button
            type="button"
            onClick={onConfirm}
            disabled={pending}
            className="flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium text-white hover:opacity-90 disabled:opacity-60"
            style={{ backgroundColor: STATUS_BAR_BG }}
          >
            <Send size={13} />
            {pending ? "Submitting…" : timeUp ? "Try again" : beforeInterview ? "I'm finished — start the interview" : "Confirm & submit"}
          </button>
        </div>
      </div>
    </div>
  );
}
