"use client";

import { useId, useState } from "react";
import clsx from "clsx";
import { ChevronRight } from "lucide-react";
import { usePreviewMode } from "./PreviewMode";

export interface TileStatus {
  label: string;
  /** green: something is there; grey: nothing yet; badge: a count to act on. */
  tone: "green" | "grey" | "badge";
}

/**
 * One section of the profile, collapsed to a summary line — icon, title,
 * what's in it, a status — that opens in place to the section itself
 * (ResumeUpload, LinkedAccounts, KnowledgeBase, EvidenceSummary,
 * NextSteps). Closed, the page reads as an overview; open, everything the
 * section did before is there, unchanged.
 *
 * `hideInPreview` is for sections a hiring team wouldn't see at all
 * ("View as others see"): guidance for the candidate, or a section with
 * nothing in it yet.
 */
export function ProfileTile({
  icon,
  title,
  subtitle,
  previewSubtitle,
  status,
  action,
  defaultOpen = false,
  hideInPreview = false,
  className,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  /** What the summary says in "View as others see", when the usual line is an instruction to the candidate. */
  previewSubtitle?: string;
  status?: TileStatus;
  /** A small button beside the status, e.g. opening the resume file. */
  action?: React.ReactNode;
  defaultOpen?: boolean;
  hideInPreview?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const preview = usePreviewMode();
  const panelId = useId();

  if (preview && hideInPreview) return null;

  return (
    <section
      className={clsx(
        "min-w-0 rounded-[20px] border border-white/90 bg-white/70 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur",
        className
      )}
    >
      <div className="flex items-center gap-3 p-4 sm:gap-5 sm:p-5">
        <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0] sm:h-[58px] sm:w-[58px]">{icon}</span>

        <div className="min-w-0 flex-1">
          <h2 className="text-[17px] font-semibold text-[#100E2E]">
            <button type="button" onClick={() => setOpen((v) => !v)} aria-expanded={open} aria-controls={panelId} className="text-left hover:underline">
              {title}
            </button>
          </h2>
          <p className="mt-0.5 truncate text-[14px] text-[#6E6A8F]">{preview && previewSubtitle ? previewSubtitle : subtitle}</p>
        </div>

        {status && (
          <span
            className={clsx(
              "hidden shrink-0 items-center gap-2 text-[13.5px] sm:inline-flex",
              status.tone === "badge" ? "rounded-xl bg-[#EEEBFA] px-4 py-2 text-[#3D3A63]" : "text-[#4C4874]"
            )}
          >
            {status.tone !== "badge" && (
              <span className={clsx("h-2 w-2 rounded-full", status.tone === "green" ? "bg-[#1A9E6B]" : "bg-[#9C99B3]")} />
            )}
            {status.label}
          </span>
        )}

        {action}

        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-controls={panelId}
          aria-label={open ? `Close ${title}` : `Open ${title}`}
          tabIndex={-1}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-[#1B1840] transition-colors hover:bg-[#F1EEFA]"
        >
          <ChevronRight size={20} strokeWidth={2} className={clsx("transition-transform", open && "rotate-90")} />
        </button>
      </div>

      {open && (
        <div id={panelId} className="border-t border-[#ECE9F6] px-5 pt-4 pb-5">
          {children}
        </div>
      )}
    </section>
  );
}
