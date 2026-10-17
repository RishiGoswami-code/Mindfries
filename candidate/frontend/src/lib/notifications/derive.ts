import type { NotificationTone } from "./data";

/**
 * Working out a candidate's notifications from their real records.
 *
 * Nothing writes a notification anywhere. Each one is derived, every time
 * the bell is opened, from something that is true right now: an invitation
 * waiting for an answer, a due date coming up, a report that has finished,
 * an application that moved, an interview to confirm or coming up, a
 * message from a hiring team.
 * So a notification can't outlive the thing it is about — answer the
 * invitation and it is gone — and a candidate who signs up after being
 * invited still sees the invitation.
 *
 * Pure, so Node can check it (derive.check.ts). Each item's `id` is stable
 * for the thing it is about, which is what read and dismissed state is
 * stored against (candidate_notification_state).
 */

/** What a notification is about — the Notifications tab filters by it. */
export type NotificationKind = "assessment" | "application" | "interview" | "message";

export interface DerivedNotification {
  id: string;
  kind: NotificationKind;
  tone: NotificationTone;
  title: string;
  body: string;
  /** ISO time the notification became true — what it is ordered by. */
  at: string;
  /** Where it leads. */
  href: string;
}

export interface InvitationFact {
  id: string;
  role: string;
  company: string;
  /** assessments.status: invited|in_progress|submitted|closed|declined */
  status: string;
  accepted: boolean;
  createdAt: string;
  /** YYYY-MM-DD, or null. */
  dueDate: string | null;
}

export interface ReportFact {
  sessionId: string;
  /** What the session was for, when known. */
  role: string | null;
  /** assessment_reports.status */
  status: string;
  updatedAt: string;
}

/** An application on the job board or from an invitation (candidate_applications, 0025). */
export interface ApplicationFact {
  id: string;
  role: string;
  company: string;
  /** candidate_applications.stage */
  stage: string;
  /** When it reached that stage, or null when not recorded. */
  stageChangedAt: string | null;
}

export interface InterviewFact {
  id: string;
  applicationId: string;
  title: string;
  role: string;
  company: string;
  startsAt: string;
  /** application_interviews.status */
  status: string;
  updatedAt: string;
}

/** An email a hiring team wrote to the candidate (application_emails). */
export interface MessageFact {
  id: string;
  applicationId: string;
  subject: string;
  company: string;
  sentAt: string;
}

export interface HiringFacts {
  applications: ApplicationFact[];
  interviews: InterviewFact[];
  messages: MessageFact[];
}

/** A due date is announced this many days ahead. */
export const DUE_SOON_DAYS = 3;
const DAY = 86_400_000;

/** The end of a YYYY-MM-DD day, in UTC — a due date is good through the day it names. */
const endOfDay = (date: string) => Date.parse(`${date}T23:59:59Z`);

const dayWord = (days: number) => (days <= 0 ? "today" : days === 1 ? "tomorrow" : `in ${days} days`);

/** An interview is announced as coming up this long before it starts. */
export const INTERVIEW_SOON_MS = DAY;
/** Messages older than this are left to the Applications page. */
export const MESSAGE_DAYS = 30;

const STAGE_NOTICE: Record<string, { tone: NotificationTone; title: (role: string, company: string) => string; body: (role: string) => string }> = {
  shortlisted: { tone: "success", title: (role, company) => `${company} shortlisted you for ${role}`, body: () => "You're through to their next round." },
  offered: { tone: "success", title: (role, company) => `Offer from ${company}: ${role}`, body: () => "See the details in your application." },
  hired: { tone: "success", title: (_role, company) => `You're hired at ${company}`, body: (role) => `Congratulations — ${role} is yours.` },
  rejected: { tone: "neutral", title: (role, company) => `${company} isn't moving forward for ${role}`, body: () => "Your application stays in Applications." },
};

const appHref = (id: string) => `/applications?id=${encodeURIComponent(id)}`;

