/**
 * Live activity tracking during a proctored session (issue #120): the
 * moments a reviewer should look at, flagged as they happen, each with a
 * camera still and a screen still taken right then.
 *
 * What is flagged, and why each is only a flag:
 *
 *   left_workspace       the workspace tab or window lost focus for more than
 *                        a few seconds. The screen still shows what was in
 *                        front of the candidate instead. Reading the docs in
 *                        another tab is ordinary work; that is for a person
 *                        to judge, which is why the still is there.
 *   fullscreen_exit      the workspace left fullscreen.
 *   screen_share_stopped the screen share ended (the workspace also locks).
 *   extra_display        more than one display is attached; the shared
 *                        screen may not be the one being looked at.
 *   external_paste       a large paste that didn't come from inside the
 *                        workspace.
 *   face_missing,        from the camera (face-watch.ts) — the least reliable
 *   multiple_faces       of these, and marked so in the report.
 *   voices_detected,     from the microphone (audio-watch.ts), each with a
 *   loud_noise           short clip of the sound.
 *   devtools_attempt     a developer-tools shortcut — refused, and recorded.
 *   screenshot_attempt   PrintScreen was pressed.
 *   second_window        the same session is open in another tab or window.
 *
 * The design follows what the research on automated proctoring found:
 * signals are combined and shown together with their evidence for a person
 * to judge (Atoum et al., IEEE Trans. Multimedia 2017, combine several
 * detectors over time rather than trust one), flags are not findings
 * (Belzak et al., 2025, on false positives and reviewer bias), and a
 * browser can't see everything — a second device is invisible to it
 * (Burgess et al., 2022; Simko et al., CCS 2024, on evasion). Nothing here
 * ends or scores a session.
 *
 * All of this is recorded by the candidate's own browser and is described
 * to them before the session starts (lib/policies.ts).
 */

export type FlagKind =
  | "left_workspace"
  | "fullscreen_exit"
  | "screen_share_stopped"
  | "extra_display"
  | "external_paste"
  | "face_missing"
  | "multiple_faces"
  | "phone_visible"
  | "voices_detected"
  | "loud_noise"
  | "devtools_attempt"
  | "screenshot_attempt"
  | "second_window";

export interface Flag {
  kind: FlagKind;
  /** Short facts about it, e.g. how many characters a paste was. */
  detail?: Record<string, unknown>;
}

/** What the candidate is told when a moment is flagged. Null: nothing shown. */
export const FLAG_NOTICE: Record<FlagKind, string | null> = {
  left_workspace: "You left the workspace. That moment was recorded, with your screen and camera, for the hiring team to see.",
  fullscreen_exit: "The workspace left fullscreen. That was recorded — return to fullscreen to carry on.",
  screen_share_stopped: null, // the workspace locks and says so itself
  extra_display: "More than one display is connected. This was recorded; the hiring team may ask about it.",
  external_paste: "A large paste from outside the workspace was recorded.",
  face_missing: "The camera can't see you. Stay in front of it — this was recorded.",
  multiple_faces: "More than one person seems to be in view of the camera. This was recorded.",
  phone_visible: "A phone seems to be in view of the camera. This was recorded.",
  voices_detected: "Voices were picked up by the microphone. A short clip was kept for the hiring team.",
  loud_noise: "A loud noise was picked up by the microphone. A short clip was kept for the hiring team.",
  devtools_attempt: "Developer tools are not available during the assessment. The attempt was recorded.",
  screenshot_attempt: "A screenshot attempt was recorded.",
  second_window: "This assessment is open in another window or tab. That was recorded — use only one.",
};

/** The same kind isn't flagged again within this time — one moment, one flag. */
const SAME_KIND_GAP_MS = 30_000;
/** An absence shorter than this is a glance, not a moment. */
const AWAY_MS = 3_000;
/** A paste this large from outside is flagged (smaller ones are still recorded as pastes). */
// An external paste worth a flag: anything multi-line, or a single line of
// real length. The old 200-char floor let small snippets through — a one-line
// fix copied from a chat is exactly what a reviewer most wants to see.
const EXTERNAL_PASTE_MIN_CHARS = 40;

/**
 * Starts watching the page. `raise` is called for each flagged moment.
 * Returns the function that stops it.
 */
