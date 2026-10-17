import type { Metadata } from "next";
import Link from "next/link";
import clsx from "clsx";
import { Briefcase, CheckCircle2, ChevronRight, MapPin, Search, Users } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { CompanyTile } from "@/components/jobs/CompanyTile";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { listJobs, myAppliedJobIds } from "@/lib/jobs/data";
import { EMPLOYMENT_LABEL, WORK_MODE_LABEL, type EmploymentType, type WorkMode } from "@/lib/jobs/shape";
import { display } from "@/lib/dashboard/fonts";

export const metadata: Metadata = {
  title: "Jobs · Mindfries",
  description: "Roles companies on Mindfries are hiring for.",
};

export const dynamic = "force-dynamic";

const fmtDate = (iso: string) => new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });

/**
 * The job board: roles companies on Mindfries have listed, while they're
 * open. Search and the two filters are the query string, so a filtered
 * board is a link. Nothing here is a scraped or sample listing — an empty
 * board says it's empty.
 */
export default async function JobsPage({ searchParams }: { searchParams: Promise<{ q?: string; mode?: string; type?: string }> }) {
  const { q, mode, type } = await searchParams;
  const query = typeof q === "string" ? q.trim().slice(0, 100) : "";
  const workMode = mode && mode in WORK_MODE_LABEL ? (mode as WorkMode) : undefined;
  const employmentType = type && type in EMPLOYMENT_LABEL ? (type as EmploymentType) : undefined;

  const [session, profile] = await Promise.all([currentCandidate(), getProfile()]);
  const [jobs, applied] = await Promise.all([
    listJobs({ q: query, workMode, employmentType }),
    session ? myAppliedJobIds(session) : Promise.resolve(new Set<string>()),
  ]);

  const href = (patch: { q?: string; mode?: string; type?: string }) => {
    const p = new URLSearchParams();
    const next = { q: query, mode: workMode, type: employmentType, ...patch };
    if (next.q) p.set("q", next.q);
    if (next.mode) p.set("mode", next.mode);
    if (next.type) p.set("type", next.type);
    const s = p.toString();
    return s ? `/jobs?${s}` : "/jobs";
  };

  return (
    <CandidateShell sessionName={session?.name} profile={profile} search={false}>
      <header className="mt-6 flex flex-col gap-5 px-1 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className={clsx(display.className, "text-[44px] leading-none font-semibold tracking-[-0.02em] text-[#100E2E] sm:text-[52px]")}>Jobs</h1>
          <p className="mt-3 text-[15px] text-[#5E5A82]">Roles companies on Mindfries are hiring for. Apply, and the hiring team takes it from there.</p>
        </div>
        <form action="/jobs" method="get" role="search" className="w-full lg:w-[360px]">
          {workMode && <input type="hidden" name="mode" value={workMode} />}
          {employmentType && <input type="hidden" name="type" value={employmentType} />}
          <label className="flex h-[46px] items-center gap-3 rounded-2xl border border-white/80 bg-white/70 px-4 focus-within:border-[#C9BCF7]">
            <Search size={19} className="shrink-0 text-[#1B1840]" />
            <span className="sr-only">Search jobs</span>
            <input
              type="search"
              name="q"
              defaultValue={query}
              maxLength={100}
              placeholder="Role, company, skill or place"
              className="min-w-0 flex-1 bg-transparent text-[14px] text-[#1B1840] outline-none placeholder:text-[#8A86A8]"
            />
          </label>
        </form>
      </header>

      <div className="mt-6 flex flex-wrap gap-2 px-1" aria-label="Filters">
        <FilterChip href={href({ mode: undefined })} active={!workMode}>
          Any work mode
        </FilterChip>
        {(Object.keys(WORK_MODE_LABEL) as WorkMode[]).map((m) => (
          <FilterChip key={m} href={href({ mode: workMode === m ? undefined : m })} active={workMode === m}>
            {WORK_MODE_LABEL[m]}
          </FilterChip>
        ))}
        <span className="mx-1 hidden w-px bg-[#E4E0F2] sm:block" />
        <FilterChip href={href({ type: undefined })} active={!employmentType}>
          Any type
        </FilterChip>
        {(Object.keys(EMPLOYMENT_LABEL) as EmploymentType[]).map((t) => (
          <FilterChip key={t} href={href({ type: employmentType === t ? undefined : t })} active={employmentType === t}>
            {EMPLOYMENT_LABEL[t]}
          </FilterChip>
        ))}
      </div>

      <section className="mt-5 rounded-[22px] border border-white/90 bg-white/55 p-3 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.4)] backdrop-blur sm:p-4">
        {!jobs.available ? (
          <Empty title="The job board isn't open yet" body="Jobs will appear here once the board is set up on the server." />
        ) : jobs.value.length === 0 ? (
          query || workMode || employmentType ? (
            <Empty title="No jobs match" body="Try fewer filters, or another word." action={{ href: "/jobs", label: "Clear the filters" }} />
          ) : (
            <Empty title="No jobs on the board yet" body="When companies on Mindfries list a role, it shows here. Invitations you get still arrive on your dashboard." />
          )
        ) : (
          <ul className="space-y-2">
            {jobs.value.map((job) => (
              <li key={job.id}>
                <Link href={`/jobs/${job.id}`} className="group flex items-center gap-4 rounded-2xl bg-white/75 p-4 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.95)] transition-colors hover:bg-white sm:p-5">
                  <CompanyTile name={job.companyName} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[16.5px] font-semibold text-[#100E2E] group-hover:underline">{job.title}</p>
                    <p className="truncate text-[14px] text-[#5E5A82]">
                      {[job.companyName, job.employmentType ? EMPLOYMENT_LABEL[job.employmentType] : null].filter(Boolean).join(" · ")}
                    </p>
                    <p className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-[#6E6A8F]">
                      {(job.location || job.workMode) && (
                        <span className="inline-flex items-center gap-1.5">
                          <MapPin size={14} /> {[job.location, job.workMode ? WORK_MODE_LABEL[job.workMode] : null].filter(Boolean).join(" · ")}
                        </span>
                      )}
                      {job.publishedAt && <span>Posted {fmtDate(job.publishedAt)}</span>}
                      <span className="inline-flex items-center gap-1.5">
                        <Users size={14} /> {job.applicants} {job.applicants === 1 ? "applicant" : "applicants"}
                      </span>
                    </p>
                  </div>
                  {applied.has(job.id) ? (
                    <span className="hidden shrink-0 items-center gap-1.5 rounded-lg bg-[#E3F6EC] px-3 py-1.5 text-[13px] font-medium text-[#14693a] sm:inline-flex">
                      <CheckCircle2 size={15} /> Applied
                    </span>
                  ) : (
                    job.selfAssessment && (
                      <span className="hidden shrink-0 rounded-lg bg-[#EEEBFA] px-3 py-1.5 text-[13px] font-medium text-[#4B2BB8] sm:inline">Assessment open to all</span>
                    )
                  )}
                  <ChevronRight size={20} className="shrink-0 text-[#1B1840]" />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </CandidateShell>
  );
}

function FilterChip({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={active ? "true" : undefined}
      className={clsx(
        "rounded-xl px-3.5 py-2 text-[13.5px] transition-colors",
        active ? "bg-[#EDE7FD] font-medium text-[#3D22A8]" : "bg-white/60 text-[#2A2654] hover:bg-white"
      )}
    >
      {children}
    </Link>
  );
}

function Empty({ title, body, action }: { title: string; body: string; action?: { href: string; label: string } }) {
  return (
    <div className="flex flex-col items-center px-6 py-16 text-center">
      <span className="flex h-16 w-16 items-center justify-center rounded-full bg-[#EFEAFD] text-[#5B3BE0]">
        <Briefcase size={28} strokeWidth={1.7} />
      </span>
      <p className={clsx(display.className, "mt-4 text-[24px] font-semibold text-[#100E2E]")}>{title}</p>
      <p className="mt-1.5 max-w-md text-[14.5px] text-[#5E5A82]">{body}</p>
      {action && (
        <Link href={action.href} className="mt-4 text-[14px] font-semibold text-[#5B3BE0] hover:underline">
          {action.label}
        </Link>
      )}
    </div>
  );
}
