"use client";

import { useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import clsx from "clsx";
import { X } from "lucide-react";
import { display, ui } from "@/lib/dashboard/fonts";

const WIDTH = {
  md: "max-w-[680px]",
  lg: "max-w-[900px]",
  xl: "max-w-[1100px]",
};

/**
 * The window a profile card opens: icon, serif title, one line on what it
 * is, a close button, and the section itself.
 *
 * Portalled to <body> so it sits above everything — which also takes it out
 * of the page's font class, so it applies the UI font itself. Escape or a
 * click outside closes it; the page behind doesn't scroll while it's open,
 * and focus goes back to whatever opened it.
 */
export function ProfileDialog({
  icon,
  title,
  subtitle,
  size = "lg",
  footer,
  onClose,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  subtitle: string;
  size?: keyof typeof WIDTH;
  footer?: React.ReactNode;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const panel = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const close = useRef(onClose);
  useEffect(() => {
    close.current = onClose;
  });

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const { overflow } = document.body.style;
    document.body.style.overflow = "hidden";
    panel.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") close.current();
    }
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = overflow;
      opener?.focus?.();
    };
  }, []);

  return createPortal(
    <div
      className={clsx(ui.className, "fixed inset-0 z-50 flex items-end justify-center bg-[#1B1840]/35 p-0 backdrop-blur-[3px] sm:items-center sm:p-6")}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={clsx(
          "flex max-h-[92dvh] w-full flex-col overflow-hidden rounded-t-[24px] bg-[#FBFAFE] text-[#1B1840] shadow-[0_40px_90px_-30px_rgba(27,24,64,0.55)] outline-none sm:max-h-[88dvh] sm:rounded-[24px]",
          WIDTH[size]
        )}
      >
        <div className="flex items-start gap-4 px-5 pt-5 pb-4 sm:gap-5 sm:px-7 sm:pt-7">
          <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0] sm:h-[62px] sm:w-[62px]">{icon}</span>
          <div className="min-w-0 flex-1 pt-0.5">
            <h2 id={titleId} className={clsx(display.className, "text-[25px] leading-tight font-semibold tracking-[-0.01em] text-[#100E2E] sm:text-[29px]")}>
              {title}
            </h2>
            <p className="mt-1 text-[14.5px] leading-snug text-[#5E5A82]">{subtitle}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="-mt-1 -mr-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-[#2A2654] transition-colors hover:bg-[#EEEBFA]"
          >
            <X size={22} strokeWidth={1.8} />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-6 sm:px-7 sm:pb-7">{children}</div>

        {footer && <div className="border-t border-[#ECE9F6] bg-white/70 px-5 py-4 sm:px-7">{footer}</div>}
      </div>
    </div>,
    document.body
  );
}
