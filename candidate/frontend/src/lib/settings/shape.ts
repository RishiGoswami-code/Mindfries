/**
 * What a candidate can set about their account, and how it is stored
 * (migration 0026_candidate_settings.sql). Pure, so Node can check it
 * (shape.check.ts) and the company portal's copy can be compared to it.
 */

/** Emails a candidate can turn off. Account emails (sign-in, password, address) always go. */
export const EMAIL_KINDS = ["invitations", "interviews", "messages"] as const;
export type EmailKind = (typeof EMAIL_KINDS)[number];
export type EmailPrefs = Record<EmailKind, boolean>;

export const EMAIL_KIND_COPY: Record<EmailKind, { label: string; detail: string }> = {
  invitations: {
    label: "Assessment invitations",
    detail: "When a company invites you to an assessment. Off, it still appears on your dashboard.",
  },
  interviews: {
    label: "Interview scheduling",
    detail: "When a hiring team schedules or moves an interview. Off, it still appears in Applications.",
  },
  messages: {
    label: "Messages from hiring teams",
    detail: "Emails a hiring team writes to you. Off, you read them in Applications instead.",
  },
};

/** A missing or malformed value means on: nobody stops getting mail by accident. */
export function readEmailPrefs(raw: unknown): EmailPrefs {
  const r = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  return Object.fromEntries(EMAIL_KINDS.map((k) => [k, r[k] !== false])) as EmailPrefs;
}

/** Parts of the profile a candidate can hide from hiring teams. Must match the check in 0026. */
export const HIDEABLE_FIELDS = ["summary", "location", "photo", "code", "linkedin", "portfolio", "resume", "preferences", "documents"] as const;
export type HideableField = (typeof HIDEABLE_FIELDS)[number];

export const FIELD_COPY: Record<HideableField, { label: string; detail: string }> = {
  summary: { label: "Headline and summary", detail: "Your headline, about you, notice period and work modes." },
  location: { label: "Location", detail: "Where you're based." },
  photo: { label: "Profile photo", detail: "The picture from an account you linked." },
  code: { label: "GitHub and GitLab", detail: "Your accounts, projects, languages and activity." },
  linkedin: { label: "LinkedIn", detail: "The LinkedIn account you signed in with, and its headline." },
  portfolio: { label: "Portfolio", detail: "Your portfolio website." },
  resume: { label: "Resume", detail: "The file, and the skills and years read from it." },
  preferences: { label: "Job preferences", detail: "Roles, employment types, locations and salary you're after." },
  documents: { label: "Supporting documents", detail: "Certificates, reports and other files you uploaded." },
};

/** Always shown to a company you applied to or that invited you — listed so the page says so. */
export const ALWAYS_SHARED = [
  { label: "Name and email", detail: "So the hiring team knows who you are and can reach you." },
  { label: "Your assessments for them", detail: "The work and evidence from an assessment that company sent you." },
];

export function readHiddenFields(raw: unknown): HideableField[] {
  return Array.isArray(raw) ? HIDEABLE_FIELDS.filter((f) => raw.includes(f)) : [];
}

/** Hidden projects are stored by address; only http(s) addresses are kept, at most 200. */
export function readHiddenProjects(raw: unknown): string[] {
  if (!Array.isArray(raw)) return [];
  return [...new Set(raw.filter((u): u is string => typeof u === "string" && /^https?:\/\//.test(u) && u.length <= 500))].slice(0, 200);
}

export interface AccountSettings {
  /** False before migration 0026 — the page says settings aren't set up, rather than showing switches that do nothing. */
  available: boolean;
  email: EmailPrefs;
  hiddenFields: HideableField[];
  hiddenProjects: string[];
}

/** Typed to confirm deleting the account. */
export const DELETE_PHRASE = "delete my account";
