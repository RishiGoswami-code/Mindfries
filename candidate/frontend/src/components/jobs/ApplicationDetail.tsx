"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import clsx from "clsx";
import {
  Briefcase,
  CalendarClock,
  CalendarDays,
  Check,
  CheckCircle2,
  Circle,
  ExternalLink,
  FileText,
  Loader2,
  Mail,
  MapPin,
  Paperclip,
  Users,
  X,
} from "lucide-react";
import { CompanyTile } from "./CompanyTile";
import { LocalTime } from "./LocalTime";
import { respondToInterview, withdrawApplication, type InterviewResponse } from "@/lib/jobs/actions";
import {
  buildTimeline,
  canWithdraw,
  EMPLOYMENT_LABEL,
  headline,
  interviewOpen,
  WORK_MODE_LABEL,
  type Interview,
  type MyApplication,
  type TimelineStep,
} from "@/lib/jobs/shape";

type Tab = "overview" | "timeline" | "messages" | "documents";

/** What goes with every application: the resume and supporting documents from the profile. */
export interface SharedDocs {
  resume: { name: string; url: string | null } | null;
  documents: { id: string; title: string; kind: string }[];
}


/**
 * One application, beside the list: the job, where it stands, its timeline,
 * the interviews to answer, the emails the hiring team sent, and what was
 * shared with them. Rendered in the browser so every time is the
 * candidate's own local time.
 */
