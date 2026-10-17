import Link from "next/link";
import clsx from "clsx";
import { ChevronRight, Mail, Send, TimerReset, type LucideIcon } from "lucide-react";
import type { Assessment, AssessmentStatus } from "@/lib/dashboard/data";
import { display } from "@/lib/dashboard/fonts";

interface CardLook {
  label: string;
  icon: LucideIcon;
  /** Card wash, icon tile, icon colour, and the wave in the corner. */
  card: string;
  tile: string;
  ink: string;
  wave: [string, string];
}

const CARDS: Record<Exclude<AssessmentStatus, "closed">, CardLook> = {
  invited: {
    label: "Open invitations",
    icon: Mail,
    card: "from-white/85 via-[#F7F5FE]/80 to-[#EFEAFD]/80",
    tile: "bg-[#E7E0FD]",
    ink: "text-[#6A47E6]",
    wave: ["#D9CEFB", "#B9A7F6"],
  },
  "in-progress": {
    label: "In progress",
    icon: TimerReset,
    card: "from-white/85 via-[#FDF3FA]/80 to-[#F8E6F4]/80",
    tile: "bg-[#FBE0F1]",
    ink: "text-[#3A2F5C]",
    wave: ["#F6D3EC", "#E8B3DD"],
  },
  submitted: {
    label: "Submitted",
    icon: Send,
    card: "from-[#EAFAF6]/85 via-white/80 to-[#EEEBFD]/80",
    tile: "bg-[#D7F3EC]",
    ink: "text-[#1E3B3A]",
    wave: ["#DCD5FB", "#C3B8F6"],
  },
};

/**
 * The three counters across the top of the dashboard. Each counts the same
 * list "Your assessments" shows, and opens the full Assessments page
 * filtered to that state — so a number here is always one click from the
 * assessments behind it.
 */
export function StatCards({ items }: { items?: Assessment[] }) {
  const count = (status: AssessmentStatus) => items?.filter((a) => a.status === status).length ?? 0;

  // Side by side from sm up; below xl each card stacks its icon over the
  // label, so a long label never runs under the arrow.
  return (
    <div className="grid gap-5 sm:grid-cols-3">
      {(Object.keys(CARDS) as (keyof typeof CARDS)[]).map((status) => {
        const look = CARDS[status];
        const Icon = look.icon;
        const value = count(status);
        return (
          <Link
            key={status}
            href={`/assessments?status=${status}`}
            className={clsx(
              "group relative flex min-h-[162px] flex-col overflow-hidden rounded-[22px] border border-white/90 bg-gradient-to-br p-6 xl:h-[162px] xl:flex-row shadow-[0_10px_30px_-18px_rgba(76,58,160,0.35)] transition-shadow hover:shadow-[0_16px_36px_-18px_rgba(76,58,160,0.5)]",
              look.card
            )}
          >
            <svg aria-hidden viewBox="0 0 200 70" preserveAspectRatio="none" className="absolute right-0 bottom-0 h-[62px] w-[62%]">
              <defs>
                <linearGradient id={`wave-${status}`} x1="0" x2="1" y1="0" y2="0">
                  <stop offset="0" stopColor={look.wave[0]} stopOpacity="0.15" />
                  <stop offset="1" stopColor={look.wave[1]} stopOpacity="0.9" />
                </linearGradient>
              </defs>
              <path d="M0 70 C 70 70, 110 10, 200 4 L 200 70 Z" fill={`url(#wave-${status})`} />
            </svg>

            <span className={clsx("flex h-[68px] w-[68px] shrink-0 items-center justify-center rounded-[18px]", look.tile)}>
              <Icon size={30} strokeWidth={1.7} className={look.ink} />
            </span>

            <div className="relative mt-4 min-w-0 flex-1 xl:mt-0 xl:ml-8 xl:pt-3">
              <p className="text-[15.5px] text-[#1B1840]">{look.label}</p>
              <p className={clsx(display.className, "mt-1 text-[56px] leading-none font-semibold tabular-nums text-[#100E2E]")}>
                {value}
              </p>
            </div>

            <span className="absolute top-6 right-6 flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-white xl:static xl:mt-4 shadow-[0_4px_14px_-4px_rgba(27,24,64,0.18)] transition-transform group-hover:translate-x-0.5">
              <ChevronRight size={20} strokeWidth={2} className="text-[#1B1840]" />
              <span className="sr-only">See {look.label.toLowerCase()}</span>
            </span>
          </Link>
        );
      })}
    </div>
  );
}
