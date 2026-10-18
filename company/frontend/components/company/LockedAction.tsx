"use client";

import { useEffect, useRef, useState } from "react";
import { Lock, X } from "lucide-react";
import { RemindButton } from "./RemindButton";

/**
 * A hiring action the company can't take yet (New role, while access is
 * pending). Looks like the button it replaces, with a lock, and explains
 * instead of navigating: why it's locked, and the one thing to do about it.
 */
export function LockedAction({ label, reason, size = "sm" }: { label: string; reason: string; size?: "sm" | "md" }) {
  const [open, setOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  const pad = size === "sm" ? "px-4 py-2 text-[13px]" : "px-6 py-3 text-[15px]";
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={`inline-flex items-center justify-center gap-2 rounded-[15px] bg-[#ece9f6] font-extrabold text-dim ${pad}`}
        aria-haspopup="dialog"
      >
        <Lock size={14} strokeWidth={2.2} aria-hidden />
        {label}
      </button>

      <dialog
        ref={dialog}
        onClose={() => setOpen(false)}
        aria-labelledby="locked-title"
        className="m-auto w-[min(520px,calc(100vw-2rem))] rounded-[26px] bg-[#FBFAFE] p-0 text-ink shadow-[0_40px_90px_-30px_rgba(27,24,64,0.6)] backdrop:bg-[#1B1840]/40 backdrop:backdrop-blur-[3px]"
      >
        <div className="flex flex-col gap-5 p-7">
          <div className="flex items-start gap-4">
            <span className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-accent-soft text-accent">
              <Lock size={22} strokeWidth={1.8} aria-hidden />
            </span>
            <div className="min-w-0 flex-1">
              <h2 id="locked-title" className="text-[22px] font-extrabold tracking-tight">
                Credits needed
              </h2>
              <p className="mt-1 text-[14.5px] leading-relaxed text-dim">{reason}</p>
            </div>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Close"
              className="grid h-9 w-9 shrink-0 place-items-center rounded-full text-dim transition hover:bg-black/[0.05] hover:text-ink"
            >
              <X size={18} aria-hidden />
            </button>
          </div>
          <RemindButton />
        </div>
      </dialog>
    </>
  );
}
