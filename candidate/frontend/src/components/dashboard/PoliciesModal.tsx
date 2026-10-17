"use client";

import { useEffect, useRef } from "react";
import clsx from "clsx";
import {
  Ban,
  Bot,
  Box,
  Bug,
  CheckCircle2,
  Clock,
  Cloud,
  Database,
  Eye,
  FileText,
  Folder,
  FlaskConical,
  Globe,
  Info,
  Lightbulb,
  Link2,
  MessageSquare,
  MessagesSquare,
  Mic,
  MousePointer2,
  PenLine,
  Play,
  ShieldCheck,
  SquareTerminal,
  Users,
  Wrench,
  X,
  type LucideIcon,
} from "lucide-react";
import {
  DELETION,
  EVIDENCE_LEAD,
  EVIDENCE_POINTS,
  PRIVACY_CONTACT,
  RECORDED_SIGNALS,
  RETENTION,
  TOOL_RULES,
  WHAT_IS_RECORDED,
} from "@/lib/policies";
import { ui } from "@/lib/dashboard/fonts";

const SIGNAL_ICONS: Record<string, LucideIcon> = {
  navigation: Folder,
  code: PenLine,
  terminal: SquareTerminal,
  tests: Play,
  assistant: Bot,
  time: Clock,
  paste: MousePointer2,
  media: Mic,
  interview: MessagesSquare,
};
const EVIDENCE_ICONS: Record<string, LucideIcon> = { approach: Lightbulb, debugging: Bug, testing: FlaskConical, communication: MessageSquare };
const TOOL_ICONS: Record<string, LucideIcon> = {
  docs: Globe,
  cloud: Cloud,
  apis: Link2,
  devtools: Wrench,
  packages: Box,
  assistant: Bot,
  ai: Ban,
  people: Users,
};

/**
 * The privacy and recording policy: what a session records, how it's used,
 * what may be used while working, and how long it's kept. Every word comes
 * from lib/policies.ts — the same copy the candidate agrees to in
 * onboarding's ConsentStep — so this window and the agreement can't say
 * different things.
 */
