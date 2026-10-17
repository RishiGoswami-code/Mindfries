"use client";

import { Check } from "lucide-react";
import clsx from "clsx";
import { PLATFORM_ORDER, PLATFORMS } from "@/lib/profile/links";
import type { CandidateProfile } from "@/lib/profile/types";

/**
 * One number for how complete this profile is — the same kind of signal the
 * assessment notes already show as "92% match", turned around to describe
 * the candidate's own profile instead of a role.
 *
 * It's a checklist, not a black box: every factor behind the percentage is
 * named right below it, because a mystery score would work against the
 * product's whole premise — the PRD's own line is that the system should
 * show its work, not just render a verdict.
 */
export function ProfileStrength({ profile, hasResume }: { profile: CandidateProfile | null; hasResume: boolean }) {
  const bio = profile?.bio || "";
  const links = profile?.links || {};

  const items = [
    { label: "About you", done: bio.trim().length > 0 },
    { label: "Resume", done: hasResume },
    ...PLATFORM_ORDER.map((id) => ({ label: PLATFORMS[id].label, done: !!links[id] })),
  ];

  const done = items.filter((i) => i.done).length;
  const pct = Math.round((done / items.length) * 100);

  return (
    <section className="rounded-[20px] border border-white/90 bg-white/70 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-[20px] font-semibold text-[#100E2E]">Profile strength</h2>
        <span className="text-[20px] text-[#6E6A8F]">
          <span className="font-semibold tabular-nums text-[#5B3BE0]">{pct}%</span> complete
        </span>
      </div>

      <div
        className="mt-5 h-2 overflow-hidden rounded-full bg-[#E9E6F6]"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Profile strength"
      >
        <div
          className="h-full rounded-full bg-gradient-to-r from-[#5B3BE0] to-[#7C5CFC] transition-[width] duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>

      <ul className="mt-5 flex flex-wrap gap-x-12 gap-y-3">
        {items.map((item) => (
          <li key={item.label} className="flex items-center gap-3 text-[15px]">
            <span
              className={clsx(
                "flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-full",
                item.done ? "bg-[#5B3BE0] text-white" : "border-2 border-[#D4CEEA] bg-white"
              )}
            >
              {item.done && <Check size={14} strokeWidth={3} />}
            </span>
            <span className="text-[#1B1840]">{item.label}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
