import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import { ArrowRight, ChevronRight, CircleX, ClipboardList, FileText, Gift, MapPin, MessagesSquare, SearchCheck } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { CompanyTile } from "@/components/jobs/CompanyTile";
import { ApplicationDetail, type SharedDocs } from "@/components/jobs/ApplicationDetail";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile, getResumeUrl } from "@/lib/profile/actions";
import { getResumeFile, listDocuments } from "@/lib/profile/extras";
import { listMyApplications } from "@/lib/jobs/data";
import { candidateStatus, EMPLOYMENT_LABEL, STATUS_TABS, WORK_MODE_LABEL, type MyApplication, type StatusGroup } from "@/lib/jobs/shape";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Applications · Mindfries",
  description: "Your job applications, interviews and offers, in one place.",
};

export const dynamic = "force-dynamic";

const PILL: Record<StatusGroup, string> = {
  review: "bg-[#E6F1FD] text-[#1E5AA8]",
  assessment: "bg-[#EFEAFD] text-[#4B2BB8]",
  interview: "bg-[#EFEAFD] text-[#5B3BE0]",
  offer: "bg-[#E3F6EC] text-[#14693a]",
  hired: "bg-[#E3F6EC] text-[#14693a]",
  rejected: "bg-[#FDECEF] text-[#a6203c]",
  withdrawn: "bg-[#F1EFF8] text-[#4C4874]",
};

/** Applications made in the last 30 days — kept out of the component so rendering stays pure. */
function appliedInLast30Days(apps: MyApplication[]): number {
  const since = Date.now() - 30 * 86_400_000;
  return apps.filter((a) => new Date(a.createdAt).getTime() >= since).length;
}

const fmtDate = (iso: string) => new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });

/**
 * Every application the candidate has — from the job board and from
 * invitations — with where each stands. The status tab and the open
 * application are the query string (?status=, ?id=), so a view is a link.
 * Counts and percentages are of the candidate's own applications.
 */
