import type { Metadata } from "next";
import clsx from "clsx";
import { CandidateSidebar } from "@/components/dashboard/CandidateSidebar";
import { DashboardTopBar } from "@/components/dashboard/DashboardTopBar";
import { StatCards } from "@/components/dashboard/StatCards";
import { PracticeBanner } from "@/components/dashboard/PracticeBanner";
import { DashboardAssessments } from "@/components/dashboard/DashboardAssessments";
import { RecordedPanel } from "@/components/dashboard/RecordedPanel";
import { TodayDate } from "@/components/dashboard/TodayDate";
import { EmailNotice } from "@/components/dashboard/EmailNotice";
import { listAssessmentsOrUndefined } from "@/lib/backend/client";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { emailLinksReady } from "@/lib/auth/email-links";
import { getSetupState } from "@/lib/setup-state";
import { display, ui } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Dashboard · Mindfries",
  description: "Your assessments, sessions and evidence reports.",
};

// Assessments are read per request from the shared Supabase, so this page
// can't be prerendered once at build time.
export const dynamic = "force-dynamic";

/**
 * The candidate's home, outside the workspace.
 *
 * Everything on it is the candidate's own: the counters and "Your
 * assessments" are the same real list (listAssessmentsOrUndefined), the
 * name and the role/location line come from their saved profile, and the
 * date is today in their own timezone. The only fixed copy is the quote and
 * the practice-run pitch.
 */
export default async function DashboardPage({ searchParams }: { searchParams: Promise<{ email?: string }> }) {
  const [items, session, profile, canSendMail, query] = await Promise.all([
    listAssessmentsOrUndefined(),
    currentCandidate(),
    getProfile(),
    emailLinksReady(),
    searchParams,
  ]);

  const setup = await getSetupState(session?.id);
  const displayName = profile?.name || session?.name || "there";
  const lastSpace = displayName.trimEnd().lastIndexOf(" ");
  const leading = lastSpace === -1 ? "" : displayName.slice(0, lastSpace + 1);
  const lastWord = displayName.slice(lastSpace + 1);
  // Role and location only once the candidate has saved a real profile —
  // never empty strings, never placeholders.
  const subtitle = profile?.profile_updated_at && profile.role ? [profile.role, profile.location].filter(Boolean) : null;

  return (
    <div className={clsx(ui.className, "relative isolate flex min-h-dvh flex-1 bg-[#F3F1FA] text-[#1B1840] max-lg:flex-col")}>
      {/* The soft lilac light behind everything. */}
      <div
        aria-hidden
        className="pointer-events-none fixed inset-0 -z-10"
        style={{
          background:
            "radial-gradient(40% 35% at 8% 70%, rgba(214,204,250,0.55), transparent 70%), radial-gradient(35% 30% at 92% 18%, rgba(226,214,252,0.7), transparent 70%), radial-gradient(45% 40% at 55% 0%, rgba(255,255,255,0.9), transparent 70%)",
        }}
      />

      <CandidateSidebar />

      <main className="min-w-0 flex-1 px-4 pt-5 pb-10 sm:px-8 lg:pt-6">
        <DashboardTopBar sessionName={session?.name} profile={profile} />

        {session && profile && (
          <div className="mt-6">
            <EmailNotice email={session.email} verified={!!profile.email_verified_at} canSend={canSendMail} outcome={query.email} />
          </div>
        )}

        <header className="mt-8 flex flex-col gap-8 px-1 md:mt-9 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <p className="text-[14px] font-medium tracking-[0.3em] text-[#6E6A8F] uppercase">Welcome back</p>
            <h1
              className={clsx(
                display.className,
                "mt-3 text-[46px] leading-[1.02] font-semibold tracking-[-0.02em] text-[#100E2E] [overflow-wrap:anywhere] sm:text-[72px]"
              )}
            >
              {leading}
              {/* The last word and the dot never part, so the dot can't wrap onto a line of its own. */}
              <span className="whitespace-nowrap">
                {lastWord}
                <span aria-hidden className="ml-4 inline-block h-[18px] w-[18px] rounded-full bg-gradient-to-br from-[#B9A5FB] to-[#6D4AE8] align-[0.15em]" />
              </span>
            </h1>
            {subtitle ? (
              <p className="mt-4 text-[14px] font-medium tracking-[0.3em] text-[#6E6A8F] uppercase">{subtitle.join("  ·  ")}</p>
            ) : (
              <a href="/profile" className="mt-4 inline-block text-[14px] text-[#5B3BE0] underline-offset-4 hover:underline">
                Complete your profile
              </a>
            )}
          </div>

          <div className="shrink-0 md:w-[290px] md:pt-1">
            <TodayDate className="h-6 text-[16px] text-[#5E5A82]" />
            <span className="mt-4 block h-px w-9 bg-[#B7B0D6]" />
            <p className={clsx(display.className, "mt-5 text-[21px] leading-[1.3] text-[#4C4874]")}>
              “Consistent practice today, bigger opportunities tomorrow.”
            </p>
          </div>
        </header>

        <div className="mt-8">
          <StatCards items={items} />
        </div>

        <div className="mt-5">
          <PracticeBanner environmentChecked={!!setup.environmentCheckedAt} />
        </div>

        <div className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.68fr)_minmax(0,1fr)]">
          <DashboardAssessments items={items} />
          <RecordedPanel />
        </div>
      </main>
    </div>
  );
}
