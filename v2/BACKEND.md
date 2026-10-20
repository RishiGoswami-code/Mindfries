# V2 backend: everything left to build

This is the to-do list for finishing V2's backend (`v2/api`), integrating it
with the three portals as V2, and then (only with the owner's confirmation)
merging it to main and switching it on. The product is defined in
[`SPEC.md`](SPEC.md); the CLI contract is in
[`contracts/cli-api.yaml`](contracts/cli-api.yaml); the floating panel is
designed in [`PANEL.md`](PANEL.md); how V2 joins V1 is in
[`INTEGRATION.md`](INTEGRATION.md). This file doesn't repeat them: it lists the
work, in order, and says when each piece is done.

Written 2026-10-20, from a comparison of `v2/api` against the contract, the fake
API (`v2/cli/fakeapi`) and the panel's needs.

## Status: what is built (updated 2026-10-21)

Branch `feat/v2-backend`. Everything below has tests that run in CI against a
real Postgres, and the real `mindfries` binary runs setup, start and submit
against it (`v2/api/internal/httpapi/cli_e2e_test.go`).

| Part | State |
|---|---|
| B0 foundations (config, problems, middleware, hashed tokens, Postgres in CI) | Done |
| B1 migrations 0017-0025 (renumbered after main's 0013-0016) | Done |
| B2 entry: setup codes, install scripts, browser sign-in, portal routes | Done |
| B3 GitHub: OAuth, one-time token handoff, repo record | Done |
| B4 clock and monitoring: begin gate, uploads, gaps | Done |
| B5 live tracker: rules, SSE feed with resume | Done |
| B6 panel API: config per candidate, artifacts with history, check runs | Done |
| B7 assistant gateway: Gemini, policy, tools via the CLI, server-side research, budgets | Done |
| B8 avatar: locked Gemini Live tokens, kept transcripts | Done; **needs a smoke test with a real Gemini key** |
| B9 submission verification | Not mine: the out-of-process grader (`cmd/grade`) is the team's |
| B10 evaluation and report | Not mine: built by a teammate. This API serves the evidence they read (`/v2/portal/sessions/{id}/evidence`) |
| B11 company and admin routes | Done (config, sessions, evidence, end, extend, flag reviews, registry) |
| B12 retention, privacy, security | Done; see `api/SECURITY.md` |
| B13 operations | Done: README, CI, Dockerfile fix, housekeeping. Cost caps are in; an alert on them is a log line, not a pager |

### Left, and where it lives

- **CLI:** a `mindfries check` command that runs the task's checks and sends the
  `check.run` event the Testing tab reads (the server and the contract already
  accept it). Today the checks run at `submit`.
- **Candidate portal (Part I):** the sign-in page that calls
  `/v2/portal/device/*`, the Connect GitHub step, the floating panel on live
  data (`/v2/panel/*`), PiP, and the Start assessment button calling
  `/v2/portal/setup-codes`. The panel's `tabs.ts` should also map the
  `system_design` mode (the real name) to the design tab.
- **Company and admin portals (Part I):** call the staff routes. Setting an
  assessment's reviewers is `PUT /v2/portal/assessments/{id}/config`.
- **The fake API** still serves the older shapes for the CLI's own tests; keep
  it in step when the contract changes (the real e2e now catches drift).
- **Secrets to set on Render:** `V2_PORTAL_SECRET`, `V2_ENCRYPTION_KEY`,
  `V2_PORTAL_URL`, the GitHub OAuth app (callback
  `<V2_PUBLIC_URL>/github/callback`), `V2_STORAGE_URL`/`KEY` and a private
  `v2-evidence` bucket, `V2_GEMINI_API_KEY`, `V2_ALLOWED_ORIGINS`.

## Where things stand

| Piece | State |
|---|---|
| CLI (`v2/cli`) | Done against the fake API: browser sign-in, GitHub in one click, setup → start → begin, monitoring, watcher, `mindfries agent`, submit. |
| Fake API (`v2/cli/fakeapi`) | Implements the **whole** contract in memory. It is the reference for behaviour. |
| Real API (`v2/api`) | Engine done (select product, resolve task with a JD-keyed cache, deliver the workspace, verify). Server has the **older** contract only. |
| Database (`v2/db`) | `products`, `product_tasks`, `setup_codes`, `sessions`, `events`, `snapshots`, `submissions` are live. Sessions can't be `ready`. |
| Floating panel (`candidate/frontend /v2/session/[id]`) | UI only, mock data (PR #173). |
| V1 seam | `assessments.engine` + JD migration (0030) only. Nothing reads it yet. |

### The gap, endpoint by endpoint (the CLI's contract)

| Contract endpoint | `v2/api` today | Needed for |
|---|---|---|
| `GET /version` | ✅ | |
| `POST /sessions/exchange` | ✅ (setup codes; no `ready` state) | `mindfries setup` |
| `POST /sessions/device`, `POST /sessions/device/token` | ❌ | browser sign-in |
| `POST /session/begin` | ❌ | starting the clock |
| `GET /session/monitoring` | ❌ | `start` waits for camera/screen/mic |
| `GET /session` | ✅ (no `ready`, no GitHub, no monitoring fields) | `status`, resume |
| `POST /session/github/token`, `POST /session/github/repo` | ❌ | the candidate's own private repo |
| `POST /session/revoke` | ✅ | |
| `GET /session/bundle`, `GET /session/bundle/archive` | ✅ | |
| `POST /session/heartbeat` | ✅ (no monitoring gaps in the reply) | |
| `POST /session/events`, `POST /session/snapshots` | ✅ | |
| `POST /session/assistant/turns` (+ `/tool-results`) | ❌ | `mindfries agent` |
| `POST/GET /session/submission`, `PUT …/bundle` | ✅ (verification runs on the server, unsandboxed) | submit |

The fake also serves things outside the contract that the real system needs:
the dashboard's Start button and one-time setup command (`/dashboard/*`,
`/s/{code}.ps1|.sh`), the installers and CLI downloads (`/install.*`,
`/cli/latest/*`), the device sign-in page (`/device`), GitHub OAuth
(`/github/start`, `/github/callback`), the monitoring panel and its uploads
(`/monitor/{key}/*`), and admin routes (invitations, reviewers). Each of those
has a home below: the API part in `v2/api`, the page in the candidate portal.

## Decisions the owner needs to make

Marked **[D1]…[D8]** where they block a task. My recommendation is first.

- **[D1] Where `v2/api` runs while we build.** (a) Render, replacing the
  fake API service at `mindfries-cli.onrender.com` (recommended: it is already
  there and the CLI already points at it); (b) inside `candidate/backend` now,
  as INTEGRATION.md plans for cut-over. Folding into `candidate/backend` stays
  the cut-over step either way.
- **[D2] Where candidate code is run for verification.** Running a
  candidate's submission on the API server is unsafe. (a) Daytona sandboxes,
  already used by V1 (recommended); (b) a container job (Render/Fly machine)
  with no network and no secrets; (c) GitHub Actions in the candidate's repo
  only (the result is then the candidate's CI, not ours).
- **[D3] Where stills, screenshots, audio and bundles are stored.** Supabase
  Storage, a private `v2-evidence` bucket, written only by `v2_api`
  (recommended).
- **[D4] The model.** Gemini, called directly (per the 2026-10-14 decision),
  for the assistant, the evaluators and the avatar (Gemini Live). A
  `V2_GEMINI_API_KEY` on the server.
- **[D5] How the panel proves who it is.** (a) The candidate portal page
  checks the signed-in candidate owns the session (server to server, with a
  shared `V2_PORTAL_SECRET`) and gets a short-lived panel token for the
  browser (recommended); (b) a per-session panel key in the URL, as the fake
  does (simpler, but anyone with the link is the panel).
- **[D6] Retention.** Evidence kept 90 days, as V1 (recommended), then deleted
  by a job; reports kept until the company deletes them.
- **[D7] Does the avatar share one conversation with `mindfries agent`?**
  Yes, one assistant record per session (recommended), so the reviewer sees one
  history. The spec keeps the avatar's weight bounded either way.
- **[D8] The real GitHub OAuth app.** Its client ID and secret on the
  server, and the callback set to the deployed API's `/github/callback`.

## Part B: the backend (`v2/api`)

Each task ends with **Done when**. A task isn't done until its tests pass
in CI.

### B0. Foundations

1. **Config and secrets:** one `config` package reads every `V2_*` variable,
   fails fast on missing required ones, and is documented in `api/README.md`.
2. **Errors in the contract's shape:** one helper writes the contract's
   problem JSON (`code`, `title`, `status`, `session_status`) everywhere.
3. **Session auth middleware:** a bearer token goes to a session, its status
   is checked per route (`ready` vs `active` vs ended), and `last_acked_seq` is
   kept.
4. **Hash session tokens at rest.** Tokens are stored in plain text today
   (`v2.sessions.token`). Store a SHA-256 and compare in constant time.
5. **Postgres in CI:** the API's store tests run against a real Postgres
   (GitHub Actions service container) with the `v2/db` migrations applied.
6. **Structured logs, request IDs, panic recovery,** and per-token and per-IP
   rate limits on the sign-in and upload routes.

**Done when:** every handler uses the middleware and the error helper; the
store tests run in CI against Postgres; no token is stored in plain text.

### B1. Database migrations (`v2/db`, additive, all in the `v2` schema)

1. `0012_session_ready`: allow status `ready`; `started_at`/`ends_at` become
   nullable until begin; add `duration_seconds`, `begun_at`, `token_hash`.
2. `0013_device_codes`: device code, user code, the candidate/invitation it is
   for, expiry, approved/denied, polled interval.
3. `0014_github`: one GitHub connection per candidate (encrypted token,
   login, scopes), and the repo per session (owner/name, invited reviewers).
4. `0015_monitoring`: the panel's state per session (camera/screen/mic, last
   seen), the evidence objects (still/screenshot/audio clip, storage path,
   time), and recorded gaps.
5. `0016_assessment_config`: per assessment: duration, AI policy mode,
   reviewers (GitHub logins), required monitoring, panel tabs override.
6. `0017_activity`: the tracker's items (`at`, severity, source, category,
   text, refs to the evidence) and flags with their review state.
7. `0018_panel_artifacts`: diagram, notes, review comments, debug
   hypotheses: one row per artifact per session, versioned.
8. `0019_assistant`: turns, steps, tool calls and results, token use and
   budget, for both `mindfries agent` and the avatar [D7].
9. `0020_evaluation`: evaluator runs, competency scores (score, confidence,
   coverage, "not assessed"), the report, and the human review decision.

**Done when:** applying every migration twice is a no-op; a test shows
`v2_api` still can't write any V1 table.

### B2. Candidate entry: invitation → session → setup command

1. **Create a V2 session from a V1 invitation:** when an invitation's
   assessment has `engine = 'v2'`, resolve the product and task from the
   role and JD (the engine already does this), and create a `ready` session
   for that candidate.
2. **Start assessment → one-time setup command:**
   `POST /v2/portal/sessions/{id}/setup` (called by the candidate portal
   server) mints a short-lived setup code and returns the one-line command
   for Windows and macOS/Linux.
3. **Setup scripts:** `GET /s/{code}.ps1` and `/s/{code}.sh`: install the
   CLI if missing, then run `mindfries setup <code>`.
4. **CLI distribution:** `GET /install.ps1|.sh` and `/cli/latest/*` serve the
   signed release (or redirect to GitHub Releases), with the production
   signing key.
5. **Browser sign-in (RFC 8628):** `POST /sessions/device` and
   `/sessions/device/token`, plus `POST /v2/portal/device/approve`, which the
   portal's sign-in page calls for the signed-in candidate.

**Done when:** the CLI goes from a portal-minted code, or from the browser
sign-in, to a `ready` session on the real API.

### B3. GitHub: the candidate's own private repo

1. `GET /github/start` and `/github/callback`: web OAuth through the server,
   with the token stored encrypted [D8]; this happens on the sign-in page, in
   one click.
2. `POST /session/github/token`: hand the token to the CLI once, as the fake
   does.
3. `POST /session/github/repo`: create the private repo, invite the
   assessment's reviewers (from B1.5, not an env var), record it.
4. At the session's end, reviewers stay invited; the token is revoked or
   expired.

**Done when:** the CLI's GitHub e2e tests pass against the real API with a
test GitHub (the fake GitHub server in `fakeapi/fakegithub` is reused as the
test double).

### B4. The clock and monitoring

1. `POST /session/begin`: refuses with `monitoring_not_ready` until what the
   assessment requires is live; then `ready → active`, sets the start and end
   times.
2. `GET /session/monitoring`: what the panel has on now.
3. **Panel uploads:** `POST /v2/panel/{session}/status`, `/still`,
   `/screenshot` and `/audio`, with the panel token [D5]; the media goes to
   storage [D3]; size and rate limits.
4. **Gaps:** a share that stops is recorded as a gap and reported in the
   heartbeat reply, never as a verdict.
5. **Expiry:** a job ends `active` sessions past `ends_at` as `expired`.

**Done when:** `mindfries start` can't begin without the panel live, then
begins; a stopped share shows as a gap in the heartbeat reply and the activity
feed.

### B5. The live tracker (the panel's Activity tab)

1. **Activity from evidence:** every event the CLI sends
   (`workspace.ready`, `ide.launched`, `git.commit`, `file.burst`,
   `agent_signal.*`, `watcher.gap`, `task_file.modified`, …) and every panel
   and server event becomes an activity item with a severity
   (info/notice/flag) and a category (observed/verified/signal, SPEC §16).
2. **Flag rules,** each one a small tested function: a coding agent running,
   TASK.md modified, a monitoring gap, a file burst after a long idle, a
   watcher gap, a pushed commit not matching the watched work.
3. **`GET /v2/panel/{session}/feed`:** Server-Sent Events for the panel: the
   backlog, then live items; it resumes from `Last-Event-ID`.
4. **Notifications to the candidate:** a flag tells the candidate what was
   recorded and why, at the time (as V1 does).

**Done when:** a CLI event shows in the panel's Activity tab within two
seconds, and each rule has a test for when it does and doesn't fire.

### B6. The panel API

1. `GET /v2/panel/{session}`: the panel's config: the timer, the tabs from
   the manifest's modes (the same `tabsFor` logic, ported to Go), resources from
   the task, avatar settings.
2. **Artifacts:** `PUT /v2/panel/{session}/artifacts/{kind}` for the diagram,
   notes, review comments and debug hypotheses; autosaved, versioned, kept as
   evidence.
3. **Code Review data:** the diff the review tab shows comes from the task
   (review tasks ship one) or from the candidate's own commits.
4. **Testing and Debugging data:** a new contract event, `check.run` (the
   command, pass/fail, the test names, a short output tail), sent by the CLI
   when the candidate runs the task's checks (`mindfries check`, or a check
   run the watcher sees). Debug tasks also ship their logs.