export default async function ApplicationsPage({ searchParams }: { searchParams: Promise<{ status?: string; id?: string }> }) {
  const { status, id } = await searchParams;
  const [session, profile] = await Promise.all([currentCandidate(), getProfile()]);
  const [apps, resumeUrl, resumeFile, documents] = await Promise.all([
    session ? listMyApplications(session) : Promise.resolve({ available: false as const, value: [] as MyApplication[] }),
    getResumeUrl(profile?.resume_path || null),
    getResumeFile(profile?.resume_path),
    session ? listDocuments(session.id) : Promise.resolve({ available: false as const, items: [] as [] }),
  ]);

  const all = apps.value;
  const group = (a: MyApplication) => candidateStatus(a.stage).group;
  const count = (...groups: StatusGroup[]) => all.filter((a) => groups.includes(group(a))).length;
  const active = STATUS_TABS.some((t) => t.group === status) ? (status as StatusGroup) : null;
  const shown = active ? all.filter((a) => group(a) === active || (active === "offer" && group(a) === "hired")) : all;
  const selected = shown.find((a) => a.id === id) ?? shown[0] ?? null;

  const thisMonth = appliedInLast30Days(all);
  const pct = (n: number) => (all.length ? `${Math.round((n / all.length) * 100)}% of total` : "—");

  const stats = [
    { label: "Total applications", value: all.length, note: thisMonth ? `+${thisMonth} in the last 30 days` : "None in the last 30 days", icon: <FileText size={22} />, tint: "bg-[#E6F1FD] text-[#1E5AA8]" },
    // Same grouping as the tabs below, so a card and its tab never disagree.
    { label: "Under review", value: count("review"), note: pct(count("review")), icon: <SearchCheck size={22} />, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
    { label: "Interviews", value: count("interview"), note: pct(count("interview")), icon: <MessagesSquare size={22} />, tint: "bg-[#E6F1FD] text-[#1E5AA8]" },
    { label: "Offers", value: count("offer", "hired"), note: pct(count("offer", "hired")), icon: <Gift size={22} />, tint: "bg-[#E3F6EC] text-[#14693a]" },
    { label: "Not progressing", value: count("rejected"), note: pct(count("rejected")), icon: <CircleX size={22} />, tint: "bg-[#FDECEF] text-[#a6203c]" },
  ];

  const tabHref = (g: StatusGroup | null) => (g ? `/applications?status=${g}` : "/applications");
  const itemHref = (a: MyApplication) => `/applications?${new URLSearchParams({ ...(active ? { status: active } : {}), id: a.id }).toString()}`;

  const docs: SharedDocs = {
    resume: profile?.resume_path ? { name: resumeFile?.fileName ?? "Resume", url: resumeUrl } : null,
    documents: documents.items.map((d) => ({ id: d.id, title: d.title, kind: d.kind })),
  };

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <header className="mt-6 flex flex-col gap-4 px-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className={clsx(display.className, "text-[44px] leading-none font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[52px]")}>Applications</h1>
          <p className="mt-3 text-[15px] text-[#5E5A82]">Track your job applications, interviews and offers in one place.</p>
        </div>
        <Link
          href="/jobs"
          className="inline-flex h-12 shrink-0 items-center justify-center gap-2 rounded-2xl bg-gradient-to-r from-[#7C5CFC] to-[#5B3BE0] px-5 text-[15px] font-semibold text-white shadow-[0_10px_24px_-10px_rgba(91,59,224,0.8)] transition-transform hover:-translate-y-px"
        >
          Find more jobs <ArrowRight size={18} />
        </Link>
      </header>

      {!apps.available ? (
        <EmptyPanel title="Applications aren't set up yet" body="Once the job board is set up on the server, everything you apply to shows here." />
      ) : all.length === 0 ? (
        <EmptyPanel title="No applications yet" body="Apply to a job and it shows here, with every step — reviews, assessments, interviews and offers." cta />
      ) : (
        <>
          <div className="mt-6 grid grid-cols-2 gap-4 lg:grid-cols-5">
            {stats.map((s) => (
              <div key={s.label} className="flex items-start gap-4 rounded-[20px] border border-white/90 bg-white/75 p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)]">
                <span className={clsx("hidden h-12 w-12 shrink-0 items-center justify-center rounded-2xl sm:flex", s.tint)}>{s.icon}</span>
                <div className="min-w-0">
                  <p className="text-[14px] text-[#4C4874]">{s.label}</p>
                  <p className={clsx(display.className, "mt-1 text-[30px] leading-none font-semibold tabular-nums text-[#100E2E]")}>{s.value}</p>
                  <p className="mt-1.5 text-[12.5px] text-[#6E6A8F]">{s.note}</p>
                </div>
              </div>
            ))}
          </div>

          <div className="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)]">
            <section className="min-w-0 rounded-[22px] border border-white/90 bg-white/60 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.4)] backdrop-blur">
              <nav className="flex gap-1 overflow-x-auto overflow-y-hidden border-b border-[#ECE9F6] px-3 [scrollbar-width:none]" aria-label="Filter by status">
                <Tab href={tabHref(null)} active={!active}>
                  All ({all.length})
                </Tab>
                {STATUS_TABS.map((t) => (
                  <Tab key={t.group} href={tabHref(t.group)} active={active === t.group}>
                    {t.label} ({t.group === "offer" ? count("offer", "hired") : count(t.group)})
                  </Tab>
                ))}
              </nav>

              {shown.length === 0 ? (
                <p className="px-6 py-12 text-center text-[14.5px] text-[#6E6A8F]">Nothing here right now.</p>
              ) : (
                <ul className="divide-y divide-[#F0EEF7]">
                  {shown.map((a) => {
                    const st = candidateStatus(a.stage);
                    return (
                      <li key={a.id}>
                        <Link
                          href={itemHref(a)}
                          scroll={false}
                          aria-current={selected?.id === a.id ? "true" : undefined}
                          className={clsx("flex items-center gap-4 px-4 py-4 transition-colors sm:px-5", selected?.id === a.id ? "bg-[#F3EFFD]" : "hover:bg-white/80")}
                        >
                          <CompanyTile name={a.job.companyName} size={46} />
                          <div className="min-w-0 flex-1">
                            <p className="line-clamp-2 text-[15.5px] font-semibold text-[#100E2E] [overflow-wrap:anywhere] sm:line-clamp-1">{a.job.title}</p>
                            <p className="truncate text-[13.5px] text-[#5E5A82]">
                              {[a.job.companyName, a.job.employmentType ? EMPLOYMENT_LABEL[a.job.employmentType] : null].filter(Boolean).join(" · ")}
                            </p>
                            {/* Below 1800px the list is too narrow for its own columns; where and when go here instead. */}
                            <p className="truncate text-[12.5px] text-[#6E6A8F] min-[1800px]:hidden">
                              {[
                                `${a.source === "applied" ? "Applied" : "Invited"} ${fmtDate(a.createdAt)}`,
                                [a.job.location, a.job.workMode ? WORK_MODE_LABEL[a.job.workMode] : null].filter(Boolean).join(" · "),
                              ]
                                .filter(Boolean)
                                .join(" · ")}
                            </p>
                          </div>
                          {(a.job.location || a.job.workMode) && (
                            <span className="hidden w-[190px] shrink-0 items-center gap-1.5 truncate text-[13px] text-[#6E6A8F] min-[1800px]:flex">
                              <MapPin size={14} className="shrink-0" />
                              <span className="truncate">{[a.job.location, a.job.workMode ? WORK_MODE_LABEL[a.job.workMode] : null].filter(Boolean).join(" · ")}</span>
                            </span>
                          )}
                          <span className="hidden w-[130px] shrink-0 text-[13px] text-[#6E6A8F] min-[1800px]:block">
                            {a.source === "applied" ? "Applied" : "Invited"} {fmtDate(a.createdAt)}
                          </span>
                          <span className={clsx("shrink-0 rounded-lg px-2.5 py-1 text-[12.5px] font-medium whitespace-nowrap", PILL[st.group])}>{st.label}</span>
                          <ChevronRight size={18} className="shrink-0 text-[#1B1840]" />
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              )}
            </section>

            {selected && (
              <div className="min-w-0 xl:sticky xl:top-6 xl:self-start">
                <ApplicationDetail key={selected.id} app={selected} docs={docs} />
              </div>
            )}
          </div>
        </>
      )}
    </CandidateShell>
  );
}

function Tab({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={clsx("relative shrink-0 px-3 py-4 text-[14px] whitespace-nowrap", active ? "font-medium text-[#5B3BE0]" : "text-[#4C4874] hover:text-[#100E2E]")}
    >
      {children}
      {active && <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-full bg-[#5B3BE0]" />}
    </Link>
  );
}

function EmptyPanel({ title, body, cta = false }: { title: string; body: string; cta?: boolean }) {
  return (
    <section className="mt-6 flex flex-col items-center rounded-[22px] border border-white/90 bg-white/55 px-6 py-16 text-center">
      <span className="flex h-16 w-16 items-center justify-center rounded-full bg-[#EFEAFD] text-[#5B3BE0]">
        <ClipboardList size={28} strokeWidth={1.7} />
      </span>
      <p className={clsx(display.className, "mt-4 text-[24px] font-semibold text-[#100E2E]")}>{title}</p>
      <p className="mt-1.5 max-w-md text-[14.5px] text-[#5E5A82]">{body}</p>
      {cta && (
        <Link href="/jobs" className="mt-5 inline-flex h-11 items-center gap-2 rounded-full bg-[#5B3BE0] px-6 text-[14px] font-semibold text-white hover:bg-[#4B2BB8]">
          Browse jobs <ArrowRight size={16} />
        </Link>
      )}
    </section>
  );
}