export function PoliciesModal({ onClose }: { onClose: () => void }) {
  const panel = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    panel.current?.focus();
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className={clsx(ui.className, "fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-[#1B1840]/40 p-3 py-6 backdrop-blur-[3px] sm:p-6 sm:py-10")}
      onClick={onClose}
    >
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby="policy-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-[1180px] rounded-[26px] bg-[#FBFAFE] p-5 text-[#1B1840] shadow-[0_40px_90px_-30px_rgba(27,24,64,0.55)] outline-none sm:p-8"
      >
        <div className="flex items-start gap-4 sm:gap-6">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0] sm:h-[72px] sm:w-[72px]">
            <ShieldCheck size={32} strokeWidth={1.7} />
          </span>
          <div className="min-w-0 flex-1 pt-1">
            <h2 id="policy-title" className="text-[24px] leading-tight font-bold tracking-tight text-[#100E2E] sm:text-[32px]">
              Privacy &amp; recording policy
            </h2>
            <p className="mt-1.5 text-[15px] text-[#5E5A82] sm:text-[17px]">
              An assessment is a monitored workspace. Here&apos;s what we record, how it&apos;s used, and what&apos;s allowed — the terms
              you agree to before every session.
            </p>
          </div>
          <button type="button" onClick={onClose} aria-label="Close" className="-mt-1 -mr-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-[#2A2654] hover:bg-[#EEEBFA]">
            <X size={24} strokeWidth={1.8} />
          </button>
        </div>

        <div className="mt-7 grid gap-5 lg:grid-cols-[1.1fr_1fr_1fr]">
          {/* What is recorded */}
          <Column icon={Eye} title="What is recorded" lead={WHAT_IS_RECORDED.body.split(".")[0] + "."}>
            <ul className="divide-y divide-[#ECE9F6]">
              {RECORDED_SIGNALS.map((s) => {
                const Icon = SIGNAL_ICONS[s.key] ?? FileText;
                return (
                  <li key={s.key} className="flex items-start gap-3.5 py-3">
                    <Icon size={21} strokeWidth={1.7} className="mt-0.5 shrink-0 text-[#4B2BB8]" />
                    <div>
                      <p className="text-[15px] font-semibold text-[#100E2E]">{s.label}</p>
                      <p className="text-[13.5px] leading-snug text-[#5E5A82]">{s.detail}</p>
                    </div>
                  </li>
                );
              })}
            </ul>
            <p className="mt-2 text-[12.5px] leading-relaxed text-[#6E6A8F]">Nothing is recorded until you click Enter Workspace, and nothing on a practice run.</p>
          </Column>

          {/* How it's used */}
          <Column icon={FileText} title="How it's used" lead={EVIDENCE_LEAD}>
            <ul className="space-y-3">
              {EVIDENCE_POINTS.map((p) => {
                const Icon = EVIDENCE_ICONS[p.key] ?? Lightbulb;
                return (
                  <li key={p.key} className="flex items-start gap-3.5 rounded-2xl bg-[#F6F4FC] p-4">
                    <Icon size={22} strokeWidth={1.7} className="mt-0.5 shrink-0 text-[#2A2654]" />
                    <div>
                      <p className="text-[15.5px] font-semibold text-[#100E2E]">{p.label}</p>
                      <p className="text-[14px] leading-snug text-[#5E5A82]">{p.detail}</p>
                    </div>
                  </li>
                );
              })}
            </ul>
            <p className="mt-3 text-[13px] text-[#6E6A8F]">The hiring team makes the final call.</p>
          </Column>

          {/* Allowed tools */}
          <Column icon={Link2} title={TOOL_RULES.title} lead={TOOL_RULES.body}>
            <div className="overflow-hidden rounded-2xl border border-[#D6EFE1] bg-white">
              <p className="flex items-center gap-2.5 bg-[#E8F7EF] px-4 py-3 text-[16px] font-semibold text-[#14693a]">
                <CheckCircle2 size={21} /> Allowed
              </p>
              <ul className="space-y-2.5 px-4 py-3.5">
                {TOOL_RULES.allowed.map((t) => {
                  const Icon = TOOL_ICONS[t.key] ?? Globe;
                  return (
                    <li key={t.key} className="flex items-start gap-3 text-[14px] text-[#2A2654]">
                      <Icon size={19} strokeWidth={1.7} className="mt-px shrink-0 text-[#4C4874]" />
                      {t.label}
                    </li>
                  );
                })}
              </ul>
            </div>
            <div className="mt-4 overflow-hidden rounded-2xl border border-[#F4D3DA] bg-white">
              <p className="flex items-center gap-2.5 bg-[#FDEEF1] px-4 py-3 text-[16px] font-semibold text-[#a6203c]">
                <Ban size={20} /> Not allowed
              </p>
              <ul className="space-y-2.5 px-4 py-3.5">
                {TOOL_RULES.notAllowed.map((t) => {
                  const Icon = t.key === "people" ? Users : Bot;
                  return (
                    <li key={t.key} className="flex items-start gap-3 text-[14px] text-[#2A2654]">
                      <Icon size={19} strokeWidth={1.7} className="mt-px shrink-0 text-[#4C4874]" />
                      {t.label}
                    </li>
                  );
                })}
              </ul>
            </div>
            <p className="mt-3 text-[12.5px] leading-relaxed text-[#6E6A8F]">{TOOL_RULES.note}</p>
          </Column>
        </div>

        {/* Retention */}
        <div className="mt-5 flex flex-col gap-4 rounded-[20px] border border-[#ECE9F6] bg-white/80 p-5 lg:flex-row lg:items-center">
          <div className="flex min-w-0 flex-1 items-start gap-4">
            <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
              <Database size={22} strokeWidth={1.7} />
            </span>
            <div>
              <p className="text-[17px] font-semibold text-[#100E2E]">Data retention</p>
              <p className="text-[14.5px] leading-snug text-[#5E5A82]">{RETENTION.body}</p>
            </div>
          </div>
          <div className="flex min-w-0 items-start gap-3 lg:max-w-[380px] lg:border-l lg:border-[#ECE9F6] lg:pl-5">
            <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#EFEAFD] text-[#5B3BE0]">
              <Info size={19} />
            </span>
            <p className="text-[13.5px] leading-snug text-[#5E5A82]">
              {DELETION.before}{" "}
              <a href={`mailto:${PRIVACY_CONTACT}?subject=${encodeURIComponent("Please delete my Mindfries data")}`} className="font-medium text-[#5B3BE0] underline-offset-2 hover:underline [overflow-wrap:anywhere]">
                {PRIVACY_CONTACT}
              </a>{" "}
              {DELETION.after}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="h-12 shrink-0 rounded-2xl bg-[#5B3BE0] px-9 text-[16px] font-semibold text-white shadow-[0_10px_24px_-10px_rgba(91,59,224,0.8)] hover:bg-[#4B2BB8]"
          >
            Got it
          </button>
        </div>
      </div>
    </div>
  );
}

function Column({ icon: Icon, title, lead, children }: { icon: LucideIcon; title: string; lead: string; children: React.ReactNode }) {
  return (
    <section className="min-w-0 rounded-[22px] border border-[#ECE9F6] bg-white/80 p-5 sm:p-6">
      <div className="flex items-start gap-4 border-b border-[#ECE9F6] pb-4">
        <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
          <Icon size={26} strokeWidth={1.7} />
        </span>
        <div className="min-w-0">
          <h3 className="text-[20px] leading-tight font-bold text-[#100E2E]">{title}</h3>
          <p className="mt-1 text-[14.5px] leading-snug text-[#5E5A82]">{lead}</p>
        </div>
      </div>
      <div className="mt-3">{children}</div>
    </section>
  );
}
