import type { Metadata } from "next";
import { DashboardNav } from "@/components/dashboard/DashboardNav";
import { AssessmentWall } from "@/components/assessments/AssessmentWall";
import { StatusTabs } from "@/components/assessments/StatusTabs";
import {
  ASSESSMENT_STATUSES,
  UNAVAILABLE,
  sortByAttention,
  statusLabels,
  type AssessmentStatus,
} from "@/lib/dashboard/data";
import { listAssessmentsOrUndefined } from "@/lib/backend/client";
import { currentCandidate } from "@/lib/auth/users";

export const metadata: Metadata = {
  title: "Assessments · Mindfries",
  description: "Every invitation, session and result, in one place.",
};

// Same reasoning as the dashboard: assessments come from the real backend
// per request, so this can't be prerendered once at build time.
export const dynamic = "force-dynamic";

/**
 * The full wall the dashboard's "Your assessments" only previews three of.
 * Filtering by status is a real query string (?status=submitted), not client
 * state, so a filtered view is something you can bookmark or hand to
 * someone — filtered server-side, the same as the admin's Targets page.
 */
export default async function AssessmentsPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string; q?: string }>;
}) {
  const { status, q } = await searchParams;
  // The dashboard's search box lands here: a case-insensitive match on the
  // role, company, location or tags of the candidate's own assessments.
  const query = typeof q === "string" ? q.trim().slice(0, 100) : "";
  const [items, session] = await Promise.all([listAssessmentsOrUndefined(), currentCandidate()]);
  const all = sortByAttention(items ?? []);

  const counts = Object.fromEntries(
    ASSESSMENT_STATUSES.map((s) => [s, all.filter((a) => a.status === s).length])
  ) as Record<AssessmentStatus, number>;

  const active = status && ASSESSMENT_STATUSES.includes(status as AssessmentStatus) ? (status as AssessmentStatus) : undefined;
  const needle = query.toLowerCase();
  const matches = (a: (typeof all)[number]) =>
    !needle || [a.role, a.company, a.location, ...a.tags].some((field) => field?.toLowerCase().includes(needle));
  const filtered = all.filter((a) => (!active || a.status === active) && matches(a));

  return (
    <div className="min-h-full flex-1 bg-[#F6FAFD]">
      <DashboardNav sessionName={session?.name} />

      <main className="mx-auto max-w-7xl px-5 py-8 sm:px-8">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-[28px] leading-tight font-semibold tracking-tight text-[#0A1931]">
              Assessments
            </h1>
            <p className="mt-1.5 text-sm text-[#4A7FA7]">
              {query ? (
                <>
                  Results for “{query}” ·{" "}
                  <a href={active ? `/assessments?status=${active}` : "/assessments"} className="underline underline-offset-2 hover:text-[#1A3D63]">
                    clear search
                  </a>
                </>
              ) : (
                "Every invitation, session and result, in one place."
              )}
            </p>
          </div>
        </div>

        <StatusTabs active={active} counts={counts} total={all.length} />

        <div className="mt-8">
          <AssessmentWall
            items={filtered}
            emptyTitle={
              !items ? UNAVAILABLE.title : query ? `Nothing matches “${query}”` : active ? `No ${statusLabels[active].toLowerCase()} assessments` : undefined
            }
            emptyBody={
              !items ? UNAVAILABLE.body : query ? "Try a role or a company name." : active ? "Nothing here right now — check back later." : undefined
            }
          />
        </div>
      </main>
    </div>
  );
}
