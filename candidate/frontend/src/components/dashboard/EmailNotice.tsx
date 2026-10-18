"use client";

import { useState, useTransition } from "react";
import clsx from "clsx";
import { MailCheck, MailWarning, Send } from "lucide-react";
import { resendVerification } from "@/lib/auth/verify-actions";

/**
 * The strip at the top of the dashboard about the candidate's email: a
 * request to confirm it while it is unconfirmed, and the outcome of opening
 * a confirmation link.
 *
 * `canSend` is false when this site can't send mail; the strip then isn't
 * shown at all for an unconfirmed address, since there would be nothing the
 * candidate could do about it.
 */
export function EmailNotice({
  email,
  verified,
  canSend,
  outcome,
}: {
  email: string;
  verified: boolean;
  canSend: boolean;
  /** From the confirmation link: "confirmed", "link_expired", or nothing. */
  outcome?: string;
}) {
  const [message, setMessage] = useState<string | null>(null);
  const [pending, start] = useTransition();

  if (verified) {
    if (outcome !== "confirmed") return null;
    return (
      <div className={clsx(strip, "from-[#EAFAF6]/90 via-white/80 to-[#EEEBFD]/80")}>
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#D7F3EC] text-[#1E7A5A]">
          <MailCheck size={20} strokeWidth={1.8} />
        </span>
        <p className="min-w-0 flex-1 text-[14.5px] font-semibold text-[#100E2E]">Your email is confirmed.</p>
      </div>
    );
  }
  if (!canSend) return null;

  const expired = outcome === "link_expired";
  return (
    <div className={clsx(strip, "from-[#FFF7E8]/90 via-white/80 to-[#EFEAFD]/80")}>
      <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#FDECCB] text-[#B26A0E]">
        <MailWarning size={20} strokeWidth={1.8} />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-[14.5px] font-semibold text-[#100E2E]">{expired ? "Confirmation link expired" : "Confirm your email"}</p>
        <p className="mt-0.5 text-[13.5px] leading-snug text-[#5E5A82]">
          {expired
            ? "That confirmation link has expired or was already used."
            : `Confirm ${email} — hiring teams see whether a candidate's email is confirmed.`}
          {message && <span className="text-[#2A2654]"> {message}</span>}
        </p>
      </div>
      <button
        type="button"
        disabled={pending}
        onClick={() =>
          start(async () => {
            setMessage(await resendVerification());
          })
        }
        className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[#1F1B4D] px-4 py-2 text-[13px] font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
      >
        {pending ? "Sending…" : "Send the link"}
        {!pending && <Send size={13} strokeWidth={2} />}
      </button>
    </div>
  );
}

/* The dashboard's soft card: same radius, edge and shadow as the stat cards. */
const strip =
  "mb-6 flex flex-wrap items-center gap-x-4 gap-y-3 rounded-[22px] border border-white/90 bg-gradient-to-r px-5 py-4 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)]";
