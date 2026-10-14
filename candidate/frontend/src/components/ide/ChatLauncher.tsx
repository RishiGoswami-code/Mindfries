"use client";

import clsx from "clsx";
import { idePalette } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

/**
 * Floating launcher for the Mindfries AI panel — a small bubble at the
 * top right of the editor.
 *
 * It hides while the panel is open: the panel has its own close control, and
 * leaving the bubble up would put it on top of the panel it opens.
 */
export function ChatLauncher({
  theme,
  hidden,
  onClick,
}: {
  theme: IdeTheme;
  hidden: boolean;
  onClick: () => void;
}) {
  const palette = idePalette(theme);
  if (hidden) return null;

  return (
    <button
      type="button"
      onClick={onClick}
      title="Mindfries AI"
      aria-label="Open Mindfries AI"
      className={clsx(
        // At the right end of the editor's tab row, where nothing else sits —
        // clear of the task drawer and the camera along the bottom.
        "fixed right-5 top-[74px] z-40 flex h-9 w-9 items-center justify-center rounded-full border shadow-lg transition hover:scale-105",
        palette.border,
        "bg-[#F6FAFD]"
      )}
    >
      {/* eslint-disable-next-line @next/next/no-img-element -- tiny static local SVG, no optimization needed */}
      <img src="/mindfries-logo.svg" alt="" width={20} height={20} />

      {/* Status dot, sat on the circle's outline at the bottom right. The
          ring matches the page behind it so the dot reads as attached to the
          edge rather than floating over it. */}
      <span
        className={clsx(
          "absolute -right-0.5 -bottom-0.5 h-2.5 w-2.5 rounded-full border-2",
          theme === "dark" ? "border-[#0E1431] bg-[#8B6CFF]" : "border-[#F6FAFD] bg-[#4A7FA7]"
        )}
      />
    </button>
  );
}
