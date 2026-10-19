# Mindfries V2

**[`SPEC.md`](SPEC.md) is the source of truth for V2.** It is the product
specification as given, unedited. This README says what it means for this
repository: where V2 fits beside what's here, how the spec's isolation rules
map onto the real pieces, where today's V1 differs from it, and what has to
be decided before anything is built. It links to the spec rather than
repeating it — two copies drift apart.

**[`ROLE_FLOWS.md`](ROLE_FLOWS.md) shows V2 working end to end** for two
roles — a Forward Deployed Engineer and a DevOps Engineer: recruiter setup,
each stage of the candidate's session with the tabs and evidence involved,
the rubric weights, scenario variations, and the final report. Both run on
the same platform; only the scenario, tabs, stages and rubric change.

**[`PLAN.md`](PLAN.md) is how V2 gets built:** the guards that keep V1
safe, the path to production, and the tasks in order — the CLI first.

**Status: specification only. Nothing in V2 is built, and V1 is unchanged.**

## V2 in one paragraph

V2 turns the assessment from "code in our browser IDE, chat with our
assistant" into an evidence-first work session across several kinds of work.
- **Where the candidate codes:** in their own IDE, set up by a session-scoped local CLI and a short-lived agent ([§9](SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension)).
- **The browser:** stays the hub, with one floating panel of tabs — AI Help, System Design, Research, Debugging, Notes/Evidence ([§10](SPEC.md#10-the-unified-candidate-workspace)).
- **Routes:** each assessment adapts to the candidate — a common core, an evidence-driven route and targeted follow-ups — and "not assessed" is kept apart from "weak" ([§11](SPEC.md#11-adaptive-assessment-candidates-can-be-strong-in-different-ways)).
- **Assistant:** a backend policy gateway decides what the AI assistant and developer platforms may do ([§13](SPEC.md#13-floating-ai-help-assistant-and-tool-permissions)).
- **Result:** a capability profile — per competency: score, confidence, coverage, evidence links. The avatar is capped at about 5% ([§14](SPEC.md#14-evaluation-and-weighting-model), [§19](SPEC.md#19-candidate-report--capability-profile)).
- **Integrity:** three separate categories — observed events, verified violations, review signals — never one "cheating score" ([§16](SPEC.md#16-integrity-monitoring-and-anti-cheating-design)).

## The rule that governs everything

> Build, test, and run V2 independently until the V2 acceptance criteria
> pass. Only then perform a planned, reversible cutover. — [§2](SPEC.md#2-v1-baseline-and-non-negotiable-parallel-build-strategy)

V1 stays the only production candidate path. No V1 URL, session, table,
env var, job or deployment setting is changed for V2.

## What "isolated" means in this repo

The spec's isolation table ([§2](SPEC.md#required-isolation-boundaries)), against what V1 actually is today:

| Area | V1 today (do not touch) | V2 needs |
|---|---|---|
| Source | `candidate/frontend`, `company/frontend`, `internal-admin/frontend`, `candidate/backend`, `company/backend` | Its own packages under `v2/` (and the parked `candidate/v2/`, see below) |
| URLs | `candidate.mindfries…`, company and admin Vercel projects | A separate domain or Vercel project — never a route inside the V1 apps |
| Deployment | Vercel: three projects. Railway: `candidate/backend` | New V2 Vercel/Railway projects with their own pipelines |
| Env / secrets | `SESSION_SECRET`, `SUPABASE_*`, `CANDIDATE_BACKEND_URL`, Gemini, Resend, Composio, Daytona keys | V2-prefixed variables and V2-scoped credentials; nothing shared or overwritten |
| Database | One shared Supabase project, `supabase/migrations/0001–0027`, applied by `candidate/frontend/scripts/migrate.mts` into one `schema_migrations` | A separate Supabase project, or a `v2` schema with its own migrations folder and runner — **decision needed** |
| API | Go `candidate/backend` at `/api/v1/…`, plus Next route handlers | A versioned `/v2/…` API in its own service |
| Jobs | Go workers (evaluation, task generation), internal-admin crons (retention, discovery) | V2's own workers, queues and schedules |
| Sessions / auth | `mf_candidate` cookie, `candidate_users`, `sessions` | Separate V2 session state and credentials |
| AI | Gemini, direct (per-session message cap; interview off for the pilot) | The V2 assistant behind the policy gateway, its own keys and budgets |
| Telemetry | `activity_events`, `assessment_reports`, `evidence_items` | A separate V2 evidence store and event namespace |
| Tests | Almost none: a few `*.check.ts` files, one vitest file, Go unit tests, **no CI** | V1 contract tests and CI first (see Phase 0) |

## Where V1 differs from the spec

V1 stays as it is; these are points where V2 is meant to work differently,
so V1 code can't simply be carried over:

1. **Coding surface.** V1 is a browser IDE: Monaco, Pyodide, an in-browser shell and git, or a Daytona sandbox. V2 puts coding in the candidate's own IDE through the CLI; a browser workspace is at most a fallback ([§9 failure handling](SPEC.md#environment-failure-handling)).
2. **Monitoring.** V1 keeps camera stills every 30 seconds, screen stills, and microphone clips on noise, and flags leaving the workspace, pastes and second windows. V2 says to avoid webcam, microphone and screen capture unless specifically necessary and separately consented ([§16](SPEC.md#privacy-and-security-safeguards)), and says window switching or pasting is not proof of anything ([§16](SPEC.md#signals-that-are-not-proof-by-themselves)).
3. **Flags.** V1 labels flags High / Medium / Low. V2 has three distinct categories, plus a candidate response and appeal process ([§16](SPEC.md#integrity-workflow)).
4. **Report.** V1's report leads with a recommendation (Strong hire … No hire). V2 forbids single-score verdicts. It is a capability profile with confidence, coverage and explicit unknowns, and the decision is recorded as a human one ([§19](SPEC.md#19-candidate-report--capability-profile)).
5. **Assistant.** V1's assistant is prompt-limited, with a message cap. V2 needs server-side policy enforcement: tool, domain, scope, credentials, audit ([§13](SPEC.md#assistant-policy-gateway)).
6. **Recruiter setup.** V1's company portal creates roles and invites candidates; tasks are generated per candidate. V2 adds blueprints, essential versus differentiator competencies, versioned rubrics, the AI policy mode, preview and dry-run ([§7](SPEC.md#7-recruiter-setup-flow)).
7. **Retention.** V1 promises 90-day deletion. Its cron only removes media and has never run. V2 makes retention and deletion a release-gate item ([release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass)).

## About `candidate/v2/`

`candidate/v2/` holds a parked refactor of V1's **browser** workspace: the
1,300-line `IdeShell` split into hooks, plus fixes to time-up, saving and the
clock. It predates this spec. Since V2 moves coding into the candidate's own
IDE, it fits V2 only as the browser fallback, if V2 keeps one. Whether it
stays is one of the decisions below. Some of its fixes (the telemetry
backlog bug, saving on close) would help V1 too — but that's a separate,
explicitly approved V1 change, not part of V2.

## Order of work

The spec's phases are in [§21](SPEC.md#21-mvp-to-v2-implementation-plan). Before Phase 1, this repo needs a **Phase 0** that the spec's safe workflow ([§2, steps 1–3](SPEC.md#safe-implementation-workflow)) implies:

0. **Protect V1.**
   - **Inventory:** list V1's entry points, routes, tables, jobs and env vars.
   - **CI:** run typecheck, lint, the check files, vitest and `go test` on every PR.
   - **Contract tests:** write them for V1's critical journeys — sign-in, dashboard, IDE, assistant, save, submit, report. These become the "V1 regression suite" that the release gate requires.
1. **Assessment foundation:**
   - blueprints and versioned rubrics
   - the V2 session and consent flow
   - task delivery and a common core
   - Git status/diff and submission
   - the evidence schema
   - a report with explicit unknowns
   - human review
2. **Local CLI and reliability:**
   - a signed, versioned CLI
   - preflight checks
   - IDE launch
   - a short-lived agent with an offline queue
   - credential cleanup
3. **Multi-mode:** the design canvas, Research Lab, debugging, review, testing and delivery evidence, and links between artifacts.
4. **Avatar and adaptive follow-ups:** bounded in weight, with pause/defer/text controls.
5. **Integrity, integrations, calibration:** the three integrity categories, GitHub private repos, optional CI and deployment, retention and audit, fairness studies.
6. **Production hardening**, then the [release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass), a pilot cohort, and a reversible cut-over.

## Decisions needed before building

- **Data isolation.** A separate Supabase project, or a `v2` schema in the current one? The spec defaults to separate. A schema is cheaper, but shares the project's auth, storage and service-role key.
- **Backend.** A new V2 service (Go, to match `candidate/backend`?) or something else? Where is it hosted?
- **Web app.** One V2 Next.js app holding both the candidate and recruiter sides, or two? On which domain?
- **CLI.** Language and distribution (signed binaries per OS), and which IDEs are supported at launch.
- **Browser fallback.** Does V2 keep a browser coding workspace? If not, `candidate/v2/` is retired.
- **First role family.** Broad software engineering, or the quant-research blueprint ([§14](SPEC.md#quantitative-researcher-assessment-blueprint))?
- **AI.** Which models the assistant, evaluators and avatar use (V1 uses Gemini directly), and the per-company and global cost caps.
- **Accounts.** Can V2 candidates and recruiters use their V1 accounts, or are V2 accounts separate as the spec's isolation implies?
