import type { KnowledgeProject, KnowledgeSource } from "./knowledge-shape";

/**
 * What the candidate's linked code accounts add up to, across GitHub and
 * GitLab together — the Projects & activity card and its window. Pure, and
 * typed structurally so it can be checked in Node (knowledge-summary.check.ts).
 */

export interface SummarySource {
  source: KnowledgeSource;
  handle: string;
  verified: boolean;
  fetchedAt: string;
  knowledge: {
    languages: Array<{ name: string; projects: number }>;
    projects: KnowledgeProject[];
    projectCount: number;
    privateProjects?: number;
    activity: {
      lastActiveAt: string | null;
      since: string | null;
      events: number;
      pushes: number;
      pullRequests: number;
      reviews: number;
      issues: number;
    } | null;
  };
}

export interface KnowledgeSummary {
  /** Public projects of their own, across accounts. */
  publicProjects: number;
  privateProjects: number;
  /** The projects that were read (each account keeps its most recent few), newest first. */
  projects: Array<KnowledgeProject & { source: KnowledgeSource }>;
  /** Languages by how many projects use them, with that as a share of all projects. */
  languages: Array<{ name: string; projects: number; share: number }>;
  /** The most recent public activity on any account, or null if none is known. */
  lastActiveAt: string | null;
  activity: { events: number; pushes: number; pullRequests: number; reviews: number; issues: number };
  /** True when at least one account's activity isn't public at all. */
  activityHidden: boolean;
}

export function summarizeKnowledge(sources: SummarySource[]): KnowledgeSummary {
  const publicProjects = sources.reduce((n, s) => n + s.knowledge.projectCount, 0);
  const privateProjects = sources.reduce((n, s) => n + (s.knowledge.privateProjects ?? 0), 0);

  const seen = new Set<string>();
  const projects = sources
    .flatMap((s) => s.knowledge.projects.map((p) => ({ ...p, source: s.source })))
    .filter((p) => (seen.has(p.url) ? false : (seen.add(p.url), true)))
    .sort((a, b) => (b.updatedAt ?? "").localeCompare(a.updatedAt ?? ""));

  const byName = new Map<string, { name: string; projects: number }>();
  for (const s of sources) {
    for (const l of s.knowledge.languages) {
      const key = l.name.toLowerCase();
      const cur = byName.get(key);
      if (cur) cur.projects += l.projects;
      else byName.set(key, { name: l.name, projects: l.projects });
    }
  }
  // Languages count private projects too (knowledge-shape.ts), so the share is of all of them.
  const total = Math.max(publicProjects + privateProjects, 1);
  const languages = [...byName.values()]
    .sort((a, b) => b.projects - a.projects || a.name.localeCompare(b.name))
    .map((l) => ({ ...l, share: Math.min(100, Math.round((l.projects / total) * 100)) }));

  const activity = { events: 0, pushes: 0, pullRequests: 0, reviews: 0, issues: 0 };
  let lastActiveAt: string | null = null;
  let activityHidden = false;
  for (const s of sources) {
    const a = s.knowledge.activity;
    if (!a) {
      activityHidden = true;
      continue;
    }
    activity.events += a.events;
    activity.pushes += a.pushes;
    activity.pullRequests += a.pullRequests;
    activity.reviews += a.reviews;
    activity.issues += a.issues;
    if (a.lastActiveAt && (!lastActiveAt || a.lastActiveAt > lastActiveAt)) lastActiveAt = a.lastActiveAt;
  }

  return { publicProjects, privateProjects, projects, languages, lastActiveAt, activity, activityHidden };
}

/** "today", "yesterday", "3 days ago", "2 weeks ago", "4 months ago" — relative to `now`. */
export function ago(iso: string | null, now: Date = new Date()): string {
  if (!iso) return "";
  const days = Math.floor((now.getTime() - new Date(iso).getTime()) / 86_400_000);
  if (days <= 0) return "today";
  if (days === 1) return "yesterday";
  if (days < 7) return `${days} days ago`;
  if (days < 30) return `${Math.floor(days / 7)} ${Math.floor(days / 7) === 1 ? "week" : "weeks"} ago`;
  if (days < 365) return `${Math.floor(days / 30)} ${Math.floor(days / 30) === 1 ? "month" : "months"} ago`;
  return `${Math.floor(days / 365)} ${Math.floor(days / 365) === 1 ? "year" : "years"} ago`;
}

/** "Active this week", "Active this month", "Last active 3 months ago", or null. */
export function activeLabel(lastActiveAt: string | null, now: Date = new Date()): string | null {
  if (!lastActiveAt) return null;
  const days = (now.getTime() - new Date(lastActiveAt).getTime()) / 86_400_000;
  if (days < 7) return "Active this week";
  if (days < 31) return "Active this month";
  return `Last active ${ago(lastActiveAt, now)}`;
}