**Done when:** the panel runs on live data with no mock, and what the
candidate draws, writes and comments survives a reload and appears in the
evidence.

### B7. The assistant gateway (`mindfries agent`)

1. `POST /session/assistant/turns` with streaming, and `/tool-results`:
   the model runs on the server; the CLI never holds a key.
2. **Gemini** [D4], with the task, the policy mode (`off`, `explain_only`,
   `code_allowed`) and the tool limits as the system instructions.
3. **Budgets:** per-turn and per-session token caps; `budget_exhausted` and
   `model_unavailable` are honest errors.
4. **Every turn is evidence:** prompts, tool calls, proposed edits and what
   the candidate accepted, attributed (`change.attributed`).

**Done when:** the CLI's agent e2e tests pass against the real API with a
stub model, and a live smoke test against Gemini works.

### B8. The AI avatar (Gemini Live)

1. `POST /v2/panel/{session}/avatar/token`: mints a short-lived Gemini Live
   token, so the browser talks to Gemini directly and the key stays on the
   server.
2. **Its instructions:** the task, the stage, the elapsed time, recent
   activity, and the avatar's rules (SPEC §12: it asks, it doesn't decide; what
   it must not do).
3. **Screen, camera and audio** go to the live session from the panel.
4. **Transcripts** are posted back and stored as evidence, in the same
   record as the agent [D7]; the avatar's weight in the evaluation is
   bounded.

