# V2 build plan

How V2 gets built beside V1 without disturbing it, then tested and deployed.
The tasks run in order. **The CLI comes first.**

It follows [`SPEC.md`](SPEC.md). The governing rule is [§2](SPEC.md#the-governing-rule):

> Build, test, and run V2 independently until the V2 acceptance criteria
> pass. Only then perform a planned, reversible cutover.

[`README.md`](README.md) lists the open decisions. [`ROLE_FLOWS.md`](ROLE_FLOWS.md)
shows what the finished product does for an FDE and a DevOps role.

---

## Part 1 — How V2 reuses V1, and how V1 stays safe

### The approach: V2 is a new kind of assessment inside the existing product

V2 does not rebuild what V1 already does well. Candidates and companies keep
the same accounts and screens, and V2 adds only what is new.

| Stays V1, reused as is | New in V2 |
|---|---|
| Sign-in and accounts (candidates and companies) | The CLI and the V2 API it talks to |
| Candidate dashboard, profile, settings, job portal | The workspace hub: setup command, live session status, stage view |
| Company portal: roles, invites, candidates, credits | `mindfries agent`, our coding and research assistant, in the terminal; and the floating panel for everything else |
| Internal admin | The evidence timeline, evaluation and capability-profile report |
| V1 assessments, from start to finish | Recruiter blueprints and versioned rubrics |

Every assessment gets an **`engine`** field, `v1` or `v2`, with a default of
`v1`. A V2 assessment appears on the same V1 dashboard. Its Start button and
its report link go to V2. **Cut-over is changing the default from `v1` to
`v2`; nothing is copied or migrated** (see section 7).

The spec allows this: V2 may reuse stable V1 parts, and V1 may have V2 code
paths "isolated and feature-disabled by default" ([§2](SPEC.md#safe-implementation-workflow), step 4).

### 1. One folder, and a check that enforces it

Every line of V2 lives under `v2/`. V2 work is merged into `main` in small
PRs, not kept on a long-lived branch that drifts from `main`.

A CI job, the **V1 guard**, fails any PR that mixes V2 work with changes
outside `v2/`. The only V1 changes V2 ever needs are the **connection
points** (section 5). Each goes in its own small PR, labelled `v1-seam`, and
the owner approves it on purpose. The root orientation file and `.github/`
are the other exceptions.

### 2. Its own modules

V2 has its own modules:

| Module | Language | Contents |
|---|---|---|
| `v2/cli` | Go | The CLI |
| `v2/api` | Go | The API |
| `v2/web` | Next.js | Only the new V2 screens; no sign-in, dashboard or settings of its own |
| `v2/db` | SQL | Migrations for the `v2` schema and their own runner |

None of them imports from a V1 package. If V2 needs something V1 has, it
copies a thin piece and owns it, as the spec prefers in [§2](SPEC.md#what-v2-adds-or-changes).
A V1 refactor can then never break V2, and the reverse holds too.

### 3. Its own data, beside V1's

V2's data lives in a **`v2` schema in the current Supabase project**. It is
keyed to V1's existing candidate, company, role and assessment IDs, so
nothing has to be copied across.

The protection is enforced by the database, not by care. The V2 API connects
as its own Postgres role with these grants:
- read-only on the V1 tables it needs: `candidate_users`, `companies`, `company_users`, `job_roles`, `assessments`;
- full access to the `v2` schema;
- nothing else.

So a V2 bug *cannot* write to a V1 table.

V2's migrations live in `v2/db`, with their own `v2.schema_migrations`. The
one V1 migration is additive: an `engine` column on `assessments`, with a
default of `v1`. Tests use synthetic data only.

### 4. Its own deployments and secrets

V2 has its own Vercel project for `v2/web` and its own Railway service for
`v2/api`. Its environment variables are prefixed with `V2_`. It has its own
keys (Gemini, Resend, GitHub app) and its own database role.

Nothing in V1's Vercel or Railway settings changes, apart from the switch
in section 5.

### 5. The connection points: the only V1 changes

| Where | Change | With the switch off |
|---|---|---|
| Database | `assessments.engine` column, with a default of `v1` | Every row is `v1`, as today |
| Sign-in handoff | V1 signs a short-lived, single-use link token (user, assessment). `v2/web` exchanges it for its own session. V1's cookie is not touched | Never issued |
| Candidate dashboard | A `v2` assessment's Start button and report link go to V2 through the handoff | No `v2` assessments exist, so nothing shows |
| Company portal | Choosing the engine for an assessment, and opening a V2 candidate's report | Hidden |
| The switch | `V2_ENABLED` (off or on) and `V2_DEFAULT_ENGINE` (`v1` or `v2`) | Off: V1 behaves exactly as today |

### 6. Proof that V1 still works

V2 now touches V1 at those few points. So V1 gets a CI run on every PR
**before the first connection point lands**:

- typecheck, lint and build for the three apps;
- `go test` for the backends;
- a contract suite for V1's critical journeys.

That suite is the **V1 regression suite** that the [release gate](SPEC.md#v1-non-regression-and-coexistence) requires.

### 7. Cut-over is a switch

Cut-over has four parts:
- **Pilot:** turn on `V2_ENABLED` for a pilot cohort's assessments.
- **Cut-over:** after the release gate, set `V2_DEFAULT_ENGINE=v2`, so new assessments are V2.
- **Rollback:** set it back to `v1`.
- **Existing work:**
  - V1 assessments already running finish on V1;
  - V1 reports stay readable in V1's format;
  - accounts and company data never move.

---

## Part 2 — The path from code to production

```
local (fake API)  →  CI (V1 guard + V1 suite + V2 tests)  →  merge to main (v2/ only)
     →  connection points merged into V1, switch off (V1 suite proves no change)
     →  V2 staging (own services, v2 schema, synthetic data)  →  coexistence test with V1
     →  pilot cohort (switch on for them)  →  release gate sign-off
     →  cut-over: default engine v1 → v2  (rollback: back to v1)
```

---

## Part 3 — Tasks, in order

Each task is one PR and has a "done when". Tick it off here when it lands.

### Stage 0 — Guard V1 (before any V2 code)

- [ ] **0.1 V2 workspace layout.**
  - Create `v2/cli`, `v2/api`, `v2/web`, `v2/db` and `v2/contracts`, each with a README saying what goes there.
  - *Done when:* the folders exist and the top-level `v2/README.md` maps them.
- [ ] **0.2 CI, with the V1 guard.**
  - Add `.github/workflows/`, with:
    - the V1 guard, keyed on the PR's changed paths;
    - V1 typecheck, lint and build;
    - `go test`;
    - a V2 job that is empty for now.
  - The one exception is a PR labelled `v1-seam`, for the connection points.
  - *Done when:* a test PR that touches `candidate/` from a V2 branch fails the guard, and a clean one passes.
- [ ] **0.3 V1 inventory.**
  - Write `v2/V1_INVENTORY.md`, listing V1's:
    - routes and entry points;
    - tables and migrations;
    - auth and session cookies;
    - jobs and crons;
    - environment variables;
    - deployments.
  - Mark the files that V2 must never change. List the connection points (Part 1, section 5) and the V1 tables V2 may read.
  - *Done when:* reviewed by the owner.
- [ ] **0.4 V1 regression suite.**
  - Contract tests for sign-in, the dashboard, opening the IDE, an assistant reply, saving, submitting and the report, on both the candidate and company sides.
  - Run them in CI.
  - *Done when:* they're green on `main`, and they fail when one of those journeys is broken on purpose.

*0.4 must land before any connection point (task 2.3). It doesn't block the
CLI, which runs only on a laptop, so Stage 1 can start alongside it.*

### Stage 1 — The CLI ([§9](SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension), [§17](SPEC.md#git-workflow))

The CLI is built against a **fake V2 API**, a small local server that
follows the contract. It can therefore be finished and tested before the real
backend exists. The candidate's flow is the spec's 14 steps, with one change:
steps 2–4 are "install once, then `mindfries start`", which signs in through
the browser ([README, decision 4](README.md)).

In short:
1. The candidate installs `mindfries` once with **one command**, which puts it
   on their PATH (task 1.12).
2. `mindfries start` opens a Mindfries sign-in page, where they confirm the
   code their terminal shows and pick the assessment they were invited to;
   the CLI receives that session's token (task 1.4).
3. The CLI downloads the codebase and `TASK.md`. `TASK.md` is locked (task 1.5).
4. The candidate works in their own IDE, typing and using `mindfries agent`,
   which can also research. They may research in their own browser too; outside
   coding agents are not allowed and are flagged (task 1.8).
5. The floating panel in the browser holds the camera and the other tabs (Stage 5).
6. After submission, the evaluator assesses everything in the evidence (Stage 6).

The commands are:

| Command | What it does |
|---|---|
| `mindfries start` | Signs in through the browser, sets up the workspace and runs the session watcher |
| `mindfries agent` | Opens **our coding and research assistant** in the terminal (tasks 1.14–1.17) |
| `mindfries status` | Shows the session, sync and Git state |
| `mindfries submit` | Reviews the diff and hands in |
| `mindfries stop` | Ends the session watcher and cleans up |
| `mindfries doctor` | Runs preflight checks only |

Two different things could both be called an "agent", so the plan names them apart:
- **The session watcher** is the spec's "local assessment agent" ([§9](SPEC.md#local-agent-responsibilities)). It runs quietly beside the work and records evidence.
- **`mindfries agent`** is the AI assistant the candidate talks to.

- [ ] **1.1 The CLI ↔ API contract.**
  - Write `v2/contracts/cli-api.yaml` (OpenAPI). It covers:
    - the browser sign-in, and exchanging a setup code;
    - the session token and its expiry;
    - the task bundle and its manifest;
    - heartbeat;
    - event batches, including the coding-agent signals;
    - diff snapshots;
    - assistant turns for `mindfries agent`, streamed, with tool calls and the edits it applies;
    - the submission manifest and upload;
    - revoke;
    - the minimum CLI version.
  - *Done when:* the contract is reviewed. Every later task is built against it.
- [ ] **1.2 Skeleton and fake API.**
  - Create the Go module `v2/cli` with the six commands as stubs, plus `v2/cli/fakeapi`, a scriptable server that serves the contract.
  - Add version output and a build for Windows, macOS and Linux (amd64 and arm64).
  - *Done when:* `mindfries --version` works on all three operating systems in CI.
- [ ] **1.3 Preflight (`doctor`).**
  - Checks:
    - OS and architecture;
    - that Git is installed, and its version;
    - free disk space;
    - that the API can be reached;
    - which supported IDEs are installed;
    - the runtimes the task needs.
  - Every failure prints the fix, as in the spec's [failure table](SPEC.md#environment-failure-handling). Nothing counts against the candidate.
  - *Done when:* each check has a unit test, including the missing-tool cases.
- [ ] **1.4 Session authentication.**
  - `mindfries start` signs the candidate in through the browser (an RFC 8628 device sign-in): it opens a Mindfries page showing the same short code as the terminal, the candidate confirms and picks the assessment they were invited to, and the CLI receives that one session's token. Nothing is pasted, so shell history holds nothing.
  - A dashboard setup command (`mindfries start <setup-code>`) still works: the code is **single-use** and expires in about 10 minutes.
  - Either way `start` ends up with a session-scoped token. The token is stored in the OS keychain, or failing that in a file only the user can read. It is never printed.
  - The token expires with the session.
  - *Done when:* reusing a code, an expired code, an expired token, and a refused or expired sign-in are all refused (tested against the fake API).
- [ ] **1.5 Workspace setup.**
  - The candidate confirms the target folder.
  - The CLI downloads the task bundle and checks every file's hash against the manifest. It records the task version.
  - It runs `git init`, or clones, then:
    - makes an assessment branch;
    - sets a local Git identity, scoped to the repo;
    - writes `TASK.md` and marks it read-only;
    - makes a baseline commit.
  - `TASK.md` is locked: the candidate can't change it. The CLI can't truly stop an edit on the candidate's own machine, so it also records the file's hash for 1.7 to watch, and the server's copy is the one that counts.
  - It refuses a folder that isn't empty unless the candidate confirms.
  - *Done when:* a tampered bundle is rejected, and a re-run resumes rather than duplicating.
- [ ] **1.6 IDE launch.**
  - Supported IDEs: VS Code, JetBrains IDEs (via their launchers) and Zed. Agent-first IDEs (Antigravity, Cursor, Windsurf) are not offered, and 1.8 marks them if they're open anyway.
  - The CLI detects which are installed and opens the one the candidate picks. If none is found, it prints the path to open by hand.
  - *Done when:* launch works or fails gracefully on each OS.
- [ ] **1.7 The session watcher** (the spec's local agent).
  - It runs in the foreground: never a service, and never at login.
  - It sends a heartbeat.
  - It reads Git status and diff stats on an interval.
  - It sees file changes **inside the workspace only**, as the policy allows.
  - It watches `TASK.md`'s hash. An edit is recorded as an observed event, and the file is restored from the bundle.
  - Its offline queue is a file in which every event is chained to the one before it, so tampering shows.
  - Events go in batches with retry and backoff. Batches are capped in size, and a batch the server refuses as malformed is dropped. These are the lessons from V1's telemetry backlog bug.
  - It shows "last synced …".
  - *Done when:*
    - if the network is cut for 10 minutes, nothing is lost and nothing is duplicated after reconnecting;
    - an edit to `TASK.md` is recorded and undone.
- [ ] **1.8 Coding-agent signals.**
  - The policy: candidates may use **our assistant, `mindfries agent`,** to write code, but not their own coding agents (Claude Code, Gemini CLI, Codex, Aider, Copilot CLI, Antigravity, Cursor's agent, Windsurf and the like). The candidate is told this, and told what the CLI checks, before they start.
  - The CLI collects five signals. None of them needs special permissions or reads anything private:
    1. **Agents running.** It matches running process names against a list of known coding agents and agent IDEs. At `start`, a match asks the candidate to close it before the timer begins. During the session it checks every few minutes, and a match becomes a review signal. Only the matches are sent, never the full list of processes.
    2. **AI extensions in the IDE.** It reads the editor's list of installed extensions, for example `code --list-extensions`, and reports the AI ones: Copilot, Cline, Continue, Roo, the Claude Code and Gemini extensions. It does the same for JetBrains' AI Assistant and Junie, and for Zed's agent settings.
    3. **Leftovers in the folder.** It scans for `CLAUDE.md`, `.claude/`, `AGENTS.md`, `GEMINI.md`, `.cursor/`, `.windsurf/`, `.aider*` and Copilot instruction files. It also scans commit messages for agent trailers such as "Co-Authored-By: …" or "Generated with …".
    4. **How the code arrives.** It records each file change's size and timing. A person writes in small edits spread out over time. An agent writes whole files, often several at once, within seconds. The CLI marks these bursts.
    5. **Where each change came from.** Every edit `mindfries agent` applies is recorded exactly (task 1.17), so the watcher can sort each change. It came from our agent, it was built up gradually by the candidate, or it came from neither. Large blocks that came from neither are marked. Periodic Git diff snapshots let the server check this.
  - Every signal is an observed event or a review signal ([§16](SPEC.md#use-three-distinct-categories)). None is a verdict, and a human decides. The real check is whether the candidate understands the code, which the avatar's follow-ups test in Stage 7.
  - *Done when:* each signal has a test, using a fake agent process, a fake extension list, a planted `CLAUDE.md` and a scripted file burst. Nothing outside the listed checks is read.
- [ ] **1.9 Submit.**
  - The candidate sees the diff and writes a commit message.
  - The CLI commits and builds the submission manifest:
    - task version;
    - base and head commits;
    - changed files, with hashes;
    - test and build results;
    - timestamp.
  - It uploads a Git bundle, and the server checks it against the manifest.
  - *Done when:* a bundle that doesn't match is refused, and a successful submit is shown in `status`.
- [ ] **1.10 Stop and clean up.**
  - `stop`, a submit or the session expiring each:
    - end the agent;
    - revoke the token on the server;
    - delete local credentials and session state;
    - **leave the candidate's code in place**.
  - *Done when:* no token or credential is left on disk after any of the three.
- [ ] **1.11 Failure paths.**
  - Each row of the spec's failure table has a clear message, a recovery path and a test:
    - Git missing;
    - IDE missing;
    - dependency install fails;
    - clone fails;
    - network loss;
    - IDE crash;
    - agent crash;
    - unsupported OS;
    - a coding agent still open at `start`.
  - *Done when:* every row is covered.
- [ ] **1.12 Signed, versioned distribution.**
  - Release builds for each OS, with checksums and a signature.
  - A one-line installer that verifies the checksum, puts `mindfries` in a per-user folder and adds that folder to the user's PATH (no admin rights), so the candidate then types `mindfries start`. Given a setup code, it also runs `start` with it.
  - The API address is baked into each build; the candidate sets no environment variables.
  - The server refuses CLI versions older than its minimum.
  - Windows (Authenticode) and macOS (notarization) need certificates; see the decisions below.
  - *Done when:* a tampered binary is refused by the installer.
- [ ] **1.13 CLI end-to-end in CI.**
  - Run the full `start → work → submit → stop` flow against the fake API on Windows, macOS and Linux runners.
  - *Done when:* the flow is green on all three.

### Stage 1b — `mindfries agent`: our coding and research assistant

The assistant lives in the terminal, beside the candidate's IDE, and not
in the browser. It works like the coding agents candidates already know, but
it runs under the assessment's policy, and everything it does is evidence.
It only runs while a session started with `mindfries start` is active.

- [ ] **1.14 The assistant gateway** (on the fake API first, for real in 2.2).
  - **Server-side model calls.** The model is called by the server, never from the laptop, so no model key ever reaches the candidate's machine.
  - **Policy mode,** set per assessment:
    - off;
    - explain only, with no code written;
    - code allowed.
  - **Limits:** a token and cost budget for each session.
  - **Logging:** every prompt, reply and tool call is logged as evidence, linked to the session.
  - *Done when:*
    - in "explain only" mode, the server refuses a turn that asks to write a file;
    - a session over budget is refused with a clear message.
- [ ] **1.15 The terminal interface.**
  - `mindfries agent` opens a chat in the terminal; the IDE's built-in terminal works too.
  - Replies stream in. Before any change is applied, the candidate sees it as a diff and accepts or rejects it.
  - It shows the policy mode and the budget left.
  - *Done when:* it works in Windows Terminal, the macOS Terminal, a Linux terminal and the VS Code terminal.
- [ ] **1.16 Its tools, and their limits.**
  - **Files:** read files and propose edits, inside the task folder only.
  - **Commands:** run commands (tests, builds) in the task folder, after the candidate confirms each one.
  - **Research:** look things up only through the server. The domains are those the assessment allows (official docs, package registries). Every source it uses is recorded and shows up in the floating panel's Notes & Evidence.
  - **Never:**
    - read outside the task folder;
    - touch credentials;
    - push to Git;
    - install anything without asking.
  - *Done when:* a test shows each "never" is refused, and research to a domain that isn't allowed is refused by the server.
- [ ] **1.17 Attribution.**
  - Every edit the agent applies is recorded with the file, the lines and a hash.
  - The session watcher (1.7) uses these records to sort each change:
    - written by our agent;
    - typed by the candidate;
    - from neither, which feeds 1.8.
  - The report can then show how much of the work came from each source.
  - *Done when:* in a scripted session that mixes agent edits, typing and a pasted block, each change is attributed correctly.

### Stage 2 — The V2 API, and connecting V2 to V1 ([§6 core services](SPEC.md#core-services))

- [ ] **2.1 V2 schema and database role.**
  - Create the `v2` schema and migrations in `v2/db` for sessions, setup codes, tasks and versions, evidence events and submissions. Rows are keyed to V1 IDs.
  - Create a V2 Postgres role: read-only on the V1 tables listed in 0.3, full access to `v2`, nothing else.
  - *Done when:*
    - applying the migrations twice is a no-op the second time;
    - a test shows the V2 role is refused when it tries to write any V1 table.
- [ ] **2.2 `v2/api`: the contract from 1.1, for real.**
  - Implement sessions, setup codes, token issue and revoke, task bundles, events and submissions, with manifest verification.
  - Implement the assistant gateway from 1.14 with real model calls, policy modes and budgets.
  - *Done when:* the CLI's end-to-end tests from 1.13 pass against the real API as well as the fake.
- [ ] **2.3 Connection points in V1** (`v1-seam` PRs, switch off).
  - Add the migration for `assessments.engine` (default `v1`) and the `V2_ENABLED` and `V2_DEFAULT_ENGINE` switch.
  - Add the signed handoff link, and its exchange endpoint in `v2/api`.
  - Route the Start button and report link for `v2` assessments on the candidate dashboard.
  - On the company portal, add the engine choice and the V2 report link.
  - *Done when:*
    - with the switch off, the V1 regression suite passes unchanged;
    - with it on, a `v2` assessment goes from the dashboard to the V2 workspace and back without signing in again;
    - a handoff link can't be reused or used after it expires.
- [ ] **2.4 V2 staging.**
  - Its own Railway service and Vercel project, the `v2` schema, `V2_` secrets, and synthetic data.
  - *Done when:* it deploys, and the V1 suite and the coexistence check still pass with V2 running.

### Stage 3 onward — the rest of V2

These follow the spec's phases ([§21](SPEC.md#21-mvp-to-v2-implementation-plan)).
Each will be split into tasks like Stage 1 when it starts.

- **3. Candidate web app (`v2/web`):** only the new screens. Candidates arrive through the handoff from the V1 dashboard; there is no sign-in, dashboard or settings of its own.
  - V2 consent: what the CLI checks, including 1.8, and the camera;
  - "Set up my workspace", which shows the install command and `mindfries start`;
  - the sign-in page that `mindfries start` opens (confirm the code, pick the assessment);
  - live session status from the agent;
  - the stage and journey view from [`ROLE_FLOWS.md`](ROLE_FLOWS.md);
  - the submission manifest review.
- **4. Recruiter setup:** blueprints, competencies, versioned rubrics, AI policy mode, preview and dry-run ([§7](SPEC.md#7-recruiter-setup-flow)). The first role family comes from the open decision.
- **5. The floating tabbed panel:** responsible for everything that isn't coding with the assistant ([§10](SPEC.md#10-the-unified-candidate-workspace)). It has no AI chat; that is `mindfries agent`. Its tabs:
  - **Camera:** the candidate's camera view, with periodic stills kept as evidence, as in V1, after consent;
  - **Stage & Task:** the current stage, goal, time and deliverables;
  - **System Design** canvas;
  - **Debugging & Reliability:** hypotheses, logs, reproductions;
  - **Notes & Evidence,** which includes the sources the agent researched;
  - **Conversation:** the simulated customer in the FDE flow, and the avatar's follow-ups;
  - **Submission review.**
- **6. Evidence timeline, evaluation and the capability-profile report:** a score, confidence and coverage for each competency, and "not assessed" ([§14](SPEC.md#14-evaluation-and-weighting-model), [§15](SPEC.md#15-evidence-model-and-timeline), [§18](SPEC.md#18-evaluation-engine), [§19](SPEC.md#19-candidate-report--capability-profile)). Includes human review.
- **7. Adaptive follow-ups and the avatar** (bounded weight) ([§11](SPEC.md#11-adaptive-assessment-candidates-can-be-strong-in-different-ways), [§12](SPEC.md#12-ai-avatar-important-but-not-the-main-judge)).
- **8. Integrity, integrations and retention:**
  - the three integrity categories and the candidate response process ([§16](SPEC.md#16-integrity-monitoring-and-anti-cheating-design));
  - optional GitHub, CI and deployment ([§17](SPEC.md#17-git-github-ci-and-deployment));
  - enforced retention and deletion.
- **9. Hardening:** load and failure tests, a threat model, security review, monitoring, and cost caps ([§21 Phase 6](SPEC.md#phase-6-----production-hardening)).
- **10. Pilot and cut-over:**
  - `V2_ENABLED` turned on for a pilot cohort's assessments;
  - the [release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass), ticked off item by item;
  - a rehearsed rollback (`V2_DEFAULT_ENGINE` back to `v1`);
  - a recorded go/no-go;
  - then `V2_DEFAULT_ENGINE=v2`.

  **No data migration:**
  - accounts, companies, roles and dashboards are the same ones;
  - V1 assessments already running finish on V1;
  - old reports stay readable.

---

## Decisions

The rest of the decisions in the [README](README.md#decisions-needed-before-building)
can wait until Stage 2. The CLI needs these now. The rows marked
**Decided** are settled and shape everything after Stage 1.

| Decision | Recommendation | Why |
|---|---|---|
| CLI language | **Go** | Single static binary per OS, easy cross-compiling, no runtime on the candidate's machine; the team already writes Go (`candidate/backend`) |
| Supported OS at launch | Windows, macOS and Linux; amd64 and arm64 | The spec requires a tested path on each supported OS |
| IDEs at launch | **Decided:** VS Code, JetBrains and Zed; manual open as the fallback. No agent-first IDEs (Antigravity, Cursor, Windsurf) | Candidates code with our assistant, not their own agents; these three all have a command-line launcher |
| Signing | Checksums plus a signature from day one. Windows and macOS code-signing certificates before the pilot | An unsigned binary triggers SmartScreen and Gatekeeper warnings for candidates |
| Where the CLI is downloaded from | GitHub Releases on a V2 repository or path, behind a short V2-domain installer URL | Versioned, cacheable, auditable |
| What the agent observes | **Decided:** Git state, file changes and diff snapshots inside the workspace; outside it, only the names of running processes and the IDE's installed extensions that match known coding agents (task 1.8). No keystrokes, screen or other files | Enough to see coding-agent use, without a keylogger. The spec rules out claiming total coverage ([§9](SPEC.md#what-the-cli-cannot-promise)); someone with an agent on a second device can still evade, which is why understanding checks matter |
| Data | **Decided:** a `v2` schema in the current Supabase project, with a V2 database role that can only read V1 tables | Reuses V1's accounts and data with no migration. The database itself blocks V2 from writing V1 tables |
| Accounts | **Decided:** shared with V1; V2 is entered through a signed handoff from V1 | Candidates and companies keep one login |
| Web app | **Decided:** `v2/web` holds only the new V2 screens; the dashboard, settings, profile and company portal stay V1 | Least to rebuild, least to migrate |
| Setup | **Decided:** one command from the dashboard, unique to the candidate and assessment, that installs the CLI and connects it | One step for the candidate; the code inside it is single-use and short-lived |
| `TASK.md` | **Decided:** locked. Read-only, its hash watched, any edit flagged and restored | The task can't drift, and the server's copy is the one that counts |
| How the candidate writes code | **Decided:** by typing in their IDE and with `mindfries agent`; research in their own browser is allowed; outside coding agents are not | Both kinds of edit are attributed (1.17); only outside agents are flagged (1.8) |
| Camera | **Decided:** kept as in V1, in the floating panel, after consent | Departs from the spec's §16 default; see the README |
| Where the assistant lives | **Decided:** in the terminal, as `mindfries agent`; the floating panel handles everything else | The candidate codes with an assistant where they code. Every edit it makes is attributed exactly, which makes spotting outside agents more reliable. This departs from the spec's floating AI Help tab ([§13](SPEC.md#13-floating-ai-help-assistant-and-tool-permissions)); its policy gateway still applies |
