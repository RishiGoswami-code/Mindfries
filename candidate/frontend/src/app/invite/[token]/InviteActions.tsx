"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { beginFromInvite } from "./actions";
import type { InviteTaskStatus } from "@/lib/invite-link";

/**
 * Start, once the candidate's own task exists. Until then the page says it
 * is being made and looks again every few seconds, so a candidate who opens
 * the link straight after being invited doesn't have to keep reloading.
 */
export function InviteActions({ token, taskStatus }: { token: string; taskStatus: InviteTaskStatus }) {
  const router = useRouter();
  const [pending, start] = useTransition();
  const [error, setError] = useState<string | null>(null);
  const preparing = taskStatus === "pending" || taskStatus === "generating";

  useEffect(() => {
    if (!preparing) return;
    const timer = setInterval(() => router.refresh(), 8000);
    return () => clearInterval(timer);
  }, [preparing, router]);

  if (preparing) {
    return (
      <p role="status" className="rounded-xl border border-[#B3CFE5] bg-[#F6FAFD] px-4 py-3 text-[13.5px] leading-relaxed text-[#1A3D63]">
        Your assessment is being prepared — a task is being written just for you. It usually takes a minute or two; this page will update by itself.
      </p>
    );
  }
  if (taskStatus === "failed") {
    return (
      <p role="status" className="rounded-xl border border-[#E4A0A0] bg-[#FBEAEA] px-4 py-3 text-[13.5px] leading-relaxed text-[#8A2C2C]">
        Your assessment couldn&apos;t be prepared. The team that invited you can see this and try again — there&apos;s nothing you need to do.
      </p>
    );
  }
  return (
    <div>
      <button
        type="button"
        disabled={pending}
        onClick={() =>
          start(async () => {
            // Success ends in a redirect to the onboarding steps; only a
            // refusal comes back.
            const result = await beginFromInvite(token);
            if (result?.error) setError(result.error);
          })
        }
        className="flex w-full items-center justify-center gap-1.5 rounded-xl bg-[#0A1931] px-4 py-2.5 text-[13px] font-semibold text-white hover:opacity-90 disabled:opacity-60"
      >
        {pending ? "Opening…" : "Start your assessment"} {!pending && <ArrowRight size={14} />}
      </button>
      {error && <p className="mt-2 text-[12.5px] text-[#a6203c]">{error}</p>}
    </div>
  );
}
