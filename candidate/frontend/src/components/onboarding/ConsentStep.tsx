"use client";

import { useState } from "react";
import clsx from "clsx";
import {
  ArrowRight,
  Ban,
  BookOpen,
  Bot,
  Box,
  Camera,
  Check,
  CheckCircle2,
  ClipboardPaste,
  Clock,
  Cloud,
  Code2,
  Cpu,
  Eye,
  Link2,
  Mic,
  Monitor,
  PanelsTopLeft,
  ShieldCheck,
  SquareTerminal,
  Users,
  type LucideIcon,
} from "lucide-react";
import { PoliciesLink } from "@/components/dashboard/PoliciesLink";
import { display } from "@/lib/dashboard/fonts";
import { HOW_EVIDENCE_IS_USED, RECORDED_SUMMARY, RETENTION, TOOL_RULES } from "@/lib/policies";

interface ConsentStepProps {
  onContinue: () => void;
}

const RECORDED_ICONS: Record<string, { icon: LucideIcon; tint: string }> = {
  screen: { icon: Monitor, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
  camera: { icon: Camera, tint: "bg-[#F1E8FD] text-[#8B3FD9]" },
  microphone: { icon: Mic, tint: "bg-[#E3F6EA] text-[#1E9A57]" },
  code: { icon: Code2, tint: "bg-[#EEEBFA] text-[#4B2BB8]" },
  terminal: { icon: SquareTerminal, tint: "bg-[#F1EFF8] text-[#2A2654]" },
  behaviour: { icon: PanelsTopLeft, tint: "bg-[#FDEFE3] text-[#E0711B]" },
};

const ALLOWED_ICONS: Record<string, { icon: LucideIcon; tint: string }> = {
  docs: { icon: BookOpen, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
  cloud: { icon: Cloud, tint: "bg-[#EEEBFA] text-[#4B2BB8]" },
  apis: { icon: Link2, tint: "bg-[#E3F6EA] text-[#1E9A57]" },
  devtools: { icon: SquareTerminal, tint: "bg-[#FDEFE3] text-[#E0711B]" },
  packages: { icon: Box, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
  assistant: { icon: Bot, tint: "bg-[#F1E8FD] text-[#8B3FD9]" },
};

const NOT_ALLOWED_ICONS: Record<string, LucideIcon> = { ai: Cpu, people: Users, paste: ClipboardPaste, external: Ban };

/**
 * Step 1: before you start — what is recorded, what may be used, how the
 * evidence is used and how long it's kept, and the agreement.
 *
 * Every word comes from lib/policies.ts, the copy the "Privacy & recording
 * policy" window shows too, so what's agreed to here and what the policy
 * says can't drift apart. The policy link opens the full text.
 */
export function ConsentStep({ onContinue }: ConsentStepProps) {
  const [agreed, setAgreed] = useState(false);

  return (
    <div>
      <p className="flex items-center gap-4 text-[13px] font-medium tracking-[0.28em] text-[#6E6A8F] uppercase">
        Before you start <span className="h-px w-9 bg-[#C9BCF7]" />
      </p>
      <h1 className={clsx(display.className, "mt-3 text-[34px] leading-[1.05] font-semibold tracking-tight text-[#100E2E] sm:text-[48px]")}>A fair and transparent assessment</h1>
      <p className="mt-3 text-[16px] text-[#5E5A82] sm:text-[18.5px]">We capture your work to create an evidence report for the hiring team. Here&apos;s a quick overview.</p>

      <div className="mt-8 grid gap-5 xl:grid-cols-[1fr_1fr_1.05fr]">
        {/* What is recorded */}
        <Card tone="blue" icon={<Eye size={26} />} title="What is recorded" subtitle="We capture your workspace activity.">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {RECORDED_SUMMARY.map((r) => {
              const { icon: Icon, tint } = RECORDED_ICONS[r.key] ?? RECORDED_ICONS.code;
              return <Tile key={r.key} icon={<Icon size={22} />} tint={tint} label={r.label} detail={r.detail} />;
            })}
          </div>
        </Card>

        {/* Allowed */}
        <Card tone="green" icon={<CheckCircle2 size={26} className="fill-[#1E9A57] text-white" />} title="Allowed tools & integrations" subtitle="You can use external platforms as needed.">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {TOOL_RULES.allowed.map((t) => {
              const { icon: Icon, tint } = ALLOWED_ICONS[t.key] ?? ALLOWED_ICONS.docs;
              return <Tile key={t.key} icon={<Icon size={22} />} tint={tint} label={t.short} detail={t.example} />;
            })}
          </div>
        </Card>

        {/* Not allowed */}
        <Card tone="red" icon={<Ban size={26} />} title="Not allowed" subtitle="To keep the evaluation fair, these are not permitted.">
          <ul className="divide-y divide-[#F4D3DA]">
            {TOOL_RULES.notAllowed.map((t) => {
              const Icon = NOT_ALLOWED_ICONS[t.key] ?? Ban;
              return (
                <li key={t.key} className="flex items-center gap-4 py-3.5 first:pt-1">
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-[#FCE4E8] text-[#D6293F]">
                    <Icon size={22} />
                  </span>
                  <span className="min-w-0 border-l border-[#F4D3DA] pl-4">
                    <span className="block text-[15.5px] font-medium text-[#100E2E]">{t.short}</span>
                    <span className="block text-[14px] text-[#6E6A8F]">{t.example}</span>
                  </span>
                </li>
              );
            })}
          </ul>
          <p className="mt-2 text-[12.5px] leading-snug text-[#8A6670]">{TOOL_RULES.note}</p>
        </Card>
      </div>

      {/* How evidence is used, and retention */}
      <div className="mt-5 grid gap-6 rounded-[22px] border border-white/90 bg-white/70 p-6 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)] backdrop-blur md:grid-cols-2 md:divide-x md:divide-[#ECE9F6]">
        <div className="flex items-start gap-5">
          <span className="flex h-[68px] w-[68px] shrink-0 items-center justify-center rounded-2xl bg-[#EEEBFA] text-[#4B2BB8]">
            <ShieldCheck size={30} />
          </span>
          <div>
            <h2 className={clsx(display.className, "text-[22px] leading-tight font-semibold text-[#100E2E]")}>{HOW_EVIDENCE_IS_USED.title}</h2>
            <p className="mt-1 text-[15px] leading-relaxed text-[#5E5A82]">{HOW_EVIDENCE_IS_USED.short}</p>
          </div>
        </div>
        <div className="flex items-start gap-5 md:pl-8">
          <span className="flex h-[68px] w-[68px] shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
            <Clock size={30} />
          </span>
          <div>
            <h2 className={clsx(display.className, "text-[22px] leading-tight font-semibold text-[#100E2E]")}>Data retention</h2>
            <p className="mt-1 text-[15px] leading-relaxed text-[#5E5A82]">{RETENTION.body}</p>
          </div>
        </div>
      </div>

      {/* Agreement */}
      <div className="mt-6 flex flex-col gap-4 border-t border-[#E4E0F2] pt-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3.5">
          <span className="relative mt-0.5 flex h-7 w-7 shrink-0">
            <input
              id="consent-checkbox"
              type="checkbox"
              checked={agreed}
              onChange={(e) => setAgreed(e.target.checked)}
              className="peer absolute inset-0 h-full w-full cursor-pointer appearance-none rounded-lg border-2 border-[#C9BCF7] bg-white transition-colors checked:border-[#5B3BE0] checked:bg-[#5B3BE0] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#5B3BE0]"
            />
            <Check size={18} strokeWidth={3} className="pointer-events-none absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-white opacity-0 peer-checked:opacity-100" />
          </span>
          <p className="text-[15.5px] leading-relaxed text-[#2A2654]">
            <label htmlFor="consent-checkbox" className="cursor-pointer">
              I understand that my screen, camera, microphone, code activity and workspace behaviour are recorded as described here, and I agree to the
            </label>{" "}
            <PoliciesLink className="font-medium text-[#5B3BE0] underline-offset-2 hover:underline">privacy &amp; recording policy</PoliciesLink>
            <label htmlFor="consent-checkbox" className="cursor-pointer">
              , and to the rules on what I may use.
            </label>
          </p>
        </div>
        <button
          id="consent-continue"
          type="button"
          disabled={!agreed}
          onClick={onContinue}
          className="inline-flex h-14 shrink-0 items-center justify-center gap-3 rounded-2xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-12 text-[18px] font-semibold text-white shadow-[0_14px_30px_-14px_rgba(91,59,224,0.8)] transition-[transform,opacity] hover:-translate-y-px disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
        >
          Continue <ArrowRight size={20} />
        </button>
      </div>
    </div>
  );
}

const CARD_TONE = {
  blue: { card: "bg-gradient-to-br from-white/85 via-[#F7F5FE]/80 to-[#EFEAFD]/80", badge: "bg-[#E7E0FD] text-[#5B3BE0]" },
  green: { card: "bg-gradient-to-br from-[#EAFAF6]/90 via-white/80 to-white/70", badge: "bg-[#D7F3EC]" },
  red: { card: "bg-gradient-to-br from-[#FDF1F3]/90 via-white/80 to-white/70", badge: "bg-[#FCE4E8] text-[#D6293F]" },
};

function Card({ tone, icon, title, subtitle, children }: { tone: keyof typeof CARD_TONE; icon: React.ReactNode; title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <section className={clsx("min-w-0 rounded-[22px] border border-white/90 p-5 shadow-[0_10px_30px_-22px_rgba(76,58,160,0.45)]", CARD_TONE[tone].card)}>
      <div className="flex items-center gap-4">
        <span className={clsx("flex h-[64px] w-[64px] shrink-0 items-center justify-center rounded-2xl", CARD_TONE[tone].badge)}>{icon}</span>
        <div className="min-w-0">
          <h2 className={clsx(display.className, "text-[23px] leading-tight font-semibold text-[#100E2E]")}>{title}</h2>
          <p className="mt-0.5 text-[14.5px] text-[#5E5A82]">{subtitle}</p>
        </div>
      </div>
      <div className="mt-5">{children}</div>
    </section>
  );
}

function Tile({ icon, tint, label, detail }: { icon: React.ReactNode; tint: string; label: string; detail?: string }) {
  return (
    <div className="flex min-h-[132px] flex-col items-center rounded-2xl border border-[#ECE9F6] bg-white/90 px-2 py-4 text-center">
      <span className={clsx("flex h-12 w-12 items-center justify-center rounded-xl", tint)}>{icon}</span>
      <span className="mt-3 text-[14.5px] leading-tight font-medium text-[#100E2E]">{label}</span>
      {detail && <span className="mt-1 text-[12.5px] leading-snug text-[#6E6A8F]">{detail}</span>}
    </div>
  );
}
