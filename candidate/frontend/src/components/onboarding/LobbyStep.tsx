"use client";

import { Clock, Terminal, Bot, GitBranch, ArrowLeft, ArrowRight } from "lucide-react";
import clsx from "clsx";
import { buttonClass } from "@/components/settings/Card";
import { display } from "@/lib/dashboard/fonts";

export interface LobbyAssessment {
  role: string;
  company: string;
  /** Short descriptors as they come off the assessment card — e.g. ["Bug Fix", "90 min", "Remote"]. Shown verbatim, nothing re-derived or guessed here. */
  tags: string[];
}

const RULES = [
  {
    icon: Clock,
    title: "Timer starts when you enter",
    body: "The countdown begins the moment you click \u201cEnter Workspace\u201d. The timer does not pause.",
  },
  {
    icon: Bot,
    title: "AI assistant is available",
    body: "You can use the built-in AI assistant to ask questions or think through approaches. How you use it is part of the evidence — use it naturally.",
  },
  {
    icon: Terminal,
    title: "Real environment",
    body: "You have a real terminal, a code editor, and a file explorer. Run tests, write files, use git — treat it like a real working environment.",
  },
  {
    icon: GitBranch,
    title: "Your work is captured continuously",
    body: "Every file change, terminal command, and test run is captured. The hiring team sees your process, not just the final state.",
  },
];

interface LobbyStepProps {
  assessment: LobbyAssessment;
  onBack: () => void;
  onEnter: () => void;
  pending: boolean;
  /** Set when a previous attempt to start the session failed — shown above the CTA so the candidate knows to retry rather than assume they're already in. */
  error?: string;
}

/**
 * Step 3: Instructions Lobby.
 *
 * The final screen before the clock starts. Shows the candidate what assessment
 * they are about to enter, the rules they need to know, and the "Enter Workspace"
 * button that fires the server action creating the live session.
 *
 * Deliberately calm and clear — the candidate should feel prepared, not pressured.
 */
export function LobbyStep({ assessment, onBack, onEnter, pending, error }: LobbyStepProps) {
  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <p className="flex items-center gap-4 text-[13px] font-medium tracking-[0.28em] text-[#6E6A8F] uppercase">
          Step 3 — Ready <span className="h-px w-9 bg-[#C9BCF7]" />
        </p>
        <h1 className={clsx(display.className, "mt-3 text-[34px] leading-[1.05] font-semibold tracking-tight text-[#100E2E] sm:text-[40px]")}>
          You&apos;re good to go
        </h1>
        <p className="mt-2.5 text-[15.5px] leading-relaxed text-[#5E5A82]">
          Read through what to expect, then enter the workspace when you&apos;re ready.
        </p>
      </div>

      {/* Assessment card */}
      <div className="relative isolate overflow-hidden rounded-[22px] bg-[#16143A] p-6 text-white shadow-[0_20px_44px_-24px_rgba(27,24,64,0.6)] sm:p-7">
        <div aria-hidden className="absolute inset-0 -z-10 bg-[radial-gradient(120%_140%_at_100%_100%,#3B2C86_0%,#221C58_38%,#16143A_70%)]" />
        <p className="flex items-center gap-4 text-[12.5px] font-medium tracking-[0.28em] text-[#D6D0F2] uppercase">
          Your assessment <span className="h-px w-9 bg-[#8C85B8]" />
        </p>
        <h2 className={clsx(display.className, "mt-3 text-[30px] leading-tight font-medium tracking-tight")}>{assessment.role}</h2>
        <p className="mt-1 text-[15px] text-[#E4E0F7]">{assessment.company}</p>
        <div className="mt-5 flex flex-wrap gap-2">
          {assessment.tags.map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-white/10 px-3.5 py-1.5 text-[13px] font-medium text-[#E4E0F7] ring-1 ring-white/15"
            >
              {tag}
            </span>
          ))}
        </div>
      </div>

      {/* Rules */}
      <div className="divide-y divide-[#ECE9F6] rounded-[22px] border border-white/90 bg-white/70 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)] backdrop-blur">
        {RULES.map(({ icon: Icon, title, body }) => (
          <div key={title} className="flex items-start gap-4 px-5 py-4">
            <div className="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#EFEAFD]">
              <Icon size={18} strokeWidth={1.8} className="text-[#5B3BE0]" />
            </div>
            <div>
              <p className="text-[15px] font-semibold text-[#100E2E]">{title}</p>
              <p className="mt-0.5 text-[14px] leading-relaxed text-[#5E5A82]">{body}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Starting a session makes its machine and puts the task on it, which
          takes around ten seconds. Said while it happens, so a candidate
          isn't left wondering whether the button worked — or whether the
          clock is already running against them. */}
      {pending && (
        <p role="status" className="rounded-2xl border border-[#E4E0F2] bg-white/80 px-5 py-3.5 text-center text-[14px] leading-relaxed text-[#2A2654]">
          Preparing your workspace — setting up your machine and putting the task on it. This usually takes 10–15 seconds, and your
          time starts only once the workspace opens.
        </p>
      )}

      {error && (
        <p className="rounded-2xl border border-[#F4D3DA] bg-[#FDF1F3] px-5 py-3.5 text-center text-[14px] font-medium text-[#a6203c]">
          {error}
        </p>
      )}

      {/* Final CTA note */}
      <p className="text-center text-[13.5px] text-[#5E5A82]">
        The timer starts when you click <strong className="text-[#100E2E]">Enter Workspace</strong>.
        Take a moment and enter only when you&apos;re ready.
      </p>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <button type="button" onClick={onBack} disabled={pending} className={clsx(buttonClass.quiet, "h-11 px-4 text-[14px]")}>
          <ArrowLeft size={16} />
          Back
        </button>
        {/* The one button that costs something to press — the timer starts
            here — so it gets the largest size on the flow. */}
        <button
          id="enter-workspace"
          type="button"
          onClick={onEnter}
          disabled={pending}
          className="inline-flex h-14 shrink-0 items-center justify-center gap-3 rounded-2xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-9 text-[17px] font-semibold text-white shadow-[0_14px_30px_-14px_rgba(91,59,224,0.8)] transition-[transform,opacity] hover:-translate-y-px disabled:cursor-wait disabled:opacity-60 disabled:hover:translate-y-0"
        >
          {pending ? "Preparing…" : "Enter Workspace"}
          {!pending && <ArrowRight size={18} />}
        </button>
      </div>
    </div>
  );
}
