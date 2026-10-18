"use client";

import { useActionState } from "react";
import { BellRing, Check } from "lucide-react";
import { remindTeam, type RemindState } from "@/app/(portal)/access-actions";

/** "Remind the Mindfries team" — the one thing a company without access can do about it. */
export function RemindButton({ alreadyAt, label = "Remind the Mindfries team" }: { alreadyAt?: string | null; label?: string }) {
  const [state, action, pending] = useActionState<RemindState, FormData>(remindTeam, { sent: false, error: null });

  if (state.sent) {
    return (
      <p role="status" className="inline-flex items-center gap-2 rounded-xl bg-[#EAFAF6] px-3.5 py-2 text-[13.5px] font-medium text-[#1E7A5A]">
        <Check size={15} strokeWidth={2.4} aria-hidden />
        We&apos;ve let the team know. You&apos;ll get an email when credits are added.
      </p>
    );
  }

  return (
    <form action={action} className="flex flex-wrap items-center gap-3">
      <button
        type="submit"
        disabled={pending}
        className="inline-flex h-10 items-center gap-2 rounded-xl bg-gradient-to-r from-[#8b5cf6] to-accent px-4 text-[13.5px] font-semibold text-white shadow-[0_10px_24px_-12px_rgba(124,58,237,0.8)] disabled:opacity-60"
      >
        <BellRing size={15} aria-hidden />
        {pending ? "Sending…" : label}
      </button>
      {alreadyAt && <span className="text-[12.5px] text-dim">Last reminder {new Date(alreadyAt).toLocaleDateString("en-GB", { day: "numeric", month: "short" })}</span>}
      {state.error && <span className="text-[12.5px] text-[#a6203c]">{state.error}</span>}
    </form>
  );
}
