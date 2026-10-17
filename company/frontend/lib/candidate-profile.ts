import "server-only";
import { db } from "./supabase";

// The candidate's own profile, as the hiring team sees it beside the report:
// what they wrote about themselves, their resume and what was read from it,
// and what their linked code accounts show publicly.
//
// It is the candidate's account in the shared database (candidate_users,
// candidate_knowledge — written by candidate/frontend), found by the email
// the company invited. Callers reach this only after
// getApplicationForCompany() has established that the application belongs
// to the signed-in company; the email comes from that row, never from the
// request.
//
// Each part says where it came from, because they are not the same kind of
// claim: the identity is the candidate's own words, the resume fields are a
// pattern match over their resume, and the accounts are what GitHub, GitLab
// or LinkedIn said — through the candidate's own read-only sign-in when they
// connected one (verified), or public data about a username they typed.

/* eslint-disable @typescript-eslint/no-explicit-any */

export interface ProfileLink {
  platform: "github" | "gitlab" | "linkedin" | "portfolio";
  label: string;
  /** Null when the candidate signed in to the platform but gave no address (LinkedIn). */
  url: string | null;
  /**
   * Set when the candidate signed in to this platform: the username or, for
   * LinkedIn, the member's name they signed in under. For LinkedIn it shows
   * they hold an account in that name — not that `url` is that account.
   */
  signedInAs: string | null;
  /** The headline LinkedIn gave for the signed-in member, if any. */
  headline: string | null;
}

export interface ProfileProject {
  name: string;
  description: string;
  language: string | null;
  stars: number;
  url: string;
  updatedAt: string | null;
}

export interface ProfileCodeAccount {
  source: "github" | "gitlab";
  handle: string;
  /** True only once the candidate has signed in to the account to show it is theirs. */
  verified: boolean;
  fetchedAt: string;
  profileUrl: string;
  memberSince: string | null;
  languages: Array<{ name: string; projects: number }>;
  projects: ProfileProject[];
  projectCount: number;
  activity: { lastActiveAt: string | null; since: string | null; pushes: number; pullRequests: number; reviews: number; issues: number } | null;
  /** Private projects of their own, seen through their connection. A count only — names are never stored. */
  privateProjects: number;
  name: string;
  avatarUrl: string | null;
  /** Stars across their own public projects. */
  totalStars: number;
  followers: number | null;
  following: number | null;
  /** How it was read: through the candidate's own read-only connection, or the public API. */
  via: "connection" | "public";
}

/** What LinkedIn said about the member who signed in (candidate_knowledge, source 'linkedin'). */
export interface ProfileLinkedIn {
  name: string;
  headline: string | null;
  pictureUrl: string | null;
  profileUrl: string | null;
  fetchedAt: string;
}

export interface CandidateProfile {
  name: string;
  /** Null until someone has checked the address — see email_verified_at in 0018. */
  emailVerifiedAt: string | null;
  headline: string;
  location: string;
  bio: string;
  noticePeriod: string;
  openTo: string[];
  updatedAt: string | null;
  links: ProfileLink[];
  resume: {
    /** A link to the file that works for an hour, or null if it couldn't be signed. */
    url: string | null;
    fileName: string;
    /** Technologies the resume names. Empty when nothing could be read from the file. */
    skills: string[];
    years: { from: number; to: number } | null;
    readable: boolean;
  } | null;
  codeAccounts: ProfileCodeAccount[];
  linkedin: ProfileLinkedIn | null;
  /** A picture of the candidate from an account they linked, if any. */
  avatarUrl: string | null;
  /** What they're looking for (job_preferences, migration 0024), or null when unset or not in this database. */
  jobPreferences: ProfileJobPreferences | null;
  /** Supporting documents they added (candidate_documents, 0024). Empty when none, or not in this database. */
  documents: ProfileDocument[];
}

