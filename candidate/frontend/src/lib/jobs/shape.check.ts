// node --experimental-strip-types src/lib/jobs/shape.check.ts
//
// What a candidate is shown about an application: its status, whether it
// can be withdrawn, its timeline and the banner at the top.
import { buildTimeline, canWithdraw, candidateStatus, headline, interviewOpen, type MyApplication } from "./shape.ts";

let failures = 0;
function check(label: string, pass: boolean, detail = "") {
  if (!pass) failures++;
  console.log(`${pass ? "ok  " : "FAIL"}  ${label}${pass || !detail ? "" : `\n        ${detail}`}`);
}

const now = new Date("2026-10-17T12:00:00Z");
const base: MyApplication = {
  id: "a1",
  job: { id: "r1", title: "Backend Engineer", companyName: "Acme", location: "Remote", workMode: "remote", employmentType: "full_time", publishedAt: "2026-10-01T00:00:00Z", applicants: 4, selfAssessment: false },
  jobListed: true,
  stage: "applied",
  source: "applied",
  coverNote: null,
  createdAt: "2026-10-10T09:00:00Z",
  stageChangedAt: null,
  completedAt: null,
  assessment: null,
  interviews: [],
  emails: [],
};
const titles = (app: MyApplication) => buildTimeline(app, now).map((s) => `${s.title}:${s.state}`).join(" | ");

// Status words
check("a shortlist reads as still under review", candidateStatus("shortlisted").label === "Under review");
check("rejected reads as not progressing", candidateStatus("rejected").group === "rejected");
check("declined sits with withdrawn", candidateStatus("declined").group === "withdrawn");

// Withdraw
check("an application under review can be withdrawn", canWithdraw({ stage: "applied" }));
check("not mid-assessment", !canWithdraw({ stage: "in_progress" }));
check("not once it's closed", !canWithdraw({ stage: "rejected" }) && !canWithdraw({ stage: "withdrawn" }));

// Timeline: applied, waiting for review
const fresh = buildTimeline(base, now);
check("a fresh application: submitted, then review is current, assessment and offer upcoming", titles(base) === "Application submitted:done | Reviewed by the hiring team:current | Assessment:upcoming | Offer:upcoming", titles(base));
check("submitted is dated by the application, review isn't dated", fresh[0].at === base.createdAt && fresh[1].at === null);

// Invited to the assessment, task ready
const invited: MyApplication = {
  ...base,
  stage: "invited",
  assessment: { id: "s1", status: "invited", taskStatus: "ready", sessionId: null, startedAt: null },
};
const invitedSteps = buildTimeline(invited, now);
check("after the team invites: review done, assessment current and ready to start", invitedSteps[1].state === "done" && invitedSteps[2].state === "current" && invitedSteps[2].tag === "Ready to start", titles(invited));

// Self-assessment role: no review step
const selfApp: MyApplication = { ...invited, job: { ...base.job, selfAssessment: true } };
check("a role open to every applicant has no review step", !buildTimeline(selfApp, now).some((s) => s.key === "review"));

// Submitted, then an interview
const interviewing: MyApplication = {
  ...base,
  stage: "interviewing",
  completedAt: "2026-10-12T10:00:00Z",
  assessment: { id: "s1", status: "submitted", taskStatus: "ready", sessionId: "x", startedAt: "2026-10-12T09:00:00Z" },
  interviews: [
    { id: "i2", title: "Final interview", startsAt: "2026-10-25T10:00:00Z", durationMin: 45, location: null, notes: null, status: "scheduled", candidateMessage: null, updatedAt: "" },
    { id: "i1", title: "Technical interview", startsAt: "2026-10-20T10:00:00Z", durationMin: 60, location: null, notes: null, status: "confirmed", candidateMessage: null, updatedAt: "" },
  ],
};
const ivSteps = buildTimeline(interviewing, now);
check("interviews are in date order", ivSteps.filter((s) => s.key.startsWith("interview")).map((s) => s.title).join(",") === "Technical interview,Final interview");
check("only one step is current — the earliest waiting", ivSteps.filter((s) => s.state === "current").length === 1 && ivSteps.find((s) => s.state === "current")?.title === "Technical interview", titles(interviewing));
check("the assessment is done and dated by submission", ivSteps.find((s) => s.key === "assessment")?.state === "done" && ivSteps.find((s) => s.key === "assessment")?.at === interviewing.completedAt);
check("a cancelled interview is skipped", buildTimeline({ ...interviewing, interviews: [{ ...interviewing.interviews[0], status: "cancelled" }] }, now).find((s) => s.key === "interview-i2")?.state === "skipped");

// Outcomes
check("an offer closes the timeline with Offer made", buildTimeline({ ...interviewing, stage: "offered", interviews: [], stageChangedAt: "2026-10-16T00:00:00Z" }, now).at(-1)?.tag === "Offer made");
const withdrawn = buildTimeline({ ...base, stage: "withdrawn", stageChangedAt: "2026-10-11T00:00:00Z" }, now);
check("withdrawing before review skips the review", withdrawn.find((s) => s.key === "review")?.state === "skipped" && withdrawn.at(-1)?.tag === "You withdrew");

// Interviews the candidate can still answer
check("a future scheduled interview is open", interviewOpen({ status: "scheduled", startsAt: "2026-10-20T10:00:00Z" }, now));
check("one that's long past isn't", !interviewOpen({ status: "confirmed", startsAt: "2026-10-01T10:00:00Z" }, now));
check("a cancelled one isn't", !interviewOpen({ status: "cancelled", startsAt: "2026-10-20T10:00:00Z" }, now));

// Banner
check("the banner leads with the next interview", headline(interviewing, now)?.title === "Interview confirmed", JSON.stringify(headline(interviewing, now)));
check("an assessment still being written says so", headline({ ...invited, assessment: { ...invited.assessment!, taskStatus: "generating" } }, now)?.title === "Your assessment is being prepared");

console.log(failures ? `\n${failures} failed` : "\nall passed");
process.exit(failures ? 1 : 0);