**Done when:** a candidate can talk to the avatar in the panel, and the
transcript is in the session's evidence.

### B9. Submission and verification

1. Keep the submission endpoints as they are; move verification to a
   **queued job**.
2. **Run the check in isolation** [D2]: the product at its golden commit, plus
   the candidate's bundle, plus the task's hidden acceptance tests; no network,
   no secrets, a time limit.
3. **Store the result** (pass/fail per test, output) as verified evidence; the
   CLI's `submit` reports it.

**Done when:** a passing and a failing submission each get the right verdict
in the sandbox, and nothing the candidate wrote runs on the API server.

### B10. Evaluation and the capability report: NOT part of this work

The evaluators and the report are being built by another team member. This backend
only has to leave the evidence they read in place: events, snapshots,
activity and flags, artifacts, check runs, assistant transcripts, monitoring
media and the submission. Nothing below is built here.

<details><summary>The original outline, for reference</summary>


1. **The evidence timeline:** one ordered view of everything a session
   produced (SPEC §15).
2. **Evaluators:** per competency, from the role's rubric: verified results
   first, then the observed work, then the conversation (bounded weight)
   (SPEC §14, §18). Each score has confidence, coverage and the evidence it
   cites; "not assessed" when there isn't enough.
3. **Integrity section:** flags as prompts to look, with their evidence,
   never a verdict (SPEC §16).
