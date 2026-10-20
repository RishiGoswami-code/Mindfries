# V2 flow audit: the flow as intended, against what exists

Written 2026-10-21 after the backend (BACKEND.md B0-B13) was built. It walks the
flow the owner described step by step and says, for each step, what is built,
where, and what is still missing. "Verified" means a test runs it (the real
`mindfries` binary against the real API on Postgres where that applies).

Legend: **Done**, **Backend done, portal missing** (the API is there and
tested; the page that calls it isn't), **Not started**, **Other team**.

## The flow

| # | Step as intended | State | Where / what is missing |
|---|---|---|---|
| 1 | A company has a role with a job description, picks engine V2 | Backend done, portal missing | `assessments.engine` + JD seam (migration 0030) and internal-admin's "assign a V2 task" exist on main. **Company portal has no V2 screen** (engine choice, JD, preview of the product and task) |
| 2 | The company sets the assessment up: reviewers' GitHub names, time, AI policy, what is monitored, research sites | Backend done, portal missing | `PUT /v2/portal/assessments/{id}/config` (verified). No portal form calls it |
| 3 | The candidate is invited and sees the assessment on their dashboard | Done (V1 + main) | Dashboard shows V2 assessments; "Set up workspace" wizard exists |
| 4 | Start assessment gives a one-time command | **Mismatch** | The wizard calls the older `POST /v2/admin/setup-codes` with the admin token and **no candidate id**. It works for the CLI, but sessions made that way can't get a panel token (the panel route checks the candidate owns the session) and can't be resumed. **Switch the wizard to `POST /v2/portal/setup-codes` with `candidate_id`, `company_id`, `assessment_id`, `invitation_id`** |
| 5 | The command installs the CLI if needed and runs `mindfries setup` | Done, verified | `/s/{code}.ps1\|.sh`, installers. Needs `V2_DIST_DIR` or `V2_CLI_DOWNLOAD_URL` set on Render |
| 6 | `setup` downloads the task and the codebase | Done, verified | Signed bundle; real run in `cli_e2e_test.go` |
| 7 | `setup` authorises GitHub and creates a private repo on the candidate's GitHub, inviting reviewers | Backend + CLI done; **the real GitHub path is not verified** | OAuth, one-time token, repo record are tested against a fake GitHub only. **Needs the OAuth app's callback set to `<V2_PUBLIC_URL>/github/callback` and the id and secret on Render**, then a manual run. The portal's "Connect GitHub on the sign-in page" is missing |
| 8 | Browser sign-in (no code to type) | Backend done, portal missing | `/sessions/device*` and `/v2/portal/device/*` verified. **The page at `<V2_PORTAL_URL>/v2/device` doesn't exist** (it must call the portal routes, show the device, pick the assessment, connect GitHub) |
| 9 | `mindfries start`: pick the IDE | Done | CLI |
| 10 | Environment checks; if they fail it can't start | Done | CLI preflight; `begin` records warnings in the tracker |
| 11 | Ask for camera, screen and audio; the clock doesn't start until they're live | Backend done, **portal missing** | `begin` waits for the panel (verified). The page that does `getUserMedia`/`getDisplayMedia`, reports status, and uploads stills and clips to `/v2/panel/*` **is not built** (the fake API's page is the model) |
| 12 | The clock starts, the IDE opens in that folder | Done, verified | |
| 13 | The floating panel stays on top (Document Picture-in-Picture), minimises to the avatar | UI on main with **mock data**; PiP not built | PANEL.md steps 2-7. `GET /v2/panel/config`, `/feed`, `PUT /v2/panel/artifacts/{kind}` are ready for it |
| 14 | The tracker shows every notification, flag, activity | Backend done, verified; panel shows mock | SSE feed with resume. Candidate's explanation of a flag ("candidate response process") is **not built** |
| 15 | Tabs per kind of candidate (design, notes, resources, review, debug, test, activity) | Backend done | Tab choice from the manifest modes + an assessment override. **No manifest lists a `system_design` or `review` mode yet, so no real candidate gets those tabs** until a product's manifest adds them (or the assessment config overrides). Panel's own `tabs.ts` doesn't know `system_design` |
| 16 | System design canvas, notes, review comments, debugging notes are saved | Backend done, verified | Panel must call the save route |
| 17 | Test results shown in the Testing tab | Backend done; **CLI doesn't send them** | `check.run` is accepted and stored; **`mindfries check` doesn't exist**, so only `submit` runs checks |
| 18 | The AI avatar sees screen, video and audio, talks with the candidate | Backend done; **never tried with a real Gemini key** | Locked Live token + transcripts. The browser's Live client (audio in/out, sending screen and camera frames) is **not built**. The token request format follows Google's docs but has only been tested against a stand-in |
| 19 | `mindfries agent`: the terminal assistant | Backend done; **never tried with a real model** | Gemini gateway tested against a stand-in and a scripted model. Needs one real-key smoke test |
| 20 | Research is allowed on listed sites | Done, verified | Server-side, https only, list per assessment |
| 21 | Submit: bundle verified, assessment marked submitted | Done, verified | |
| 22 | Grading: hidden tests run | **Other team**, needs hardening | `cmd/grade` runs by hand or `-watch`; not a sandbox; no network block; no time limit (by decision). Run it in a throwaway container |
| 23 | Evaluation: a score, confidence and coverage per competency, "not assessed" | **Other team** | The report has outcome, effort, AI summary, flags, coverage and now the panel's evidence. **No per-competency scoring, confidence, or "not assessed" yet** (release gate items) |
| 24 | The company reads the report and records the hiring decision | **Other team / portal missing** | Internal-admin has a review UI. The company portal has none. `POST` for a decision is **not built** (flag reviews only) |
| 25 | The candidate sees their report if shared | Not started for V2 | V1 has the share flow; V2's report isn't wired to it |
| 26 | Retention, export, deletion | Done, verified | **No audit log of who read sensitive evidence** (the spec asks for "permissioned and audited"): not built |
| 27 | V2 consent and notice of what's collected | Not started | Spec section 8 / release gate. Needs a portal page before the CLI starts |
| 28 | The assessment's deployment is evaluated (CI, deploy to AWS, Render...) | Partly | The work is in the candidate's own repo with reviewers invited; `github.deployment` is always false, so "asks for a deployed URL at submit" isn't wired |

## Plan stages

- **Stage 0 (guard V1):** CI guard and V2 jobs exist. The V1 inventory and V1
  regression suite (0.3, 0.4) aren't confirmed here.
- **Stage 1 / 1b (CLI):** built; the PLAN.md checkboxes (1.1-1.17) are still
  unticked and should be ticked after a review. Missing: `mindfries check`.
- **Stage 2 (API):** 2.1 and 2.2 done. **2.3 (V1 connection points)** is partial:
  the engine column exists, but the plan's "signed handoff link and its
  exchange endpoint" was replaced by the browser sign-in and panel tokens
  (a deliberate change; PLAN.md should say so). **2.4 (staging)** isn't done:
  see the deploy list below.
- **Stage 3 (candidate web app):** consent, "Set up my workspace" (partly), the
  sign-in page, live status, stage and journey view, submission review: mostly
  missing.
- **Stage 4 (recruiter setup: blueprints, competencies, versioned rubrics,
  preview, dry-run):** not started beyond the per-assessment config.
- **Stage 5 (the panel):** UI with mock data; wire-up is PANEL.md steps 2-7.
- **Stage 6 (evidence timeline, evaluation, report):** other team.
- **Stage 7 (adaptive follow-ups and the avatar):** the avatar's token and
  transcripts exist; adaptive routes (SPEC section 11) are not started.
- **Stage 8 (integrity, integrations, retention):** integrity categories and
  retention done; the candidate's response process and an evidence-access
  audit are not.
- **Stage 9 (hardening):** threat model written (`api/SECURITY.md`), caps in;
  load and failure tests, a security review and alerting are not done.
- **Stage 10 (pilot and cut-over):** the release gate in SPEC.md is entirely
  unticked.

## Before anything can run for real (deploy list)

1. Apply migrations 0017-0025 to Supabase (as the owner; they are idempotent).
2. On Render set: `V2_PORTAL_SECRET`, `V2_ENCRYPTION_KEY`, `V2_PORTAL_URL`,
   `V2_PUBLIC_URL`, `V2_ALLOWED_ORIGINS`, the GitHub OAuth app (callback
   `<V2_PUBLIC_URL>/github/callback`), `V2_STORAGE_URL`, `V2_STORAGE_KEY` and a
   **private** `v2-evidence` bucket, `V2_GEMINI_API_KEY`, and the production
   signing key. The candidate portal needs the same `V2_PORTAL_SECRET`.
3. The single-instance limit on the live feed (SECURITY.md) means one Render
   instance for now.
4. A real-key smoke test of the assistant gateway and the avatar token.

## The order I would do the rest in

1. **Portal glue** (v1-seam PRs, flag-gated): switch the Start button to the
   portal route with candidate and company ids; the device sign-in page with
   Connect GitHub; the monitoring page in the panel (capture and upload); the
   panel on live data, then PiP. That unlocks a real end-to-end run.
2. **`mindfries check`** in the CLI.
3. **Real-key smoke tests** (agent, avatar), then the avatar's Live client.
4. **Company portal:** engine choice and JD, the assessment's setup form, the
   report and the decision.
5. **Consent and notice page, the candidate's response to a flag, an
   evidence-access audit.**
6. The other team's evaluation and the grader's sandbox; then Stage 4's
   blueprints and rubrics, Stage 9, and the release gate.
