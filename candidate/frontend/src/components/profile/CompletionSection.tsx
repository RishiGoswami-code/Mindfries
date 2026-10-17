"use client";

import { Award, Briefcase, ChevronRight, CheckCircle2, Clock3, FileText, Layers, Link2, UserRound } from "lucide-react";
import type { Completion, SectionId } from "@/lib/profile/completion";
import { EDIT_PROFILE_EVENT } from "./EditProfileModal";
import { StatePill, useProfileSections } from "./ProfileCard";

const ICONS: Record<SectionId, React.ReactNode> = {
  resume: <FileText size={22} strokeWidth={1.7} />,
  details: <UserRound size={22} strokeWidth={1.7} />,
  linked: <Link2 size={22} strokeWidth={1.7} />,
  projects: <Layers size={22} strokeWidth={1.7} />,
  evidence: <Award size={22} strokeWidth={1.7} />,
  preferences: <Briefcase size={22} strokeWidth={1.7} />,
};

/**
 * The Profile completion window: the progress bar, and one row per
 * section with where it stands. Each row opens that section — the others'
 * windows, or Edit profile for the details — so the checklist is the way
 * to finish, not just a report on it.
 */
export function CompletionSection({ completion }: { completion: Completion }) {
  const { setOpen } = useProfileSections();

  function go(id: SectionId) {
    if (id === "details") {
      setOpen(null);
      window.dispatchEvent(new Event(EDIT_PROFILE_EVENT));
    } else setOpen(id);
  }

  return (
    <div>
      <div className="flex items-baseline justify-between gap-3 px-1">
        <p className="text-[17px] font-semibold text-[#5B3BE0]">{completion.percent}% complete</p>
        <p className="text-[14.5px] text-[#4C4874]">
          {completion.done} of {completion.total} sections finished
        </p>
      </div>
      <div
        className="mt-3 h-2.5 overflow-hidden rounded-full bg-[#E9E6F6]"
        role="progressbar"
        aria-valuenow={completion.percent}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Profile completion"
      >
        <div className="h-full rounded-full bg-gradient-to-r from-[#5B3BE0] to-[#7C5CFC]" style={{ width: `${completion.percent}%` }} />
      </div>

      <ul className="mt-6 divide-y divide-[#F0EEF7] rounded-[18px] border border-[#ECE9F6] bg-white px-4 sm:px-5">
        {completion.sections.map((s) => (
          <li key={s.id}>
            <button type="button" onClick={() => go(s.id)} className="group flex w-full items-center gap-4 py-4 text-left">
              <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#F1EDFC] text-[#5B3BE0]">{ICONS[s.id]}</span>
              <span className="min-w-0 flex-1">
                <span className="block text-[16px] font-semibold text-[#100E2E] group-hover:underline">{s.title}</span>
                <span className="block text-[13.5px] text-[#6E6A8F]">{s.description}</span>
              </span>
              {s.state === "done" ? (
                <StatePill state="done">
                  <CheckCircle2 size={15} /> Completed
                </StatePill>
              ) : s.state === "in-progress" ? (
                <StatePill state="in-progress">
                  <Clock3 size={15} /> In progress
                </StatePill>
              ) : (
                <span className="shrink-0 rounded-lg bg-[#EEEBFA] px-4 py-1.5 text-[13.5px] font-semibold text-[#4B2BB8]">Complete</span>
              )}
              <ChevronRight size={19} className="shrink-0 text-[#1B1840]" />
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
