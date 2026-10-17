/**
 * The job board and the candidate's applications (migration 0025): shapes,
 * labels, and the rules that turn an application's records into what the
 * candidate sees — its status and its timeline. Pure, with no server or
 * browser imports, so both sides use it and it's checked in Node
 * (shape.check.ts).
 */

export type WorkMode = "remote" | "hybrid" | "onsite";
export type EmploymentType = "full_time" | "part_time" | "contract" | "internship";

export const WORK_MODE_LABEL: Record<WorkMode, string> = { remote: "Remote", hybrid: "Hybrid", onsite: "On-site" };
export const EMPLOYMENT_LABEL: Record<EmploymentType, string> = {
  full_time: "Full-time",
  part_time: "Part-time",
  contract: "Contract",
  internship: "Internship",
};

export interface JobListing {
  id: string;
  title: string;
  companyName: string;
  companyWebsite: string | null;
  description: string;
  location: string;
  workMode: WorkMode | null;
  employmentType: EmploymentType | null;
  techStack: string[];
  /** Minutes the assessment is set to take, when the role says. */
  durationMin: number | null;
  publishedAt: string | null;
  /** How many have applied or been invited — counted, not estimated. */
  applicants: number;
  /** Applying opens the assessment at once (the company's choice for this role). */
  selfAssessment: boolean;
}

/** Every stage an application can be in (company/frontend/lib/types.ts keeps the same list). */
export type Stage =
  | "applied"
  | "invited"
  | "in_progress"
  | "completed"
  | "shortlisted"
  | "interviewing"
  | "offered"
  | "hired"
  | "rejected"
  | "declined"
  | "withdrawn";

export type InterviewStatus = "scheduled" | "confirmed" | "reschedule_requested" | "cancelled_by_candidate" | "cancelled" | "completed";

export interface Interview {
  id: string;
  title: string;
  startsAt: string;
  durationMin: number;
  location: string | null;
  notes: string | null;
  status: InterviewStatus;
  candidateMessage: string | null;
  updatedAt: string;
}

export interface ReceivedEmail {
  id: string;
  subject: string;
  body: string;
  fromName: string | null;
  sentAt: string;
}

export interface MyApplication {
  id: string;
  job: Pick<JobListing, "id" | "title" | "companyName" | "location" | "workMode" | "employmentType" | "publishedAt" | "applicants" | "selfAssessment">;
  /** Whether the job is still on the board, so its page can be opened. */
  jobListed: boolean;
  stage: Stage;
  source: "invited" | "applied";
  coverNote: string | null;
  createdAt: string;
  stageChangedAt: string | null;
  completedAt: string | null;
  /** The assessment behind the application, once there is one. */
  assessment: {
    id: string;
    /** assessments.status: invited | in_progress | submitted | closed, or declined. */
    status: string;
    taskStatus: "pending" | "generating" | "ready" | "failed";
    sessionId: string | null;
    startedAt: string | null;
  } | null;
  interviews: Interview[];
  emails: ReceivedEmail[];
}

// ── What the candidate sees ──────────────────────────────────────────

export type StatusGroup = "review" | "assessment" | "interview" | "offer" | "hired" | "rejected" | "withdrawn";

export interface CandidateStatus {
  label: string;
  group: StatusGroup;
}

/**
 * The application's status in the candidate's words. Internal steps the
 * hiring team takes (a shortlist) read as still under review; nothing is
 * shown that the team hasn't done.
 */
export function candidateStatus(stage: Stage): CandidateStatus {
  switch (stage) {
    case "applied":
    case "shortlisted":
      return { label: "Under review", group: "review" };
    case "invited":
      return { label: "Assessment", group: "assessment" };
    case "in_progress":
      return { label: "Assessment under way", group: "assessment" };
    case "completed":
      return { label: "Assessment submitted", group: "assessment" };
    case "interviewing":
      return { label: "Interview", group: "interview" };
    case "offered":
      return { label: "Offer", group: "offer" };
    case "hired":
      return { label: "Hired", group: "hired" };
    case "rejected":
      return { label: "Not progressing", group: "rejected" };
    case "declined":
      return { label: "Declined", group: "withdrawn" };
    case "withdrawn":
      return { label: "Withdrawn", group: "withdrawn" };
  }
}

/** The status tabs, in the order the Applications page shows them. */
export const STATUS_TABS: { group: StatusGroup; label: string }[] = [
  { group: "review", label: "Under review" },
  { group: "assessment", label: "Assessment" },
  { group: "interview", label: "Interviews" },
  { group: "offer", label: "Offers" },
  { group: "rejected", label: "Not progressing" },
  { group: "withdrawn", label: "Withdrawn" },
];

/** True for an application still in play: something can still happen on it. */
export function isActive(stage: Stage): boolean {
  return !["hired", "rejected", "declined", "withdrawn"].includes(stage);
}

/**
 * Whether the candidate can withdraw. Not mid-assessment — a session under
 * way finishes or runs out — and not once it's closed.
 */
export function canWithdraw(app: Pick<MyApplication, "stage">): boolean {
  return isActive(app.stage) && app.stage !== "in_progress";
}

/** An interview the candidate can still answer: confirm, ask to move, or cancel. */
export function interviewOpen(iv: Pick<Interview, "status" | "startsAt">, now: Date = new Date()): boolean {
  return ["scheduled", "confirmed", "reschedule_requested"].includes(iv.status) && new Date(iv.startsAt).getTime() > now.getTime() - 60 * 60_000;
}

// ── The timeline ─────────────────────────────────────────────────────

export type StepState = "done" | "current" | "upcoming" | "skipped";

