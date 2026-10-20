# V2 integration — one system, three portals, same deployments

How the redesigned engine plugs into the **existing** Mindfries apps and
deployments, stays integrated across admin / company / candidate exactly as V1
is, and later **replaces** V1 by a switch — without standing up any new Vercel
or Railway projects.

This supersedes the "its own Vercel project / its own Railway service" detail in
[`PLAN.md`](PLAN.md) §4 and the separate-infra steps in [`SETUP.md`](SETUP.md):
V2 reuses V1's deployments.

## Principle

> Build V2 under `v2/` in isolation, test it independently (done so far), then
> wire it into the existing apps **behind a switch** and flip the default from
> `v1` to `v2`. Same accounts, same database, same deployed projects throughout.

## One shared database (already live)

- **V1 tables** (accounts, `companies`, `company_users`, `candidate_users`,
  `job_roles`, `assessments`, sessions, reports) — unchanged. V2 reads them
  read-only via the `v2_api` role.
- **`v2` schema** (`products`, `product_tasks`) — V2 owns it. Done.
- **One additive V1 change** (the only V1 write, a switch-guarded `v1-seam`):
  `assessments.engine` (`'v1'`|`'v2'`, default `'v1'`) and the role's **job
  description** on `job_roles`. Everything else is additive in `v2`.

## Three portals, integrated as in V1 (V2 surfaces added to the *same* apps)

| App (existing deployment) | V1 today | V2 adds (flag-gated, same app) |
|---|---|---|
| **internal-admin/frontend** (Mindfries ops) | library authoring, session monitor, companies | **Product Registry** (the 5 products: manifest, golden commit, verified status) and **Task Bank** curation + verify status; session monitor also shows v2 sessions. Replaces per-game authoring with product/task curation. |
| **company/frontend** | create roles, invite candidates, review | On a role: enter **job description** + choose **engine = V2**; preview which product + a sample task; review the v2 candidate's evidence and capability report. |
| **candidate/frontend** | onboarding, dashboard, browser IDE, report | Dashboard shows the v2 assessment; **"Set up workspace"** → install the CLI + device sign-in; live session status; the capability-profile report. (The browser IDE is replaced by the local CLI for v2, because the products are real Go/Python codebases.) |

The candidate portal stays the hub — it launches the CLI and shows the report —
so the experience is integrated end-to-end, not a separate tool.

## The engine, in the existing backend (not a new service)

V1's `candidate/backend` (Go, deployed) currently does **per-candidate task
generation** (`internal/llm/GenerateTask` + the invitation worker). V2 replaces
that path with the engine built under `v2/api`:

- **select** product from role+JD → **resolve** task (JD-cached, variant pool) →
  **deliver** the candidate workspace → **verify** the submission (real check +
  acceptance), plus the assistant gateway for `mindfries agent`.

Deployment: the engine ships **inside the existing Go backend deployment** —
`candidate/backend` imports the `v2/api` engine packages (or they are folded in
at cut-over). No separate Railway service. During the isolated test phase the
packages live under `v2/` and are exercised by their own tests + the CLI's fake
API; integration wires `candidate/backend`'s session/invite path to call them.

## End-to-end flow (v2)

```
admin: curate products + reference tasks (internal-admin)         → v2.products / v2.product_tasks
company: create role + JD, engine=v2 (company/frontend)           → job_roles.jd, assessments.engine='v2'
invite candidate                                                   → engine selects product, resolves task (cached by JD)
candidate: dashboard → set up workspace → CLI start (candidate/frontend + CLI)
           → works on the real product → mindfries submit          → engine verifies (suite + acceptance)
company + admin: review evidence/report, decide                    → existing report/review surfaces
```

## Cut-over = a switch (the "replace after testing")

1. **Test phase (now):** `assessments.engine` defaults to `'v1'`; V2 runs only
   for explicitly flagged pilot assessments; all V2 UI behind `V2_ENABLED`.
2. **Replace:** once the V2 round-trip passes on real assessments, flip
   `V2_DEFAULT_ENGINE` `v1 → v2`. New assessments use the product engine.
3. **In-flight:** running V1 assessments finish on V1; old reports stay
   readable. No data migration — same accounts, roles, dashboards.
4. **Rollback:** flip the default back to `v1`.

Nothing is re-hosted at any step: the same Vercel projects serve the (now
V2-enabled) Next apps, and the same Go backend deployment runs the engine.

## What this changes in the build plan

- Candidate/company/admin V2 screens are **PRs into the existing three apps**,
  flag-gated — not a separate `v2/web` Vercel project.
- The engine is **wired into `candidate/backend`** — not a separate service.
- `v2/db` migrations + the `assessments.engine`/JD `v1-seam` are the only DB
  changes; both already target the shared project.