4. **The report** (SPEC §19), plus **human review:** the hiring decision is a
   person's, recorded with who and when.

**Done when:** a finished session produces a report whose every claim links
to evidence, and a reviewer can record a decision.

</details>

### B11. Company and admin APIs

1. **Assessment config:** reviewers, duration, AI policy, required
   monitoring, per assessment (used by B3, B4, B7).
2. **Sessions:** a list and detail for the admin's session monitor and the
   company's candidate view; extend or end a session.
3. **Reports:** fetch, share with the candidate (as V1), record the
   decision.
4. **Registry:** products and the task bank, with verified status, for the
   admin portal.

**Done when:** each route has a test, and is authenticated as a company user
or a Mindfries admin by the calling portal's server (not the browser).

### B12. Retention, privacy and security

1. **Retention job** [D6]: evidence media and transcripts deleted after the
   retention period; a deletion log.
2. **Candidate rights:** export and delete their V2 data (as V1 settings).
3. **Encryption:** GitHub tokens encrypted with a `V2_ENCRYPTION_KEY`.
4. **Threat model** written down; signed upload URLs; every upload size- and
   type-checked.

**Done when:** a test session's media is gone after retention; a security
review of the API's routes has no open high findings.

### B13. Operations

1. **Deploy** [D1], with its env documented; health and readiness checks.
2. **Keep the fake API in step:** every contract change lands in the
   contract, the fake and the real API together; the CLI e2e suite runs
   against both in CI.