export function watchActivity(raise: (flag: Flag) => void, sessionId: string): () => void {
  const last = new Map<FlagKind, number>();
  const flag = (f: Flag) => {
    const now = Date.now();
    if (now - (last.get(f.kind) ?? 0) < SAME_KIND_GAP_MS) return;
    last.set(f.kind, now);
    raise(f);
  };

  // Leaving: flagged once the absence has lasted AWAY_MS, so the screen
  // still is of wherever the candidate went, not of the moment of leaving.
  let awayTimer: ReturnType<typeof setTimeout> | undefined;
  const leave = () => {
    if (awayTimer) return;
    awayTimer = setTimeout(() => flag({ kind: "left_workspace" }), AWAY_MS);
  };
  const back = () => {
    clearTimeout(awayTimer);
    awayTimer = undefined;
  };
  const onVisibility = () => (document.hidden ? leave() : back());
  const onBlur = () => {
    // Focus moving into the preview pane's iframe is still the workspace.
    if (document.activeElement?.tagName !== "IFRAME") leave();
  };

  let wasFullscreen = !!document.fullscreenElement;
  const onFullscreen = () => {
    const now = !!document.fullscreenElement;
    if (wasFullscreen && !now) flag({ kind: "fullscreen_exit" });
    wasFullscreen = now;
  };

  // Internal copies, so moving code around inside the workspace isn't
  // mistaken for bringing it in (the same rule as provenance.ts).
  let lastCopied = "";
  const normalize = (t: string) => t.replace(/\r\n?/g, "\n").trim();
  const onCopy = (e: ClipboardEvent) => {
    const text = e.clipboardData?.getData("text/plain") || window.getSelection()?.toString() || "";
    if (text) lastCopied = normalize(text);
  };
  const onPaste = (e: ClipboardEvent) => {
    const text = normalize(e.clipboardData?.getData("text/plain") ?? "");
    const lines = text ? text.split("\n").length : 0;
    // Flag any paste that came from outside the workspace (not an internal
    // move) and is plausibly code — multi-line, or one line of real length.
    if (text !== lastCopied && (lines > 1 || text.length >= EXTERNAL_PASTE_MIN_CHARS)) {
      flag({ kind: "external_paste", detail: { chars: text.length, lines } });
    }
  };

  // More than one display. window.screen.isExtended is Chrome and Edge's,
  // and so is the screen's change event; where they don't exist this check
  // is simply off.
  const screenInfo = window.screen as Screen & { isExtended?: boolean } & Partial<Pick<EventTarget, "addEventListener" | "removeEventListener">>;
  const checkDisplays = () => {
    if (screenInfo.isExtended === true) flag({ kind: "extra_display" });
  };
  checkDisplays();

  // Misattempts. The browser's developer tools are opened from the keyboard
  // with these; the shortcut is refused and the attempt recorded. (A menu
  // can still open them — a page can't prevent that — but the attempt from
  // the keyboard is the common one.) PrintScreen can't be stopped either;
  // it is recorded.
  const onKey = (e: KeyboardEvent) => {
    const k = e.key.toUpperCase();
    const devtools = e.key === "F12" || ((e.ctrlKey || e.metaKey) && e.shiftKey && ["I", "J", "C"].includes(k)) || (e.metaKey && e.altKey && ["I", "J", "C"].includes(k));
    if (devtools) {
      e.preventDefault();
      e.stopPropagation();
      flag({ kind: "devtools_attempt", detail: { keys: `${e.ctrlKey ? "Ctrl+" : ""}${e.metaKey ? "Cmd+" : ""}${e.altKey ? "Alt+" : ""}${e.shiftKey ? "Shift+" : ""}${e.key}` } });
    }
  };
  const onKeyUp = (e: KeyboardEvent) => {
    if (e.key === "PrintScreen") flag({ kind: "screenshot_attempt" });
  };

  // The same session open twice — in another tab or window of this
  // browser. Each copy announces itself; whichever hears another flags it.
  let channel: BroadcastChannel | null = null;
  if (typeof BroadcastChannel !== "undefined") {
    channel = new BroadcastChannel(`mf-session-${sessionId}`);
    channel.onmessage = (e) => {
      if (e.data === "here") {
        channel?.postMessage("also-here");
        flag({ kind: "second_window" });
      } else if (e.data === "also-here") flag({ kind: "second_window" });
    };
    channel.postMessage("here");
  }

  document.addEventListener("keydown", onKey, true);
  document.addEventListener("keyup", onKeyUp, true);
  document.addEventListener("visibilitychange", onVisibility);
  window.addEventListener("blur", onBlur);
  window.addEventListener("focus", back);
  document.addEventListener("fullscreenchange", onFullscreen);
  document.addEventListener("copy", onCopy);
  document.addEventListener("cut", onCopy);
  document.addEventListener("paste", onPaste, true);
  screenInfo.addEventListener?.("change", checkDisplays);

  return () => {
    clearTimeout(awayTimer);
    channel?.close();
    document.removeEventListener("keydown", onKey, true);
    document.removeEventListener("keyup", onKeyUp, true);
    document.removeEventListener("visibilitychange", onVisibility);
    window.removeEventListener("blur", onBlur);
    window.removeEventListener("focus", back);
    document.removeEventListener("fullscreenchange", onFullscreen);
    document.removeEventListener("copy", onCopy);
    document.removeEventListener("cut", onCopy);
    document.removeEventListener("paste", onPaste, true);
    screenInfo.removeEventListener?.("change", checkDisplays);
  };
}
