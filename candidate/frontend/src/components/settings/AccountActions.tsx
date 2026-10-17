"use client";

import { useActionState, useEffect, useRef, useState, useTransition } from "react";
import { createPortal } from "react-dom";
import clsx from "clsx";
import { Download, LogOut, Trash2, X } from "lucide-react";
import { signOutEverywhere } from "@/app/login/actions";
import { deleteAccount, type DeleteState } from "@/lib/settings/actions";
import { DELETE_PHRASE } from "@/lib/settings/shape";
import { display, ui } from "@/lib/dashboard/fonts";
import { SettingsRow, buttonClass } from "./Card";

/** "Download my data": a plain link — the route answers with the file. */
export function DownloadDataRow() {
  return (
    <SettingsRow icon={<Download size={19} />} label="Download my data" detail="Everything we hold about your account, as one JSON file.">
      <a href="/api/account/export" download className={buttonClass.quiet}>
        <Download size={15} /> Download
      </a>
    </SettingsRow>
  );
}

/** Withdraws every session the account has, this one included (lib/auth/revocation.ts). */
export function SignOutEverywhereRow() {
  const [confirming, setConfirming] = useState(false);
  const [pending, start] = useTransition();
  return (
    <SettingsRow
      icon={<LogOut size={19} />}
      label="Sign out of all devices"
      detail={confirming ? "Every browser and device signed in to your account is signed out, this one too." : "For a shared computer you left signed in, or a session you don't recognise."}
    >
      {confirming ? (
        <>
          <button type="button" className={buttonClass.quiet} onClick={() => setConfirming(false)} disabled={pending}>
            Cancel
          </button>
          <button type="button" className={clsx(buttonClass.quiet, "border-[#F4D3DA] text-[#a6203c]")} disabled={pending} onClick={() => start(() => void signOutEverywhere())}>
            {pending ? "Signing out…" : "Sign out everywhere"}
          </button>
        </>
      ) : (
        <button type="button" className={buttonClass.quiet} onClick={() => setConfirming(true)}>
          Sign out everywhere
        </button>
      )}
    </SettingsRow>
  );
}

/** The red row, and the window that asks for the phrase before anything is deleted. */
export function DeleteAccountRow() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <div className="mt-2 flex flex-col gap-3 rounded-2xl border border-[#F7D5DC] bg-[#FDF1F3] p-4 sm:flex-row sm:items-center">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white text-[#DC2643]">
          <Trash2 size={19} />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[14.5px] font-semibold text-[#B01D38]">Delete account</p>
          <p className="mt-0.5 text-[13px] leading-snug text-[#7A4A55]">Permanently deletes your account, profile and files. This can&apos;t be undone.</p>
        </div>
        <button type="button" className={clsx(buttonClass.danger, "shrink-0")} onClick={() => setOpen(true)}>
          Delete account
        </button>
      </div>
      {open && createPortal(<DeleteDialog onClose={() => setOpen(false)} />, document.body)}
    </>
  );
}

function DeleteDialog({ onClose }: { onClose: () => void }) {
  const [state, action, pending] = useActionState<DeleteState, FormData>(deleteAccount, { error: null });
  const [typed, setTyped] = useState("");
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    input.current?.focus();
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && !pending && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose, pending]);

  const matches = typed.trim().toLowerCase() === DELETE_PHRASE;

  return (
    <div className={clsx(ui.className, "fixed inset-0 z-50 flex items-center justify-center bg-[#1B1840]/40 p-4 backdrop-blur-[3px]")} onClick={() => !pending && onClose()}>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="delete-title"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-[520px] rounded-[24px] bg-[#FBFAFE] p-6 text-[#1B1840] shadow-[0_40px_90px_-30px_rgba(27,24,64,0.55)] sm:p-7"
      >
        <div className="flex items-start gap-4">
          <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#FDE8EC] text-[#DC2643]">
            <Trash2 size={22} />
          </span>
          <div className="min-w-0 flex-1">
            <h2 id="delete-title" className={clsx(display.className, "text-[24px] leading-tight font-semibold text-[#100E2E]")}>
              Delete your account?
            </h2>
            <p className="mt-1 text-[14px] text-[#5E5A82]">This is permanent. There is no undo and no copy kept.</p>
          </div>
          <button type="button" onClick={onClose} disabled={pending} aria-label="Close" className="-mt-1 -mr-1 flex h-9 w-9 items-center justify-center rounded-full hover:bg-[#EEEBFA]">
            <X size={20} />
          </button>
        </div>

        <div className="mt-5 grid gap-3 text-[13.5px] leading-snug sm:grid-cols-2">
          <div className="rounded-2xl bg-white p-4">
            <p className="font-semibold text-[#100E2E]">Deleted</p>
            <p className="mt-1 text-[#5E5A82]">Your account and sign-ins, profile, resume, documents, job preferences, linked accounts and what was read from them, and your settings.</p>
          </div>
          <div className="rounded-2xl bg-white p-4">
            <p className="font-semibold text-[#100E2E]">Kept by the companies</p>
            <p className="mt-1 text-[#5E5A82]">Applications and assessments you did for a company stay as that company&apos;s records — without your profile behind them.</p>
          </div>
        </div>

        <p className="mt-4 text-[13.5px] text-[#4C4874]">
          Want a copy first?{" "}
          <a href="/api/account/export" download className="font-medium text-[#5B3BE0] hover:underline">
            Download your data
          </a>
          .
        </p>

        <form action={action} className="mt-5">
          <label htmlFor="delete-confirm" className="text-[13.5px] text-[#1B1840]">
            Type <span className="font-semibold">{DELETE_PHRASE}</span> to confirm
          </label>
          <input
            ref={input}
            id="delete-confirm"
            name="confirm"
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            className="mt-2 h-11 w-full rounded-xl border border-[#E4E0F2] bg-white px-3.5 text-[14.5px] outline-none focus:border-[#F0A8B6]"
          />
          {state.error && (
            <p role="alert" className="mt-2 text-[13px] text-[#a6203c]">
              {state.error}
            </p>
          )}
          <div className="mt-5 flex justify-end gap-2">
            <button type="button" onClick={onClose} disabled={pending} className={buttonClass.quiet + " h-10"}>
              Cancel
            </button>
            <button type="submit" disabled={!matches || pending} className={buttonClass.danger}>
              {pending ? "Deleting…" : "Delete my account"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
