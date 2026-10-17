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
import { HOW_EVIDENCE_IS_USED, RECORDED_SUMMARY, RETENTION, TOOL_RULES } from "@/lib/policies";

interface ConsentStepProps {
  onContinue: () => void;
}

const RECORDED_ICONS: Record<string, { icon: LucideIcon; tint: string }> = {
  screen: { icon: Monitor, tint: "bg-[#E8EEFE] text-[#3557E0]" },
  camera: { icon: Camera, tint: "bg-[#F1E8FD] text-[#8B3FD9]" },
  microphone: { icon: Mic, tint: "bg-[#E3F6EA] text-[#1E9A57]" },
  code: { icon: Code2, tint: "bg-[#ECEAFB] text-[#4B45C9]" },
  terminal: { icon: SquareTerminal, tint: "bg-[#EEF0F4] text-[#3E4559]" },
  behaviour: { icon: PanelsTopLeft, tint: "bg-[#FDEFE3] text-[#E0711B]" },
};

const ALLOWED_ICONS: Record<string, { icon: LucideIcon; tint: string }> = {
  docs: { icon: BookOpen, tint: "bg-[#E8EEFE] text-[#3557E0]" },
  cloud: { icon: Cloud, tint: "bg-[#ECEAFB] text-[#4B45C9]" },
  apis: { icon: Link2, tint: "bg-[#E3F6EA] text-[#1E9A57]" },
  devtools: { icon: SquareTerminal, tint: "bg-[#FDEFE3] text-[#E0711B]" },
  packages: { icon: Box, tint: "bg-[#E8EEFE] text-[#3557E0]" },
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
      <p className="text-[13px] font-semibold tracking-[0.2em] text-[#5C7CC4] uppercase">Before you start</p>
      <h1 className="mt-2 text-[32px] leading-tight font-bold tracking-tight text-[#0B1033] sm:text-[42px]">A fair and transparent assessment</h1>
      <p className="mt-2 text-[16px] text-[#5B6385] sm:text-[19px]">We capture your work to create an evidence report for the hiring team. Here&apos;s a quick overview.</p>

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
        <Card tone="green" icon={<CheckCircle2 size={26} className="fill-[#22A55B] text-white" />} title="Allowed tools & integrations" subtitle="You can use external platforms as needed.">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {TOOL_RULES.allowed.map((t) => {
              const { icon: Icon, tint } = ALLOWED_ICONS[t.key] ?? ALLOWED_ICONS.docs;
              return <Tile key={t.key} icon={<Icon size={22} />} tint={tint} label={t.short} detail={t.example} />;
            })}
          </div>
        </Card>

        {/* Not allowed */}
        <Card tone="red" icon={<Ban size={26} />} title="Not allowed" subtitle="To keep the evaluation fair, these are not permitted.">
          <ul className="divide-y divide-[#F6DDE2]">
            {TOOL_RULES.notAllowed.map((t) => {
              const Icon = NOT_ALLOWED_ICONS[t.key] ?? Ban;
              return (
                <li key={t.key} className="flex items-center gap-4 py-3.5 first:pt-1">
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-[#FCE4E8] text-[#D6293F]">
                    <Icon size={22} />
                  </span>
                  <span className="min-w-0 border-l border-[#F6DDE2] pl-4">
                    <span className="block text-[15.5px] font-medium text-[#0B1033]">{t.short}</span>
                    <span className="block text-[14px] text-[#6B7192]">{t.example}</span>
                  </span>
                </li>
              );
            })}
          </ul>
          <p className="mt-2 text-[12.5px] leading-snug text-[#8A6670]">{TOOL_RULES.note}</p>
        </Card>
      </div>

      {/* How evidence is used, and retention */}
      <div className="mt-5 grid gap-6 rounded-[20px] border border-[#E6E9F2] bg-white/80 p-6 md:grid-cols-2 md:divide-x md:divide-[#E6E9F2]">
        <div className="flex items-start gap-5">
          <span className="flex h-[68px] w-[68px] shrink-0 items-center justify-center rounded-2xl bg-[#EEEBFB] text-[#4B45C9]">
            <ShieldCheck size={30} />
          </span>
          <div>
            <h2 className="text-[19px] font-semibold text-[#0B1033]">{HOW_EVIDENCE_IS_USED.title}</h2>
            <p className="mt-1 text-[15px] leading-relaxed text-[#5B6385]">{HOW_EVIDENCE_IS_USED.short}</p>
          </div>
        </div>
        <div className="flex items-start gap-5 md:pl-8">
          <span className="flex h-[68px] w-[68px] shrink-0 items-center justify-center rounded-2xl bg-[#E8EEFE] text-[#3557E0]">
            <Clock size={30} />
          </span>
          <div>
            <h2 className="text-[19px] font-semibold text-[#0B1033]">Data retention</h2>
            <p className="mt-1 text-[15px] leading-relaxed text-[#5B6385]">{RETENTION.body}</p>
          </div>
        </div>
      </div>

      {/* Agreement */}
      <div className="mt-6 flex flex-col gap-4 border-t border-[#E6E9F2] pt-6 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3.5">
          <span className="relative mt-0.5 flex h-7 w-7 shrink-0">
            <input
              id="consent-checkbox"
              type="checkbox"
              checked={agreed}
              onChange={(e) => setAgreed(e.target.checked)}
              className="peer absolute inset-0 h-full w-full cursor-pointer appearance-none rounded-lg border-2 border-[#B9C2E0] bg-white transition-colors checked:border-[#4361EE] checked:bg-[#4361EE] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#4361EE]"
            />
            <Check size={18} strokeWidth={3} className="pointer-events-none absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-white opacity-0 peer-checked:opacity-100" />
          </span>
          <p className="text-[15.5px] leading-relaxed text-[#2A3050]">
            <label htmlFor="consent-checkbox" className="cursor-pointer">
              I understand that my screen, camera, microphone, code activity and workspace behaviour are recorded as described here, and I agree to the
            </label>{" "}
            <PoliciesLink className="font-medium text-[#3557E0] underline-offset-2 hover:underline">privacy &amp; recording policy</PoliciesLink>
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
          className="inline-flex h-14 shrink-0 items-center justify-center gap-3 rounded-2xl bg-gradient-to-r from-[#3D5AF1] to-[#5B6CF8] px-12 text-[18px] font-semibold text-white shadow-[0_14px_30px_-14px_rgba(61,90,241,0.8)] transition-[transform,opacity] hover:-translate-y-px disabled:cursor-not-allowed disabled:opacity-45 disabled:hover:translate-y-0"
        >
          Continue <ArrowRight size={20} />
        </button>
      </div>
    </div>
  );
}

