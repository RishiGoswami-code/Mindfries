"use client";

import { useEffect, useState } from "react";

/**
 * The session's clock, counted from a fixed deadline rather than tick by
 * tick: a tab the browser throttled in the background, or a laptop that
 * slept, shows the time that's really left the moment it's looked at again —
 * never more than the server allows.
 *
 * `remainingSeconds` is what the server said was left when the page loaded;
 * undefined when nothing is being timed (a practice run, a scratch
 * workspace), and the clock then reports nothing.
 */
export function useSessionClock(remainingSeconds: number | undefined) {
  const [deadline] = useState(() => (remainingSeconds === undefined ? null : Date.now() + Math.max(0, remainingSeconds) * 1000));
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (deadline === null) return;
    const tick = () => setNow(Date.now());
    const timer = setInterval(tick, 1000);
    // Coming back to the tab corrects the clock at once, not on the next tick.
    const onVisible = () => document.visibilityState === "visible" && tick();
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [deadline]);

  if (deadline === null) return { remaining: undefined, expired: false };
  const remaining = Math.max(0, Math.ceil((deadline - now) / 1000));
  return { remaining, expired: remaining === 0 };
}
