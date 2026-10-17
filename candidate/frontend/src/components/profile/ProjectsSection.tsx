"use client";

import { useState } from "react";
import clsx from "clsx";
import { Activity, BadgeCheck, Code2, ExternalLink, FolderGit2, GitPullRequest, MessageSquareText, RefreshCw, Star, Upload, CircleDot } from "lucide-react";
import { refreshKnowledge } from "@/lib/profile/actions";
import { PLATFORMS } from "@/lib/profile/links";
import { activeLabel, ago, summarizeKnowledge, type SummarySource } from "@/lib/profile/knowledge-summary";
import { usePreviewMode } from "./PreviewMode";
import { useProfileSections } from "./ProfileCard";

type Tab = "overview" | "projects" | "languages" | "activity";

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/**
 * The Projects & activity window: what the candidate's linked GitHub and
 * GitLab accounts show, together — their own projects, the languages
 * across them, and recent public activity. All of it was read from the
 * platforms (lib/profile/knowledge.ts); none of it is typed in.
 *
 * Activity is counts (pushes, pull requests, reviews, issues) over the
 * window the platform reports, not a feed of individual events: that's
 * what is stored. Private projects are counted, never named.
 */
export function ProjectsSection({ items }: { items: SummarySource[] }) {
  const preview = usePreviewMode();
  const { setOpen } = useProfileSections();
  const [tab, setTab] = useState<Tab>("overview");
  const [busy, setBusy] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  if (items.length === 0) {
    return (
      <div className="rounded-2xl border-2 border-dashed border-[#DCD5F3] bg-white/60 px-6 py-12 text-center">
        <span className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-[#EFEAFD] text-[#5B3BE0]">
          <FolderGit2 size={28} strokeWidth={1.7} />
        </span>
        <p className="mt-4 text-[17px] font-semibold text-[#100E2E]">Nothing read yet</p>
        <p className="mx-auto mt-1 max-w-md text-[14px] text-[#5E5A82]">
          Link a GitHub or GitLab account and your own projects, the languages they&apos;re in and your recent activity appear
          here — read from the platform, not typed in.
        </p>
        {!preview && (
          <button
            type="button"
            onClick={() => setOpen("linked")}
            className="mt-5 inline-flex h-11 items-center rounded-xl bg-[#5B3BE0] px-5 text-[14.5px] font-semibold text-white hover:bg-[#4B2BB8]"
          >
            Link an account
          </button>
        )}
      </div>
    );
  }

  const s = summarizeKnowledge(items);
  const totalProjects = s.publicProjects + s.privateProjects;

  async function refresh(source: string) {
    setBusy(source);
    setNotice(null);
    try {
      const res = await refreshKnowledge(source);
      if (!res.ok) setNotice(res.error || "Couldn't refresh.");
    } catch {
      setNotice("Couldn't reach the server — check your connection and try again.");
    } finally {
      setBusy(null);
    }
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: "Overview" },
    { id: "projects", label: `Projects (${s.projects.length})` },
    { id: "languages", label: `Languages (${s.languages.length})` },
    { id: "activity", label: "Recent activity" },
  ];

  return (
    <div>
      <div role="tablist" className="-mx-1 flex gap-1 overflow-x-auto overflow-y-hidden border-b border-[#ECE9F6]">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            type="button"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={clsx(
              "relative shrink-0 px-4 py-3 text-[15px] transition-colors",
              tab === t.id ? "font-medium text-[#5B3BE0]" : "text-[#4C4874] hover:text-[#100E2E]"
            )}
          >
            {t.label}
            {tab === t.id && <span className="absolute inset-x-2 -bottom-px h-[3px] rounded-full bg-[#5B3BE0]" />}
          </button>
        ))}
      </div>

      <div className="mt-5">
        {tab === "overview" && (
          <div className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-3">
              <Stat icon={<FolderGit2 size={24} strokeWidth={1.7} />} value={String(totalProjects)} label={totalProjects === 1 ? "Project" : "Projects"}>
                {s.publicProjects} public{s.privateProjects > 0 ? ` · ${s.privateProjects} private` : ""}
              </Stat>
              <Stat icon={<Code2 size={24} strokeWidth={1.7} />} value={String(s.languages.length)} label={s.languages.length === 1 ? "Language" : "Languages"}>
                {s.languages.slice(0, 3).map((l) => l.name).join(", ") || "None detected"}
              </Stat>
              <Stat
                icon={<Activity size={24} strokeWidth={1.7} />}
                value={activeLabel(s.lastActiveAt)?.replace(/^Active /, "") ?? "—"}
                label={s.lastActiveAt ? (activeLabel(s.lastActiveAt)?.startsWith("Active") ? "Active" : "Last activity") : "Activity"}
                small
              >
                {s.lastActiveAt ? plural(s.activity.events, "public event") : s.activityHidden ? "Not public" : "No recent activity"}
              </Stat>
            </div>

            <div className="grid gap-5 lg:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
              <Panel title="Top projects" onViewAll={s.projects.length > 4 ? () => setTab("projects") : undefined}>
                <ProjectList projects={s.projects.slice(0, 4)} />
              </Panel>
              <div className="space-y-5">
                <Panel title="Languages" onViewAll={s.languages.length > 3 ? () => setTab("languages") : undefined}>
                  <LanguageBars languages={s.languages.slice(0, 3)} />
                </Panel>
                <Panel title="Recent activity" onViewAll={() => setTab("activity")}>
                  <ActivityCounts s={s} compact />
                </Panel>
              </div>
            </div>
          </div>
        )}

        {tab === "projects" && (
          <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
            <ProjectList projects={s.projects} />
            {totalProjects > s.projects.length && (
              <p className="mt-4 text-[13px] text-[#6E6A8F]">
                Showing the {s.projects.length} most recently worked on of {plural(totalProjects, "project")}
                {s.privateProjects > 0 ? ` — private ones are counted, never named.` : "."}
              </p>
            )}
          </div>
        )}

        {tab === "languages" && (
          <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
            <LanguageBars languages={s.languages} />
            <p className="mt-4 text-[13px] text-[#6E6A8F]">Each language&apos;s share of your projects — a project counts for its main language.</p>
          </div>
        )}

        {tab === "activity" && (
          <div className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
            <ActivityCounts s={s} />
          </div>
        )}

        <div className="mt-5 rounded-[18px] border border-[#ECE9F6] bg-white p-5">
          <p className="text-[14px] font-semibold text-[#100E2E]">Read from</p>
          <ul className="mt-2 space-y-2">
            {items.map((item) => (
              <li key={item.source} className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[13.5px] text-[#4C4874]">
                <span className="font-medium text-[#100E2E]">
                  {PLATFORMS[item.source].label} · {item.handle}
                </span>
                {item.verified ? (
                  <span className="inline-flex items-center gap-1 text-[#14693a]">
                    <BadgeCheck size={14} /> yours
                  </span>
                ) : (
                  <span>ownership not confirmed</span>
                )}
                <span className="text-[#6E6A8F]">read {ago(item.fetchedAt)}</span>
                {!preview && (
                  <button
                    type="button"
                    onClick={() => void refresh(item.source)}
                    disabled={busy !== null}
                    className="ml-auto inline-flex items-center gap-1 text-[13px] font-medium text-[#4B2BB8] hover:underline disabled:opacity-50"
                  >
                    <RefreshCw size={13} className={busy === item.source ? "animate-spin" : ""} /> Refresh
                  </button>
                )}
              </li>
            ))}
          </ul>
          {notice && <p className="mt-2 text-[13px] text-[#a6203c]">{notice}</p>}
          <p className="mt-3 text-[12.5px] leading-relaxed text-[#6E6A8F]">
            An account you signed in to is marked as yours; one linked only by username shows the account exists, not that
            it&apos;s yours — and hiring teams are told which.
          </p>
        </div>
      </div>
    </div>
  );
}

