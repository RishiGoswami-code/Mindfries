"use client";

import { createContext, useContext, useState, type ReactNode } from "react";
import clsx from "clsx";
import { Eye, EyeOff } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";

/**
 * Whether the page is showing itself the way the person editing it sees it
 * (edit buttons, Connect prompts for platforms not yet linked, Choose file)
 * or the way a hiring team would (none of that — just what's actually there).
 *
 * A real toggle, not decoration: a hiring team never sees a "Connect your
 * GitHub" prompt for a platform that isn't linked, because there's nothing
 * to show them — so preview mode doesn't grey those out, it removes them,
 * the same as what a hiring team's own view would render.
 */
const PreviewModeContext = createContext(false);

export const usePreviewMode = () => useContext(PreviewModeContext);

export function PreviewModeProvider({ children }: { children: ReactNode }) {
  const [preview, setPreview] = useState(false);

  return (
    <PreviewModeContext.Provider value={preview}>
      <div className="mt-8 flex flex-wrap items-start justify-between gap-5 px-1">
        <div className="min-w-0">
          <h1 className={clsx(display.className, "text-[46px] leading-none font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[58px]")}>
            Your profile
          </h1>
          <p className="mt-3 text-[16px] text-[#5E5A82]">What hiring teams see alongside the evidence from your sessions.</p>
        </div>
        <button
          type="button"
          onClick={() => setPreview((v) => !v)}
          aria-pressed={preview}
          className={clsx(
            "inline-flex h-[50px] shrink-0 items-center gap-3 rounded-2xl px-5 text-[15px] font-medium transition-colors",
            preview
              ? "bg-[#1F1B4D] text-white"
              : "border border-white/90 bg-white/80 text-[#1F1B4D] shadow-[0_8px_22px_-14px_rgba(76,58,160,0.5)] hover:bg-white"
          )}
        >
          {preview ? <EyeOff size={19} strokeWidth={1.8} /> : <Eye size={19} strokeWidth={1.8} className="text-[#5B3BE0]" />}
          {preview ? "Back to editing" : "View as others see"}
        </button>
      </div>

      {preview && (
        <p className="mt-4 rounded-2xl bg-[#1F1B4D] px-5 py-3 text-[13px] text-[#D6D0F2]">
          This is what a hiring team sees — nothing you haven&apos;t added, no edit controls.
        </p>
      )}

      {children}
    </PreviewModeContext.Provider>
  );
}
