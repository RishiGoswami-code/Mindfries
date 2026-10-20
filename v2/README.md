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

**Status: the CLI (Stage 1 and 1b) is built and tested against a fake API ([`cli/`](cli)); the V2 API, web screens and database aren't started. V1 is unchanged.**

## Layout

Every line of V2 lives under `v2/` ([PLAN, Part 1 §1](PLAN.md#1-one-folder-and-a-check-that-enforces-it)).
Each folder's README says what goes there and the rules it follows.

| Folder | What it is | Built in |
|---|---|---|
| [`cli/`](cli) | The `mindfries` CLI (Go): setup, the session watcher, `mindfries agent`, submit, plus a fake API for testing | Stage 1: built |
| [`contracts/`](contracts) | OpenAPI contracts between the pieces, starting with `cli-api.yaml` | Task 1.1: drafted |
| [`api/`](api) | The V2 API (Go), its own Railway service | Stage 2 |
| [`db/`](db) | Migrations for the `v2` schema, with their own runner | Stage 2 |
| [`web/`](web) | Only the new V2 screens and the floating panel (Next.js), its own Vercel project | Stage 3 |

## V2 in one paragraph

V2 turns the assessment from "code in our browser IDE, chat with our
assistant" into an evidence-first work session across several kinds of work.
- **Where the candidate codes:** in their own IDE, set up by a session-scoped local CLI and a short-lived agent ([§9](SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension)).
- **The assistant:** `mindfries agent`, a coding and research assistant in the terminal, under the assessment's policy (see [below](#where-this-repos-v2-departs-from-the-spec)).
- **Getting started:** starting a V2 assessment on the dashboard gives the candidate one command, unique to them and that assessment. Running it installs `mindfries`, connects it to the portal, and downloads the codebase plus a locked `TASK.md`.
- **The browser:** stays the hub, with one floating panel of tabs. Camera, Stage & Task, System Design, Debugging, Notes & Evidence, Conversation and Submission review cover everything except coding with the assistant ([§10](SPEC.md#10-the-unified-candidate-workspace)).
- **Routes:** each assessment adapts to the candidate — a common core, an evidence-driven route and targeted follow-ups — and "not assessed" is kept apart from "weak" ([§11](SPEC.md#11-adaptive-assessment-candidates-can-be-strong-in-different-ways)).
- **Policy:** a backend policy gateway decides what the assistant and developer platforms may do ([§13](SPEC.md#13-floating-ai-help-assistant-and-tool-permissions)).
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
2. **Monitoring.** V1 keeps camera stills every 30 seconds, screen stills, and microphone clips on noise, and flags leaving the workspace, pastes and second windows. V2 keeps the camera, with separate consent ([departure 6](#where-this-repos-v2-departs-from-the-spec)), but says window switching or pasting is not proof of anything ([§16](SPEC.md#signals-that-are-not-proof-by-themselves)).
3. **Flags.** V1 labels flags High / Medium / Low. V2 has three distinct categories, plus a candidate response and appeal process ([§16](SPEC.md#integrity-workflow)).
4. **Report.** V1's report leads with a recommendation (Strong hire … No hire). V2 forbids single-score verdicts. It is a capability profile with confidence, coverage and explicit unknowns, and the decision is recorded as a human one ([§19](SPEC.md#19-candidate-report--capability-profile)).
5. **Assistant.** V1's assistant is prompt-limited, with a message cap. V2 needs server-side policy enforcement: tool, domain, scope, credentials, audit ([§13](SPEC.md#assistant-policy-gateway)).
6. **Recruiter setup.** V1's company portal creates roles and invites candidates; tasks are generated per candidate. V2 adds blueprints, essential versus differentiator competencies, versioned rubrics, the AI policy mode, preview and dry-run ([§7](SPEC.md#7-recruiter-setup-flow)).
7. **Retention.** V1 promises 90-day deletion. Its cron only removes media and has never run. V2 makes retention and deletion a release-gate item ([release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass)).

## Where this repo's V2 departs from the spec

Owner decisions that change what [`SPEC.md`](SPEC.md) says. The spec is
otherwise unchanged and still governs.

1. **The assistant lives in the terminal.** The spec puts AI Help in the floating panel ([§13](SPEC.md#13-floating-ai-help-assistant-and-tool-permissions)). Here it is `mindfries agent`, a terminal coding and research assistant run by the CLI. Its policy gateway, modes, logging and limits still follow §13. The floating panel has no chat and handles everything else. Where [`ROLE_FLOWS.md`](ROLE_FLOWS.md) says "AI Help", read `mindfries agent`.
2. **Outside coding agents are not allowed.** Candidates code with `mindfries agent` and by typing in their own IDE, not with their own agents. The CLI looks for outside agents and reports what it finds as review signals ([PLAN, task 1.8](PLAN.md#stage-1--the-cli-9-17)). Agent-first IDEs are not offered. The candidate may research in their own browser; only coding agents are ruled out.
3. **V2 reuses V1.** It uses V1's accounts, dashboard and company portal, and keeps its data in a `v2` schema in the same database. The spec permits this; its default is a separate project. Cut-over is therefore a switch ([PLAN, Part 1](PLAN.md#part-1--how-v2-reuses-v1-and-how-v1-stays-safe)).
4. **Install once, then `mindfries start` with a browser sign-in.** The spec has the browser show a session-specific command that fetches the CLI and then authenticates ([§9](SPEC.md#example-candidate-flow)). Here the candidate installs `mindfries` once with a generic command (it goes on their PATH) and types `mindfries start`, with no code, key or address to paste. `start` opens a Mindfries sign-in page that shows the same short code as the terminal; the candidate signs in, checks the codes match, and picks the assessment they were invited to. The CLI then receives that one session's token and nothing else (an RFC 8628 device sign-in, [contract](contracts/cli-api.yaml) `POST /sessions/device`). §9's requirements still hold: the sign-in is specific to the candidate and the session, expires in about 10 minutes, leaves nothing reusable in shell history, and the only credential on the laptop is the session-scoped token. A dashboard command with a single-use setup code (`mindfries start <code>`) still works as a shortcut. Decided by the owner on 2026-10-19 ([PLAN, tasks 1.4 and 1.12](PLAN.md#stage-1--the-cli-9-17)).
5. **`TASK.md` is locked.** The candidate can't change it. On their own laptop we can't truly prevent an edit, so the CLI marks it read-only, watches its hash, flags any edit and restores the file. The server's copy is the one that counts ([PLAN, tasks 1.5 and 1.7](PLAN.md#stage-1--the-cli-9-17)).
6. **The camera stays.** The spec says to avoid the webcam unless necessary and separately consented ([§16](SPEC.md#privacy-and-security-safeguards)). Here it is kept as in V1: a Camera tab in the floating panel, with periodic stills kept as evidence, after explicit consent. Like every other signal, what it shows is evidence for a human, not a verdict. **Update (owner, 2026-10-20):** monitoring now covers the camera, the screen and the microphone, in a Mindfries panel in the candidate's browser (the floating panel; a terminal program can't reach them). The panel keeps a camera still and a screenshot every 30 seconds and the microphone in 30-second clips. Each assessment says which of the three it needs; `mindfries start` doesn't begin until they're on, and a share that stops is recorded as a gap, never a verdict.
7. **The work lives in the candidate's own GitHub.** The spec makes GitHub optional by role ([§17](SPEC.md#17-git-github-ci-and-deployment)). Here, so that deployment (CI on GitHub Actions, a deploy to AWS, Supabase, Render and the like, and the pipeline the candidate writes) can be assessed, the candidate connects their GitHub on the same sign-in page in one click: Connect sends the browser to GitHub's "Authorize Mindfries" page (a Mindfries OAuth app; GitHub skips the page once they've authorised it), and back. No code to type. `mindfries start` then creates a **private** repository in their account, invites the assessment's reviewers (its hiring team) with read access and pushes the work there. The Mindfries server exchanges GitHub's one-time code for the candidate's token and hands it to the CLI once, without keeping it; the CLI holds it (scope `repo`) in the OS keychain for the session, which the contract allows only for this, and deletes it when the session ends. `submit` pushes the final commit and takes the deployed URL, which Mindfries checks. The bundle is still the record of what was handed in. The sign-in page itself knows the candidate from their Mindfries browser session and doesn't ask for an email. Decided by the owner on 2026-10-19 ([contract](contracts/cli-api.yaml) `Session.github`).
8. **Start assessment, then setup, then start.** The candidate presses **Start assessment** on the dashboard and gets one command with a single-use code (installs `mindfries` if needed, then `mindfries setup <code>`). Setup downloads and checks the task and codebase, connects GitHub and pushes the codebase to the candidate's new repository; the clock doesn't start. `mindfries start` then asks which installed editor to use, checks the computer (any failure: nothing starts, no time used), turns on the required monitoring, and only then starts the clock and opens the editor in the workspace. `mindfries setup` without a code still signs in through the browser (decision 4). Decided by the owner on 2026-10-20 ([contract](contracts/cli-api.yaml) `POST /session/begin`).

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
- **Setup:** one command that installs the CLI and connects it.
- **`TASK.md`:** locked; edits are flagged and undone.
- **Camera:** kept, with consent.

Still open:

- **Supabase plan limits.** Sharing the project means V2's load counts against V1's limits. Check them before the pilot.
- **Backend.** A new V2 service (Go, to match `candidate/backend`?) or something else? Where is it hosted?
- **Web app domain.** Which domain the V2 screens are served from.
- **CLI.** Language (Go is recommended), distribution, and code-signing certificates.
- **Browser fallback.** Does V2 keep a browser coding workspace? If not, `candidate/v2/` is retired.
- **First role family.** Broad software engineering, or the quant-research blueprint ([§14](SPEC.md#quantitative-researcher-assessment-blueprint))?
- **AI.** Which models the assistant, evaluators and avatar use (V1 uses Gemini directly), and the per-company and global cost caps.
