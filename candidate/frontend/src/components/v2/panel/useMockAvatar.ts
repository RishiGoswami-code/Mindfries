"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { AvatarState } from "./Avatar2D";

/**
 * Step 1's stand-in for the Gemini Live avatar (v2/PANEL.md §6): it listens,
 * and when asked to talk it "thinks" briefly, then speaks with a simulated
 * mouth level. Step 5 replaces this with the real voice and its audio level.
 */
export function useMockAvatar() {
  const [state, setState] = useState<AvatarState>("listening");
  const [level, setLevel] = useState(0);
  const [muted, setMuted] = useState(false);
  const raf = useRef<number | null>(null);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);

  const clear = useCallback(() => {
    for (const t of timers.current) clearTimeout(t);
    timers.current = [];
    if (raf.current !== null) cancelAnimationFrame(raf.current);
    raf.current = null;
    setLevel(0);
  }, []);

  useEffect(() => clear, [clear]);

  const talk = useCallback(() => {
    clear();
    setState("thinking");
    timers.current.push(
      setTimeout(() => {
        setState("speaking");
        const start = performance.now();
        const tick = (t: number) => {
          const s = (t - start) / 1000;
          setLevel(Math.max(0, 0.5 + 0.35 * Math.sin(s * 11) + 0.15 * Math.sin(s * 23)));
          raf.current = requestAnimationFrame(tick);
        };
        raf.current = requestAnimationFrame(tick);
      }, 900),
      setTimeout(() => {
        clear();
        setState("listening");
      }, 5200),
    );
  }, [clear]);

  const toggleMute = useCallback(() => {
    setMuted((m) => {
      setState(m ? "listening" : "idle");
      return !m;
    });
  }, []);

  return { state, level, muted, talk, toggleMute };
}
