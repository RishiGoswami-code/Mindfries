import "server-only";
import { db } from "./supabase";

// Candidates, as Mindfries sees them: every candidate account, what their
// linked GitHub, GitLab and LinkedIn accounts say (candidate_knowledge,
// written by candidate/frontend — read through the candidate's own
// read-only Composio sign-in when they connected one), and how their
// assessments came out.
//
// A close cousin of company/frontend/lib/candidate-profile.ts. There is no
// shared package between the apps; the difference is scope — a company sees
// only the emails it invited, the admin sees everyone.

/* eslint-disable @typescript-eslint/no-explicit-any */

export interface CodeAccount {
  source: "github" | "gitlab";
  handle: string;
  /** True once the candidate signed in to the account, showing it is theirs. */
  verified: boolean;
  via: "connection" | "public";
  name: string;
  avatarUrl: string | null;
  profileUrl: string;
  bio: string | null;
  memberSince: string | null;
  followers: number | null;
  following: number | null;
  projectCount: number;
  privateProjects: number;
  totalStars: number;
  languages: Array<{ name: string; projects: number }>;
  projects: Array<{ name: string; description: string; language: string | null; stars: number; url: string; updatedAt: string | null }>;
  activity: { lastActiveAt: string | null; since: string | null; pushes: number; pullRequests: number; reviews: number; issues: number } | null;
  fetchedAt: string;
}

export interface LinkedInAccount {
  name: string;
  headline: string | null;
  pictureUrl: string | null;
  profileUrl: string | null;
  fetchedAt: string;
}

export interface CandidateAssessment {
  id: string;
  company: string | null;
  role: string | null;
  invitedAt: string;
  /** The latest session's status, or null when it hasn't started. */
  sessionId: string | null;
  sessionStatus: string | null;
  /**
   * strong_hire|hire|lean_no|no_hire. The decision is only ever a person's
   * (the company's reviewer); null while nobody has made one. The AI report's
   * recommendation is kept apart, as a suggestion.
   */
  decision: string | null;
  aiSuggestion: string | null;
  score: number | null;
  sectionScores: Record<string, number>;
}

export interface CandidateCard {
  id: string;
  name: string;
  email: string;
  headline: string;
  location: string;
  joinedAt: string;
  emailConfirmed: boolean;
  avatarUrl: string | null;
  github: CodeAccount | null;
  gitlab: CodeAccount | null;
  linkedin: LinkedInAccount | null;
  portfolio: string | null;
  /** Languages across their code accounts, most projects first. */
  languages: Array<{ name: string; projects: number }>;
  lastActiveAt: string | null;
  /** Newest first. */
  assessments: CandidateAssessment[];
}

function safeUrl(url: unknown): string | null {
  try {
    const u = new URL(String(url ?? ""));
    return u.protocol === "https:" || u.protocol === "http:" ? u.toString() : null;
  } catch {
    return null;
  }
}

const num = (v: unknown) => (typeof v === "number" && Number.isFinite(v) ? v : null);

function toCodeAccount(r: any): CodeAccount | null {
  if (r.source !== "github" && r.source !== "gitlab") return null;
  const k = r.payload;
  const profileUrl = safeUrl(k?.profile?.profileUrl);
  if (!k || !profileUrl) return null;
  const projects = (Array.isArray(k.projects) ? k.projects : [])
    .map((p: any) => ({
      name: String(p?.name ?? ""),
      description: String(p?.description ?? ""),
      language: typeof p?.language === "string" ? p.language : null,
      stars: Number(p?.stars) || 0,
      url: safeUrl(p?.url) ?? "",
      updatedAt: p?.updatedAt ?? null,
    }))
    .filter((p: any) => p.url);
  return {
    source: r.source,
    handle: r.handle,
    verified: !!r.verified,
    via: k.via === "connection" ? "connection" : "public",
    name: typeof k.profile?.name === "string" ? k.profile.name : r.handle,
    avatarUrl: safeUrl(k.profile?.avatarUrl),
    profileUrl,
    bio: typeof k.profile?.bio === "string" && k.profile.bio ? k.profile.bio : null,
    memberSince: k.profile?.memberSince ?? null,
    followers: num(k.profile?.followers),
    following: num(k.profile?.following),
    projectCount: Number(k.projectCount) || 0,
    privateProjects: Number(k.privateProjects) || 0,
    totalStars: Number(k.totalStars) || projects.reduce((s: number, p: any) => s + p.stars, 0),
    languages: Array.isArray(k.languages) ? k.languages : [],
    projects,
    activity: k.activity ?? null,
    fetchedAt: r.fetched_at,
  };
}

function toLinkedIn(r: any): LinkedInAccount | null {
  const p = r?.payload?.profile;
  if (!p || typeof p.name !== "string" || !p.name) return null;
  return {
    name: p.name,
    headline: typeof p.headline === "string" && p.headline ? p.headline : null,
    pictureUrl: typeof p.pictureUrl === "string" && p.pictureUrl.startsWith("https://") ? safeUrl(p.pictureUrl) : null,
    profileUrl: safeUrl(p.profileUrl),
    fetchedAt: r.fetched_at,
  };
}

