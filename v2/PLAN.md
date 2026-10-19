# V2 build plan

How V2 gets built beside V1 without disturbing it, then tested and deployed.
The tasks run in order. **The CLI comes first.**

It follows [`SPEC.md`](SPEC.md). The governing rule is [§2](SPEC.md#the-governing-rule):

> Build, test, and run V2 independently until the V2 acceptance criteria
> pass. Only then perform a planned, reversible cutover.

[`README.md`](README.md) lists the open decisions. [`ROLE_FLOWS.md`](ROLE_FLOWS.md)
shows what the finished product does for an FDE and a DevOps role.

---

## Part 1 — How V1 stays safe

V1 is protected by boundaries a mistake can't cross unnoticed, not by care
alone.

### 1. One folder, and a check that enforces it

Every line of V2 lives under `v2/`. V2 work is merged into `main` in small
PRs, not kept on a long-lived branch that drifts from `main`.

A CI job, the **V1 guard**, fails any V2 PR that changes a file outside
`v2/`. That covers `candidate/`, `company/`, `internal-admin/` and
`supabase/migrations/`. The root orientation file and `.github/` are the only
exceptions. A change that really must touch V1 goes in its own PR, which
the owner approves on purpose.

### 2. Its own modules

V2 has its own modules:

| Module | Language | Contents |
|---|---|---|
| `v2/cli` | Go | The CLI |
| `v2/api` | Go | The API |
| `v2/web` | Next.js | The web app |
| `v2/db` | SQL | Migrations and their own runner |

None of them imports from a V1 package. If V2 needs something V1 has, it
copies a thin piece and owns it, as the spec prefers in [§2](SPEC.md#what-v2-adds-or-changes).
A V1 refactor can then never break V2, and the reverse holds too.

### 3. Its own data

V2 gets its own Supabase project, or at least a `v2` schema, with its own
migrations and its own `schema_migrations` table. V2 code never reads or
writes V1 tables. Tests use synthetic data only.

### 4. Its own deployments and secrets

V2 has its own Vercel project for `v2/web`, its own Railway service for
`v2/api`, and its own domain. Its environment variables are prefixed with
`V2_`, and its keys are V2-scoped: Gemini, Resend, GitHub app.

Nothing in V1's Vercel or Railway settings changes. Merging V2 code into
`main` only adds files that V1's builds never read.

### 5. Proof that V1 still works

Before any V2 deployment, V1 gets a CI run on every PR:

- typecheck, lint and build for the three apps;
- `go test` for the backends;
- a small contract suite for V1's critical journeys.

That suite is the **V1 regression suite** that the [release gate](SPEC.md#v1-non-regression-and-coexistence) requires.

### 6. A gated, reversible cut-over

V2 goes live only on its own domain, and only for a pilot cohort enrolled
on purpose. Existing V1 links and sessions are never redirected.

The cut-over is a single routing switch with a rehearsed rollback. V1 code
and data stay intact through the rollback window.

---

## Part 2 — The path from code to production

```
local (fake API)  →  CI (V1 guard + V1 suite + V2 tests)  →  merge to main (v2/ only)
     →  V2 staging (own project, own DB, synthetic data)  →  coexistence test with V1
     →  pilot cohort on the V2 domain  →  release gate sign-off  →  reversible cut-over
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
  - *Done when:* a test PR that touches `candidate/` from a V2 branch fails the guard, and a clean one passes.
- [ ] **0.3 V1 inventory.**
  - Write `v2/V1_INVENTORY.md`, listing V1's:
    - routes and entry points;
    - tables and migrations;
    - auth and session cookies;
    - jobs and crons;
    - environment variables;
    - deployments.
  - Mark the files that V2 must never change.
  - *Done when:* reviewed by the owner.

*V1 contract tests (0.4) are not needed to write CLI code, which runs only
on a laptop. They are needed before V2 deploys anything, so they come before
Stage 2.*

### Stage 1 — The CLI ([§9](SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension), [§17](SPEC.md#git-workflow))

The CLI is built against a **fake V2 API**, a small local server that
follows the contract. It can therefore be finished and tested before the real
backend exists. The candidate's flow is the spec's 14 steps.

The commands are:

| Command | What it does |
|---|---|
| `mindfries start <setup-code>` | Sets up the workspace and runs the agent |
| `mindfries status` | Shows the session, sync and Git state |
| `mindfries submit` | Reviews the diff and hands in |
| `mindfries stop` | Ends the agent and cleans up |
| `mindfries doctor` | Runs preflight checks only |

- [ ] **1.1 The CLI ↔ API contract.**
  - Write `v2/contracts/cli-api.yaml` (OpenAPI). It covers:
    - exchanging the setup code;
    - the session token and its expiry;
    - the task bundle and its manifest;
    - heartbeat;
    - event batches;
    - the submission manifest and upload;
    - revoke;
    - the minimum CLI version.
  - *Done when:* the contract is reviewed. Every later task is built against it.
- [ ] **1.2 Skeleton and fake API.**
  - Create the Go module `v2/cli` with the five commands as stubs, plus `v2/cli/fakeapi`, a scriptable server that serves the contract.
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
  - The browser shows a **single-use setup code** that expires in about 10 minutes. It is not a reusable secret, so shell history leaks nothing.
  - `start` exchanges the code for a session-scoped token. The token is stored in the OS keychain, or failing that in a file only the user can read. It is never printed.
  - The token expires with the session.
  - *Done when:* reusing a code, an expired code and an expired token are all refused (tested against the fake API).
- [ ] **1.5 Workspace setup.**
  - The candidate confirms the target folder.
  - The CLI downloads the task bundle and checks every file's hash against the manifest. It records the task version.
  - It runs `git init`, or clones, then:
    - makes an assessment branch;
    - sets a local Git identity, scoped to the repo;
    - writes `TASK.md`;
    - makes a baseline commit.
  - It refuses a folder that isn't empty unless the candidate confirms.
  - *Done when:* a tampered bundle is rejected, and a re-run resumes rather than duplicating.
- [ ] **1.6 IDE launch.**
  - Supported IDEs: VS Code, Cursor, JetBrains IDEs (via their launchers) and Zed.
  - The CLI detects which are installed and opens the one the candidate picks. If none is found, it prints the path to open by hand.
  - *Done when:* launch works or fails gracefully on each OS.
- [ ] **1.7 The local agent.**
  - It runs in the foreground: never a service, and never at login.
  - It sends a heartbeat.
  - It reads Git status and diff stats on an interval.
  - It sees file changes **inside the workspace only**, as the policy allows.
  - Its offline queue is a file in which every event is chained to the one before it, so tampering shows.
  - Events go in batches with retry and backoff. Batches are capped in size, and a batch the server refuses as malformed is dropped. These are the lessons from V1's telemetry backlog bug.
  - It shows "last synced …".
  - *Done when:* if the network is cut for 10 minutes, nothing is lost and nothing is duplicated after reconnecting.
- [ ] **1.8 Submit.**
  - The candidate sees the diff and writes a commit message.
  - The CLI commits and builds the submission manifest:
    - task version;
    - base and head commits;
    - changed files, with hashes;
    - test and build results;
    - timestamp.
  - It uploads a Git bundle, and the server checks it against the manifest.
  - *Done when:* a bundle that doesn't match is refused, and a successful submit is shown in `status`.
- [ ] **1.9 Stop and clean up.**
  - `stop`, a submit or the session expiring each:
    - end the agent;
    - revoke the token on the server;
    - delete local credentials and session state;
    - **leave the candidate's code in place**.
  - *Done when:* no token or credential is left on disk after any of the three.
- [ ] **1.10 Failure paths.**
  - Each row of the spec's failure table has a clear message, a recovery path and a test:
    - Git missing;
    - IDE missing;
    - dependency install fails;
    - clone fails;
    - network loss;
    - IDE crash;
    - agent crash;
    - unsupported OS.
  - *Done when:* every row is covered.
- [ ] **1.11 Signed, versioned distribution.**
  - Release builds for each OS, with checksums and a signature.
  - A one-line installer that verifies the checksum before running the CLI.
  - The server refuses CLI versions older than its minimum.
  - Windows (Authenticode) and macOS (notarization) need certificates; see the decisions below.
  - *Done when:* a tampered binary is refused by the installer.
- [ ] **1.12 CLI end-to-end in CI.**
  - Run the full `start → work → submit → stop` flow against the fake API on Windows, macOS and Linux runners.
  - *Done when:* the flow is green on all three.

### Stage 2 — The V2 API the CLI talks to ([§6 core services](SPEC.md#core-services))

- [ ] **0.4 V1 contract tests** (deferred from Stage 0).
  - Cover sign-in, dashboard, IDE open, assistant reply, save, submit and report.
  - Run them in CI against a V1 preview or staging environment.
  - *Done when:* they're green on `main`.
- [ ] **2.1 V2 database.**
  - Set up the database chosen in the README.
  - Write migrations in `v2/db` for sessions, setup codes, tasks and versions, evidence events, and submissions.
  - *Done when:* applying the migrations to a fresh database twice is a no-op the second time.
- [ ] **2.2 `v2/api`: the contract from 1.1, for real.**
  - Implement sessions, setup codes, token issue and revoke, task bundles, events and submissions, with manifest verification.
  - *Done when:* the CLI's end-to-end tests from 1.12 pass against the real API as well as the fake.
- [ ] **2.3 V2 staging.**
  - Its own Railway service and database, with `V2_` secrets, on a V2-only domain.
  - *Done when:* it deploys, and the V1 suite and the coexistence check still pass with V2 running.

### Stage 3 onward — the rest of V2

These follow the spec's phases ([§21](SPEC.md#21-mvp-to-v2-implementation-plan)).
Each will be split into tasks like Stage 1 when it starts.

- **3. Candidate web app (`v2/web`):**
  - onboarding and consent;
  - "Set up my workspace", which shows the setup code;
  - live session status from the agent;
  - the stage and journey view from [`ROLE_FLOWS.md`](ROLE_FLOWS.md);
  - the submission manifest review.
- **4. Recruiter setup:** blueprints, competencies, versioned rubrics, AI policy mode, preview and dry-run ([§7](SPEC.md#7-recruiter-setup-flow)). The first role family comes from the open decision.
- **5. The floating tabbed panel:** Notes & Evidence, Debugging, System Design, Research, and AI Help behind the policy gateway ([§10](SPEC.md#10-the-unified-candidate-workspace), [§13](SPEC.md#13-floating-ai-help-assistant-and-tool-permissions)).
- **6. Evidence timeline, evaluation and the capability-profile report:** a score, confidence and coverage for each competency, and "not assessed" ([§14](SPEC.md#14-evaluation-and-weighting-model), [§15](SPEC.md#15-evidence-model-and-timeline), [§18](SPEC.md#18-evaluation-engine), [§19](SPEC.md#19-candidate-report--capability-profile)). Includes human review.
- **7. Adaptive follow-ups and the avatar** (bounded weight) ([§11](SPEC.md#11-adaptive-assessment-candidates-can-be-strong-in-different-ways), [§12](SPEC.md#12-ai-avatar-important-but-not-the-main-judge)).
- **8. Integrity, integrations and retention:**
  - the three integrity categories and the candidate response process ([§16](SPEC.md#16-integrity-monitoring-and-anti-cheating-design));
  - optional GitHub, CI and deployment ([§17](SPEC.md#17-git-github-ci-and-deployment));
  - enforced retention and deletion.
- **9. Hardening:** load and failure tests, a threat model, security review, monitoring, and cost caps ([§21 Phase 6](SPEC.md#phase-6-----production-hardening)).
- **10. Pilot and cut-over:**
  - a pilot cohort on the V2 domain;
  - the [release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass), ticked off item by item;
  - a rehearsed rollback;
  - a recorded go/no-go;
  - then the reversible switch.

---

## Decisions the CLI needs first

The rest of the decisions in the [README](README.md#decisions-needed-before-building)
can wait until Stage 2. The CLI needs these now.

| Decision | Recommendation | Why |
|---|---|---|
| CLI language | **Go** | Single static binary per OS, easy cross-compiling, no runtime on the candidate's machine; the team already writes Go (`candidate/backend`) |
| Supported OS at launch | Windows, macOS and Linux; amd64 and arm64 | The spec requires a tested path on each supported OS |
| IDEs at launch | VS Code, Cursor, JetBrains and Zed; manual open as the fallback | These cover most candidates and all have a command-line launcher |
| Signing | Checksums plus a signature from day one. Windows and macOS code-signing certificates before the pilot | An unsigned binary triggers SmartScreen and Gatekeeper warnings for candidates |
| Where the CLI is downloaded from | GitHub Releases on a V2 repository or path, behind a short V2-domain installer URL | Versioned, cacheable, auditable |
| What the agent observes | Git state and file changes inside the workspace only, nothing outside it | The spec's minimum, and honest about coverage ([§9](SPEC.md#what-the-cli-cannot-promise)) |