export function deriveNotifications(invitations: InvitationFact[], reports: ReportFact[], now: number, hiring?: HiringFacts): DerivedNotification[] {
  const out: DerivedNotification[] = [];

  for (const inv of invitations) {
    const open = inv.status === "invited" || inv.status === "in_progress";
    if (inv.status === "invited" && !inv.accepted) {
      out.push({
        id: `invite:${inv.id}`,
        kind: "assessment",
        tone: "info",
        title: `${inv.company} invited you: ${inv.role}`,
        body: "Accept or decline it from your assessments.",
        at: inv.createdAt,
        href: "/assessments?status=invited",
      });
    }
    if (open && inv.dueDate) {
      const due = endOfDay(inv.dueDate);
      if (Number.isFinite(due)) {
        const daysLeft = Math.floor((due - now) / DAY);
        if (now > due) {
          out.push({
            id: `overdue:${inv.id}`,
            kind: "assessment",
            tone: "error",
            title: `${inv.role} was due ${inv.dueDate}`,
            body: `${inv.company} asked for it by then. It is still open — whether it is still wanted is theirs to say.`,
            at: new Date(due).toISOString(),
            href: inv.status === "invited" ? "/assessments?status=invited" : "/assessments?status=in-progress",
          });
        } else if (daysLeft < DUE_SOON_DAYS) {
          out.push({
            id: `due:${inv.id}`,
            kind: "assessment",
            tone: "warning",
            title: `${inv.role} is due ${dayWord(daysLeft)}`,
            body: inv.status === "invited" ? `${inv.company} asked for it by ${inv.dueDate}. You haven't started.` : `${inv.company} asked for it by ${inv.dueDate}. It's under way.`,
            // It became true when the due date came within range.
            at: new Date(Math.max(due - DUE_SOON_DAYS * DAY, Date.parse(inv.createdAt) || 0)).toISOString(),
            href: inv.status === "invited" ? "/assessments?status=invited" : "/assessments?status=in-progress",
          });
        }
      }
    }
  }

  for (const r of reports) {
    if (r.status === "ready") {
      out.push({
        id: `report:${r.sessionId}`,
        kind: "assessment",
        tone: "success",
        title: r.role ? `Your report for ${r.role} is ready` : "Your report is ready",
        body: "See what was observed in your session.",
        at: r.updatedAt,
        href: `/assessments/${encodeURIComponent(r.sessionId)}/report`,
      });
    } else if (r.status === "failed") {
      out.push({
        id: `report-failed:${r.sessionId}`,
        kind: "assessment",
        tone: "neutral",
        title: r.role ? `The report for ${r.role} couldn't be generated` : "A report couldn't be generated",
        body: "Your session and its evidence are saved. This is a problem on our side, not with your work.",
        at: r.updatedAt,
        href: `/assessments/${encodeURIComponent(r.sessionId)}/report`,
      });
    }
  }

  for (const app of hiring?.applications ?? []) {
    const notice = STAGE_NOTICE[app.stage];
    if (!notice || !app.stageChangedAt) continue;
    out.push({
      id: `${app.stage}:${app.id}`,
      kind: "application",
      tone: notice.tone,
      title: notice.title(app.role, app.company),
      body: notice.body(app.role),
      at: app.stageChangedAt,
      href: appHref(app.id),
    });
  }

  for (const iv of hiring?.interviews ?? []) {
    const starts = Date.parse(iv.startsAt);
    if (!Number.isFinite(starts)) continue;
    const upcoming = starts > now;
    if (iv.status === "scheduled" && upcoming) {
      out.push({
        id: `interview:${iv.id}`,
        kind: "interview",
        tone: "info",
        title: `${iv.company} scheduled ${iv.title}`,
        body: `For ${iv.role}. Confirm it, or ask for another time.`,
        at: iv.updatedAt,
        href: appHref(iv.applicationId),
      });
    }
    if ((iv.status === "scheduled" || iv.status === "confirmed") && upcoming && starts - now <= INTERVIEW_SOON_MS) {
      out.push({
        id: `interview-soon:${iv.id}`,
        kind: "interview",
        tone: "warning",
        // "Within a day", not "today"/"tomorrow": this runs on the server, which
        // doesn't know the candidate's timezone.
        title: `${iv.title} with ${iv.company} starts within a day`,
        body: iv.status === "confirmed" ? `For ${iv.role}. You've confirmed it.` : `For ${iv.role}. You haven't confirmed it yet.`,
        at: new Date(Math.max(starts - INTERVIEW_SOON_MS, Date.parse(iv.updatedAt) || 0)).toISOString(),
        href: appHref(iv.applicationId),
      });
    }
    if (iv.status === "cancelled" && upcoming) {
      out.push({
        id: `interview-cancelled:${iv.id}`,
        kind: "interview",
        tone: "neutral",
        title: `${iv.company} cancelled ${iv.title}`,
        body: `For ${iv.role}.`,
        at: iv.updatedAt,
        href: appHref(iv.applicationId),
      });
    }
  }

  for (const m of hiring?.messages ?? []) {
    const sent = Date.parse(m.sentAt);
    if (!Number.isFinite(sent) || now - sent > MESSAGE_DAYS * DAY) continue;
    out.push({
      id: `message:${m.id}`,
      kind: "message",
      tone: "info",
      title: `Message from ${m.company}`,
      body: m.subject,
      at: m.sentAt,
      href: appHref(m.applicationId),
    });
  }

  return out.sort((a, b) => b.at.localeCompare(a.at));
}
