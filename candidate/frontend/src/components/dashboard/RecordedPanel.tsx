import clsx from "clsx";
import { Camera, Monitor, Shield, SquareTerminal, type LucideIcon } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";
import { PoliciesLink } from "./PoliciesLink";

/**
 * What a session records, in plain sight — something a candidate on a
 * proctored platform is entitled to see without opening a policy page.
 *
 * Every row has to be true of the product today, and match the consent copy
 * (lib/policies.ts): the camera and the whole screen are kept as stills —
 * the screen when it changes and at flagged moments — and the commands run
 * in the terminal are recorded. The footnote names the rest and opens the
 * full consent copy, so this can't drift from what the candidate agreed to.
 */
const ROWS: { label: string; icon: LucideIcon; tile: string; ink: string }[] = [
  { label: "Your camera (as stills)", icon: Camera, tile: "bg-[#EEECF6]", ink: "text-[#3A3660]" },
  { label: "Your screen (as stills)", icon: Monitor, tile: "bg-[#E2F4F1]", ink: "text-[#1E4A46]" },
  { label: "The commands you run", icon: SquareTerminal, tile: "bg-[#ECE6FD]", ink: "text-[#5B3BE0]" },
];

export function RecordedPanel() {
  return (
    <section className="rounded-[22px] border border-white/90 bg-white/55 p-6 shadow-[0_10px_30px_-20px_rgba(76,58,160,0.3)] backdrop-blur">
      <div className="flex items-center gap-4">
        <span className="flex h-12 w-12 items-center justify-center rounded-full bg-[#ECE6FD]">
          <Shield size={22} strokeWidth={1.75} className="text-[#5B3BE0]" />
        </span>
        <h2 className={clsx(display.className, "text-[25px] font-medium tracking-tight text-[#100E2E]")}>What gets recorded</h2>
      </div>

      <ul className="mt-5 space-y-3">
        {ROWS.map(({ label, icon: Icon, tile, ink }) => (
          <li key={label} className="flex items-center gap-5 rounded-2xl bg-white/60 px-4 py-2.5 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.9)]">
            <span className={clsx("flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-full", tile)}>
              <Icon size={19} strokeWidth={1.75} className={ink} />
            </span>
            <span className="text-[15px] text-[#1B1840]">{label}</span>
          </li>
        ))}
      </ul>

      <p className="mt-4 px-1 text-[12.5px] leading-relaxed text-[#6E6A8F]">
        Also sounds that last a few seconds, the changes you make and what you ask the AI assistant. Moments like
        leaving the workspace are flagged for the hiring team to look at. Only during an assessment, never on a
        practice run.{" "}
        <PoliciesLink className="font-medium text-[#5B3BE0] underline-offset-2 hover:underline">Read the full list</PoliciesLink>
      </p>
    </section>
  );
}