const CARD_TONE = {
  blue: { card: "border-[#E6E9F2] bg-white/80", badge: "bg-[#E8EEFE] text-[#3557E0]" },
  green: { card: "border-[#D8F0E2] bg-gradient-to-br from-[#F2FBF5] to-white/80", badge: "bg-[#DDF4E6]" },
  red: { card: "border-[#F8DEE3] bg-gradient-to-br from-[#FDF2F4] to-white/80", badge: "bg-[#FCE4E8] text-[#D6293F]" },
};

function Card({ tone, icon, title, subtitle, children }: { tone: keyof typeof CARD_TONE; icon: React.ReactNode; title: string; subtitle: string; children: React.ReactNode }) {
  return (
    <section className={clsx("min-w-0 rounded-[20px] border p-5", CARD_TONE[tone].card)}>
      <div className="flex items-center gap-4">
        <span className={clsx("flex h-[64px] w-[64px] shrink-0 items-center justify-center rounded-2xl", CARD_TONE[tone].badge)}>{icon}</span>
        <div className="min-w-0">
          <h2 className="text-[21px] leading-tight font-semibold text-[#0B1033]">{title}</h2>
          <p className="mt-0.5 text-[14.5px] text-[#6B7192]">{subtitle}</p>
        </div>
      </div>
      <div className="mt-5">{children}</div>
    </section>
  );
}

function Tile({ icon, tint, label, detail }: { icon: React.ReactNode; tint: string; label: string; detail?: string }) {
  return (
    <div className="flex min-h-[132px] flex-col items-center rounded-2xl border border-[#EDEFF5] bg-white/90 px-2 py-4 text-center">
      <span className={clsx("flex h-12 w-12 items-center justify-center rounded-xl", tint)}>{icon}</span>
      <span className="mt-3 text-[14.5px] leading-tight font-medium text-[#0B1033]">{label}</span>
      {detail && <span className="mt-1 text-[12.5px] leading-snug text-[#7A809E]">{detail}</span>}
    </div>
  );
}
