import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import { ArrowRight, Play } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { SearchBox } from "@/components/dashboard/SearchBox";
import { StatusTabs, assessmentsHref, type AssessmentView } from "@/components/assessments/StatusTabs";
import { AssessmentCard, AssessmentRow } from "@/components/assessments/AssessmentItem";
import { ASSESSMENT_STATUSES, UNAVAILABLE, sortByAttention, statusLabels, type AssessmentStatus } from "@/lib/dashboard/data";
import { listAssessmentsOrUndefined } from "@/lib/backend/client";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Assessments · Mindfries",
  description: "Every invitation, session and result, in one place.",
};

// Same reasoning as the dashboard: assessments come from the real backend
// per request, so this can't be prerendered once at build time.
export const dynamic = "force-dynamic";

/**
 * Every assessment the candidate has, filtered by status (?status=), by a
 * search over role, company, location and tags (?q= — the dashboard's
 * search box lands here), and shown as a list or a grid (?view=grid). All
 * three are the query string, filtered server-side, so any view is a URL
 * that can be bookmarked or shared.
 *
 * There is no "new assessment" here: a candidate can't create one — a
 * company invites them, or they start one from the open pool. The header's
 * button is the practice run instead, which is the one thing a candidate
 * can start on their own at any time.
 */
export default async function AssessmentsPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string; q?: string; view?: string }>;
}) {
  const { status, q, view: viewParam } = await searchParams;
  const [items, session, profile] = await Promise.all([listAssessmentsOrUndefined(), currentCandidate(), getProfile()]);
  const all = sortByAttention(items ?? []);

  const query = typeof q === "string" ? q.trim().slice(0, 100) : "";
  const view: AssessmentView = viewParam === "grid" ? "grid" : "list";
  const active = status && ASSESSMENT_STATUSES.includes(status as AssessmentStatus) ? (status as AssessmentStatus) : undefined;

  const needle = query.toLowerCase();
  const matches = (a: (typeof all)[number]) =>
    !needle || [a.role, a.company, a.location, ...a.tags].some((field) => field?.toLowerCase().includes(needle));
  const searched = all.filter(matches);
  const counts = Object.fromEntries(
    ASSESSMENT_STATUSES.map((s) => [s, searched.filter((a) => a.status === s).length])
  ) as Record<AssessmentStatus, number>;
  const shown = active ? searched.filter((a) => a.status === active) : searched;

  const empty = !items
    ? { title: UNAVAILABLE.title, body: UNAVAILABLE.body, cta: false }
    : query
      ? { title: `Nothing matches “${query}”`, body: "Try a role or a company name.", cta: false }
      : active
        ? { title: `No ${statusLabels[active].toLowerCase()} assessments`, body: "Nothing here right now — check back later.", cta: false }
        : { title: "No assessments yet", body: "Assessments you're invited to will show up here.", cta: true };

  return (
    <CandidateShell sessionName={session?.name} profile={profile} search={false}>
      <header className="mt-6 flex flex-col gap-5 px-1 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <h1 className={clsx(display.className, "text-[44px] leading-none font-medium tracking-[-0.02em] text-[#100E2E] sm:text-[52px]")}>
            Assessments
          </h1>
          <p className="mt-3 text-[15px] text-[#5E5A82]">
            {query ? (
              <>
                Results for “{query}” ·{" "}
                <Link href={assessmentsHref({ status: active, view })} className="text-[#5B3BE0] underline-offset-2 hover:underline">
                  clear search
                </Link>
              </>
            ) : (
              "Every invitation, session and result, in one place."
            )}
          </p>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row sm:items-center lg:pt-1">
          <SearchBox key={query} defaultValue={query} status={active} className="sm:w-[300px]" />
          <Link
            href="/practice"
            className="inline-flex h-[46px] shrink-0 items-center justify-center gap-2.5 rounded-2xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-5 text-[13.5px] font-semibold text-white shadow-[0_10px_24px_-10px_rgba(91,59,224,0.8)] transition-transform hover:-translate-y-px"
          >
            <Play size={15} strokeWidth={2.2} />
            Practice run
          </Link>
        </div>
      </header>

      <div className="mt-6">
        <StatusTabs active={active} counts={counts} total={searched.length} q={query} view={view} />
      </div>

      <section className="relative mt-4 overflow-hidden rounded-[22px] border border-white/90 bg-white/45 p-4 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.4)] backdrop-blur sm:p-5">
        {shown.length === 0 ? (
          <EmptyState {...empty} />
        ) : view === "grid" ? (
          <ul className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
            {shown.map((a) => (
              <AssessmentCard key={a.id} assessment={a} />
            ))}
          </ul>
        ) : (
          <ul className="space-y-3">
            {shown.map((a) => (
              <AssessmentRow key={a.id} assessment={a} />
            ))}
          </ul>
        )}
      </section>
    </CandidateShell>
  );
}

function EmptyState({ title, body, cta }: { title: string; body: string; cta: boolean }) {
  return (
    <div className="relative isolate flex min-h-[370px] flex-col items-center justify-center px-6 py-12 text-center">
      {/* Faint grid and two soft discs behind the illustration. */}
      <div
        aria-hidden
        className="absolute inset-0 -z-10 opacity-60"
        style={{
          backgroundImage:
            "linear-gradient(rgba(124,92,252,0.06) 1px, transparent 1px), linear-gradient(90deg, rgba(124,92,252,0.06) 1px, transparent 1px)",
          backgroundSize: "56px 56px",
          maskImage: "radial-gradient(60% 70% at 50% 45%, black, transparent)",
        }}
      />
      <div aria-hidden className="absolute -bottom-40 -left-32 -z-10 h-96 w-96 rounded-full bg-gradient-to-tr from-[#C9B8FA]/70 to-transparent blur-[2px]" />
      <div aria-hidden className="absolute top-10 -right-40 -z-10 h-[420px] w-[420px] rounded-full bg-gradient-to-bl from-[#E3D9FD]/80 to-transparent" />

      <div aria-hidden className="relative flex h-[200px] w-[250px] items-center justify-center">
        <span className="absolute h-[250px] w-[250px] rounded-full border border-[#E7E2F6]" />
        <span className="absolute h-[200px] w-[200px] rounded-full border border-[#EDE9F8]" />
        <span className="absolute h-[104px] w-[86px] translate-x-5 -translate-y-3 rotate-[8deg] rounded-[14px] bg-gradient-to-br from-[#C9B6FB] to-[#9C80F5]" />
        <span className="relative flex h-[104px] w-[90px] -rotate-[6deg] flex-col justify-center gap-2.5 rounded-[14px] bg-white px-5 shadow-[0_18px_34px_-14px_rgba(91,59,224,0.45)]">
          <span className="h-1.5 w-9 rounded-full bg-[#A78BFA]" />
          <span className="h-1.5 w-12 rounded-full bg-[#A78BFA]" />
          <span className="h-1.5 w-8 rounded-full bg-[#A78BFA]" />
        </span>
      </div>

      <p className={clsx(display.className, "mt-6 text-[27px] font-semibold text-[#100E2E]")}>{title}</p>
      <p className="mt-2 max-w-md text-[15px] text-[#5E5A82]">{body}</p>
      {cta && (
        <Link
          href="/practice"
          className="mt-6 inline-flex h-[42px] items-center gap-3 rounded-full bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-7 text-[13.5px] font-semibold text-white shadow-[0_12px_26px_-10px_rgba(91,59,224,0.85)] transition-transform hover:-translate-y-px"
        >
          Try a practice run
          <ArrowRight size={16} />
        </Link>
      )}
    </div>
  );
}
