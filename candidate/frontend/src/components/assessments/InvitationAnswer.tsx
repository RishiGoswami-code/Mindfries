"use client";

import { useState, useTransition } from "react";
import clsx from "clsx";
import { Check } from "lucide-react";
import { acceptInvitation, declineInvitation } from "@/lib/invitations";

/**
 * Accept or decline, on an invitation the candidate hasn't answered.
 *
 * Declining asks once more, in place, and offers a line for a reason — it
 * closes the invitation for good, and the reason (if any) goes to the
 * company, which the form says before anything is sent.
 */
export function InvitationAnswer({ assessmentId }: { assessmentId: string }) {
  const [declining, setDeclining] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  const run = (action: () => Promise<{ ok: true } | { ok: false; error: string }>) =>
    start(async () => {
      try {
        const result = await action();
        setError(result.ok ? null : result.error);
      } catch {
        // The request never arrived, so the invitation is as it was.
        setError("Couldn't reach the server — check your connection and try again.");
      }
    });

  if (declining) {
    return (
      <div className="w-full rounded-2xl border border-[#F4D3DA] bg-[#FDF4F6] p-3.5">
        <label className="block text-[13px] font-medium text-[#2A2654]">
          Decline this invitation? It can&apos;t be reopened from here.
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={500}
            placeholder="Reason (optional — the company sees it)"
            className="mt-2 h-10 w-full rounded-xl border border-[#E4E0F2] bg-white px-3.5 text-[13.5px] text-[#100E2E] outline-none placeholder:text-[#8A86A8] focus:border-[#F0A8B6]"
          />
        </label>
        <div className="mt-3 flex items-center justify-end gap-2">
          <button type="button" onClick={() => setDeclining(false)} disabled={pending} className={clsx(pill, "bg-[#EEEBFA] text-[#3D22A8]")}>
            Keep it
          </button>
          <button
            type="button"
            disabled={pending}
            onClick={() => run(() => declineInvitation(assessmentId, reason))}
            className={clsx(pill, "bg-[#DC2643] text-white")}
          >
            {pending ? "Declining…" : "Decline"}
          </button>
        </div>
        {error && <p className="mt-2 text-right text-[12.5px] text-[#a6203c]">{error}</p>}
      </div>
    );
  }

  return (
    <div className="flex shrink-0 flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => setDeclining(true)}
          disabled={pending}
          className={clsx(pill, "text-[#5E5A82] hover:bg-[#FDEEF1] hover:text-[#a6203c] hover:opacity-100")}
        >
          Decline
        </button>
        <button
          type="button"
          disabled={pending}
          onClick={() => run(() => acceptInvitation(assessmentId))}
          className={clsx(pill, "bg-[#1F1B4D] text-white")}
        >
          {pending ? "Saving…" : "Accept"}
          {!pending && <Check size={14} strokeWidth={2.4} />}
        </button>
      </div>
      {error && <p className="text-[12.5px] text-[#a6203c]">{error}</p>}
    </div>
  );
}

/* Same pill as the Start / Resume actions beside it (AssessmentItem's ActionLink). */
const pill =
  "inline-flex shrink-0 items-center gap-1.5 rounded-full px-4 py-2 text-[13px] font-semibold transition-opacity hover:opacity-90 disabled:opacity-50";