export interface ProfileJobPreferences {
  roles: string[];
  employment: string[];
  locations: string[];
  salary: { min: number; max: number; currency: string } | null;
}

export interface ProfileDocument {
  id: string;
  kind: "certificate" | "report" | "other";
  title: string;
  mimeType: string;
  sizeBytes: number;
  uploadedAt: string;
  /** A link that works for an hour, or null if it couldn't be signed. */
  url: string | null;
}

const DOCUMENTS_BUCKET = "candidate-documents";

const LINK_URL: Record<ProfileLink["platform"], (v: string) => string> = {
  github: (v) => `https://github.com/${encodeURIComponent(v)}`,
  gitlab: (v) => `https://gitlab.com/${encodeURIComponent(v)}`,
  linkedin: (v) => `https://www.linkedin.com/in/${encodeURIComponent(v)}`,
  portfolio: (v) => v,
};
const LINK_LABEL: Record<ProfileLink["platform"], string> = { github: "GitHub", gitlab: "GitLab", linkedin: "LinkedIn", portfolio: "Portfolio" };

/** Only ever an http(s) link — a stored portfolio value is the candidate's input. */
function safeUrl(url: string): string | null {
  try {
    const u = new URL(url);
    return u.protocol === "https:" || u.protocol === "http:" ? u.toString() : null;
  } catch {
    return null;
  }
}

function toLinks(raw: any): ProfileLink[] {
  const out: ProfileLink[] = [];
  for (const platform of ["github", "gitlab", "linkedin", "portfolio"] as const) {
    const value = raw?.[platform]?.value;
    const as = raw?.[platform]?.verified?.as;
    const signedInAs = typeof as === "string" && as ? as.slice(0, 200) : null;
    const fromPlatform = raw?.[platform]?.profile;
    // An address LinkedIn itself gave for the signed-in member beats one that was typed.
    const url =
      (typeof fromPlatform?.profileUrl === "string" && safeUrl(fromPlatform.profileUrl)) ||
      (typeof value === "string" && value ? safeUrl(LINK_URL[platform](value)) : null);
    const headline = typeof fromPlatform?.headline === "string" && fromPlatform.headline ? fromPlatform.headline.slice(0, 300) : null;
    if (url || signedInAs) out.push({ platform, label: LINK_LABEL[platform], url, signedInAs, headline });
  }
  return out;
}

function toCodeAccount(r: any): ProfileCodeAccount | null {
  if (r.source !== "github" && r.source !== "gitlab") return null;
  const k = r.payload;
  if (!k || typeof k !== "object") return null;
  const profileUrl = safeUrl(String(k.profile?.profileUrl ?? ""));
  if (!profileUrl) return null;
  return {
    source: r.source,
    handle: r.handle,
    verified: !!r.verified,
    fetchedAt: r.fetched_at,
    profileUrl,
    memberSince: k.profile?.memberSince ?? null,
    languages: Array.isArray(k.languages) ? k.languages : [],
    projects: (Array.isArray(k.projects) ? k.projects : [])
      .map((p: any) => ({ ...p, url: safeUrl(String(p.url ?? "")) }))
      .filter((p: any) => p.url),
    projectCount: Number(k.projectCount) || 0,
    activity: k.activity ?? null,
    privateProjects: Number(k.privateProjects) || 0,
    name: typeof k.profile?.name === "string" ? k.profile.name.slice(0, 200) : r.handle,
    avatarUrl: safeUrl(String(k.profile?.avatarUrl ?? "")),
    totalStars: Number(k.totalStars) || (Array.isArray(k.projects) ? k.projects.reduce((s: number, p: any) => s + (Number(p?.stars) || 0), 0) : 0),
    followers: typeof k.profile?.followers === "number" ? k.profile.followers : null,
    following: typeof k.profile?.following === "number" ? k.profile.following : null,
    via: k.via === "connection" ? "connection" : "public",
  };
}

