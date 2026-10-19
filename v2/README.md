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

**[`PLAN.md`](PLAN.md) is how V2 gets built:** V2 runs as a new kind of
assessment inside the existing product, reusing V1's accounts, dashboard and
company portal, so cut-over is a switch, not a migration. It also covers the
guards that keep V1 safe, the path to production, and the tasks in order,
with the CLI first.

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

V1 stays the only production candidate path. V2 changes V1 at a handful
of connection points only, all hidden while the switch is off
([PLAN, Part 1, section 5](PLAN.md#5-the-connection-points-the-only-v1-changes)).
No V1 session, existing data, job or deployment setting is changed.

## What "isolated" means in this repo

The spec's isolation table ([§2](SPEC.md#required-isolation-boundaries)), against what V1 actually is today:

| Area | V1 today (do not touch) | V2 needs |
|---|---|---|
| Source | `candidate/frontend`, `company/frontend`, `internal-admin/frontend`, `candidate/backend`, `company/backend` | Its own packages under `v2/` (and the parked `candidate/v2/`, see below) |
| URLs | `candidate.mindfries…`, company and admin Vercel projects | Its own Vercel project for the new V2 screens, entered from the V1 dashboard through a signed handoff link |
| Deployment | Vercel: three projects. Railway: `candidate/backend` | New V2 Vercel/Railway projects with their own pipelines |
| Env / secrets | `SESSION_SECRET`, `SUPABASE_*`, `CANDIDATE_BACKEND_URL`, Gemini, Resend, Composio, Daytona keys | V2-prefixed variables and V2-scoped credentials; nothing shared or overwritten |
| Database | One shared Supabase project, `supabase/migrations/0001–0027`, applied by `candidate/frontend/scripts/migrate.mts` into one `schema_migrations` | **Decided:** a `v2` schema in the same project, with its own migrations and runner. A V2 database role can only read the V1 tables it needs. The one V1 migration adds `assessments.engine` (default `v1`) |
| API | Go `candidate/backend` at `/api/v1/…`, plus Next route handlers | A versioned `/v2/…` API in its own service |
| Jobs | Go workers (evaluation, task generation), internal-admin crons (retention, discovery) | V2's own workers, queues and schedules |
| Sessions / auth | `mf_candidate` cookie, `candidate_users`, `sessions` | **Decided:** the same V1 accounts. V2 keeps its own session state, started from a single-use handoff link; V1's cookie is not touched |
| AI | Gemini, direct (per-session message cap; interview off for the pilot) | The V2 assistant behind the policy gateway, its own keys and budgets |
| Telemetry | `activity_events`, `assessment_reports`, `evidence_items` | A separate V2 evidence store and event namespace |
| Tests | Almost none: a few `*.check.ts` files, one vitest file, Go unit tests, **no CI** | V1 contract tests and CI first ([PLAN, Stage 0](PLAN.md#stage-0--guard-v1-before-any-v2-code)) |

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

The order is set in [`PLAN.md`](PLAN.md#part-3--tasks-in-order). It
starts by guarding V1 (CI, an inventory, a V1 regression suite), then
builds the CLI, then the V2 API and the connection points into V1, then the
rest of the spec's phases ([§21](SPEC.md#21-mvp-to-v2-implementation-plan)).

## Decisions needed before building

Already decided, see [PLAN, Decisions](PLAN.md#decisions):
- **Data:** a `v2` schema in the current project, with a V2 database role.
- **Accounts:** shared with V1, through a handoff link.
- **Web app:** only the new V2 screens.
- **Supported IDEs:** no agent-first IDEs.
- **What the CLI observes.**

Still open:

- **Supabase plan limits.** Sharing the project means V2's load counts against V1's limits. Check them before the pilot.
- **Backend.** A new V2 service (Go, to match `candidate/backend`?) or something else? Where is it hosted?
- **Web app domain.** Which domain the V2 screens are served from.
- **CLI.** Language (Go is recommended), distribution, and code-signing certificates.
- **Browser fallback.** Does V2 keep a browser coding workspace? If not, `candidate/v2/` is retired.
- **First role family.** Broad software engineering, or the quant-research blueprint ([§14](SPEC.md#quantitative-researcher-assessment-blueprint))?
- **AI.** Which models the assistant, evaluators and avatar use (V1 uses Gemini directly), and the per-company and global cost caps.
