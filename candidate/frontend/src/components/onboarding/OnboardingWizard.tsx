"use client";

import { useState, useTransition } from "react";
import Link from "next/link";
import { ArrowLeft, ClipboardList } from "lucide-react";
import { ConsentStep } from "@/components/onboarding/ConsentStep";
import { DeviceCheckStep } from "@/components/onboarding/DeviceCheckStep";
import { LobbyStep, type LobbyAssessment } from "@/components/onboarding/LobbyStep";
import { enterWorkspace } from "@/app/onboarding/actions";
import { BrandMark } from "@/components/dashboard/CandidateSidebar";
import { display, ui } from "@/lib/dashboard/fonts";
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
      <Shell>
        <main className="mx-auto flex max-w-xl px-5 py-20 sm:px-10">
          <div className="w-full rounded-[22px] border border-white/90 bg-white/70 p-8 text-center shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)] backdrop-blur sm:p-10">
            <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
              <ClipboardList size={26} strokeWidth={1.8} />
            </span>
            <h1 className={clsx(display.className, "mt-5 text-[30px] leading-tight font-semibold text-[#100E2E]")}>No assessment selected</h1>
            <p className="mx-auto mt-2 max-w-sm text-[15px] leading-relaxed text-[#5E5A82]">
              Start an assessment from your dashboard first — this page needs to know which one you&apos;re entering.
            </p>
            <Link
              href="/dashboard"
              className="mt-7 inline-flex items-center gap-2 rounded-full bg-[#1F1B4D] px-6 py-3 text-[14px] font-semibold text-white transition-opacity hover:opacity-90"
            >
              <ArrowLeft size={16} />
              Back to dashboard
            </Link>
          </div>
        </main>
      </Shell>
    );
  }

  return (
    <Shell step={step}>
      {/* Active step */}
      {/* The consent step is laid out wide; the device check and lobby keep their narrow column. */}
      <main className={clsx("mx-auto px-5 py-8 sm:px-10", step === 1 ? "max-w-[1560px] sm:py-12" : "max-w-2xl sm:py-12")}>
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
    </Shell>
  );
}

/** The dashboard's lavender canvas with a minimal top bar — no full nav, keeps focus on the flow. */
function Shell({ step, children }: { step?: 1 | 2 | 3; children: React.ReactNode }) {
  return (
    <div className={clsx(ui.className, "relative isolate min-h-screen bg-[#F3F1FA] text-[#1B1840]")}>
      <div
        aria-hidden
        className="pointer-events-none fixed inset-0 -z-10"
        style={{
          background:
            "radial-gradient(40% 35% at 8% 70%, rgba(214,204,250,0.55), transparent 70%), radial-gradient(35% 30% at 92% 18%, rgba(226,214,252,0.7), transparent 70%), radial-gradient(45% 40% at 55% 0%, rgba(255,255,255,0.9), transparent 70%)",
        }}
      />
      <header className="border-b border-white/80 bg-white/55 backdrop-blur">
        <div className="mx-auto flex h-[72px] max-w-[1560px] items-center gap-3 px-5 sm:px-10">
          <BrandMark size={32} />
          <span className={clsx(display.className, "text-[25px] leading-none font-medium tracking-tight text-[#1B1840]")}>Mindfries</span>
          {step && (
            <>
              <span className="ml-auto text-[14px] text-[#5E5A82] sm:text-[15px]">Step {step} of 3</span>
              <div className="hidden items-center gap-2 sm:flex" aria-hidden>
                {([1, 2, 3] as const).map((n) => (
                  <span
                    key={n}
                    className={clsx(
                      "h-1.5 w-16 rounded-full transition-colors duration-300 md:w-28",
                      step >= n ? "bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0]" : "bg-[#E4E0F2]"
                    )}
                  />
                ))}
              </div>
            </>
          )}
        </div>
      </header>
      {children}
    </div>
  );
}
