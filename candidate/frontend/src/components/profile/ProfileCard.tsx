"use client";

import { createContext, useContext, useState } from "react";
import clsx from "clsx";
import { ChevronRight } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";
import type { SectionId } from "@/lib/profile/completion";
import { ProfileDialog } from "./ProfileDialog";
import { usePreviewMode } from "./PreviewMode";

type CardId = SectionId | "completion";

const OpenSection = createContext<{ open: CardId | null; setOpen: (id: CardId | null) => void }>({
  open: null,
  setOpen: () => {},
});

/** Which card's window is open — one at a time, and any card can open another's (the completion checklist does). */
export function ProfileSections({ initial = null, children }: { initial?: CardId | null; children: React.ReactNode }) {
  const [open, setOpen] = useState<CardId | null>(initial);
  return <OpenSection.Provider value={{ open, setOpen }}>{children}</OpenSection.Provider>;
}

export const useProfileSections = () => useContext(OpenSection);

/**
 * One section of the profile as a card — icon, serif title, a summary
 * worked out from the section's own data — that opens the section's window
 * (ProfileDialog) with everything it does.
 *
 * `hideInPreview` drops the card from "View as others see": guidance meant
 * for the candidate, or a section with nothing in it yet.
 */
export function ProfileCard({
  id,
  icon,
  title,
  summary,
  action,
  dialog,
  hideInPreview = false,
  className,
  children,
}: {
  id: CardId;
  icon: React.ReactNode;
  title: string;
  summary: React.ReactNode;
  /** A small button beside the chevron, e.g. opening the resume file. */
  action?: React.ReactNode;
  dialog: { title?: string; subtitle: string; size?: "md" | "lg" | "xl" };
  hideInPreview?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  const { open, setOpen } = useProfileSections();
  const preview = usePreviewMode();
  if (preview && hideInPreview) return null;

  return (
    <>
      <section
        className={clsx(
          "group relative flex min-w-0 items-center gap-4 rounded-[20px] border border-white/90 bg-white/75 p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur transition-shadow hover:shadow-[0_16px_36px_-22px_rgba(76,58,160,0.55)] sm:gap-6 sm:p-6",
          className
        )}
      >
        <span className="flex h-14 w-14 shrink-0 items-center justify-center self-start rounded-2xl bg-[#EFEAFD] text-[#5B3BE0] sm:h-[68px] sm:w-[68px]">
          {icon}
        </span>

        <div className="min-w-0 flex-1">
          <h2 className={clsx(display.className, "text-[22px] leading-tight font-semibold tracking-[-0.01em] text-[#100E2E] sm:text-[24px]")}>
            {/* The whole card is the click target; this button is what keyboards and screen readers reach. */}
            <button
              type="button"
              onClick={() => setOpen(id)}
              aria-haspopup="dialog"
              className="text-left after:absolute after:inset-0 after:rounded-[20px] after:content-[''] focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-[#7C5CFC]"
            >
              {title}
            </button>
          </h2>
          <div className="mt-1 text-[15px] text-[#5E5A82]">{summary}</div>
        </div>

        {action && <div className="relative z-10 shrink-0">{action}</div>}
        <ChevronRight size={22} strokeWidth={2} className="shrink-0 text-[#1B1840] transition-transform group-hover:translate-x-0.5" aria-hidden />
      </section>

      {open === id && (
        <ProfileDialog icon={icon} title={dialog.title ?? title} subtitle={dialog.subtitle} size={dialog.size} onClose={() => setOpen(null)}>
          {children}
        </ProfileDialog>
      )}
    </>
  );
}

/** The small grey pills under a card's summary. */
export function Chips({ items, more = 0 }: { items: string[]; more?: number }) {
  if (items.length === 0) return null;
  return (
    <div className="mt-3 flex flex-wrap gap-2">
      {items.map((item) => (
        <span key={item} className="max-w-full truncate rounded-xl bg-[#F1EFF8] px-3 py-1 text-[13.5px] text-[#2A2654]">
          {item}
        </span>
      ))}
      {more > 0 && <span className="rounded-xl bg-[#F1EFF8] px-3 py-1 text-[13.5px] text-[#2A2654]">+{more}</span>}
    </div>
  );
}

/** A status pill: done, under way, or still to do. */
export function StatePill({ state, children }: { state: "done" | "in-progress" | "todo" | "info"; children: React.ReactNode }) {
  return (
    <span
      className={clsx(
        "inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1 text-[12.5px] font-medium whitespace-nowrap",
        state === "done" && "bg-[#E3F6EC] text-[#14693a]",
        state === "in-progress" && "bg-[#FFF3D6] text-[#8A5A00]",
        state === "todo" && "bg-[#EEEBFA] text-[#4B2BB8]",
        state === "info" && "bg-[#F1EFF8] text-[#4C4874]"
      )}
    >
      {children}
    </span>
  );
}