const MAX_CANDIDATES = 500;

/**
 * Candidate cards, newest account first — all of them, or one when `id` is
 * given. Empty when Supabase isn't wired.
 */
export async function listCandidateCards(id?: string): Promise<CandidateCard[]> {
  const c = db();
  if (!c) return [];

  let q = c
    .from("candidate_users")
    .select("id, name, email, role, location, links, created_at, email_verified_at")
    .eq("status", "active")
    .order("created_at", { ascending: false })
    .limit(MAX_CANDIDATES);
  if (id) q = q.eq("id", id);
  const { data: users } = await q;
  if (!users?.length) return [];
  const ids = users.map((u: any) => u.id);
  const emails = users.map((u: any) => String(u.email).toLowerCase());

  // An assessment is theirs by the account that started it (0021) or by the
  // email it was sent to — before they start, only the email links them.
  const columns = "id, candidate_id, candidate_email, role, created_at, companies(name)";
  const [{ data: knowledge }, { data: byEmail }, { data: byAccount }] = await Promise.all([
    c.from("candidate_knowledge").select("candidate_id, source, handle, verified, payload, fetched_at").in("candidate_id", ids),
    c.from("assessments").select(columns).in("candidate_email", emails),
    c.from("assessments").select(columns).in("candidate_id", ids),
  ]);
  const assessments = [...new Map([...(byEmail ?? []), ...(byAccount ?? [])].map((a: any) => [a.id, a])).values()].sort((x: any, y: any) =>
    String(y.created_at).localeCompare(String(x.created_at)),
  );
  const assessmentIds = assessments.map((a: any) => a.id);
  const [{ data: sessions }, { data: applications }] = assessmentIds.length
    ? await Promise.all([
        c.from("sessions").select("id, assessment_id, status, started_at").in("assessment_id", assessmentIds).order("started_at", { ascending: false }),
        c.from("candidate_applications").select("assessment_id, score, section_scores").in("assessment_id", assessmentIds),
      ])
    : [{ data: [] }, { data: [] }];
  const sessionIds = (sessions ?? []).map((s: any) => s.id);
  const { data: reports } = sessionIds.length
    ? await c.from("assessment_reports").select("session_id, status, recommendation, reviewer_recommendation").in("session_id", sessionIds)
    : { data: [] };

  return users.map((u: any) => {
    const mine = (knowledge ?? []).filter((k: any) => k.candidate_id === u.id);
    const code = mine.map(toCodeAccount).filter((a): a is CodeAccount => a !== null);
    const github = code.find((a) => a.source === "github") ?? null;
    const gitlab = code.find((a) => a.source === "gitlab") ?? null;
    const li = mine.find((k: any) => k.source === "linkedin");
    const linkedin = li ? toLinkedIn(li) : null;

    const languages = new Map<string, number>();
    for (const a of code) for (const l of a.languages) languages.set(l.name, (languages.get(l.name) ?? 0) + l.projects);

    const email = String(u.email).toLowerCase();
    const theirs = assessments.filter((a: any) => a.candidate_id === u.id || String(a.candidate_email).toLowerCase() === email);
    const cards: CandidateAssessment[] = theirs.map((a: any) => {
      const session = (sessions ?? []).find((s: any) => s.assessment_id === a.id) ?? null;
      const report = session ? (reports ?? []).find((r: any) => r.session_id === session.id) : null;
      const app = (applications ?? []).find((p: any) => p.assessment_id === a.id);
      const reviewer = report?.reviewer_recommendation ?? null;
      const ai = report?.status === "ready" ? (report.recommendation ?? null) : null;
      return {
        id: a.id,
        company: a.companies?.name ?? null,
        role: a.role ?? null,
        invitedAt: a.created_at,
        sessionId: session?.id ?? null,
        sessionStatus: session?.status ?? null,
        decision: reviewer,
        aiSuggestion: ai,
        score: app?.score ?? null,
        sectionScores: app?.section_scores ?? {},
      };
    });

    return {
      id: u.id,
      name: u.name ?? "",
      email,
      headline: u.role ?? linkedin?.headline ?? "",
      location: u.location ?? "",
      joinedAt: u.created_at,
      emailConfirmed: !!u.email_verified_at,
      avatarUrl: code.find((a) => a.verified && a.avatarUrl)?.avatarUrl ?? linkedin?.pictureUrl ?? code.find((a) => a.avatarUrl)?.avatarUrl ?? null,
      github,
      gitlab,
      linkedin,
      portfolio: safeUrl(u.links?.portfolio?.value),
      languages: [...languages.entries()].map(([name, projects]) => ({ name, projects })).sort((a, b) => b.projects - a.projects),
      lastActiveAt:
        code
          .map((a) => a.activity?.lastActiveAt)
          .filter((t): t is string => !!t)
          .sort()
          .at(-1) ?? null,
      assessments: cards,
    };
  });
}
