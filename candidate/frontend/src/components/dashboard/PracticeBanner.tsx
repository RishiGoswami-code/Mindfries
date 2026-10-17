import Link from "next/link";
import clsx from "clsx";
import { ChevronsRight } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";

/**
 * The way into a practice run: the real workspace on a throwaway task,
 * where nothing is recorded, timed or submitted (app/practice).
 *
 * Until the candidate has run the environment check, a quiet link to it
 * sits under the promise line — it's the one setup step with no other way
 * in from the dashboard.
 */
export function PracticeBanner({ environmentChecked }: { environmentChecked: boolean }) {
  return (
    <section className="relative isolate overflow-hidden rounded-[22px] bg-[#16143A] px-8 py-10 text-white sm:px-16 sm:py-11">
      {/* The planet: a dark disc with a violet rim light, off the right edge. */}
      <div aria-hidden className="absolute inset-0 -z-10 bg-[radial-gradient(120%_140%_at_100%_100%,#3B2C86_0%,#221C58_38%,#16143A_70%)]" />
      <div
        aria-hidden
        className="absolute -z-10 h-[620px] w-[620px] rounded-full max-sm:hidden"
        style={{
          right: -170,
          top: -10,
          background: "radial-gradient(circle at 60% 60%, #2A2262 0%, #1E1A4D 55%, #1A1745 100%)",
          boxShadow: "-18px -10px 40px -6px rgba(178,150,255,0.55), inset 14px 14px 30px -8px rgba(196,174,255,0.75), 0 0 120px 10px rgba(124,92,252,0.25)",
        }}
      />

      <div className="flex flex-col gap-8 xl:flex-row xl:items-center xl:justify-between">
        <div className="max-w-xl">
          <p className="flex items-center gap-4 text-[13px] font-medium tracking-[0.28em] text-[#D6D0F2] uppercase">
            Practice run
            <span className="h-px w-9 bg-[#8C85B8]" />
          </p>
          <h2 className={clsx(display.className, "mt-5 text-[40px] leading-[1.05] font-medium tracking-tight sm:text-[54px]")}>
            Start a practice run
          </h2>
          <p className="mt-5 text-[16.5px] leading-relaxed text-[#E4E0F7]">
            The real workspace with a throwaway task.
            <br />
            Nothing you do is recorded, scored or shared.
          </p>
          {!environmentChecked && (
            <Link href="/environment-check" className="mt-4 inline-block text-[13.5px] text-[#C9BCF7] underline-offset-4 hover:underline">
              Check your camera, microphone and browser first →
            </Link>
          )}
        </div>

        <div className="flex flex-col items-stretch sm:items-start xl:items-center">
          <Link
            href="/practice"
            className="inline-flex h-[60px] w-full items-center justify-center gap-6 rounded-full bg-gradient-to-r from-[#B6A4F8] to-[#C6B7FA] text-[17px] font-semibold whitespace-nowrap text-[#16143A] shadow-[0_12px_30px_-10px_rgba(182,164,248,0.7)] transition-transform hover:-translate-y-px sm:w-[314px]"
          >
            Open practice run
            <ChevronsRight size={18} strokeWidth={2} className="text-[#16143A]/30" />
          </Link>
          <p className="mt-5 text-center text-[11px] font-medium tracking-[0.18em] text-[#D6D0F2] uppercase sm:text-left sm:text-[12px] sm:tracking-[0.26em] sm:whitespace-nowrap xl:text-center">
            Safe <span className="mx-2.5 text-[#8C85B8]">·</span> Private <span className="mx-2.5 text-[#8C85B8]">·</span> Just practice
          </p>
        </div>
      </div>
    </section>
  );
}
