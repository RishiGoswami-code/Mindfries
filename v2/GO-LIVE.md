# Go-live roadmap — making the codebase engine customer-available

What's left to turn the redesigned engine (products + selection + task delivery
+ verification, all built and tested in `v2/api` and `v2/products`) into
something a real company can use to evaluate real candidates, on top of the
already-deployed V1. Ordered, with the decisions called out.

## 0. The gating decision — where candidates run the product

The products are real codebases: **Go** (P1 Support SaaS, P2 RAG, P4 Fintech)
and **Python** (P3 Quant). That determines the execution environment:

| Path | Runs Go? | Runs Python? | What it needs | Reuses deployed V1? |
|---|---|---|---|---|
| **V2 CLI** (candidate's own machine) | ✅ | ✅ | `v2/api` deployed (Railway) + signed CLI distribution | No (new surface) |
| **V1 Daytona sandbox** (real shell, browser-driven) | ✅ | ✅ | `candidate/backend` deployed + `DAYTONA_API_KEY` | Yes (candidate frontend) |
| **V1 browser IDE** (Pyodide/TS) | ❌ | ⚠️ partial | nothing new | Yes, but **Go products excluded** |

The browser IDE cannot run Go, so launching the full product set requires the
**V2 CLI** or the **V1 Daytona sandbox**. This is question 1 below; the rest of
the roadmap branches on it.

---

## Track A — Database (Supabase) — ~done

The `v2` schema is live (you ran 0001–0003). Remaining:

1. Run `v2/db/migrations/0004_v2_role_grants.sql` (set a password) — creates the
   `v2_api` role with full `v2` access + read-only on the five V1 tables.
2. Run the product seeds: `0005` (P1), `0006` (P2), and the P3/P4/P5 seeds as
   they land — registers each product + manifest in `v2.products`.
3. Nothing else: the engine reads the registry and writes the task cache
   (`v2.product_tasks`) as `v2_api`.

---

## Track B — The engine server (`v2/api`)

Built and tested today: selection, task resolution (JD cache + variant pool),
workspace delivery, the verification harness, the manifest validator. To become
a live service it needs:

1. **HTTP server** implementing the contract: create a session for
   `(role, JD)` → resolve product+task → serve the candidate workspace; accept
   a submission → run the verification harness → store results.
2. **Postgres repository** for `v2.products` / `v2.product_tasks` (read registry,
   write the JD-keyed task cache). Interfaces exist; add the pgx-backed
   implementation and a connection via `V2_DATABASE_URL`.
3. **Product source access** — how the server gets `product@golden_commit`:
   clone this repo at the commit (simplest), or store per-product tarballs in a
   Supabase Storage `products` bucket and fetch them. *Default: clone at the
   golden commit.*
4. **Model provider (optional)** — the Gemini/OpenRouter `Provider` for brief
   adaptation. Launch works without it (authored briefs); adds JD-tailored
   wording when you supply a key. *Needs a model API key.*
5. **Deploy** — Railway (per the plan) or wherever your backends run. Env:
   `V2_DATABASE_URL`, `V2_PRODUCTS_*`, optional `V2_GEMINI_API_KEY`.

---

## Track C — Candidate execution (branches on decision 0)

**If V2 CLI:** deploy `v2/api`; set the API URL into signed CLI builds; host the
one-line installer (GitHub Releases). The CLI (already built) does
`start → work → submit`; submission verification runs the product's check +
the task's acceptance via the harness.

**If V1 Daytona sandbox:** deploy `candidate/backend` with `DAYTONA_API_KEY`;
have the engine assemble the candidate workspace (it already does — `delivery`)
and drop it into the sandbox at session start; the existing candidate frontend
shows the editor/terminal. Submission verification runs the check in the
sandbox.

---

## Track D — Company input surface (the V1 seam)

So a company can actually request a V2 assessment:

1. In `company/frontend`, when creating a role/assessment, capture **role +
   job description** and an **engine = V2** choice (behind the `V2_ENABLED`
   switch).
2. Persist `assessments.engine = 'v2'` + the JD (the one additive V1 change —
   a `v1-seam` migration + PR, switch-guarded).
3. On candidate invite, the engine resolves `(product, task)` from role+JD
   (selector + taskgen) and the chosen execution path delivers it.
4. Editing the JD regenerates the task pool (already handled by the cache key).

---

## Track E — Candidate report

1. Each task carries the role rubric it was calibrated against.
2. The report scores per competency using the verified submission + harness
   results + the manifest's coverage/"not assessed" honesty.
3. Human review before the hiring decision (reuses V1's report/review surface).

---

## Recommended minimal launch sequence

1. **Decide execution path** (question 1).
2. Run **Track A** steps 1–2 (grants + product seeds). *~15 min, you.*
3. Build **Track B** 1–3 (server + pg repo + clone-at-commit). *me.*
4. Wire **Track C** for the chosen path. *me + one deploy, you.*
5. Wire **Track D** (company role+JD → engine=v2, switch off until ready). *me,
   `v1-seam` PR you approve.*
6. Pilot with one role on one product; verify a full candidate round-trip.
7. Turn the switch on for the pilot cohort; then **Track E** report polish.

Items needing **you**: the execution-path decision, a model key (optional), a
DB connection string for the deployed server, and approving the one `v1-seam`
PR. I'll flag each when we reach it.