function toLinkedIn(r: any): ProfileLinkedIn | null {
  const p = r?.payload?.profile;
  if (!p || typeof p.name !== "string" || !p.name) return null;
  return {
    name: p.name.slice(0, 200),
    headline: typeof p.headline === "string" && p.headline ? p.headline.slice(0, 300) : null,
    pictureUrl: typeof p.pictureUrl === "string" && p.pictureUrl.startsWith("https://") ? safeUrl(p.pictureUrl) : null,
    profileUrl: typeof p.profileUrl === "string" ? safeUrl(p.profileUrl) : null,
    fetchedAt: r.fetched_at,
  };
}

/** A picture to show for the candidate: GitHub's, then GitLab's, then LinkedIn's. */
function pictureOf(accounts: ProfileCodeAccount[], linkedin: ProfileLinkedIn | null): string | null {
  return accounts.find((a) => a.verified && a.avatarUrl)?.avatarUrl ?? linkedin?.pictureUrl ?? accounts.find((a) => a.avatarUrl)?.avatarUrl ?? null;
}

/** What the candidate list shows beside each name: a picture and which accounts are linked. */
export interface ProfileBadge {
  avatarUrl: string | null;
  accounts: Array<{ platform: "github" | "gitlab" | "linkedin"; verified: boolean; projects: number | null }>;
  languages: string[];
}

/**
 * Badges for many invited emails at once, keyed by lower-cased email. An
 * email with no candidate account has no entry. Callers pass only emails
 * from their own company's applications.
 */
export async function listProfileBadges(emails: string[]): Promise<Record<string, ProfileBadge>> {
  const c = db();
  const wanted = [...new Set(emails.map((e) => e.trim().toLowerCase()).filter(Boolean))];
  if (!c || wanted.length === 0) return {};
  const { data: users } = await c.from("candidate_users").select("id, email").in("email", wanted).eq("status", "active");
  if (!users?.length) return {};
  const { data: rows } = await c
    .from("candidate_knowledge")
    .select("candidate_id, source, handle, verified, payload, fetched_at")
    .in("candidate_id", users.map((u: any) => u.id));
  const out: Record<string, ProfileBadge> = {};
  for (const u of users as any[]) {
    const mine = (rows ?? []).filter((r: any) => r.candidate_id === u.id);
    const code = mine.filter((r: any) => r.source !== "linkedin").map(toCodeAccount).filter((a): a is ProfileCodeAccount => a !== null);
    const li = mine.find((r: any) => r.source === "linkedin");
    const linkedin = li ? toLinkedIn(li) : null;
    const languages = new Map<string, number>();
    for (const a of code) for (const l of a.languages) languages.set(l.name, (languages.get(l.name) ?? 0) + l.projects);
    out[String(u.email).toLowerCase()] = {
      avatarUrl: pictureOf(code, linkedin),
      accounts: [
        ...code.map((a) => ({ platform: a.source, verified: a.verified, projects: a.projectCount + a.privateProjects })),
        ...(linkedin ? [{ platform: "linkedin" as const, verified: true, projects: null }] : []),
      ],
      languages: [...languages.entries()].sort((a, b) => b[1] - a[1]).slice(0, 3).map(([n]) => n),
    };
  }
  return out;
}

/**
 * The profile behind an invited email, or null when that person has no
 * candidate account yet (invited, hasn't signed up) or Supabase isn't wired.
 */
