# Runbook — running an assessment end to end

Who this is for: anyone taking the platform through a full dry run — a
company admin inviting a candidate, and the candidate taking the assessment.
It covers the deployed portals; nothing here requires a local checkout.

The flow in one line:

> Company creates a role → invites a candidate → the platform prepares that
> candidate's own task → the candidate opens the invite link, onboards, and
> works in the timed workspace → both sides see the evidence report.

---

## Before you start (operator checklist)

One person with access to the deployments checks these once. Everything else
in this runbook degrades gracefully; these four do not.

| Check | Why |
|---|---|
| All three portals point at the **same Supabase project** | Invites written by the company portal must be visible to the candidate portal |
| The **Go backend** (`candidate/backend`) is running, with `DATABASE_URL` and `OPENROUTER_API_KEY` set | It generates each candidate's task and evaluates the submission — without it, invites stay "being prepared" forever |
| The candidate portal has `CANDIDATE_BACKEND_URL` set to that backend, and the backend's `CANDIDATE_SESSION_SECRET` equals the candidate portal's `SESSION_SECRET` | Starting a timed session is a frontend→backend call; a secret mismatch means every start is refused |
| (Optional) Resend email on the company portal | Without it, invites still work — the portal shows you the link to send yourself |

## Part 1 — Company admin

1. **Sign in to the company portal.** There is no signup page: the first
   login for a company is created by Mindfries ops from the internal admin's
   onboarding flow, which emails a set-password link. If you don't have a
   login, that's the step you're missing.
2. **Create a role** — Roles → New role. The role brief describes what the
   assessment should test. Each invited candidate gets their **own task
   generated from this brief**, so write the brief as the thing you want
   tested, not as a specific puzzle.
3. **Invite the candidate** — open the role → Invite candidate. Enter their
   email (exactly the address they'll sign in with), name, and an optional
   due date.
   - If email is configured, they're sent their invite link.
   - If not, the portal says so and shows the link — copy it and send it to
     them yourself. The invitation is real either way.
4. **Wait for "task ready".** Right after inviting, the candidate's task is
   being prepared (the backend generates it from your role brief, usually
   within a minute or two). The candidate can open their link immediately,
   but can only *start* once the task is ready.
5. **Watch the pipeline.** The role page shows the candidate move through
   invited → in progress → submitted. After submission, the Candidates pages
   show the full evidence report: how they worked, not just what they
   submitted.

## Part 2 — Candidate

1. **Open the invite link** (`/invite/<token>`). It shows who invited you
   and for what role. The link is personal: it only works signed in as the
   invited email address — forwarded to anyone else, it shows nothing
   usable.
2. **Sign in or create an account with the invited address.** Signed in as
   a different address, the page offers to switch accounts.
3. **Start the assessment.** If it still says your task is being prepared,
   wait — the page updates when it's ready. Starting accepts the invitation
   and ties it to your account.
4. **Onboarding:** consent → device/camera check → lobby. Nothing is timed
   yet. Optional but recommended on a first run: the **practice run** from
   the dashboard opens the same workspace on a throwaway task — nothing in
   it is recorded or scored.
5. **Enter the workspace.** Clicking "Enter Workspace" in the lobby is the
   moment the clock starts and evidence capture begins — not before. The
   header shows your remaining time; a page reload does not reset it.
6. **Work like you normally would.** The workspace is a real editor,
   terminal, and git. Run your code, run the tests, commit as you go. The
   platform records *how* you work; there is no benefit to performing.
7. **Submit.** After submitting, the workspace closes for good — the session
   can't be re-entered. Your report appears under Assessments once
   evaluation finishes.

## Part 3 — After submission

- **Candidate** sees their report at Assessments → that session → Report.
- **Company admin** sees the submission in the role's pipeline and the full
  evidence on the candidate's page.
- Evaluation runs on the backend and takes a few minutes. A report marked
  **failed** means evaluation genuinely couldn't run (most often a missing
  `OPENROUTER_API_KEY` on the backend) — it is never silently faked.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Invite stuck on "your assessment is still being prepared" | The Go backend isn't running (it's what generates the task) | Start `candidate/backend` against the shared database; pending invites are picked up automatically, including ones created while it was down |
| Lobby says "the assessment service isn't connected" | `CANDIDATE_BACKEND_URL` unset or unreachable from the candidate portal | Set it and redeploy; starting a session is a live call to the backend |
| Every session start is refused / auth errors from the backend | `CANDIDATE_SESSION_SECRET` on the backend ≠ `SESSION_SECRET` on the candidate portal | Make them identical — the backend verifies cookies the portal signs |
| Invite email never arrives | Resend not configured on the company portal | The invite form already showed the link — send it manually; nothing else is affected |
| Candidate sees "this invitation is for a different email address" | Signed in with another account | Use "switch account" on the invite page and sign in as the invited address |
| "The candidate_users table doesn't exist yet" at sign-up | Database migrations haven't run | Run `npm run migrate` in `candidate/frontend` with `DATABASE_URL` set |
| Report status is `failed` | Backend has no `OPENROUTER_API_KEY` | Set it and re-run; the failure is honest, not a bug |
| Dashboard shows sample assessments instead of the real invite | Candidate portal missing Supabase env, or `CANDIDATE_BACKEND_URL` unset | Set `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `CANDIDATE_BACKEND_URL` |

For what each portal is and how they relate, see
[`System_Archetect_And_PRD.md`](System_Archetect_And_PRD.md); for what is
actually built today, see [`spec.md`](spec.md).
