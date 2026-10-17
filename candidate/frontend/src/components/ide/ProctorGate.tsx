"use client";

import clsx from "clsx";
import { Monitor, Video, ShieldAlert } from "lucide-react";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";
import type { ProctorCamera } from "@/lib/ide/proctor-camera";
import type { ProctorScreen } from "@/lib/ide/screen-capture";
import { enterFullscreen } from "@/lib/ide/fullscreen";

/**
 * Blocks the workspace until the proctoring camera is live.
 *
 * It states plainly, before the permission prompt, that the session is
 * recorded-on-camera and that the camera stays on throughout. Springing that
 * on someone after the fact would be the wrong way to build this.
 *
 * The start button also takes the workspace fullscreen. That isn't an
 * afterthought about where to put it: `requestFullscreen()` needs an active
 * user gesture, so it can't run on page load, and this click is the one
 * gesture every session is guaranteed to pass through. See lib/ide/fullscreen.ts.
 */
export function ProctorGate({
  theme,
  camera,
  screen,
}: {
  theme: IdeTheme;
  camera: ProctorCamera;
  /** The screen share, for a real session; null where none is needed (a practice run). */
  screen: ProctorScreen | null;
}) {
  const palette = idePalette(theme);
  if (camera.status === "live" && (!screen || screen.status === "live")) return null;
  if (camera.status === "live" && screen) return <ScreenStep theme={theme} screen={screen} />;

  const interrupted = camera.status === "ended";
  const busy = camera.status === "requesting";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div
        role="dialog"
        aria-modal="true"
        className={clsx(
          "w-full max-w-md overflow-hidden rounded-xl border shadow-2xl",
          palette.border,
          palette.appBg,
          palette.text
        )}
      >
        <div className={clsx("flex items-center gap-2 border-b px-4 py-3", palette.border)}>
          {interrupted ? (
            <ShieldAlert size={16} className="text-[#ff8a8a]" />
          ) : (
            <Video size={16} className={palette.accent} />
          )}
          <h2 className="text-sm font-semibold">
            {interrupted ? "Session paused — camera stopped" : "Camera required"}
          </h2>
        </div>

        <div className="space-y-2 px-4 py-3 text-sm">
          <p className={palette.textMuted}>
            This is a proctored session. Your camera must stay on the whole time you&apos;re using
            the workspace, and your video is shown on screen so you can see exactly what&apos;s
            being captured.
            {screen &&
              " Your microphone is on too: it isn't recorded, but voices or a loud noise that last a few seconds are flagged, with a short clip kept."}
          </p>
          {camera.message && (
            <p className={clsx("text-xs", interrupted ? "text-[#ff8a8a]" : palette.textMuted)}>
              {camera.message}
            </p>
          )}
          <p className={clsx("text-xs", palette.textMuted)}>
            The workspace stays locked until the camera is running.
          </p>
        </div>

        <div className={clsx("flex justify-end border-t px-4 py-3", palette.border)}>
          <button
            type="button"
            onClick={() => {
              // Synchronously first: this click is the user gesture that
              // makes fullscreen legal, and awaiting the camera permission
              // would consume the activation before we got to ask.
              void enterFullscreen();
              void camera.request();
            }}
            disabled={busy}
            className={clsx(
              "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium",
              busy
                ? clsx(palette.textMuted, "opacity-60")
                : "bg-[#4A7FA7] text-[#F6FAFD] hover:opacity-90"
            )}
          >
            <Video size={13} />
            {busy ? "Waiting for camera…" : interrupted ? "Reconnect camera" : screen ? "Enable camera & microphone" : "Enable camera & start"}
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * The second step of a proctored session: the entire screen, shared. Said
 * plainly before the browser's picker opens — what is kept (stills when the
 * screen changes and at flagged moments, not video) and that stopping the
 * share pauses the workspace.
 */
function ScreenStep({ theme, screen }: { theme: IdeTheme; screen: ProctorScreen }) {
  const palette = idePalette(theme);
  const interrupted = screen.status === "ended";
  const busy = screen.status === "requesting";
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div role="dialog" aria-modal="true" className={clsx("w-full max-w-md overflow-hidden rounded-xl border shadow-2xl", palette.border, palette.appBg, palette.text)}>
        <div className={clsx("flex items-center gap-2 border-b px-4 py-3", palette.border)}>
          {interrupted ? <ShieldAlert size={16} className="text-[#ff8a8a]" /> : <Monitor size={16} className={palette.accent} />}
          <h2 className="text-sm font-semibold">{interrupted ? "Session paused — screen sharing stopped" : "Share your entire screen"}</h2>
        </div>
        <div className="space-y-2 px-4 py-3 text-sm">
          <p className={palette.textMuted}>
            Share your <strong>entire screen</strong> — not a window or a tab. A still of it is kept when it changes, every few minutes, and at
            any moment that is flagged (for example, leaving the workspace). Nothing is recorded as video.
          </p>
          <p className={clsx("text-xs", palette.textMuted)}>
            Flagged moments are for the hiring team to look at, with your camera and screen at that time. A flag is not a decision.
          </p>
          {screen.message && <p className={clsx("text-xs", interrupted || screen.status !== "idle" ? "text-[#ff8a8a]" : palette.textMuted)}>{screen.message}</p>}
          <p className={clsx("text-xs", palette.textMuted)}>The workspace stays locked until your screen is shared.</p>
        </div>
        <div className={clsx("flex justify-end border-t px-4 py-3", palette.border)}>
          <button
            type="button"
            onClick={() => {
              void enterFullscreen();
              void screen.request();
            }}
            disabled={busy}
            className={clsx(
              "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium",
              busy ? clsx(palette.textMuted, "opacity-60") : "bg-[#4A7FA7] text-[#F6FAFD] hover:opacity-90",
            )}
          >
            <Monitor size={13} />
            {busy ? "Waiting for the picker…" : interrupted ? "Share the screen again" : "Share entire screen"}
          </button>
        </div>
      </div>
    </div>
  );
}
