# Build status

A quick, plain-language look at what's actually built, checked against the
product plan ([`System_Archetect_And_PRD.md`](System_Archetect_And_PRD.md)
§2.1). For the full technical detail behind any line here — exact files,
what was tested and how, the full security review — see
[`AUDIT.md`](AUDIT.md). Detail on the in-browser code editor itself lives in
its own [task.md](candidate/frontend/src/app/ide/task.md).

**Last updated: 2026-10-18.**

**Legend:** ✅ Done and real · 🟡 Partly done · ❌ Not built yet

---

## Company side

The Company Portal (`company/frontend`) exists and covers the whole company
flow. What's missing is mostly proof against real services: no real email
has been sent from it, and the score that lands in the pipeline after
evaluation has never been written back by a real model run.

| # | What a company should be able to do | Status |
|---|---|---|
| 1 | Sign up | ✅ Two ways. A company can sign itself up at `/signup` (migration 0027): it starts with access pending and 0 credits — it can sign in and look around, but creating roles and inviting stay locked until a Mindfries admin grants access and credits in internal-admin's `/admin/access`; 1 credit is spent per candidate invited. Or our team onboards it from internal-admin, which emails a `/set-password` link; those companies aren't metered. A company admin invites teammates from Settings → Team |
| 2 | Create a role | ✅ Title, tech stack, duration, visibility, and a brief. A role can be closed and reopened, and published to the job board (applicants can be answered, invited to the assessment, and given an interview time) |
| 3 | Pick or configure an assessment | 🟡 Configure: yes — the role's brief and kind of task drive a task generated per candidate, and the AI assistant (on/off, message cap) and AI interviewer (questions, tone, language, time per answer) are set per role. Pick a template from the Game Library: not in the portal — the data function exists (`setJobRoleTemplate`) but no screen calls it |
| 4 | Invite a candidate | ✅ Both a company (from a role in its portal) and our own team (from the admin portal, on a company's behalf) can invite. Each invitation has its own link, and each candidate gets their own task, generated from the role's brief (since 2026-10-14). Emailed only once `RESEND_API_KEY`, `RESEND_FROM` and `CANDIDATE_PORTAL_URL` are set |
| 5 | See how an assessment is going | ✅ An overview (stat cards, invite trend, activity heatmap, due-dates calendar, recent activity) and a pipeline per role, with stage badges, one-at-a-time and bulk stage changes, accepted/declined answers, and whether each candidate's task is ready. Stages move by themselves when a candidate starts and submits. The portal polls rather than listening live. No score or date filters |
| 6 | Review a candidate's report | ✅ A summary card (picture, headline, recommendation, scores, linked accounts), the profile and resume, and the evidence report with cited moments, interview answers and recordings, camera stills and flagged moments. A reviewer can add notes and record their own decision. 2–4 candidates can be compared side by side. What a candidate chose to hide is respected |

Billing: Settings → Billing is backed by `company/backend`'s real Stripe
checkout, billing portal and webhook — but that service isn't deployed and
there is no real Stripe account or keys yet.

## Candidate side

Mostly real and working.

| # | What a candidate should be able to do | Status |
|---|---|---|
| 1 | Accept an invitation | 🟡 Works once an invitation exists — but only our team can create one today. Candidates can also just sign up themselves, with email/password or with Google, GitHub, GitLab, or LinkedIn |
| 2 | Complete basic setup (consent, camera check, instructions) | ✅ |
| 3 | Enter the coding workspace | 🟡 Works, but the workspace page itself has no login check — reachable by anyone with the link |
| 4 | Read the task | ✅ Real, as long as someone has written a real task description for that assessment. If not, it says so honestly instead of showing a fake one |
| 5 | Work in a real codebase | ✅ Real, same condition as above — starts empty if nobody's added starter files yet |
| 6 | Use a terminal, run tests | 🟡 The terminal is fully real. There's no automatic test runner yet |
| 7 | Talk to an AI coding assistant | 🟡 Built and answering — it sees the task and the open file, and won't write the solution. Runs on Gemini; needs a Gemini key with billing on for real use (a free-tier key allows 20 requests a day per model) |
| 8 | Do a follow-up AI interview | 🟡 Built — four questions about the candidate's own changes before submit, typed or by turn-based voice. Not a live call. Same hosting and credit condition |
| 9 | Submit | ✅ Real, and can only be done successfully once per session — a candidate can't replay or reset their own submission |

### Candidate side — everything still to build

Checked against the code on 2026-10-07. Decisions already made: every AI
job — task generation, assistant, interviewer, evaluation, report — uses
**Gemini through Google's API directly** (2026-10-14, replacing OpenRouter);
assessments are **by invitation only**, each candidate getting their own
generated task (2026-10-14); and social accounts are connected through
**Composio**.

**AI**

| # | What | State today |
|---|---|---|
| 1 | AI chat assistant in the workspace | Built. Refuses to write the fix, to say where the defect is, or to confirm a guess; still answers genuine concept questions (that behaviour was run live on a real model). Replies are streamed. It is shown the whole project and the terminal's recent output, not only the open file. A company can switch it off or cap its messages per role (migration 0015, applied). Closed once the interview starts. The streaming, wider context and role settings were run end to end on 2026-10-07 against a stand-in model, not a real one, and the panel has not been opened in a browser |
| 2 | AI interviewer (follow-up questions about the candidate's work) | Built and run live through the backend. A company sets the number of questions, tone, language and time per answer on the role (migration 0014 is applied; run live on 2026-10-07 with a two-question Spanish interview). Each answer is timed by the server; at zero it is sent as it stands or recorded as unanswered. When the session clock runs out the interview opens by itself and the work is submitted after it. The dialog has not been clicked through on screen (needs a camera and microphone) |
| 3 | Voice for the interviewer | Built as a hands-free spoken conversation: the question is read aloud, the microphone opens, and a pause after speaking sends the answer. Two forms. With `GEMINI_API_KEY` set on the candidate backend it is one live voice call on Gemini's Live API, relayed through the backend so the server keeps the transcript and the time limits; run end to end on 2026-10-07 against a stand-in for Gemini, never yet against Gemini itself or in a browser. Without the key, or if the call drops, it is turn-based over the browser's speech features (Chrome and Edge) and resumes where the call stopped. Answers are recorded (voice, plus camera when on) and playable from the company's report page, and deleted after 90 days by a daily job in the admin app (needs `CRON_SECRET` set there; not yet run). Untested on a real microphone |
| 4 | Recording AI usage as evidence (prompts, what was accepted) | Built. Every assistant and interview turn is recorded and reaches the report; every model call's tokens and billed cost are stored per session. The report has an "AI assistant usage" item: how many messages were sent, how many lines of code the assistant showed, which of them appear word for word in the submitted work, and how often text was copied out of the panel. It only sees verbatim lines — an idea rewritten in the candidate's own words is not detected. Pasting into the editor is not captured (see 26) |
| 5 | Task generation per company / role | Built. From the admin library or from a company's own role page; from notes, a job description and a sample of the company's code. Every draft comes with a reference solution (never sent to a candidate) and is run before saving — tests must fail as given and pass with the solution — in a Daytona sandbox, or a local process when `TASK_VERIFY=local`; with neither it is marked "not run". A template can carry variants, dealt to candidates in turn. Company tasks are private to the company. Run end to end on 2026-10-07 against a stand-in model with the local runner (the Daytona runner has never been run). Not checked on a real model: whether generated code still labels its own bug, and the quality of what it writes from a job description |
| 6 | Evaluation report | Built. Code evaluation reads the task brief; the rubric is scored per criterion; attempts to instruct the AI are reported as an integrity finding. An agent that fails is retried once and, if it fails again, named in the report as missing. Observations cite the session moments they rest on, shown to the hiring team as times and what happened. The report says plainly when no interview took place or it was cut short, and lists large outside pastes and time away from the tab as signals. A reviewer can add notes and corrections and record their own decision beside the AI's (migration 0016). Run end to end on 2026-10-07 against a stand-in model. The default model (Claude Sonnet 4.5) has still never been run, and only Gemini has been run for real |

**Profile and accounts**

| # | What | State today |
|---|---|---|
| 7 | Connect GitHub / GitLab / LinkedIn through Composio | Built, off until `COMPOSIO_API_KEY` is set on the candidate app and the Composio project has an auth config per platform (the app looks them up, never creates one, and offers sign-in only where one exists). A candidate signs in on a page hosted by Composio; the app asks Composio who signed in and records that account as theirs. The connection is kept while the account stays linked and deleted when the candidate removes it; only read tools are run through it. GitHub and GitLab become "Verified — yours". LinkedIn gives a name, headline and picture but usually no profile address, so it is shown as "signed in as …". LinkedIn messages or conversations cannot be read: neither LinkedIn nor Composio offers that. Run end to end in the browser on 2026-10-08 against a stand-in Composio. Against real Composio: the key, the auth-config lookup and every tool name are confirmed; **a real sign-in has not been done** (the key saw no auth configs in its project), so the shape of the tools' answers is from the docs and handled tolerantly |
| 8 | Candidate knowledge base (projects, languages, activity pulled from connected accounts) | Built (migration 0018). For a connected account it is read through the candidate's Composio connection: their own projects, the languages across them, recent activity. Private projects are counted and add to the language totals, but their names, descriptions and links are never stored or shown — a private repository is often an employer's. If the connection can't answer, that read falls back to the platform's public API. An account linked only by username is read from the public API and stored as unverified. The candidate can refresh it. GitLab activity always comes from the public API (Composio has no tool for it). Checked in the browser against a stand-in Composio on 2026-10-08; the public reads were run against real GitHub and GitLab on 2026-10-07 |
| 9 | Resume parsing into the profile / knowledge base | Built. A PDF or DOCX is read in the browser on upload; the server keeps the text and what it finds in it — technologies named (from a fixed list), a headline, location, summary, links, the span of years. The candidate can fill empty profile fields from it with one click; nothing is written without that. Pattern matching, not a model: it finds words, it does not judge skill, and the hiring team's view says so. A .doc or a scanned PDF is stored but not read. Checked in the browser with a real PDF on 2026-10-07. Also fixed: uploads over 1 MB used to fail |
| 10 | The profile a hiring team actually sees | Built. The company portal's candidate page shows what the candidate wrote, their resume (a link that works for an hour) and what was read from it, and their linked code accounts — each labelled with where it came from, plus whether the email is confirmed. Checked in the browser on 2026-10-07 |
| 11 | Forgot / reset password | Built, off until `RESEND_API_KEY` and `RESEND_FROM` are set on the candidate app (the form says so when they aren't). A link that works once, for an hour; only a hash of it is stored; three an hour per account. Run end to end on 2026-10-07 against a stand-in mail server — no real email has been sent. A reset does not sign out sessions already open elsewhere |
| 12 | Email verification on sign-up | Built, off until the same two mail settings are set. Sign-up mails a confirmation link (48 hours); the account works meanwhile, the dashboard asks for confirmation, and hiring teams see "email not confirmed". Accounts made before this stay unconfirmed until they confirm. Social sign-in counts as confirmed — and when it links to an existing account whose email nobody confirmed, that account's password is cleared, since whoever set it never showed the mailbox was theirs. Run end to end against a stand-in mail server |
| 13 | Sign-in with Google / GitHub / GitLab / LinkedIn | Built; each provider is off until its keys are added |

**Invitations and dashboard**

| # | What | State today |
|---|---|---|
| 14 | Invitation email with a link | Built, off until the company portal has `RESEND_API_KEY`, `RESEND_FROM` and `CANDIDATE_PORTAL_URL`. Inviting a candidate mails them a link to their assessments page and says which address to sign in with; the invite form reports what actually happened to the email, and the candidate page shows whether one was sent. No real email has been sent — the message is built and the send path is the mailer the portal already uses for team invites. The link is not a one-time token: the invitation belongs to whoever signs in with the invited address |
| 15 | Accept / decline an invitation | Built (migration 0019). An invitation asks for an answer before it offers Start. Declining asks once more, takes an optional reason, and closes it — a declined invitation can't be started. The company sees "Declined" in the pipeline with the reason, and "accepted" with the date. Checked in the browser on 2026-10-07 |
| 16 | Notifications (invite, deadline, report ready) | Built. The bell shows an invitation waiting for an answer, a due date within three days or passed, and a report that is ready or failed. Nothing is stored as a notification — each is worked out from the candidate's records when the bell is read, so it disappears when the thing it is about is dealt with. Read and dismissed are kept on the server. In-app only: no email or push is sent for a deadline or a report. Checked in the browser on 2026-10-07 |
| 17 | Activity feed "See all" | Built. The feed shows the five most recent, each linking to its workspace or report; "See all" opens the full assessments list |
| 18 | Environment check (camera, mic, browser) | Built. Opens the camera and shows the picture, opens the microphone and shows the level it hears, and asks the browser for each thing the workspace uses; a failure says what to do about it. Passing all three ticks the step on the dashboard. What is saved is the browser's own report (three yes/no answers), not something the server measured. Run in the browser with stand-in devices, including a refused permission — not with a real camera or microphone |
| 19 | Practice run | Built. Opens the real workspace on a small task with one bug and a test that catches it (checked with real Python: fails as given, passes when fixed). There is no session behind it, so nothing is recorded, timed or submitted; only that one was opened is kept, for the dashboard's counter and setup step. The AI assistant is off in a practice run, and the brief says so |
| 20 | Login check on `/practice` and `/environment-check` | Built. Both now require sign-in, like the rest of the portal |
| 21 | ~~Per-candidate limit on self-started sessions~~ | Removed 2026-10-14 with the open pool: assessments are by invitation only. A candidate can start only an invitation addressed to their own email whose task is ready |

**Workspace**

| # | What | State today |
|---|---|---|
| 22 | Hosted backend so an assessment can be started at all | Deployed on Railway (`mindfries-candidate-backend.up.railway.app`), database and OpenRouter key configured. Whether the deployed candidate and admin sites point at it (`CANDIDATE_BACKEND_URL` / `ADMIN_BACKEND_URL` on Vercel) has not been checked |
| 23 | Real sandbox (Daytona) instead of in-browser execution | Built. With `DAYTONA_API_KEY` set, a session gets a real machine when it starts: the task is put there with git set up, the terminal is a real shell on it (anything can be installed and run, with internet access), the editor's files are kept in step with its disk in both directions, the Tests panel runs there, and checkpoints and the final snapshot are read from the sandbox itself rather than from the page. Commands are reported by the shell and recorded by the server. Submitting deletes the sandbox. If Daytona isn't configured or doesn't come up, the session runs in the browser as before; a practice run always does. Run end to end against real Daytona on 2026-10-08 (API and browser). Since then (run against real Daytona and in a browser, 2026-10-08): a shell outlives a dropped connection or a reload and the page reattaches to it; test results are read by the server from the output it relayed, not posted by the page; the Ports panel lists what is listening and links to it; `SANDBOX_NETWORK` (open / essentials / none) sets what a sandbox may reach and the workspace says which; `SANDBOX_MAX_LIVE` caps sandboxes running at once; the session clock starts when the workspace is ready, not while it is being prepared. Known limits: notebook cells, live preview and `pip`/`npm` shims are the browser workspace's and aren't wired to the sandbox (the terminal does those jobs, and the notebook says so); files over 512 KB and binary files are named but not opened in the editor; shells are held in the backend's memory, so a backend restart ends them; the command record can be tampered with by a candidate who edits their shell setup; "essentials" still reaches the large AI providers' APIs, because Daytona decides that list |
| 24 | Test runner and test results panel | Built. `python -m unittest` and `node --test` / `npm test` run a project's tests, and a Tests tab shows the last run as results — each test, pass or fail, and what a failure said — whether run from the panel or the terminal. Seen working in a browser on 2026-10-07 |
| 25 | Raw terminal command capture | Built. Each finished command is recorded with its exit code and duration; a test run with its counts and failing tests. Output is not sent, apart from test counts |
| 26 | Navigation, edit-by-edit, paste and tab-switch capture | Mostly built. Recorded: the file being looked at as it changes, lines added and removed on each save, pastes (size and where, never the text), and time with the tab out of view. Not keystroke-level |
| 27 | Camera recording / snapshots | Built as stills: one small image every half minute during a real session, in a private bucket, shown on the company's report page as an even sample, deleted after 90 days. Not video, and nothing analyses them. Seen storing real images on 2026-10-07 |
| 28 | Screen recording | Not built, and the dashboard no longer says the screen is recorded. Whether to build it is an open decision |
| 29 | Auto-submit when the timer reaches zero | Built. In the browser the interview opens at zero and the work is submitted after it. If the tab is closed instead, the backend submits the session itself once the time and the interview's allowance have passed, using a copy of the workspace the browser saves every two minutes. Run live on 2026-10-07: the server submitted an abandoned session and moved the invitation and pipeline with it; the evaluation that follows could not be checked because the test OpenRouter key was out of credit |
| 30 | Workspace saved to the server | Built. Saved every minute and when the tab is hidden; restored from the server when this browser has no copy or an older one. Seen restoring in a browser on 2026-10-07 |
| 31 | Separate saved workspace per assessment | Built. Each session has its own browser storage key |
| 32 | Resume an in-progress session from the dashboard | Built. An assessment under way has a Resume link; the clock keeps running on the server, and a session resumed after its interview began opens on the interview |
| 33 | Git changes view and diff viewer | Built as a Changes tab: files that differ from the task as given, with lines added and removed, each opening as a side-by-side diff. It compares with the starting files rather than git's index. Seen working in a browser on 2026-10-07 |
| 34 | `git clone` / `push` / `pull` | Partly, on purpose. `clone`, `fetch`, `pull` and `remote` work for public repositories on GitHub, GitLab, Bitbucket and Codeberg, through a narrow read-only proxy; clone needs an empty workspace. `push` is refused: work is submitted through the assessment, and no git credentials pass through the app. A real clone and pull were run in a browser on 2026-10-07. Each session now has its own git storage |
| 35 | Live status over WebSocket | Built for the candidate's report page: it listens to the session's event stream and refreshes the moment the report changes, with slow polling as a fallback. The stream was checked against a local backend on 2026-10-07; the page using it has not been watched in a browser. The company portal still polls |

**After submitting**

| # | What | State today |
|---|---|---|
| 36 | Report page | Built; shows "failed" without an OpenRouter key |
| 37 | The company's pipeline updating when a candidate starts or submits | Built: in progress on start, completed with time taken on submit, score and per-criterion scores after evaluation; a stage set by hand is left alone. Start and submit run live on 2026-10-07; the score write-back still has not been, for lack of OpenRouter credit |
| 38 | Candidate-facing outcome (shortlisted, rejected) | Not built |

## What the platform should do behind the scenes

| # | What | Status |
|---|---|---|
| 1 | Give each candidate an isolated sandbox | 🟡 Built, but switched off — no API key configured yet |
| 2 | Track what a candidate actually does | 🟡 Real, but partial — captures file saves and terminal *commands run through the system* (git, package installs, etc.), not every raw keystroke typed into the terminal |
| 3 | Run tests automatically | ❌ |
| 4 | Store code changes safely | 🟡 Saved in the candidate's own browser only — not backed up anywhere else yet |
| 5 | Generate evaluation evidence | 🟡 Built, but switched off — no AI API key configured yet |
| 6 | Generate a final report | ✅ |

## Our own internal admin tools

All four working.

| # | What | Status |
|---|---|---|
| 1 | Onboard a new company | ✅ |
| 2 | Write and publish assessment templates | ✅ |
| 3 | See every company's sessions in one place | ✅ |
| 4 | Reset a stuck session | ✅ |

## Built, but not part of the original checklist above

- **Real sign-in with Google, GitHub, GitLab, or LinkedIn** — works, but
  each one only turns on once someone registers a real app with that
  provider and adds the keys.
- **A small sales CRM** for our own outreach team (`/admin/targets`) —
  fully real, just never mentioned here before.
- **Real email sending** (waitlist replies, lead outreach) — built, but
  switched off until someone adds a real email-sending key.

---

## Known gaps, in plain terms

- **The Company Portal hasn't met real services yet.** It's built (see
  above), but no real email has been sent from it, the score write-back
  after evaluation hasn't run against a real model, and its backend
  (Stripe billing) isn't deployed anywhere.
- **A candidate can start as many separate practice sessions as they want**
  from the open, non-invited pool of assessments — there's no per-candidate
  limit yet. (A candidate *can't* replay or resubmit the same session
  anymore — that's fixed.)
- **Terminal keystrokes aren't tracked** as evidence yet — everything else
  a candidate does is.
- **Most of the database is still empty.** Every feature listed as ✅ has
  been tested against the real database, but nothing has real customers or
  candidates in it yet.
- **Every candidate gets the same fixed task and codebase** unless someone
  writes a custom one by hand. A real system to generate a codebase
  specific to each company was discussed and written up in the product
  plan ([PRD §1.4](System_Archetect_And_PRD.md)) — decided, not built.

## Security, in plain terms

The core login system (passwords, sessions, admin access) is solid — we
checked it carefully. We also found and fixed several real problems: a
session could be resubmitted endlessly, a lower-permission admin account
could do anything a full admin could, two settings pages would let anyone
in if a security key was left blank, and there was no limit on how much
data a candidate's browser could send us. All of those are fixed now. A
short list of smaller things is still open — see `AUDIT.md` for the full
write-up with exact detail.

## Waiting on a decision (not something we can just build)

- **Where does a candidate's code actually run** — in the browser, like
  today, or in a real isolated sandbox on our own servers?
- **Where does our own backend server run?** Right now it only runs on a
  developer's laptop — it needs a real home online.
- **Which AI model should each evaluation step use?**
- **Should candidates keep being able to sign themselves up**, or should
  that eventually require a real invitation only?
- **Should we capture raw terminal keystrokes**, given the extra work
  needed to do it safely?
- **How should a company-specific codebase actually get generated?**
  Decided in principle (see "Built, but not part of the original checklist"
  and [PRD §1.4](System_Archetect_And_PRD.md)). Partly answered: each
  candidate now gets a task generated from the role's brief (see
  "Invitation-only assessments" below); a codebase generated from a
  company's own code is still not built.

## What's next, in order

1. **Get real API keys** for the AI evaluation service, the sandbox
   service and email — everything downstream is already built and just
   waiting, including the Company Portal's invite emails and score
   write-back.
2. **Deploy `company/backend` and set up a real Stripe account**, or decide
   billing stays manual (credits granted by our team) for now.
3. **Decide the AI interview's timeline** — build it for real, or say
   clearly it's not happening soon.
4. **Connect the live status/terminal streaming** that's already built but
   nothing uses yet.
5. **Merge the two "reset a session" tools into one** — one is the real
   one, the other is a temporary backup.

### Invitation-only assessments (2026-10-14)

| What | State |
|---|---|
| No open pool | Built. The candidate's list holds their invitations only; a library task can't be started directly (checked against the real database) |
| A link per invitation | Built (migration 0022). `/invite/<token>` on the candidate site: signed out, it shows who invited them with the address masked and offers sign-in or sign-up, coming back to the link after; signed in as someone else, it says so and offers to switch; signed in as the invited address, it starts the assessment. Checked in a browser |
| A task per candidate | Built. candidate/backend notices each new invitation, generates a task from the role's brief that differs from the role's other candidates' tasks, runs it (tests must fail as given and pass with the solution) and links it. A rejected task is written again with the reason; a used-up quota or a busy model makes the invitation wait instead of failing. Run end to end on 2026-10-14 against real Gemini and Daytona: two candidates invited to one payments role each got a different task (a split-payment compensation bug; a settlement withholding bug), both passing the run check, in about six minutes each on `gemini-3.8-flash`; one was then opened from its link, through onboarding, into a sandbox workspace on that task. With thinking turned down, six of six tasks had failed the check |
| Company portal | A role has a brief and a kind of task; inviting shows the link to copy and whether the task is ready, with "Try again" if it failed. Type-checked; not driven in a browser |
| Admin portal | Invite on a company's behalf to one of its roles or a new one described there; the link is shown. Type-checked; not driven in a browser |
| AI provider | Gemini, called directly (`AI_PROVIDER=gemini`, the default). Every agent on `gemini-3.5-flash` until the key has billing; Pro models can be set per agent with `GEMINI_MODEL_<AGENT>` |
