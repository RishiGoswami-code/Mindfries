// node --experimental-strip-types src/lib/profile/profile-cards.check.ts
//
// The rules behind the profile's cards: job preferences validation, the
// completion checklist, and the merged projects-and-activity summary.
import { cleanJobPreferences, formatSalaryRange, hasPreferences, readJobPreferences } from "./job-preferences.ts";
import { profileCompletion } from "./completion.ts";
import { activeLabel, ago, summarizeKnowledge, type SummarySource } from "./knowledge-summary.ts";

let failures = 0;
function check(label: string, pass: boolean, detail = "") {
  if (!pass) failures++;
  console.log(`${pass ? "ok  " : "FAIL"}  ${label}${pass || !detail ? "" : `\n        ${detail}`}`);
}

// ── Job preferences ──────────────────────────────────────────────────
const prefs = cleanJobPreferences({
  roles: ["  AI/ML Engineer ", "ai/ml engineer", "LLM Engineer", 7, ""],
  employment: ["Full-time", "Freelance", "Full-time"],
  locations: ["India", "Europe"],
  salary: { min: 80000, max: 120000, currency: "USD" },
});
check("roles are trimmed, de-duplicated case-insensitively, non-strings dropped", prefs.ok && prefs.value.roles.join("|") === "AI/ML Engineer|LLM Engineer", JSON.stringify(prefs));
check("unknown employment types are dropped, duplicates collapsed", prefs.ok && prefs.value.employment.join("|") === "Full-time");
check("a salary range low to high is kept", prefs.ok && prefs.value.salary?.min === 80000 && prefs.value.salary?.max === 120000);
const backwards = cleanJobPreferences({ salary: { min: 10, max: 5, currency: "USD" } });
check("a range high to low is refused", !backwards.ok && /above/.test(backwards.error));
const fractional = cleanJobPreferences({ salary: { min: 1.5, max: 5, currency: "USD" } });
check("a salary has to be whole numbers", !fractional.ok);
const noSalary = cleanJobPreferences({ roles: ["SRE"], salary: null });
check("no salary is allowed", noSalary.ok && noSalary.value.salary === null);
const oddCurrency = cleanJobPreferences({ salary: { min: 1, max: 2, currency: "DOGE" } });
check("an unknown currency falls back to USD", oddCurrency.ok && oddCurrency.value.salary?.currency === "USD");
const many = cleanJobPreferences({ roles: Array.from({ length: 20 }, (_, i) => `Role ${i}`) });
check("at most eight roles are kept", many.ok && many.value.roles.length === 8);
check("formatSalaryRange in thousands", formatSalaryRange({ min: 80000, max: 120000, currency: "USD" }) === "$80K – $120K");
check("formatSalaryRange in lakhs for INR", formatSalaryRange({ min: 1200000, max: 1850000, currency: "INR" }) === "₹12L – ₹18.5L", formatSalaryRange({ min: 1200000, max: 1850000, currency: "INR" }));
check("nothing set is not preferences", !hasPreferences({ roles: [], employment: [], locations: [], salary: null }, []));
check("a work mode alone counts", hasPreferences({ roles: [], employment: [], locations: [], salary: null }, ["Remote"]));
check("a stored row with a bad salary reads as nothing rather than throwing", readJobPreferences({ roles: ["x"], salary: { min: 9, max: 1 } }) === null);
check("null reads as null", readJobPreferences(null) === null);

