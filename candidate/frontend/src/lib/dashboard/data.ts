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
}

export interface Stat {
  kind: AssessmentStatus | "practice";
  label: string;
  value: number;
  hint: string;
}

/**
 * The counters, derived from the same list `AssessmentNotes` renders, so the
 * two can never disagree. `kind` ties each counter to the same state an
 * assessment can be in, so a counter and the assessments it counts share a
 * colour.
 *
 * "Practice runs" counts the practice runs the candidate has opened
 * (practice_runs) — that one happened is all that is kept about it.
 */
export function deriveStats(items: Assessment[], practiceRuns = 0): Stat[] {
  const count = (status: AssessmentStatus) => items.filter((a) => a.status === status).length;
  const invited = count("invited");
  const inProgress = count("in-progress");
  const submitted = count("submitted");

  return [
    {
      kind: "invited",
      label: "Open invitations",
      value: invited,
      hint: invited === 0 ? "Nothing open right now" : `${invited} open right now`,
    },
    {
      kind: "in-progress",
      label: "In progress",
      value: inProgress,
      hint: inProgress === 0 ? "Nothing in progress" : "Resume from your assessments",
    },
    {
      kind: "submitted",
      label: "Submitted",
      value: submitted,
      hint: submitted === 0 ? "Nothing submitted yet" : submitted === 1 ? "Under review" : "All under review",
    },
    {
      kind: "practice",
      label: "Practice runs",
      value: practiceRuns,
      hint: practiceRuns === 0 ? "Try the workspace first" : "Never recorded or scored",
    },
  ];
}

export const resources: { title: string; body: string; href?: string; action?: "policies" }[] = [
  {
    title: "What the workspace records",
    body: "Every signal captured during a session, and what a hiring team sees.",
    action: "policies",
  },
  {
    title: "Explaining your decisions",
    body: "The reasoning prompts matter as much as the code. How to answer them well.",
    href: "/practice",
  },
  {
    title: "Before your first session",
    body: "Camera, microphone and a five-minute practice run.",
    href: "/practice",
  },
];

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