export function ApplicationDetail({ app, docs }: { app: MyApplication; docs: SharedDocs }) {
  const [tab, setTab] = useState<Tab>("overview");
  const banner = headline(app);
  const steps = buildTimeline(app);
  const openInterviews = app.interviews.filter((iv) => interviewOpen(iv)).sort((a, b) => a.startsAt.localeCompare(b.startsAt));

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "timeline", label: "Timeline" },
    { id: "messages", label: app.emails.length ? `Messages (${app.emails.length})` : "Messages" },
    { id: "documents", label: "Documents" },
  ];

  return (
    <section className="rounded-[22px] border border-white/90 bg-white/75 p-5 shadow-[0_10px_30px_-24px_rgba(76,58,160,0.45)] backdrop-blur sm:p-6">
      <div className="flex items-start gap-4">
        <CompanyTile name={app.job.companyName} size={64} />
        <div className="min-w-0 flex-1">
          <h2 className="text-[20px] leading-tight font-semibold text-[#100E2E] [overflow-wrap:anywhere]">{app.job.title}</h2>
          <p className="mt-0.5 text-[15px] text-[#4C4874]">
            {[app.job.companyName, app.job.employmentType ? EMPLOYMENT_LABEL[app.job.employmentType] : null].filter(Boolean).join(" · ")}
          </p>
          {(app.job.location || app.job.workMode) && (
            <p className="mt-1 flex items-center gap-1.5 text-[13.5px] text-[#6E6A8F]">
              <MapPin size={14} /> {[app.job.location, app.job.workMode ? WORK_MODE_LABEL[app.job.workMode] : null].filter(Boolean).join(" · ")}
            </p>
          )}
        </div>
        {app.jobListed && (
          <Link href={`/jobs/${app.job.id}`} className="inline-flex shrink-0 items-center gap-1.5 rounded-xl bg-[#5B3BE0] px-3.5 py-2 text-[13.5px] font-semibold text-white hover:bg-[#4B2BB8]">
            View job <ExternalLink size={14} />
          </Link>
        )}
      </div>

      <div role="tablist" className="-mx-1 mt-5 flex gap-1 overflow-x-auto overflow-y-hidden border-b border-[#ECE9F6] [scrollbar-width:none]">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            type="button"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={clsx("relative shrink-0 px-4 py-3 text-[14.5px]", tab === t.id ? "font-medium text-[#5B3BE0]" : "text-[#4C4874] hover:text-[#100E2E]")}
          >
            {t.label}
            {tab === t.id && <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-full bg-[#5B3BE0]" />}
          </button>
        ))}
      </div>

      <div className="mt-5 space-y-5">
        {tab === "overview" && (
          <>
            {banner && (
              <div className="flex gap-3 rounded-2xl bg-[#F1EDFC] p-4">
                <CalendarClock size={22} className="mt-0.5 shrink-0 text-[#5B3BE0]" />
                <div>
                  <p className="text-[15px] font-semibold text-[#100E2E]">{banner.title}</p>
                  <p className="text-[13.5px] text-[#5E5A82]">{banner.body}</p>
                </div>
              </div>
            )}
            {openInterviews.map((iv) => (
              <InterviewCard key={iv.id} interview={iv} />
            ))}
            <Timeline steps={steps} />
            <JobFacts app={app} />
            <Withdraw app={app} />
          </>
        )}

        {tab === "timeline" && (
          <>
            <Timeline steps={steps} />
            {app.interviews.length > 0 && (
              <div className="space-y-3">
                <h3 className="text-[15px] font-semibold text-[#100E2E]">Interviews</h3>
                {app.interviews.map((iv) => (
                  <InterviewCard key={iv.id} interview={iv} readOnly={!interviewOpen(iv)} />
                ))}
              </div>
            )}
            {app.coverNote && (
              <div>
                <h3 className="text-[15px] font-semibold text-[#100E2E]">Your note</h3>
                <p className="mt-1.5 text-[14px] whitespace-pre-line text-[#2A2654]">{app.coverNote}</p>
              </div>
            )}
          </>
        )}

        {tab === "messages" &&
          (app.emails.length === 0 ? (
            <p className="text-[14px] text-[#6E6A8F]">
              No messages yet. When the hiring team writes to you, it arrives by email and shows here too — reply from your inbox and it goes
              straight to them.
            </p>
          ) : (
            <ul className="divide-y divide-[#F0EEF7]">
              {app.emails.map((m) => (
                <li key={m.id} className="py-4 first:pt-0">
                  <div className="flex items-start gap-3">
                    <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-[#F1EDFC] text-[#5B3BE0]">
                      <Mail size={16} />
                    </span>
                    <div className="min-w-0">
                      <p className="text-[15px] font-semibold text-[#100E2E]">{m.subject}</p>
                      <p className="text-[12.5px] text-[#6E6A8F]">
                        {m.fromName ? `${m.fromName}, ${app.job.companyName}` : app.job.companyName} · <LocalTime iso={m.sentAt} />
                      </p>
                      <p className="mt-2 text-[14px] leading-relaxed whitespace-pre-line text-[#2A2654]">{m.body}</p>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          ))}

        {tab === "documents" && (
          <div>
            <p className="text-[13.5px] text-[#5E5A82]">What the hiring team sees from your profile, with this application and every other.</p>
            <ul className="mt-3 divide-y divide-[#F0EEF7] rounded-2xl border border-[#ECE9F6] bg-white px-4">
              <li className="flex items-center gap-3 py-3.5">
                <FileText size={18} className="text-[#5B3BE0]" />
                {docs.resume ? (
                  docs.resume.url ? (
                    <a href={docs.resume.url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate text-[14.5px] font-medium text-[#100E2E] hover:underline">
                      {docs.resume.name}
                    </a>
                  ) : (
                    <span className="min-w-0 flex-1 truncate text-[14.5px] text-[#100E2E]">{docs.resume.name}</span>
                  )
                ) : (
                  <span className="flex-1 text-[14.5px] text-[#6E6A8F]">No resume yet</span>
                )}
                <span className="text-[12.5px] text-[#6E6A8F]">Resume</span>
              </li>
              {docs.documents.map((d) => (
                <li key={d.id} className="flex items-center gap-3 py-3.5">
                  <Paperclip size={18} className="text-[#5B3BE0]" />
                  <span className="min-w-0 flex-1 truncate text-[14.5px] text-[#100E2E]">{d.title}</span>
                  <span className="text-[12.5px] text-[#6E6A8F] capitalize">{d.kind}</span>
                </li>
              ))}
            </ul>
            <Link href="/profile" className="mt-3 inline-block text-[14px] font-semibold text-[#5B3BE0] hover:underline">
              Manage them on your profile →
            </Link>
          </div>
        )}
      </div>
    </section>
  );
}

function Timeline({ steps }: { steps: TimelineStep[] }) {
  return (
    <ol className="relative">
      {steps.map((s, i) => (
        <li key={s.key} className="relative flex gap-3 pb-4 last:pb-0">
          {i < steps.length - 1 && <span aria-hidden className="absolute top-6 bottom-0 left-[11px] w-px bg-[#E4E0F2]" />}
          <span className="relative z-10 mt-0.5 shrink-0">
            {s.state === "done" ? (
              <span className="flex h-[23px] w-[23px] items-center justify-center rounded-full bg-[#5B3BE0] text-white">
                <Check size={14} strokeWidth={3} />
              </span>
            ) : s.state === "current" ? (
              <span className="flex h-[23px] w-[23px] items-center justify-center rounded-full border-2 border-[#5B3BE0] bg-white">
                <span className="h-2.5 w-2.5 rounded-full bg-[#5B3BE0]" />
              </span>
            ) : s.state === "skipped" ? (
              <span className="flex h-[23px] w-[23px] items-center justify-center rounded-full bg-[#EEECF4] text-[#8A86A8]">
                <X size={13} strokeWidth={2.5} />
              </span>
            ) : (
              <Circle size={23} strokeWidth={1.5} className="text-[#C9C4DE]" />
            )}
          </span>
          <div className="min-w-0 flex-1">
            <p className={clsx("text-[14.5px]", s.state === "upcoming" || s.state === "skipped" ? "text-[#6E6A8F]" : "font-medium text-[#100E2E]")}>{s.title}</p>
            <p className="text-[12.5px] text-[#6E6A8F]">{s.at ? <LocalTime iso={s.at} /> : s.state === "upcoming" ? "Not yet" : ""}</p>
          </div>
          {s.tag && (
            <span
              className={clsx(
                "h-fit shrink-0 rounded-lg px-2.5 py-1 text-[12px] font-medium",
                s.state === "current" ? "bg-[#EEEBFA] text-[#4B2BB8]" : s.state === "done" ? "bg-[#E3F6EC] text-[#14693a]" : "bg-[#F1EFF8] text-[#4C4874]"
              )}
            >
              {s.tag}
            </span>
          )}
        </li>
      ))}
    </ol>
  );
}

function JobFacts({ app }: { app: MyApplication }) {
  const facts = [
    { icon: <MapPin size={17} />, label: "Location", value: [app.job.location, app.job.workMode ? WORK_MODE_LABEL[app.job.workMode] : null].filter(Boolean).join(" · ") || "—" },
    { icon: <Briefcase size={17} />, label: "Employment type", value: app.job.employmentType ? EMPLOYMENT_LABEL[app.job.employmentType] : "—" },
    { icon: <CalendarDays size={17} />, label: "Posted on", value: app.job.publishedAt ? <LocalTime iso={app.job.publishedAt} format="date" /> : "—" },
    { icon: <Users size={17} />, label: "Applicants", value: String(app.job.applicants) },
  ];
  return (
    <div className="border-t border-[#ECE9F6] pt-4">
      <h3 className="text-[15px] font-semibold text-[#100E2E]">Job details</h3>
      <dl className="mt-3 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {facts.map((f) => (
          <div key={f.label} className="flex items-start gap-2">
            <span className="mt-0.5 text-[#4C4874]">{f.icon}</span>
            <div className="min-w-0">
              <dt className="text-[12px] text-[#6E6A8F]">{f.label}</dt>
              <dd className="text-[13.5px] text-[#100E2E]">{f.value}</dd>
            </div>
          </div>
        ))}
      </dl>
    </div>
  );
}

/** An interview: when and where, and the candidate's answer — confirm, ask for another time, or cancel. */
function InterviewCard({ interview: iv, readOnly = false }: { interview: Interview; readOnly?: boolean }) {
  const router = useRouter();
  const [mode, setMode] = useState<InterviewResponse | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();
  const isLink = iv.location ? /^https?:\/\//.test(iv.location) : false;

  function send(response: InterviewResponse) {
    start(async () => {
      setError(null);
      try {
        const res = await respondToInterview(iv.id, response, message);
        if (res.ok) {
          setMode(null);
          setMessage("");
          router.refresh();
        } else setError(res.error);
      } catch {
        setError("Couldn't reach the server — try again.");
      }
    });
  }

  const statusText: Record<Interview["status"], string> = {
    scheduled: "Waiting for you to confirm",
    confirmed: "You confirmed",
    reschedule_requested: "You asked for another time",
    cancelled_by_candidate: "You cancelled",
    cancelled: "Cancelled by the company",
    completed: "Done",
  };

  return (
    <div className="rounded-2xl border border-[#ECE9F6] bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="text-[15px] font-semibold text-[#100E2E]">{iv.title}</p>
          <p className="text-[13.5px] text-[#4C4874]">
            <LocalTime iso={iv.startsAt} /> · {iv.durationMin} min
          </p>
        </div>
        <span className={clsx("rounded-lg px-2.5 py-1 text-[12px] font-medium", iv.status === "confirmed" ? "bg-[#E3F6EC] text-[#14693a]" : "bg-[#F1EFF8] text-[#4C4874]")}>
          {statusText[iv.status]}
        </span>
      </div>
      {iv.location && (
        <p className="mt-2 flex items-center gap-1.5 text-[13.5px] text-[#2A2654]">
          <MapPin size={14} className="shrink-0" />
          {isLink ? (
            <a href={iv.location} target="_blank" rel="noreferrer" className="truncate text-[#5B3BE0] hover:underline">
              {iv.location}
            </a>
          ) : (
            <span className="truncate">{iv.location}</span>
          )}
        </p>
      )}
      {iv.notes && <p className="mt-2 text-[13.5px] whitespace-pre-line text-[#5E5A82]">{iv.notes}</p>}
      {iv.candidateMessage && <p className="mt-2 text-[13px] text-[#6E6A8F]">You said: &ldquo;{iv.candidateMessage}&rdquo;</p>}

      {!readOnly && (
        <>
          {mode === null ? (
            <div className="mt-3 grid gap-2 sm:grid-cols-3">
              {iv.status !== "confirmed" && (
                <button type="button" disabled={pending} onClick={() => send("confirm")} className="inline-flex h-10 items-center justify-center gap-1.5 rounded-xl bg-[#5B3BE0] text-[13.5px] font-semibold text-white hover:bg-[#4B2BB8] disabled:opacity-60">
                  {pending ? <Loader2 size={15} className="animate-spin" /> : <CheckCircle2 size={15} />} Confirm
                </button>
              )}
              <button type="button" onClick={() => setMode("reschedule")} className="inline-flex h-10 items-center justify-center gap-1.5 rounded-xl border border-[#D9D1F7] bg-[#F6F4FC] text-[13.5px] font-semibold text-[#4B2BB8] hover:bg-[#EEEBFA]">
                <CalendarClock size={15} /> Reschedule
              </button>
              <button type="button" onClick={() => setMode("cancel")} className="inline-flex h-10 items-center justify-center gap-1.5 rounded-xl border border-[#E4E0F2] bg-white text-[13.5px] font-semibold text-[#2A2654] hover:bg-[#F6F4FC]">
                <X size={15} /> Cancel interview
              </button>
            </div>
          ) : (
            <div className="mt-3 space-y-2">
              <label className="block text-[13px] font-medium text-[#2A2654]">
                {mode === "reschedule" ? "Which days or times would suit you?" : "Anything to tell them? (optional)"}
                <textarea
                  value={message}
                  onChange={(e) => setMessage(e.target.value.slice(0, 1000))}
                  rows={3}
                  className="mt-1 w-full rounded-xl border border-[#DCD7EE] bg-white px-3 py-2 text-[14px] font-normal outline-none focus:border-[#7C5CFC]"
                />
              </label>
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setMode(null)} className="h-9 rounded-lg px-3 text-[13.5px] text-[#4C4874] hover:bg-[#F1EFF8]">
                  Back
                </button>
                <button
                  type="button"
                  disabled={pending || (mode === "reschedule" && message.trim().length < 3)}
                  onClick={() => send(mode)}
                  className={clsx("inline-flex h-9 items-center gap-1.5 rounded-lg px-4 text-[13.5px] font-semibold text-white disabled:opacity-60", mode === "cancel" ? "bg-[#a6203c]" : "bg-[#5B3BE0]")}
                >
                  {pending && <Loader2 size={14} className="animate-spin" />}
                  {mode === "reschedule" ? "Ask for another time" : "Cancel the interview"}
                </button>
              </div>
            </div>
          )}
          {error && <p className="mt-2 text-[13px] text-[#a6203c]">{error}</p>}
        </>
      )}
    </div>
  );
}

function Withdraw({ app }: { app: MyApplication }) {
  const router = useRouter();
  const [asking, setAsking] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, start] = useTransition();
  if (!canWithdraw(app)) return null;

  return (
    <div className="border-t border-[#ECE9F6] pt-4">
      {!asking ? (
        <button type="button" onClick={() => setAsking(true)} className="text-[13.5px] font-medium text-[#6E6A8F] hover:text-[#a6203c]">
          Withdraw this application
        </button>
      ) : (
        <div className="space-y-2 rounded-2xl bg-[#FDF1F3] p-4">
          <p className="text-[14px] text-[#7A1F33]">
            Withdraw from {app.job.title} at {app.job.companyName}? It can&apos;t be undone
            {app.assessment && app.assessment.status === "invited" ? ", and the assessment closes with it" : ""}.
          </p>
          <input
            value={reason}
            onChange={(e) => setReason(e.target.value.slice(0, 500))}
            placeholder="Reason (optional — the hiring team sees it)"
            className="h-10 w-full rounded-lg border border-[#F2C9D2] bg-white px-3 text-[14px] outline-none"
          />
          <div className="flex justify-end gap-2">
            <button type="button" onClick={() => setAsking(false)} className="h-9 rounded-lg px-3 text-[13.5px] text-[#7A1F33] hover:bg-white/70">
              Keep it
            </button>
            <button
              type="button"
              disabled={pending}
              onClick={() =>
                start(async () => {
                  setError(null);
                  try {
                    const res = await withdrawApplication(app.id, reason);
                    if (res.ok) router.refresh();
                    else setError(res.error);
                  } catch {
                    setError("Couldn't reach the server — try again.");
                  }
                })
              }
              className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-[#a6203c] px-4 text-[13.5px] font-semibold text-white disabled:opacity-60"
            >
              {pending && <Loader2 size={14} className="animate-spin" />}
              Withdraw
            </button>
          </div>
          {error && <p className="text-[13px] text-[#a6203c]">{error}</p>}
        </div>
      )}
    </div>
  );
}
