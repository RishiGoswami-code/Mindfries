"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { CheckCircle2, Loader2, Send } from "lucide-react";
import { applyToJob } from "@/lib/jobs/actions";

const MAX_NOTE = 2000;

/**
 * Applying: an optional note to the hiring team, and the button. Their
 * profile — resume, linked accounts, preferences — goes with it; the panel
 * says so. Once applied, the candidate lands on the application.
 */
export function ApplyPanel({
  roleId,
  selfAssessment,
  appliedId,
}: {
  roleId: string;
  selfAssessment: boolean;
  /** The candidate's existing application for this job, if any. */
  appliedId: string | null;
}) {
  const router = useRouter();
  const [note, setNote] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();

  if (appliedId) {
    return (
      <div className="rounded-[20px] border border-white/90 bg-white/75 p-6">
        <p className="flex items-center gap-2 text-[16px] font-semibold text-[#14693a]">
          <CheckCircle2 size={20} /> You&apos;ve applied
        </p>
        <Link href={`/applications?id=${appliedId}`} className="mt-3 inline-block text-[14.5px] font-semibold text-[#5B3BE0] hover:underline">
          See your application →
        </Link>
      </div>
    );
  }

  return (
    <div className="rounded-[20px] border border-white/90 bg-white/75 p-6">
      <h2 className="text-[17px] font-semibold text-[#100E2E]">Apply</h2>
      <p className="mt-1 text-[13.5px] leading-relaxed text-[#5E5A82]">
        Your profile goes with it — resume, linked accounts and job preferences.{" "}
        {selfAssessment
          ? "This company opens its assessment to every applicant: it's prepared for you as soon as you apply."
          : "The hiring team reviews applications and invites the people they want to take the assessment."}
      </p>
      <label className="mt-4 block text-[13.5px] font-medium text-[#2A2654]">
        A note to the hiring team <span className="font-normal text-[#8A86A8]">(optional)</span>
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value.slice(0, MAX_NOTE))}
          rows={5}
          placeholder="Why this role, and anything your profile doesn't say."
          className="mt-1.5 w-full rounded-xl border border-[#DCD7EE] bg-white px-3 py-2.5 text-[14px] font-normal text-[#100E2E] outline-none focus:border-[#7C5CFC]"
        />
      </label>
      <p className="mt-1 text-right text-[12px] text-[#8A86A8]">
        {note.length}/{MAX_NOTE}
      </p>
      {error && <p className="mt-2 rounded-xl bg-[#FDF1F3] px-3 py-2 text-[13.5px] text-[#a6203c]">{error}</p>}
      <button
        type="button"
        disabled={pending}
        onClick={() =>
          start(async () => {
            setError(null);
            try {
              const res = await applyToJob(roleId, note);
              if (res.ok) router.push(`/applications?id=${res.applicationId}`);
              else setError(res.error);
            } catch {
              setError("Couldn't reach the server — check your connection and try again.");
            }
          })
        }
        className="mt-3 inline-flex h-12 w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] text-[15px] font-semibold text-white shadow-[0_10px_24px_-10px_rgba(91,59,224,0.8)] disabled:opacity-60"
      >
        {pending ? <Loader2 size={18} className="animate-spin" /> : <Send size={17} />}
        {pending ? "Applying…" : "Apply"}
      </button>
    </div>
  );
}
