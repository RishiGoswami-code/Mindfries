/**
 * Telling the candidate, as it happens, that a moment was flagged — even
 * when the workspace isn't on screen.
 *
 * The workspace's own notice (IdeShell's flagNotice) can't be seen from
 * another tab, and a "you left the workspace" flag is raised exactly while
 * the candidate is in another tab. So when the page is hidden the notice is
 * also sent as a system notification, which shows over whatever they are
 * looking at, and the in-page notice waits until they are back to start its
 * countdown. Both are best-effort: a browser without notifications, or a
 * candidate who declined them, still gets the in-page notice on return.
 *
 * Browser only; nothing here is recorded — the flag itself is (IdeShell's
 * raiseFlag).
 */

const TITLE = "Mindfries assessment";

/** Asks once for permission to show notifications. Safe to call repeatedly. */
export function askForFlagAlerts(): void {
  if (typeof Notification === "undefined" || Notification.permission !== "default") return;
  void Notification.requestPermission().catch(() => undefined);
}

/** Shows `text` as a system notification when the workspace isn't visible. Returns whether it did. */
export function alertWhileAway(text: string): boolean {
  if (typeof document === "undefined" || !document.hidden) return false;
  if (typeof Notification === "undefined" || Notification.permission !== "granted") return false;
  try {
    const n = new Notification(TITLE, { body: text, tag: "mindfries-flag", requireInteraction: false });
    n.onclick = () => {
      window.focus();
      n.close();
    };
    return true;
  } catch {
    return false;
  }
}

/**
 * Calls `onVisible` once the page is visible — now, if it already is. Returns
 * the function that cancels the wait.
 */
export function whenVisible(onVisible: () => void): () => void {
  if (typeof document === "undefined" || !document.hidden) {
    onVisible();
    return () => undefined;
  }
  const listener = () => {
    if (document.hidden) return;
    document.removeEventListener("visibilitychange", listener);
    onVisible();
  };
  document.addEventListener("visibilitychange", listener);
  return () => document.removeEventListener("visibilitychange", listener);
}