// ── Completion ───────────────────────────────────────────────────────
const blank = profileCompletion({
  hasResume: false,
  detailsSaved: false,
  links: {},
  knowledgeCount: 0,
  completedAssessments: 0,
  assessmentsInProgress: 0,
  documents: 0,
  preferencesSet: false,
});
check("a new profile is 0 of 6", blank.done === 0 && blank.total === 6 && blank.percent === 0);
const partial = profileCompletion({
  hasResume: true,
  detailsSaved: true,
  links: { linkedin: { value: "x" } },
  knowledgeCount: 0,
  completedAssessments: 0,
  assessmentsInProgress: 1,
  documents: 0,
  preferencesSet: false,
});
const state = (id: string) => partial.sections.find((s) => s.id === id)?.state;
check("LinkedIn alone is a start, not done", state("linked") === "in-progress");
check("an assessment under way puts evidence in progress", state("evidence") === "in-progress");
check("2 of 6 is 33%", partial.done === 2 && partial.percent === 33, `${partial.done}/${partial.total} ${partial.percent}%`);
const withDoc = profileCompletion({ ...{ hasResume: false, detailsSaved: false, links: {}, knowledgeCount: 0, completedAssessments: 0, assessmentsInProgress: 0, preferencesSet: false }, documents: 1 });
check("a document alone completes evidence", withDoc.sections.find((s) => s.id === "evidence")?.state === "done");
const noPrefs = profileCompletion({ hasResume: true, detailsSaved: false, links: {}, knowledgeCount: 0, completedAssessments: 0, assessmentsInProgress: 0, documents: 0, preferencesSet: null });
check("without migration 0024, preferences are left out, not counted undone", noPrefs.total === 5 && !noPrefs.sections.some((s) => s.id === "preferences"));

// ── Projects & activity ──────────────────────────────────────────────
const project = (name: string, updatedAt: string, language: string | null = "Python") => ({
  name,
  description: "",
  language,
  stars: 0,
  url: `https://example.test/${name}`,
  updatedAt,
});
const sources: SummarySource[] = [
  {
    source: "github",
    handle: "a",
    verified: true,
    fetchedAt: "2026-10-10T00:00:00Z",
    knowledge: {
      languages: [
        { name: "Python", projects: 3 },
        { name: "TypeScript", projects: 1 },
      ],
      projects: [project("old", "2026-01-01T00:00:00Z"), project("new", "2026-10-01T00:00:00Z")],
      projectCount: 2,
      privateProjects: 2,
      activity: { lastActiveAt: "2026-10-12T00:00:00Z", since: "2026-09-01T00:00:00Z", events: 10, pushes: 6, pullRequests: 2, reviews: 1, issues: 1 },
    },
  },
  {
    source: "gitlab",
    handle: "a",
    verified: false,
    fetchedAt: "2026-10-10T00:00:00Z",
    knowledge: {
      languages: [{ name: "python", projects: 1 }],
      projects: [project("mid", "2026-06-01T00:00:00Z", "Python")],
      projectCount: 1,
      activity: null,
    },
  },
];
const sum = summarizeKnowledge(sources);
check("public and private projects are counted apart", sum.publicProjects === 3 && sum.privateProjects === 2);
check("projects from both accounts, newest first", sum.projects.map((p) => p.name).join(",") === "new,mid,old");
check("languages merge across accounts regardless of case", sum.languages[0].name === "Python" && sum.languages[0].projects === 4, JSON.stringify(sum.languages));
check("a language's share is of all projects, public and private", sum.languages[0].share === 80, String(sum.languages[0].share));
check("activity adds up, and a hidden account is noted", sum.activity.pushes === 6 && sum.activityHidden);
check("no accounts sums to nothing", summarizeKnowledge([]).projects.length === 0 && summarizeKnowledge([]).lastActiveAt === null);

const now = new Date("2026-10-17T12:00:00Z");
check("ago: days", ago("2026-10-14T12:00:00Z", now) === "3 days ago");
check("ago: weeks", ago("2026-10-03T12:00:00Z", now) === "2 weeks ago");
check("active this week", activeLabel("2026-10-12T12:00:00Z", now) === "Active this week");
check("last active months ago", activeLabel("2026-06-01T12:00:00Z", now) === "Last active 4 months ago", String(activeLabel("2026-06-01T12:00:00Z", now)));

console.log(failures ? `\n${failures} failed` : "\nall passed");
process.exit(failures ? 1 : 0);