export async function getCandidateProfile(candidateEmail: string): Promise<CandidateProfile | null> {
  const c = db();
  const email = candidateEmail.trim().toLowerCase();
  if (!c || !email) return null;

  const { data: row } = await c
    .from("candidate_users")
    .select("id, name, role, location, bio, notice_period, open_to, resume_path, resume_parsed, links, profile_updated_at, email_verified_at")
    .eq("email", email)
    .eq("status", "active")
    .maybeSingle();
  if (!row) return null;
  const r = row as any;

  const [{ data: knowledge }, signed] = await Promise.all([
    c.from("candidate_knowledge").select("source, handle, verified, payload, fetched_at").eq("candidate_id", r.id).order("source"),
    r.resume_path ? c.storage.from("resumes").createSignedUrl(r.resume_path, 3600) : Promise.resolve(null),
  ]);

  const codeAccounts = (knowledge ?? [])
    .filter((k: any) => k.source === "github" || k.source === "gitlab")
    .map(toCodeAccount)
    .filter((a): a is ProfileCodeAccount => a !== null);
  const li = (knowledge ?? []).find((k: any) => k.source === "linkedin");
  const linkedin = li ? toLinkedIn(li) : null;

  const parsed = r.resume_parsed as { skills?: string[]; years?: { from: number; to: number } } | null;
  // Read apart from the profile row: they depend on migration 0024, and a
  // select naming a column that isn't there fails the whole query.
  const [jobPreferences, documents] = await Promise.all([readJobPreferences(c, r.id), readDocuments(c, r.id)]);

  return {
    name: r.name ?? "",
    emailVerifiedAt: r.email_verified_at ?? null,
    headline: r.role ?? "",
    location: r.location ?? "",
    bio: r.bio ?? "",
    noticePeriod: r.notice_period ?? "",
    openTo: r.open_to ?? [],
    updatedAt: r.profile_updated_at ?? null,
    links: toLinks(r.links),
    resume: r.resume_path
      ? {
          url: signed?.data?.signedUrl ?? null,
          // Stored as "<candidate id>/<timestamp>-<name>".
          fileName: String(r.resume_path).split("/").pop()?.replace(/^\d+-/, "") || "Resume",
          skills: Array.isArray(parsed?.skills) ? parsed.skills : [],
          years: parsed?.years ?? null,
          readable: !!parsed,
        }
      : null,
    codeAccounts,
    linkedin,
    avatarUrl: pictureOf(codeAccounts, linkedin),
    jobPreferences,
    documents,
  };
}

const strings = (v: unknown, max = 8): string[] =>
  Array.isArray(v) ? v.filter((x): x is string => typeof x === "string" && x.trim() !== "").slice(0, max).map((x) => x.slice(0, 60)) : [];

async function readJobPreferences(c: NonNullable<ReturnType<typeof db>>, candidateId: string): Promise<ProfileJobPreferences | null> {
  const { data, error } = await c.from("candidate_users").select("job_preferences").eq("id", candidateId).maybeSingle();
  const p = !error ? (data as any)?.job_preferences : null;
  if (!p || typeof p !== "object") return null;
  const s = p.salary;
  const salary =
    s && Number.isInteger(s.min) && Number.isInteger(s.max) && s.min <= s.max
      ? { min: s.min as number, max: s.max as number, currency: typeof s.currency === "string" ? s.currency.slice(0, 3) : "USD" }
      : null;
  const prefs = { roles: strings(p.roles), employment: strings(p.employment), locations: strings(p.locations), salary };
  return prefs.roles.length || prefs.employment.length || prefs.locations.length || prefs.salary ? prefs : null;
}

async function readDocuments(c: NonNullable<ReturnType<typeof db>>, candidateId: string): Promise<ProfileDocument[]> {
  const { data, error } = await c
    .from("candidate_documents")
    .select("id, kind, title, path, mime_type, size_bytes, uploaded_at")
    .eq("candidate_id", candidateId)
    .order("uploaded_at", { ascending: false });
  if (error || !data) return [];
  return Promise.all(
    (data as any[]).map(async (d) => {
      const { data: signed } = await c.storage.from(DOCUMENTS_BUCKET).createSignedUrl(d.path, 3600);
      return {
        id: d.id,
        kind: d.kind,
        title: d.title,
        mimeType: d.mime_type,
        sizeBytes: d.size_bytes,
        uploadedAt: d.uploaded_at,
        url: signed?.signedUrl ?? null,
      };
    })
  );
}
