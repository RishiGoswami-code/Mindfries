/**
 * Checks derive.ts, in Node:
 *
 *   node src/lib/notifications/derive.check.ts
 */
import { deriveNotifications, type ApplicationFact, type HiringFacts, type InterviewFact, type InvitationFact } from "./derive.ts";

let failures = 0;
function check(name: string, ok: boolean, detail = "") {
  if (!ok) failures++;
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  — ${detail}` : ""}`);
}

const NOW = Date.parse("2026-10-07T12:00:00Z");
const inv = (over: Partial<InvitationFact>): InvitationFact => ({
  id: "a1", role: "Backend Engineer", company: "Ledgerly", status: "invited", accepted: false, createdAt: "2026-10-01T09:00:00Z", dueDate: null, ...over,
});
const ids = (list: { id: string }[]) => list.map((n) => n.id).join(", ");

let got = deriveNotifications([inv({})], [], NOW);
check("an unanswered invitation is a notification", got.length === 1 && got[0].id === "invite:a1" && got[0].title.includes("Ledgerly"), ids(got));

got = deriveNotifications([inv({ accepted: true })], [], NOW);
check("an accepted invitation is no longer one", got.length === 0, ids(got));

got = deriveNotifications([inv({ status: "declined" })], [], NOW);
check("a declined invitation says nothing", got.length === 0, ids(got));

got = deriveNotifications([inv({ accepted: true, dueDate: "2026-10-20" })], [], NOW);
check("a due date far off is not announced", got.length === 0, ids(got));

got = deriveNotifications([inv({ accepted: true, dueDate: "2026-10-09" })], [], NOW);
check("a due date within three days is", got.length === 1 && got[0].id === "due:a1" && got[0].title.endsWith("in 2 days"), got[0]?.title);

got = deriveNotifications([inv({ accepted: true, dueDate: "2026-10-07" })], [], NOW);
check("due today reads as today, and is not overdue until the day ends", got[0]?.id === "due:a1" && got[0].title.endsWith("today"), got[0]?.title);

got = deriveNotifications([inv({ accepted: true, dueDate: "2026-10-05" })], [], NOW);
check("a passed due date on an open invitation is overdue", got.length === 1 && got[0].id === "overdue:a1", ids(got));

got = deriveNotifications([inv({ status: "submitted", dueDate: "2026-10-05" })], [], NOW);
check("a submitted assessment is never overdue", got.length === 0, ids(got));

got = deriveNotifications([inv({ status: "in_progress", accepted: true, dueDate: "2026-10-08" })], [], NOW);
check("one under way is told it's due, and sent to the in-progress list", got[0]?.href === "/assessments?status=in-progress" && got[0].body.includes("under way"), got[0]?.body);

got = deriveNotifications([], [
  { sessionId: "s1", role: "Backend Engineer", status: "ready", updatedAt: "2026-10-06T10:00:00Z" },
  { sessionId: "s2", role: null, status: "generating", updatedAt: "2026-10-07T10:00:00Z" },
  { sessionId: "s3", role: null, status: "failed", updatedAt: "2026-10-05T10:00:00Z" },
], NOW);
check("a finished report is announced, one still generating is not", ids(got) === "report:s1, report-failed:s3", ids(got));
check("a failed report says whose problem it is", got[1].body.includes("our side"));

got = deriveNotifications([inv({ createdAt: "2026-10-02T09:00:00Z" })], [{ sessionId: "s1", role: "X", status: "ready", updatedAt: "2026-10-06T10:00:00Z" }], NOW);
check("newest first", ids(got) === "report:s1, invite:a1", ids(got));
check("assessment items are tagged as such", got.every((n) => n.kind === "assessment"));

// Hiring: applications, interviews, messages (0025).
const hiring = (over: Partial<HiringFacts>): HiringFacts => ({ applications: [], interviews: [], messages: [], ...over });
const app = (over: Partial<ApplicationFact>): ApplicationFact => ({ id: "p1", role: "Backend Engineer", company: "Ledgerly", stage: "applied", stageChangedAt: "2026-10-06T09:00:00Z", ...over });
const iv = (over: Partial<InterviewFact>): InterviewFact => ({
  id: "i1", applicationId: "p1", title: "Technical interview", role: "Backend Engineer", company: "Ledgerly", startsAt: "2026-10-10T09:00:00Z", status: "scheduled", updatedAt: "2026-10-06T09:00:00Z", ...over,
});

got = deriveNotifications([], [], NOW, hiring({ applications: [app({}), app({ id: "p2", stage: "offered" }), app({ id: "p3", stage: "rejected" }), app({ id: "p4", stage: "shortlisted", stageChangedAt: null })] }));
check("an offer and a rejection are announced; applied, and a stage with no time, are not", ids(got) === "offered:p2, rejected:p3", ids(got));
check("they lead to that application", got[0]?.href === "/applications?id=p2" && got[0].kind === "application", got[0]?.href);

got = deriveNotifications([], [], NOW, hiring({ interviews: [iv({})] }));
check("an interview waiting for an answer is announced", ids(got) === "interview:i1" && got[0].body.includes("Confirm"), ids(got));

got = deriveNotifications([], [], NOW, hiring({ interviews: [iv({ status: "confirmed" })] }));
check("a confirmed one days away says nothing yet", got.length === 0, ids(got));

got = deriveNotifications([], [], NOW, hiring({ interviews: [iv({ status: "confirmed", startsAt: "2026-10-08T09:00:00Z" })] }));
check("one starting within a day is coming up", ids(got) === "interview-soon:i1" && got[0].title.endsWith("within a day"), got[0]?.title);

got = deriveNotifications([], [], NOW, hiring({ interviews: [iv({ startsAt: "2026-10-06T09:00:00Z" })] }));
check("one that has passed says nothing", got.length === 0, ids(got));

got = deriveNotifications([], [], NOW, hiring({ interviews: [iv({ status: "cancelled" })] }));
check("a cancelled upcoming one is announced", ids(got) === "interview-cancelled:i1", ids(got));

got = deriveNotifications([], [], NOW, hiring({ messages: [
  { id: "m1", applicationId: "p1", subject: "Next steps", company: "Ledgerly", sentAt: "2026-10-06T09:00:00Z" },
  { id: "m2", applicationId: "p1", subject: "Old", company: "Ledgerly", sentAt: "2026-08-01T09:00:00Z" },
] }));
check("a recent message is announced with its subject; an old one is left to Applications", ids(got) === "message:m1" && got[0].body === "Next steps", ids(got));

console.log(failures === 0 ? "\nall passed" : `\n${failures} failed`);
process.exit(failures === 0 ? 0 : 1);