export interface TimelineStep {
  key: string;
  title: string;
  state: StepState;
  /** When it happened (or is due, for an interview). Null when not known or not yet. */
  at: string | null;
  /** The short tag beside the step: "Completed", "Next", "Confirmed"… */
  tag: string | null;
}

const INTERVIEW_TAG: Record<InterviewStatus, string> = {
  scheduled: "Please confirm",
  confirmed: "Confirmed",
  reschedule_requested: "New time asked",
  cancelled_by_candidate: "You cancelled",
  cancelled: "Cancelled",
  completed: "Completed",
};

/**
 * The application's steps, from what's recorded: applying (or being
 * invited), review, the assessment, each interview, and the outcome. Steps
 * that haven't happened are "upcoming"; the one waiting on someone now is
 * "current". Nothing is dated that wasn't recorded.
 */
export function buildTimeline(app: MyApplication, now: Date = new Date()): TimelineStep[] {
  const steps: TimelineStep[] = [];
  const closed = !isActive(app.stage);

  steps.push({
    key: "start",
    title: app.source === "applied" ? "Application submitted" : "Invited by the company",
    state: "done",
    at: app.createdAt,
    tag: null,
  });

  if (app.source === "applied" && !app.job.selfAssessment) {
    const reviewing = app.stage === "applied" || app.stage === "shortlisted";
    steps.push({
      key: "review",
      title: "Reviewed by the hiring team",
      state: reviewing ? (closed ? "skipped" : "current") : app.stage === "withdrawn" && !app.assessment ? "skipped" : "done",
      // When the team reviewed it isn't recorded on its own — stage_changed_at
      // is the latest change, not this one — so the step isn't dated.
      at: null,
      tag: reviewing ? "In review" : null,
    });
  }

  if (app.assessment) {
    const a = app.assessment;
    const submitted = a.status === "submitted" || app.completedAt !== null;
    const started = a.sessionId !== null || app.stage === "in_progress";
    steps.push({
      key: "assessment",
      title: "Assessment",
      state: submitted ? "done" : a.status === "declined" || (closed && !started) ? "skipped" : "current",
      at: submitted ? app.completedAt : a.startedAt,
      tag: submitted
        ? "Submitted"
        : a.status === "declined"
          ? "Declined"
          : started
            ? "Under way"
            : a.taskStatus === "ready"
              ? "Ready to start"
              : a.taskStatus === "failed"
                ? "Couldn't be prepared"
                : "Being prepared",
    });
  } else if (isActive(app.stage) && app.source === "applied" && (app.stage === "applied" || app.stage === "shortlisted")) {
    steps.push({ key: "assessment", title: "Assessment", state: "upcoming", at: null, tag: null });
  }

  const interviews = [...app.interviews].sort((x, y) => x.startsAt.localeCompare(y.startsAt));
  for (const iv of interviews) {
    const done = iv.status === "completed";
    const off = iv.status === "cancelled" || iv.status === "cancelled_by_candidate";
    steps.push({
      key: `interview-${iv.id}`,
      title: iv.title,
      state: done ? "done" : off ? "skipped" : new Date(iv.startsAt).getTime() < now.getTime() ? "done" : "current",
      at: iv.startsAt,
      tag: INTERVIEW_TAG[iv.status],
    });
  }

  const outcome: Record<string, { title: string; tag: string }> = {
    offered: { title: "Offer", tag: "Offer made" },
    hired: { title: "Hired", tag: "Hired" },
    rejected: { title: "Decision", tag: "Not progressing" },
    withdrawn: { title: "Withdrawn", tag: "You withdrew" },
    declined: { title: "Declined", tag: "You declined" },
  };
  if (outcome[app.stage]) {
    steps.push({ key: "outcome", title: outcome[app.stage].title, state: "done", at: app.stageChangedAt, tag: outcome[app.stage].tag });
  } else {
    steps.push({ key: "outcome", title: "Offer", state: "upcoming", at: null, tag: null });
  }

  // Only one step is "current" — the earliest one still waiting.
  let seen = false;
  for (const s of steps) {
    if (s.state !== "current") continue;
    if (seen) s.state = "upcoming";
    seen = true;
  }
  return steps;
}

/** The most recent thing to tell the candidate about, for the banner at the top of an application. */
export function headline(app: MyApplication, now: Date = new Date()): { title: string; body: string } | null {
  const next = [...app.interviews]
    .filter((iv) => interviewOpen(iv, now))
    .sort((x, y) => x.startsAt.localeCompare(y.startsAt))[0];
  if (next) {
    return {
      title: next.status === "confirmed" ? "Interview confirmed" : next.status === "reschedule_requested" ? "You asked for another time" : "Interview scheduled",
      body: `${next.title} — ${next.status === "scheduled" ? "please confirm it, or ask for another time" : "see the details below"}.`,
    };
  }
  switch (app.stage) {
    case "applied":
    case "shortlisted":
      return { title: "Under review", body: "The hiring team has your application. You'll hear from them here and by email." };
    case "invited":
      return app.assessment?.taskStatus === "ready"
        ? { title: "Your assessment is ready", body: "Start it from your dashboard whenever you're ready — the clock starts when you enter the workspace." }
        : { title: "Your assessment is being prepared", body: "A task is being written for you from the role's brief. It shows on your dashboard when it's ready." };
    case "in_progress":
      return { title: "Assessment under way", body: "Pick it up from your dashboard." };
    case "completed":
      return { title: "Assessment submitted", body: "The hiring team is reviewing your work." };
    case "offered":
      return { title: "You have an offer", body: "The hiring team will be in touch by email with the details." };
    case "hired":
      return { title: "Hired", body: "Congratulations." };
    case "rejected":
      return { title: "Not progressing", body: "The hiring team has decided not to go further with this application." };
    default:
      return null;
  }
}
