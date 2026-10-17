import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import {
  ArrowLeft,
  ArrowRight,
  Bot,
  Camera,
  CheckCircle2,
  Clock,
  FileText,
  Lock,
  MonitorCheck,
  Play,
  ShieldCheck,
  SquareTerminal,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { getSetupState } from "@/lib/setup-state";
import { startPracticeRun } from "@/lib/setup-actions";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Practice Run · Mindfries",
  description: "Try the workspace with a throwaway task. Nothing is recorded or scored.",
};

/** The practice task's own three steps (lib/ide/practice-task.ts's brief). */
const STEPS: Array<{ icon: LucideIcon; title: string; body: string }> = [
  { icon: Play, title: "Run the tests", body: "Read TASK.md, then run the tests in the terminal and read the failure." },
  { icon: Wrench, title: "Fix the bug", body: "Find it in pricing/discount.py and fix it." },
  { icon: CheckCircle2, title: "Watch them pass", body: "Run the tests again until every one passes." },
];

/** What the candidate will find in the workspace — the same one a real assessment uses. */
const WORKSPACE: Array<{ icon: LucideIcon; label: string; detail: string; tint: string }> = [
  { icon: FileText, label: "TASK.md and files", detail: "The brief opens first, beside the code", tint: "bg-[#EAF1FE] text-[#2F6BDE]" },
  { icon: SquareTerminal, label: "Terminal", detail: "Real commands, tests and git", tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
  { icon: Camera, label: "Camera", detail: "Asked for, as in a real session", tint: "bg-[#FDEFE3] text-[#C26410]" },
  { icon: Bot, label: "Mindfries AI", detail: "Answers only in a real assessment", tint: "bg-[#F1E8FD] text-[#8B3FD9]" },
];

/**
 * Practice run: the real workspace on a throwaway task, where nothing is
 * recorded, timed or submitted. Opening one is the only thing kept, so the
 * dashboard can tick the step off (practice_runs).
 */
export default async function PracticeRunPage() {
  const [session, profile] = await Promise.all([currentCandidate(), getProfile()]);
  const setup = await getSetupState(session?.id);

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <div className="mt-6 px-1">
        <Link href="/dashboard" className="inline-flex items-center gap-1.5 text-[13.5px] text-[#4C4874] hover:text-[#100E2E]">
          <ArrowLeft size={15} /> Back to dashboard
        </Link>
        <h1 className={clsx(display.className, "mt-3 text-[44px] leading-none font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[52px]")}>Practice run</h1>
        <p className="mt-3 max-w-2xl text-[15px] leading-relaxed text-[#5E5A82]">
          The real workspace with a throwaway task. Get used to it before an assessment counts.
        </p>
      </div>

      {/* The way in */}
      <section className="relative isolate mt-6 overflow-hidden rounded-[22px] bg-[#16143A] px-7 py-9 text-white sm:px-12 sm:py-11">
        <div aria-hidden className="absolute inset-0 -z-10 bg-[radial-gradient(120%_140%_at_100%_100%,#3B2C86_0%,#221C58_38%,#16143A_70%)]" />
        <div
          aria-hidden
          className="absolute -z-10 h-[520px] w-[520px] rounded-full max-md:hidden"
          style={{
            right: -160,
            top: -40,
            background: "radial-gradient(circle at 60% 60%, #2A2262 0%, #1E1A4D 55%, #1A1745 100%)",
            boxShadow: "-18px -10px 40px -6px rgba(178,150,255,0.55), inset 14px 14px 30px -8px rgba(196,174,255,0.75), 0 0 120px 10px rgba(124,92,252,0.25)",
          }}
        />
        <div className="flex flex-col gap-8 lg:flex-row lg:items-center lg:justify-between">
          <div className="max-w-xl">
            <p className="flex items-center gap-4 text-[13px] font-medium tracking-[0.28em] text-[#D6D0F2] uppercase">
              Sample task
              <span className="h-px w-9 bg-[#8C85B8]" />
            </p>
            <h2 className={clsx(display.className, "mt-4 text-[34px] leading-[1.1] font-medium tracking-tight sm:text-[44px]")}>A discount that charges too much</h2>
            <p className="mt-4 text-[16px] leading-relaxed text-[#E4E0F7]">
              A small Python project with one bug and a test that catches it — the same shape as a real assessment. Try git and the terminal while
              you&apos;re there.
            </p>
          </div>

          <form action={startPracticeRun} className="flex flex-col items-stretch sm:items-start lg:items-center">
            <button
              type="submit"
              className="inline-flex h-[60px] w-full items-center justify-center gap-3 rounded-full bg-gradient-to-r from-[#B6A4F8] to-[#C6B7FA] px-10 text-[17px] font-semibold whitespace-nowrap text-[#16143A] shadow-[0_12px_30px_-10px_rgba(182,164,248,0.7)] transition-transform hover:-translate-y-px sm:w-[300px]"
            >
              {setup.practiceRuns > 0 ? "Open another practice run" : "Open a practice run"}
              <ArrowRight size={18} />
            </button>
            <p className="mt-4 text-center text-[13px] text-[#D6D0F2] sm:text-left lg:text-center">
              {setup.practiceRuns === 0
                ? "No time limit. Leave whenever you like."
                : `You've opened ${setup.practiceRuns} so far. Your earlier work in it is still there.`}
            </p>
          </form>
        </div>
      </section>

      <div className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <div className="min-w-0 space-y-5">
          {/* What to do */}
          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)]">
            <h2 className={clsx(display.className, "text-[24px] font-semibold text-[#100E2E]")}>What you&apos;ll do</h2>
            <ol className="mt-4 grid gap-3 md:grid-cols-3">
              {STEPS.map((s, i) => (
                <li key={s.title} className="rounded-2xl bg-[#F6F4FC] p-4">
                  <div className="flex items-center gap-3">
                    <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-white text-[#5B3BE0] shadow-[0_4px_12px_-8px_rgba(27,24,64,0.4)]">
                      <s.icon size={19} />
                    </span>
                    <span className="text-[12.5px] font-semibold tracking-[0.14em] text-[#8A86A8] uppercase">Step {i + 1}</span>
                  </div>
                  <p className="mt-3 text-[15.5px] font-semibold text-[#100E2E]">{s.title}</p>
                  <p className="mt-1 text-[13.5px] leading-snug text-[#5E5A82]">{s.body}</p>
                </li>
              ))}
            </ol>
          </section>

          {/* What's in the workspace */}
          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)]">
            <h2 className={clsx(display.className, "text-[24px] font-semibold text-[#100E2E]")}>In the workspace</h2>
            <p className="mt-1 text-[14px] text-[#5E5A82]">The same one a real assessment opens in.</p>
            <ul className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
              {WORKSPACE.map((w) => (
                <li key={w.label} className="flex flex-col items-center rounded-2xl border border-[#ECE9F6] bg-white px-3 py-4 text-center">
                  <span className={clsx("flex h-11 w-11 items-center justify-center rounded-xl", w.tint)}>
                    <w.icon size={20} />
                  </span>
                  <span className="mt-2.5 text-[14px] font-medium text-[#100E2E]">{w.label}</span>
                  <span className="mt-0.5 text-[12.5px] leading-snug text-[#6E6A8F]">{w.detail}</span>
                </li>
              ))}
            </ul>
          </section>
        </div>

        <div className="min-w-0 space-y-5">
          {/* Privacy */}
          <section className="rounded-[22px] border border-[#CDEBD9] bg-gradient-to-br from-[#F1FAF4] to-white/80 p-6">
            <div className="flex items-center gap-3.5">
              <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#DDF4E6] text-[#1A9E6B]">
                <ShieldCheck size={23} />
              </span>
              <h2 className={clsx(display.className, "text-[22px] font-semibold text-[#100E2E]")}>Nothing is recorded</h2>
            </div>
            <ul className="mt-4 space-y-3 text-[14px] leading-snug text-[#2A2654]">
              <li className="flex gap-3">
                <Lock size={17} className="mt-0.5 shrink-0 text-[#1A9E6B]" /> There&apos;s no session behind it: nothing is scored, shared or seen by a company.
              </li>
              <li className="flex gap-3">
                <Camera size={17} className="mt-0.5 shrink-0 text-[#1A9E6B]" /> It asks for your camera and goes fullscreen, as a real session does. The picture stays in your
                browser and no stills are taken.
              </li>
              <li className="flex gap-3">
                <Clock size={17} className="mt-0.5 shrink-0 text-[#1A9E6B]" /> No clock. The only thing kept is that you opened one, so your dashboard can tick it off.
              </li>
            </ul>
          </section>

          {/* Setup */}
          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)]">
            <div className="flex items-center gap-3.5">
              <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#EFEAFD] text-[#5B3BE0]">
                <MonitorCheck size={22} />
              </span>
              <div>
                <h2 className={clsx(display.className, "text-[22px] font-semibold text-[#100E2E]")}>Your setup</h2>
                <p className="text-[13.5px] text-[#6E6A8F]">Camera, microphone and browser.</p>
              </div>
            </div>
            {setup.environmentCheckedAt ? (
              <p className="mt-4 flex items-center gap-2 rounded-2xl bg-[#F3FBF6] px-4 py-3 text-[14px] text-[#14693a]">
                <CheckCircle2 size={17} /> Checked — you&apos;re ready for a real session.
              </p>
            ) : (
              <Link
                href="/environment-check"
                className="mt-4 flex items-center justify-between gap-3 rounded-2xl bg-[#F6F4FC] px-4 py-3 text-[14px] font-medium text-[#4B2BB8] hover:bg-[#EFEAFD]"
              >
                Check them before you start <ArrowRight size={16} />
              </Link>
            )}
          </section>
        </div>
      </div>
    </CandidateShell>
  );
}