function Stat({ icon, value, label, small = false, children }: { icon: React.ReactNode; value: string; label: string; small?: boolean; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-4 rounded-[18px] border border-[#ECE9F6] bg-white p-5">
      <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-[#F1EDFC] text-[#5B3BE0]">{icon}</span>
      <div className="min-w-0">
        <p className={clsx("leading-none font-semibold text-[#100E2E]", small ? "text-[21px]" : "text-[28px] tabular-nums")}>{value}</p>
        <p className="mt-1.5 text-[14.5px] text-[#2A2654]">{label}</p>
        <p className="truncate text-[13px] text-[#6E6A8F]">{children}</p>
      </div>
    </div>
  );
}

function Panel({ title, onViewAll, children }: { title: string; onViewAll?: () => void; children: React.ReactNode }) {
  return (
    <section className="rounded-[18px] border border-[#ECE9F6] bg-white p-5">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-[16.5px] font-semibold text-[#100E2E]">{title}</h3>
        {onViewAll && (
          <button type="button" onClick={onViewAll} className="text-[13.5px] font-medium text-[#5B3BE0] hover:underline">
            View all →
          </button>
        )}
      </div>
      {children}
    </section>
  );
}

function ProjectList({ projects }: { projects: ReturnType<typeof summarizeKnowledge>["projects"] }) {
  if (projects.length === 0) return <p className="text-[14px] text-[#6E6A8F]">No public projects of your own on these accounts.</p>;
  return (
    <ul className="divide-y divide-[#F0EEF7]">
      {projects.map((p) => (
        <li key={p.url} className="flex gap-4 py-3.5 first:pt-0 last:pb-0">
          <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-[#F1EDFC] text-[#5B3BE0]">
            <FolderGit2 size={20} strokeWidth={1.7} />
          </span>
          <div className="min-w-0 flex-1">
            <div className="flex items-start gap-2">
              <a href={p.url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 truncate text-[15px] font-semibold text-[#100E2E] hover:underline">
                {p.name}
              </a>
              <a href={p.url} target="_blank" rel="noreferrer" aria-label={`Open ${p.name} on ${PLATFORMS[p.source].label}`} className="shrink-0 text-[#6E6A8F] hover:text-[#100E2E]">
                <ExternalLink size={16} />
              </a>
            </div>
            {p.description && <p className="mt-0.5 line-clamp-2 text-[13.5px] text-[#5E5A82]">{p.description}</p>}
            <div className="mt-2 flex flex-wrap items-center gap-2 text-[12.5px] text-[#6E6A8F]">
              {p.language && <span className="rounded-md bg-[#F1EFF8] px-2 py-0.5 text-[#2A2654]">{p.language}</span>}
              {p.stars > 0 && (
                <span className="inline-flex items-center gap-1">
                  <Star size={12} /> {p.stars}
                </span>
              )}
              {p.updatedAt && <span className="ml-auto">Updated {ago(p.updatedAt)}</span>}
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}

function LanguageBars({ languages }: { languages: Array<{ name: string; projects: number; share: number }> }) {
  if (languages.length === 0) return <p className="text-[14px] text-[#6E6A8F]">No languages detected yet.</p>;
  return (
    <ul className="space-y-4">
      {languages.map((l) => (
        <li key={l.name}>
          <div className="flex items-baseline justify-between gap-3 text-[14.5px]">
            <span className="font-medium text-[#100E2E]">{l.name}</span>
            <span className="text-[13px] text-[#6E6A8F] tabular-nums">
              {l.share}% · {plural(l.projects, "project")}
            </span>
          </div>
          <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-[#E9E6F6]">
            <div className="h-full rounded-full bg-gradient-to-r from-[#5B3BE0] to-[#7C5CFC]" style={{ width: `${Math.max(l.share, 3)}%` }} />
          </div>
        </li>
      ))}
    </ul>
  );
}

function ActivityCounts({ s, compact = false }: { s: ReturnType<typeof summarizeKnowledge>; compact?: boolean }) {
  if (!s.lastActiveAt) {
    return <p className="text-[14px] text-[#6E6A8F]">{s.activityHidden ? "Recent activity isn't public for these accounts." : "No recent public activity."}</p>;
  }
  const rows = [
    { icon: <Upload size={16} />, label: "Pushes", value: s.activity.pushes, tint: "bg-[#EFEAFD] text-[#5B3BE0]" },
    { icon: <GitPullRequest size={16} />, label: "Pull requests opened", value: s.activity.pullRequests, tint: "bg-[#E3F6EC] text-[#14693a]" },
    { icon: <MessageSquareText size={16} />, label: "Reviews", value: s.activity.reviews, tint: "bg-[#E6F1FD] text-[#1E5AA8]" },
    { icon: <CircleDot size={16} />, label: "Issues", value: s.activity.issues, tint: "bg-[#FFF3D6] text-[#8A5A00]" },
  ];
  return (
    <div>
      <ul className="space-y-3">
        {rows.map((r) => (
          <li key={r.label} className="flex items-center gap-3 text-[14px] text-[#2A2654]">
            <span className={clsx("flex h-8 w-8 shrink-0 items-center justify-center rounded-lg", r.tint)}>{r.icon}</span>
            {r.label}
            <span className="ml-auto font-semibold tabular-nums text-[#100E2E]">{r.value}</span>
          </li>
        ))}
      </ul>
      <p className={clsx("text-[13px] text-[#6E6A8F]", compact ? "mt-3" : "mt-4")}>Last public activity {ago(s.lastActiveAt)}.</p>
    </div>
  );
}
