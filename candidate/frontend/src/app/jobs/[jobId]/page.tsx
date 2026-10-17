import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import clsx from "clsx";
import { ArrowLeft, Briefcase, CalendarDays, Clock, ExternalLink, MapPin, Users } from "lucide-react";
import { CandidateShell } from "@/components/dashboard/CandidateShell";
import { CompanyTile } from "@/components/jobs/CompanyTile";
import { ApplyPanel } from "@/components/jobs/ApplyPanel";
import { currentCandidate } from "@/lib/auth/users";
import { getProfile } from "@/lib/profile/actions";
import { getJob, listMyApplications } from "@/lib/jobs/data";
import { EMPLOYMENT_LABEL, WORK_MODE_LABEL } from "@/lib/jobs/shape";
import { display } from "@/lib/dashboard/fonts";

export const dynamic = "force-dynamic";

export async function generateMetadata({ params }: { params: Promise<{ jobId: string }> }): Promise<Metadata> {
  const job = await getJob((await params).jobId);
  return { title: job ? `${job.title} · ${job.companyName} · Mindfries` : "Job · Mindfries" };
}

const fmtDate = (iso: string) => new Date(iso).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });

/**
 * One job: what the company wrote, the facts of the role, and applying.
 * A job that's been unlisted or closed is not found — the board only shows
 * what a candidate can actually apply to.
 */
export default async function JobPage({ params }: { params: Promise<{ jobId: string }> }) {
  const { jobId } = await params;
  const [job, session, profile] = await Promise.all([getJob(jobId), currentCandidate(), getProfile()]);
  if (!job) notFound();
  const mine = session ? await listMyApplications(session) : null;
  const applied = mine?.value.find((a) => a.job.id === job.id) ?? null;

  const facts = [
    { icon: <MapPin size={18} />, label: "Location", value: [job.location, job.workMode ? WORK_MODE_LABEL[job.workMode] : null].filter(Boolean).join(" · ") || "Not said" },
    { icon: <Briefcase size={18} />, label: "Employment type", value: job.employmentType ? EMPLOYMENT_LABEL[job.employmentType] : "Not said" },
    { icon: <CalendarDays size={18} />, label: "Posted on", value: job.publishedAt ? fmtDate(job.publishedAt) : "—" },
    { icon: <Users size={18} />, label: "Applicants", value: String(job.applicants) },
  ];

  return (
    <CandidateShell sessionName={session?.name} profile={profile}>
      <Link href="/jobs" className="mt-6 inline-flex items-center gap-1.5 px-1 text-[14px] text-[#5E5A82] hover:text-[#100E2E]">
        <ArrowLeft size={16} /> All jobs
      </Link>

      <div className="mt-4 grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="min-w-0 space-y-5">
          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] sm:p-7">
            <div className="flex items-start gap-5">
              <CompanyTile name={job.companyName} size={64} />
              <div className="min-w-0">
                <h1 className={clsx(display.className, "text-[32px] leading-tight font-semibold tracking-[-0.01em] text-[#100E2E] [overflow-wrap:anywhere]")}>{job.title}</h1>
                <p className="mt-1 text-[16px] text-[#4C4874]">
                  {job.companyName}
                  {job.companyWebsite && /^https?:\/\//.test(job.companyWebsite) && (
                    <a href={job.companyWebsite} target="_blank" rel="noreferrer" className="ml-2 inline-flex items-center gap-1 text-[14px] text-[#5B3BE0] hover:underline">
                      Website <ExternalLink size={13} />
                    </a>
                  )}
                </p>
              </div>
            </div>

            <dl className="mt-6 grid gap-4 border-t border-[#ECE9F6] pt-5 sm:grid-cols-2 lg:grid-cols-4">
              {facts.map((f) => (
                <div key={f.label} className="flex items-start gap-3">
                  <span className="mt-0.5 text-[#4C4874]">{f.icon}</span>
                  <div>
                    <dt className="text-[13px] text-[#6E6A8F]">{f.label}</dt>
                    <dd className="text-[14.5px] text-[#100E2E]">{f.value}</dd>
                  </div>
                </div>
              ))}
            </dl>
          </section>

          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 sm:p-7">
            <h2 className="text-[18px] font-semibold text-[#100E2E]">About the role</h2>
            <p className="mt-3 text-[15px] leading-relaxed whitespace-pre-line text-[#2A2654]">{job.description}</p>
            {job.techStack.length > 0 && (
              <>
                <h3 className="mt-6 text-[15px] font-semibold text-[#100E2E]">Tech stack</h3>
                <div className="mt-2 flex flex-wrap gap-2">
                  {job.techStack.map((t) => (
                    <span key={t} className="rounded-lg bg-[#F1EFF8] px-3 py-1 text-[13.5px] text-[#2A2654]">
                      {t}
                    </span>
                  ))}
                </div>
              </>
            )}
          </section>

          <section className="rounded-[22px] border border-white/90 bg-white/75 p-6 sm:p-7">
            <h2 className="text-[18px] font-semibold text-[#100E2E]">How hiring works here</h2>
            <ol className="mt-3 space-y-2 text-[14.5px] text-[#2A2654]">
              <li>1. You apply, and your profile goes to the hiring team.</li>
              <li>
                2.{" "}
                {job.selfAssessment
                  ? "You take a practical assessment — open to every applicant, with a task written for you"
                  : "If they want to go further, they invite you to a practical assessment, with a task written for you"}
                {job.durationMin ? ` (about ${job.durationMin} minutes)` : ""}.
              </li>
              <li>3. Interviews, scheduled with you here and by email.</li>
            </ol>
            <p className="mt-3 flex items-center gap-1.5 text-[13px] text-[#6E6A8F]">
              <Clock size={14} /> The assessment shows how you work, not just the answer — you can try a practice run first.
            </p>
          </section>
        </div>

        <aside className="xl:sticky xl:top-6 xl:self-start">
          <ApplyPanel roleId={job.id} selfAssessment={job.selfAssessment} appliedId={applied?.id ?? null} />
        </aside>
      </div>
    </CandidateShell>
  );
}
