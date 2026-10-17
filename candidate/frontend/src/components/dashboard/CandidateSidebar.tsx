"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import clsx from "clsx";
import { Briefcase, ClipboardList, FileText, Home, Settings, UserRound } from "lucide-react";
import { display } from "@/lib/dashboard/fonts";

const SECTIONS = [
  { label: "Dashboard", href: "/dashboard", icon: Home },
  { label: "Jobs", href: "/jobs", icon: Briefcase },
  { label: "Applications", href: "/applications", icon: ClipboardList },
  { label: "Assessments", href: "/assessments", icon: FileText },
  { label: "Profile", href: "/profile", icon: UserRound },
  { label: "Settings", href: "/settings", icon: Settings },
];

/**
 * The logo, recoloured: the SVG's own fill is the old blue, so it is used as
 * a mask and painted with the dashboard's violet instead of being copied.
 */
export function BrandMark({ size = 34 }: { size?: number }) {
  return (
    <span
      aria-hidden
      className="inline-block shrink-0 bg-gradient-to-br from-[#8B6CF6] to-[#4B2BB8]"
      style={{
        width: size,
        height: size,
        mask: "url(/mindfries-logo.svg) center / contain no-repeat",
        WebkitMask: "url(/mindfries-logo.svg) center / contain no-repeat",
      }}
    />
  );
}

/**
 * The candidate portal's left rail: wordmark, the three sections, and a
 * quiet note at the bottom. On narrow screens it folds into a top strip
 * with the same three links, so nothing here is desktop-only.
 */
export function CandidateSidebar() {
  const pathname = usePathname();
  const isActive = (href: string) => pathname === href || pathname.startsWith(`${href}/`);

  return (
    <>
      <aside className="sticky top-0 hidden h-dvh print:!hidden w-[256px] shrink-0 flex-col border-r border-white/70 bg-white/45 px-6 pt-7 pb-6 backdrop-blur-xl lg:flex">
        <Link href="/dashboard" className="flex items-center gap-3 px-2">
          <BrandMark />
          <span className={clsx(display.className, "text-[27px] leading-none font-medium tracking-tight text-[#1B1840]")}>
            Mindfries
          </span>
        </Link>

        <nav className="mt-10 space-y-1.5" aria-label="Sections">
          {SECTIONS.map(({ label, href, icon: Icon }) => {
            const active = isActive(href);
            return (
              <Link
                key={href}
                href={href}
                aria-current={active ? "page" : undefined}
                className={clsx(
                  "flex items-center gap-4 rounded-2xl px-4 py-[17px] text-[15.5px] transition-colors",
                  active
                    ? "bg-[#EDE7FD] font-medium text-[#3D22A8] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.7)]"
                    : "text-[#1B1840] hover:bg-white/70"
                )}
              >
                <Icon size={21} strokeWidth={1.75} className={active ? "text-[#5B3BE0]" : "text-[#2A2654]"} />
                {label}
              </Link>
            );
          })}
        </nav>

        <div className="mt-auto rounded-2xl bg-gradient-to-b from-white/70 to-[#F1EDFB]/80 px-5 pt-5 pb-6 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.8)]">
          <span className="block h-4 w-4 rounded-full bg-gradient-to-br from-[#C6B5FB] to-[#6D4AE8] shadow-[0_2px_6px_rgba(109,74,232,0.45)]" />
          <p className={clsx(display.className, "mt-2 pl-7 text-[19px] leading-[1.25] text-[#4C4874]")}>
            Practice
            <br />
            Builds
            <br />
            Confidence.
          </p>
          <span className="mt-5 ml-7 block h-px w-9 bg-[#B7B0D6]" />
        </div>
      </aside>

      {/* Narrow screens: the same sections as a strip across the top. */}
      <div className="sticky top-0 z-30 border-b border-white/70 bg-white/70 backdrop-blur-xl lg:hidden print:hidden">
        <div className="flex items-center gap-2.5 px-4 pt-3">
          <BrandMark size={26} />
          <span className={clsx(display.className, "text-[21px] leading-none font-medium tracking-tight text-[#1B1840]")}>
            Mindfries
          </span>
        </div>
        <nav className="flex gap-1 overflow-x-auto px-3 pt-2 pb-2.5" aria-label="Sections">
          {SECTIONS.map(({ label, href, icon: Icon }) => {
            const active = isActive(href);
            return (
              <Link
                key={href}
                href={href}
                aria-current={active ? "page" : undefined}
                className={clsx(
                  "flex shrink-0 items-center gap-2 rounded-xl px-3 py-2 text-[13.5px]",
                  active ? "bg-[#EDE7FD] font-medium text-[#3D22A8]" : "text-[#1B1840]"
                )}
              >
                <Icon size={16} strokeWidth={1.75} />
                {label}
              </Link>
            );
          })}
        </nav>
      </div>
    </>
  );
}
