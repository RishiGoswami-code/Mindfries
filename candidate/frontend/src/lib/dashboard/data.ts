/**
 * Shapes and derivations for the candidate dashboard.
 *
 * No assessment, counter or activity row is written here any more — those
 * come from the candidate's real records (lib/backend/client.ts's
 * listAssessmentsOrUndefined). What's left is the types, the labels, and the
 * pure functions that turn a real list into what the dashboard shows.
 */

export type AssessmentStatus = "invited" | "in-progress" | "submitted" | "closed";

export interface Assessment {
  id: string;
  role: string;
  company: string;
  location: string;
  /** Short descriptors shown as chips — format, length, working style. */
  tags: string[];
  status: AssessmentStatus;
  /** Human phrasing, since "due in 3 days" reads better than a date here. */
  due: string;
  /** 0–100. Only shown for invitations, where it's a reason to start. */
  match?: number;
  /** The candidate's session for this assessment, when they have one — the workspace to resume, or the report to open. */
  sessionId?: string;
  /**
   * Whether the candidate has accepted the invitation, or declined it (a
   * declined invitation's status is "closed"). Every assessment is an
   * invitation; this is absent only when the answers couldn't be read.
   */
  invitation?: { accepted: boolean; declined: boolean };
  /**
   * Whether the candidate's own task has been made yet. Each invitation
   * gets a task written for that candidate after it is sent; until it is
   * "ready" there is nothing to start.
   */
  taskStatus?: "pending" | "generating" | "ready" | "failed";
  /** True when the company has shared the report with the candidate. */
  reportShared?: boolean;
}

/** Shown when the list couldn't be read â€” distinct from a real, empty list. */
export const UNAVAILABLE = {
  title: "Assessments unavailable",
  body: "We couldn't load your assessments just now. Nothing is lost â€” try again in a moment.",
};

export const statusLabels: Record<AssessmentStatus, string> = {
  invited: "Invited",
  "in-progress": "In progress",
  submitted: "Submitted",
  closed: "Closed",
};

/** All four statuses, in the order they're worth your attention. */
export const ASSESSMENT_STATUSES: AssessmentStatus[] = ["invited", "in-progress", "submitted", "closed"];

const ATTENTION_ORDER: Record<AssessmentStatus, number> = { invited: 0, "in-progress": 1, submitted: 2, closed: 3 };

/**
 * Needs-your-action first: an open invitation before a session you've
 * already submitted. Used wherever a list of assessments is shown without an
 * explicit sort of its own — the dashboard's preview and the full
 * Assessments page's "All" view both read this way.
 */
export function sortByAttention(items: Assessment[]): Assessment[] {
  return [...items].sort((a, b) => ATTENTION_ORDER[a.status] - ATTENTION_ORDER[b.status]);
}
