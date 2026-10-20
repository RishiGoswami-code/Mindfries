"use client";

import { useEffect, useState } from "react";
import clsx from "clsx";
import { Clock } from "lucide-react";

/** h:mm:ss, or m:ss under an hour; never negative. */
export function formatLeft(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

/** The time left, ticking every second; amber under 10 minutes, red under 5. */
export function Countdown({ endsAt, compact }: { endsAt: string; compact?: boolean }) {
  const end = new Date(endsAt).getTime();
  const [now, setNow] = useState<number | null>(null);
  // The clock starts after hydration, so the server and client render alike.
  useEffect(() => {
    const first = setTimeout(() => setNow(Date.now()), 0);
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      clearTimeout(first);
      clearInterval(t);
    };
  }, []);
  const left = now === null ? end - Date.parse(endsAt) : end - now;
  const tone = now === null ? "normal" : left < 5 * 60_000 ? "red" : left < 10 * 60_000 ? "amber" : "normal";
  return (
    <div
      className={clsx(
        "flex items-center gap-1.5 rounded-xl border font-mono tabular-nums",
        compact ? "h-8 px-2 text-xs" : "h-11 px-3 text-sm",
        tone === "red" && "border-[#F5C2C3] bg-[#FDECEC] text-[#B42318]",
        tone === "amber" && "border-[#F6DFA6] bg-[#FEF6E4] text-[#9A6400]",
        tone === "normal" && "border-[#E3E8F2] bg-white text-[#3B4568]",
      )}
      title="Time left"
      aria-label={now === null ? "Time left" : `Time left ${formatLeft(left)}`}
    >
      <Clock className={compact ? "h-3.5 w-3.5" : "h-4 w-4"} aria-hidden />
      {now === null ? "–:––" : formatLeft(left)}
    </div>
  );
}
