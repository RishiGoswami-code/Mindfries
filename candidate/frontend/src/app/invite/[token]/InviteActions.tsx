"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight, Loader2 } from "lucide-react";
import { authSubmit } from "@/components/login/AuthShell";
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
      <p role="status" className="flex items-start gap-3 rounded-2xl border border-[#E4E0F2] bg-[#F6F4FC] px-4 py-3.5 text-[14px] leading-relaxed text-[#2A2654]">
        <Loader2 size={17} className="mt-0.5 shrink-0 animate-spin text-[#5B3BE0]" />
        <span>
          Your assessment is being prepared — a task is being written just for you. It usually takes a minute or two; this page will update by itself.
        </span>
      </p>
    );
  }
  if (taskStatus === "failed") {
    return (
      <p role="status" className="rounded-2xl border border-[#F4D3DA] bg-[#FDF1F3] px-4 py-3.5 text-[14px] leading-relaxed text-[#a6203c]">
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
        className={authSubmit}
      >
        {pending ? "Opening…" : "Start your assessment"} {!pending && <ArrowRight size={16} />}
      </button>
      {error && <p className="mt-2.5 text-center text-[13.5px] text-[#a6203c]">{error}</p>}
    </div>
  );
}
