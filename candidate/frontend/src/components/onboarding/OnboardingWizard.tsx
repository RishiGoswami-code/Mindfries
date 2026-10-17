"use client";

import { useState, useTransition } from "react";
import Link from "next/link";
import { ConsentStep } from "@/components/onboarding/ConsentStep";
import { DeviceCheckStep } from "@/components/onboarding/DeviceCheckStep";
import { LobbyStep, type LobbyAssessment } from "@/components/onboarding/LobbyStep";
import { enterWorkspace } from "@/app/onboarding/actions";
import { BrandMark } from "@/components/dashboard/CandidateSidebar";
import { ui } from "@/lib/dashboard/fonts";
import clsx from "clsx";

interface OnboardingWizardProps {
  templateId: string | null;
  assessment: LobbyAssessment | null;
}

/**
 * Multi-step onboarding wizard that every candidate passes through before
 * entering the Engineering Workspace (PRD §1.5):
 *
 *   1. Identity & Consent  — recording disclosure + agreement
 *   2. Device & Camera     — verify the hardware is ready before the timer starts
 *   3. Instructions Lobby  — task overview + final "Enter Workspace" CTA
 *
 * Only on Step 3 does the server action fire (creating the live session in the
 * database and redirecting to /ide). Nothing is recorded until that point.
 *
 * templateId and assessment come from the page's own server-side lookup of
 * whichever assessment the candidate picked on the dashboard (?template=) —
 * this component never has its own idea of which assessment is running.
 */
export function OnboardingWizard({ templateId, assessment }: OnboardingWizardProps) {
  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [isPending, startTransition] = useTransition();
  const [error, setError] = useState<string | undefined>();

  const handleEnterWorkspace = () => {
    if (!templateId) return;
    setError(undefined);
    startTransition(async () => {
      const result = await enterWorkspace(templateId);
      // A successful call never returns — it redirects. Reaching here means
      // it failed and returned an error to show instead.
      if (result?.error) setError(result.error);
    });
  };

  if (!templateId || !assessment) {
    return (
      <div className="space-y-4 rounded-2xl border border-[#B3CFE5] bg-white p-8 text-center">
        <h1 className="text-xl font-semibold tracking-tight text-[#0A1931]">No assessment selected</h1>
        <p className="text-sm text-[#4A7FA7]">
          Start an assessment from your dashboard first — this page needs to know which one you&apos;re entering.
        </p>
        <Link
          href="/dashboard"
          className="btn-wipe inline-flex items-center justify-center gap-2 px-5 py-2.5 text-[13px] font-semibold"
          style={{ "--btn-bg": "#4A7FA7", "--btn-fg": "#F6FAFD", "--btn-fill": "#1A3D63", "--btn-fg-hover": "#FFFFFF" } as React.CSSProperties}
        >
          Back to dashboard
        </Link>
      </div>
    );
  }

  return (
    <div className={clsx(ui.className, "min-h-screen bg-[#F7F8FC]")}>
      {/* Minimal top bar — no full nav, keeps focus on the flow */}
      <header className="border-b border-[#E6E9F2] bg-white/85 backdrop-blur">
        <div className="mx-auto flex h-[72px] max-w-[1560px] items-center gap-3 px-5 sm:px-10">
          <BrandMark size={32} />
          <span className="text-[21px] font-bold tracking-tight text-[#0B1033]">Mindfries</span>
          <span className="ml-auto text-[14px] text-[#5B6385] sm:text-[15px]">
            Step {step} of 3
          </span>
          <div className="hidden items-center gap-2 sm:flex" aria-hidden>
            {([1, 2, 3] as const).map((n) => (
              <span key={n} className={clsx("h-1.5 w-16 rounded-full transition-colors duration-300 md:w-28", step >= n ? "bg-[#3D5AF1]" : "bg-[#E1E5EF]")} />
            ))}
          </div>
        </div>
      </header>

      {/* Active step */}
      {/* The consent step is laid out wide; the device check and lobby keep their narrow column. */}
      <main className={clsx("mx-auto px-5 py-8 sm:px-10", step === 1 ? "max-w-[1560px] sm:py-12" : "max-w-2xl")}>
        {step === 1 && <ConsentStep onContinue={() => setStep(2)} />}
        {step === 2 && <DeviceCheckStep onBack={() => setStep(1)} onContinue={() => setStep(3)} />}
        {step === 3 && (
          <LobbyStep
            assessment={assessment}
            onBack={() => setStep(2)}
            onEnter={handleEnterWorkspace}
            pending={isPending}
            error={error}
          />
        )}
      </main>
    </div>
  );
}