3. **Fix `Dockerfile.fakeapi`:** it breaks on the `../contracts` replace.
4. **Cost caps** on model use per session and per day, with an alert.
5. **Update `PLAN.md`'s checkboxes**, which are stale.

**Done when:** the CLI's whole e2e suite passes against the deployed real API.

## Part I: integration as V2 (after Part B)

All of it flag-gated (`V2_ENABLED`), in the existing apps, touching V1 only at
switch-guarded points, each its own `v1-seam` PR for the owner's approval.

1. **Candidate portal:**
   - the dashboard's Start assessment button for `engine = 'v2'`, showing the
     one-time command;
   - the device sign-in page, with GitHub in one click;
   - the floating panel on live data (B4–B8), with the PiP pop-out
     (PANEL.md step 2);
   - the V2 report.
2. **Company portal:**
   - the role's JD and the engine choice;
   - the assessment's reviewers and AI policy;
   - the V2 candidate report and the decision.
3. **Internal admin:**
   - the product registry and task bank;
   - V2 sessions in the session monitor;
   - V2 costs.
4. **The CLI** points at the deployed API by default (baked at build time,
   as now).
5. **One full rehearsal:** a company creates a V2 role, invites a candidate,
   and the candidate:
   - sets up
   - starts
   - works with the panel, the avatar and the agent
   - pushes to their repo
   - submits

   The company then reads the report and decides. Run it once on Windows and
   once on macOS.

## Part M: merge and switch on (only with the owner's confirmation)

1. Everything is on reviewed PRs; **the owner merges**; nothing is merged by
   anyone else.
2. The SPEC's [release gate](SPEC.md#v2-release-gate-do-not-cut-over-before-all-checks-pass),
   ticked item by item.
3. `V2_ENABLED` on for a pilot cohort; `V2_DEFAULT_ENGINE` stays `v1`.
4. A rehearsed rollback; then, on the owner's go, `V2_DEFAULT_ENGINE = v2`.

## Order of work

B0 → B1 → B2 → B3 → B4 → B5 → B6 → B9 → B7 → B8 → B10 → B11 → B12 → B13, then
Part I, then Part M. B2 to B4 follow the candidate's order (setup signs in and
connects GitHub; start checks, monitors and begins) and make the CLI run end to
end on the real API: the first milestone. B5 and B6 make the panel live: the second. B7 to B10 add the
AI and the report: the third.
