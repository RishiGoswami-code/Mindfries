import type { LinkPlatform } from "./links";

/**
 * A linked-account entry as seen by client components — the real
 * `connectionId` is stripped to a "connected" sentinel before it
 * leaves the server (see getProfile in actions.ts), so this type
 * intentionally uses `string` for connectionId, not the real uuid.
 */
export interface ProfileLink {
  value?: string;
  connectionId?: string;
  [key: string]: unknown;
}

/** The links JSONB column, keyed by platform. */
export type ProfileLinks = Partial<Record<LinkPlatform | string, ProfileLink | null>>;

/**
 * The shape returned by `getProfile()` (lib/profile/actions.ts) —
 * exactly the columns selected from `candidate_users` plus the
 * redacted `links` object.
 *
 * Every component that accepts a profile prop should use this type
 * instead of `any`.
 */
export interface CandidateProfile {
  name: string;
  role: string | null;
  location: string | null;
  bio: string | null;
  notice_period: string | null;
  open_to: string[] | null;
  resume_path: string | null;
  resume_parsed: Record<string, unknown> | null;
  resume_parsed_at: string | null;
  links: ProfileLinks;
  profile_updated_at: string | null;
  email_verified_at: string | null;
}

/**
 * The identity subset constructed by IdentityCard and consumed by
 * EditProfileModal — the resolved display values, not the raw DB row.
 */
export interface SavedIdentity {
  name: string;
  role: string;
  location: string;
  openTo: string[];
  noticePeriod: string;
  bio: string;
  updatedAt: string | null;
}
