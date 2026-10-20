"use client";

import { useEffect, useState } from "react";

export type AvatarState = "idle" | "listening" | "thinking" | "speaking";

/**
 * The AI avatar's face: a 2D illustration that blinks, breathes, and moves
 * its mouth with `level` (0–1), the loudness of what it's saying. In step 5
 * the level comes from Gemini Live's reply audio; until then the card
 * simulates it.
 */
export function Avatar2D({ state, level, className }: { state: AvatarState; level: number; className?: string }) {
  const [blink, setBlink] = useState(false);
  useEffect(() => {
    let t: ReturnType<typeof setTimeout>;
    const next = () => {
      t = setTimeout(() => {
        setBlink(true);
        setTimeout(() => setBlink(false), 140);
        next();
      }, 2600 + Math.random() * 2400);
    };
    next();
    return () => clearTimeout(t);
  }, []);

  const open = state === "speaking" ? 1.5 + level * 9 : 1.2;
  const eyeRy = blink ? 0.6 : state === "thinking" ? 4.2 : 5.2;
  const look = state === "thinking" ? -2.5 : 0;
  return (
    <svg viewBox="0 0 240 220" preserveAspectRatio="xMidYMid meet" className={className} role="img" aria-label={`AI avatar, ${state}`}>
      <defs>
        <linearGradient id="av-bg" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#DCE8FF" />
          <stop offset="1" stopColor="#F3F7FF" />
        </linearGradient>
        <linearGradient id="av-hair" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#6B3F24" />
          <stop offset="1" stopColor="#4A2A17" />
        </linearGradient>
        <radialGradient id="av-skin" cx="0.5" cy="0.42" r="0.6">
          <stop offset="0" stopColor="#F6D3BC" />
          <stop offset="1" stopColor="#E9B699" />
        </radialGradient>
      </defs>
      <style>{`@keyframes av-breathe{0%,100%{transform:translateY(0) scale(1)}50%{transform:translateY(-1.5px) scale(1.012)}}.av-body{transform-origin:120px 200px;animation:av-breathe 4s ease-in-out infinite}@media (prefers-reduced-motion:reduce){.av-body{animation:none}}`}</style>
      <rect width="240" height="220" fill="url(#av-bg)" />
      <g className="av-body">
        {/* hair behind */}
        <path d="M58 120c-8-58 26-92 62-92s70 34 62 92c-2 34 6 58 14 74H44c8-16 16-40 14-74z" fill="url(#av-hair)" />
        {/* shoulders */}
        <path d="M40 220c6-34 40-46 80-46s74 12 80 46z" fill="#3B4A6B" />
        <path d="M100 168h40v18c-6 8-34 8-40 0z" fill="#E9B699" />
        {/* face */}
        <ellipse cx="120" cy="112" rx="46" ry="56" fill="url(#av-skin)" />
        {/* fringe */}
        <path d="M74 98c4-36 26-54 50-54 22 0 40 14 46 42-14-14-34-22-58-20-14 1-28 12-38 32z" fill="url(#av-hair)" />
        {/* brows */}
        <path d={`M94 ${state === "listening" ? 92 : 94}q10-6 20 0`} stroke="#4A2A17" strokeWidth="3" strokeLinecap="round" fill="none" />
        <path d={`M126 ${state === "listening" ? 92 : 94}q10-6 20 0`} stroke="#4A2A17" strokeWidth="3" strokeLinecap="round" fill="none" />
        {/* eyes */}
        <ellipse cx={104 + look} cy="108" rx="5.5" ry={eyeRy} fill="#2B1B12" />
        <ellipse cx={136 + look} cy="108" rx="5.5" ry={eyeRy} fill="#2B1B12" />
        {!blink && (
          <>
            <circle cx={106 + look} cy="106" r="1.6" fill="#fff" />
            <circle cx={138 + look} cy="106" r="1.6" fill="#fff" />
          </>
        )}
        {/* nose */}
        <path d="M120 112q-4 12 0 16q3 1 5-1" stroke="#C98E70" strokeWidth="2" strokeLinecap="round" fill="none" />
        {/* cheeks */}
        <ellipse cx="96" cy="128" rx="8" ry="4.5" fill="#F2A99A" opacity="0.45" />
        <ellipse cx="144" cy="128" rx="8" ry="4.5" fill="#F2A99A" opacity="0.45" />
        {/* mouth */}
        {state === "speaking" ? (
          <ellipse cx="120" cy="143" rx={9 + level * 3} ry={open} fill="#8C3B3B" />
        ) : (
          <path d="M108 141q12 9 24 0" stroke="#A4504A" strokeWidth="3" strokeLinecap="round" fill="none" />
        )}
      </g>
    </svg>
  );
}
