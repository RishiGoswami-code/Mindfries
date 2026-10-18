import { Ban, Check, Hourglass, Sparkles } from "lucide-react";
import { SUPPORT_EMAIL, type CompanyAccess } from "@/lib/access";
import { RemindButton } from "./RemindButton";

/**
 * Shown at the top of every portal page while hiring is locked: waiting for
 * access, access declined, or out of credits. Nothing at all once the
 * company can invite.
 */
export function AccessBanner({ access }: { access: CompanyAccess }) {
  if (access.status === "pending") {
    return (
      <section className="mb-6 rounded-[22px] border border-white/90 p-6 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)] [background:linear-gradient(110deg,rgba(255,247,232,0.95),rgba(255,255,255,0.85)_45%,rgba(239,234,253,0.9))]">
        <div className="flex gap-4">
          <span className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-[#FDECCB] text-[#B26A0E]">
            <Hourglass size={22} strokeWidth={1.8} aria-hidden />
          </span>
          <div className="min-w-0">
            <h2 className="text-[20px] font-extrabold tracking-tight text-ink">Your account is waiting for access</h2>
            <p className="mt-1 max-w-2xl text-[14.5px] leading-relaxed text-dim">
              Every new company is reviewed by the Mindfries team. Once credits are added you can create roles and invite candidates — we&apos;ll email
              you when it&apos;s done. Until then you can look around and invite teammates.
            </p>
          </div>
        </div>
        <ol className="mt-5 grid gap-2.5 text-[13.5px] sm:grid-cols-3">
          <li className="flex items-center gap-2.5 rounded-2xl bg-white px-4 py-3 font-semibold text-[#1E7A5A]">
            <span className="grid h-6 w-6 place-items-center rounded-full bg-[#D7F3EC]">
              <Check size={13} strokeWidth={2.6} aria-hidden />
            </span>
            Account created
          </li>
          <li className="flex items-center gap-2.5 rounded-2xl bg-white px-4 py-3 font-semibold text-[#8A5A0B] ring-2 ring-[#F5D58F] ring-inset">
            <span className="grid h-6 w-6 place-items-center rounded-full bg-[#FDECCB] text-xs">2</span>
            Under review
          </li>
          <li className="flex items-center gap-2.5 rounded-2xl bg-white/60 px-4 py-3 font-medium text-faint">
            <span className="grid h-6 w-6 place-items-center rounded-full border border-hair-bright text-xs">3</span>
            Credits added
          </li>
        </ol>
        <div className="mt-5">
          <RemindButton alreadyAt={access.requestedAt} />
        </div>
      </section>
    );
  }

  if (access.status === "declined") {
    return (
      <section className="mb-6 flex gap-4 rounded-[22px] border border-[#F4D3DA] bg-[#FDF1F3] p-6">
        <span className="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-[#FCE4E8] text-[#D6293F]">
          <Ban size={22} strokeWidth={1.8} aria-hidden />
        </span>
        <div>
          <h2 className="text-[20px] font-extrabold tracking-tight text-ink">This account doesn&apos;t have access</h2>
          <p className="mt-1 max-w-2xl text-[14.5px] leading-relaxed text-dim">
            The Mindfries team didn&apos;t switch on hiring for this account. If you think that&apos;s a mistake, email{" "}
            <a href={`mailto:${SUPPORT_EMAIL}`} className="font-medium text-accent underline-offset-4 hover:underline">
              {SUPPORT_EMAIL}
            </a>
            .
          </p>
        </div>
      </section>
    );
  }

  if (access.credits !== null && access.credits < 1) {
    return (
      <section className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-[22px] border border-white/90 bg-[linear-gradient(110deg,#FFFFFF,#EFEAFD)] p-5 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)]">
        <div className="flex items-center gap-4">
          <span className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-accent-soft text-accent">
            <Sparkles size={20} strokeWidth={1.8} aria-hidden />
          </span>
          <div>
            <h2 className="text-[17px] font-extrabold tracking-tight text-ink">You&apos;re out of credits</h2>
            <p className="text-[14px] text-dim">Inviting a candidate uses 1 credit. Ask the Mindfries team for more.</p>
          </div>
        </div>
        <RemindButton alreadyAt={access.requestedAt} label="Ask for more credits" />
      </section>
    );
  }

  return null;
}
