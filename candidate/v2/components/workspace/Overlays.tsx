"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import clsx from "clsx";
import { AlarmClock, CheckCircle2, Loader2, TriangleAlert, X } from "lucide-react";
import { idePalette, PRIMARY_BUTTON } from "@/lib/ide/palette";
import type { IdeTheme } from "@/lib/ide/theme";

/**
 * The v2 workspace's full-screen states and small dialogs. None of them use
 * the browser's own alert/confirm: in a fullscreen, proctored session those
 * take the page out of fullscreen, which is itself a flagged moment.
 */

/** A modal card on a dimmed backdrop. Escape and the backdrop dismiss it when `onDismiss` is given. */
function Modal({ theme, labelledBy, onDismiss, children }: { theme: IdeTheme; labelledBy: string; onDismiss?: () => void; children: React.ReactNode }) {
  const palette = idePalette(theme);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.focus();
    if (!onDismiss) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onDismiss();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onDismiss]);
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" onClick={onDismiss}>
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        className={clsx("w-full max-w-md rounded-2xl border p-6 shadow-2xl outline-none", palette.border, palette.appBg, palette.text)}
      >
        {children}
      </div>
    </div>
  );
}

/**
 * Time ran out. The workspace is locked from this moment — nothing typed now
 * would count — while the last copy is saved and the work handed in. When
 * that succeeds the page moves to the report; if it fails, the candidate is
 * told their work is saved, and can try again.
 */
export function TimeUpScreen({ theme, state, error, onRetry }: { theme: IdeTheme; state: "saving" | "failed"; error?: string; onRetry: () => void }) {
  const palette = idePalette(theme);
  return (
    <Modal theme={theme} labelledBy="time-up-title">
      <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#E06C75]/15 text-[#E06C75]">
        <AlarmClock size={24} />
      </span>
      <h2 id="time-up-title" className="mt-4 text-[20px] font-semibold">
        Time&apos;s up
      </h2>
      {state === "saving" ? (
        <p className={clsx("mt-2 flex items-center gap-2 text-[14px]", palette.textMuted)}>
          <Loader2 size={16} className="animate-spin" /> Saving and handing in your work…
        </p>
      ) : (
        <>
          <p className={clsx("mt-2 text-[14px] leading-relaxed", palette.textMuted)}>
            Your work is saved on our side and will be handed in even if this doesn&apos;t go through. {error}
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            <button type="button" onClick={onRetry} className={clsx("rounded-xl px-4 py-2 text-[14px] font-semibold", PRIMARY_BUTTON)}>
              Try again
            </button>
            <Link href="/dashboard" className={clsx("rounded-xl border px-4 py-2 text-[14px]", palette.border, palette.hover)}>
              Go to dashboard
            </Link>
          </div>
        </>
      )}
    </Modal>
  );
}

/** A practice run (or scratch workspace) finished: said plainly that nothing was sent anywhere. */
export function FinishedScreen({ theme, practice }: { theme: IdeTheme; practice: boolean }) {
  const palette = idePalette(theme);
  return (
    <div className={clsx("flex h-dvh w-full items-center justify-center p-6", palette.appBg)}>
      <div className={clsx("max-w-sm text-center", palette.text)}>
        <CheckCircle2 size={40} className="mx-auto text-emerald-400" />
        <h1 className="mt-4 text-[20px] font-semibold">{practice ? "Practice run finished" : "Workspace closed"}</h1>
        <p className={clsx("mt-2 text-[14px] leading-relaxed", palette.textMuted)}>
          {practice
            ? "Nothing was recorded or sent — a practice run has no session behind it. Your files stay in this browser if you open another."
            : "There was no session behind this workspace, so nothing was submitted."}
        </p>
        <Link href={practice ? "/practice" : "/dashboard"} className={clsx("mt-5 inline-flex rounded-xl px-4 py-2 text-[14px] font-semibold", PRIMARY_BUTTON)}>
          {practice ? "Back to practice" : "Back to dashboard"}
        </Link>
      </div>
    </div>
  );
}

/** Asking before something that can't be undone, in the workspace's own style. */
export function ConfirmDialog({
  theme,
  title,
  body,
  confirmLabel,
  danger,
  onConfirm,
  onCancel,
}: {
  theme: IdeTheme;
  title: string;
  body: string;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const palette = idePalette(theme);
  return (
    <Modal theme={theme} labelledBy="confirm-title" onDismiss={onCancel}>
      <h2 id="confirm-title" className="text-[17px] font-semibold">
        {title}
      </h2>
      <p className={clsx("mt-2 text-[14px] leading-relaxed", palette.textMuted)}>{body}</p>
      <div className="mt-5 flex justify-end gap-2">
        <button type="button" onClick={onCancel} className={clsx("rounded-xl px-4 py-2 text-[14px]", palette.textMuted, palette.hover)}>
          Cancel
        </button>
        <button
          type="button"
          onClick={onConfirm}
          className={clsx("rounded-xl px-4 py-2 text-[14px] font-semibold", danger ? "bg-[#DC2643] text-white hover:bg-[#C21F3A]" : PRIMARY_BUTTON)}
        >
          {confirmLabel}
        </button>
      </div>
    </Modal>
  );
}

/** A short message at the bottom of the screen, dismissible. Used for flagged moments and for explorer errors. */
export function Toast({ tone = "info", children, onDismiss }: { tone?: "info" | "warning"; children: React.ReactNode; onDismiss: () => void }) {
  return (
    <div
      role="status"
      className={clsx(
        "fixed bottom-14 left-1/2 z-40 flex max-w-lg -translate-x-1/2 items-start gap-2.5 rounded-xl px-4 py-2.5 text-[13px] shadow-lg",
        tone === "warning" ? "bg-[#3A1D24] text-[#FDE2E6]" : "bg-[#0F172A] text-[#E2E8F0] ring-1 ring-white/10"
      )}
    >
      {tone === "warning" && <TriangleAlert size={15} className="mt-0.5 shrink-0 text-[#F87182]" />}
      <span className="min-w-0 flex-1">{children}</span>
      <button type="button" onClick={onDismiss} aria-label="Dismiss" className="-mr-1 shrink-0 rounded p-0.5 opacity-70 hover:opacity-100">
        <X size={14} />
      </button>
    </div>
  );
}
