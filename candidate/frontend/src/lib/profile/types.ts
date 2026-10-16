/**
 * `candidate_users.links` as stored: one entry per platform, each a loose
 * object (value, savedAt, stats, connectionId…) or null once removed. Read
 * fields off it defensively — older rows don't carry every key.
 */
export type StoredLinks = Record<string, Record<string, unknown> | null>;

/**
 * What lib/profile/actions.ts's getProfile() hands to the profile and
 * dashboard pages. The Supabase client is untyped, so this is the one place
 * the shape of that row is written down; the select there and this type
 * have to move together.
 */
export interface CandidateProfile {
  name: string;
  role: string | null;
  location: string | null;
  bio: string | null;
  notice_period: string | null;
  open_to: string[] | null;
  resume_path: string | null;
  resume_parsed: unknown;
  resume_parsed_at: string | null;
  links: StoredLinks;
  profile_updated_at: string | null;
  email_verified_at: string | null;
}
