"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowUpRight, ShieldCheck } from "lucide-react";
import { resources } from "@/lib/dashboard/data";
import { PoliciesModal } from "./PoliciesModal";

/**
 * Intervue's right-hand resources column, minus the blog carousel.
 *
 * Theirs rotates generic articles — "What is full-stack Development" — next to
 * a dashboard about your own interviews. Nothing there helps the person
 * reading it. These three are about *this* process: what the workspace
 * records, how the reasoning prompts are read, and what to do before a first
 * session.
 *
 * The card above them is the one thing a candidate on a proctored platform is
 * entitled to have in plain sight rather than buried in a policy page.
 *
 * Previously both the "Read the full list" button and the resource rows were
 * dead — styled as interactive but with no onClick or href. Now:
 *   - "Read the full list" and the "What the workspace records" row open
 *     PoliciesModal (the same one in AccountMenu).
 *   - The other two resource rows link to /practice, the honest target for
 *     "set up your camera" and "try the workspace" guidance.
 */

export function SideRail() {
  const [policiesOpen, setPoliciesOpen] = useState(false);

  return (
    <>
      <aside className="space-y-4">
        <section className="rounded-2xl border border-[#B3CFE5] bg-[#0A1931] p-5 text-[#F6FAFD]">
          <span className="flex h-9 w-9 items-center justify-center rounded-xl bg-[#F6FAFD]/10">
            <ShieldCheck size={17} className="text-[#B3CFE5]" />
          </span>
          <h2 className="mt-3 text-sm font-semibold">What gets recorded</h2>
          <p className="mt-1.5 text-[13px] leading-relaxed text-[#B3CFE5]">
            During a session: your camera, your screen (as stills, not video), sounds that last a
            few seconds, the commands you run, the changes you make, and what you ask the AI
            assistant. Moments like leaving the workspace are flagged for the hiring team to look
            at. Never outside a session, and never on a practice run.
          </p>
          <button
            type="button"
            onClick={() => setPoliciesOpen(true)}
            className="mt-3 inline-flex items-center gap-1 text-[13px] font-medium text-[#B3CFE5] transition-colors hover:text-[#F6FAFD]"
          >
            Read the full list
            <ArrowUpRight size={13} />
          </button>
        </section>

        <section className="rounded-2xl border border-[#B3CFE5] bg-white p-1">
          <h2 className="px-4 pt-3 pb-2 text-sm font-semibold text-[#0A1931]">Before you start</h2>
          <ul>
            {resources.map((resource) =>
              resource.action === "policies" ? (
                <li key={resource.title}>
                  <button
                    type="button"
                    onClick={() => setPoliciesOpen(true)}
                    className="group flex w-full items-start gap-2 rounded-xl px-4 py-3 text-left transition-colors hover:bg-[#B3CFE5]/25"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-[13.5px] font-medium text-[#0A1931]">
                        {resource.title}
                      </span>
                      <span className="mt-0.5 block text-xs leading-relaxed text-[#4A7FA7]">
                        {resource.body}
                      </span>
                    </span>
                    <ArrowUpRight
                      size={14}
                      className="mt-0.5 shrink-0 text-[#B3CFE5] transition-colors group-hover:text-[#4A7FA7]"
                    />
                  </button>
                </li>
              ) : (
                <li key={resource.title}>
                  <Link
                    href={resource.href ?? "/practice"}
                    className="group flex items-start gap-2 rounded-xl px-4 py-3 transition-colors hover:bg-[#B3CFE5]/25"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-[13.5px] font-medium text-[#0A1931]">
                        {resource.title}
                      </span>
                      <span className="mt-0.5 block text-xs leading-relaxed text-[#4A7FA7]">
                        {resource.body}
                      </span>
                    </span>
                    <ArrowUpRight
                      size={14}
                      className="mt-0.5 shrink-0 text-[#B3CFE5] transition-colors group-hover:text-[#4A7FA7]"
                    />
                  </Link>
                </li>
              ),
            )}
          </ul>
        </section>
      </aside>

      {policiesOpen && <PoliciesModal onClose={() => setPoliciesOpen(false)} />}
    </>
  );
}
