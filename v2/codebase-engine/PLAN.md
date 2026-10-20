# Codebase engine — build plan

Build order for the engine in [`ARCHITECTURE.md`](ARCHITECTURE.md), covering
the 5 products in [`ANALYSIS.md`](ANALYSIS.md). It targets **V2** and follows
the governing rule of [`../SPEC.md`](../SPEC.md): build and verify V2
independently, switch on only when the gate passes. It does not touch V1.

Each task is one PR with a "done when". Tick it here when it lands.

Dependency note: the whole engine rides on being able to **author one product
correctly and verify tasks against it**. So Phase 1 proves the pipeline on a
single product (P1) before Phases 2–3 scale it. Phase 0 can run alongside.

---

## Phase 0 — Contracts and schema (no product content yet)

- [x] **0.1 Product registry schema.** `products` table in `v2/db` (fields in
  ARCHITECTURE part 1). Own migration, `v2` schema, idempotent.
  *Done when:* migrations apply twice as a no-op; the V2 DB role can write
  `v2.products` and still cannot write any V1 table.
  → `db/migrations/0001_v2_schema.sql`, `0002_products.sql`; role isolation
  templated in `0004_v2_role_grants.sql` (run when the `v2_api` role is created).
- [x] **0.2 Task-bank schema.** `product_tasks` table (ARCHITECTURE part 3),
  keyed `(product_id, golden_commit, role, jd_hash, variant_no)`.
  *Done when:* a row references a product + overlay without carrying full file
  content; uniqueness on the cache key is enforced.
  → `db/migrations/0003_product_tasks.sql` (overlay/acceptance/solution are
  refs; unique on the cache key; pool index for invite-time lookup).
- [ ] **0.3 Role-coverage manifest spec.** A versioned JSON schema for
  `role_coverage`: per role, its subsystems/paths, competencies/modes, and the
  explicit "cannot assess" list. Document it; add a validator.
  *Done when:* a manifest validates in CI and a malformed one is rejected.
- [ ] **0.4 Product correctness contract.** Define what "verified" means for a
  product: suite green + typecheck + build at `golden_commit`, manifest valid,
  at least one verified reference task per covered role. A CI job enforces it.
  *Done when:* a product can only move to `status=verified` when the job passes.

## Phase 1 — Prove the model on one product (P1, Support SaaS)

This phase builds the **authoring + verification pipeline** and exercises it on
P1 end to end. It is the riskiest phase; everything later reuses it.

- [x] **1.1 Product authoring pipeline.** The repeatable way we generate an
  owned product *correctly*: author in pieces (scaffold → subsystem → tests),
  never one giant generation; iterate until the suite is green; human review;
  freeze the golden commit. Captured as a runnable checklist + helper scripts.
  *Done when:* running it on P1 yields a repo with a green suite, build, and a
  valid manifest, reviewed by a second person.
  → `codebase-engine/AUTHORING.md`. (Human review still recommended per product.)
- [x] **1.2 P1 codebase.** Support SaaS per ANALYSIS: client + API + Postgres +
  queue + webhooks + an AI suggested-reply feature + an analytics module. Seed
  data and run instructions. Registered in `products`.
  → `v2/products/p1-support-saas` — Go backend (tickets, SLA, routing,
  webhooks, suggested-reply, metrics, HTTP API, seed, runnable server), builds
  + vets + tests green; registered via `db/migrations/0005_seed_p1_product.sql`.
  **Remaining:** the Next.js console (Frontend-role coverage); until it lands
  the manifest doesn't claim Frontend, and the store is in-memory (no Postgres
  yet). Everything else is done.
- [x] **1.3 P1 role manifest.** Cover Backend, Frontend, Full-Stack, FDE, with
  AI-Eng and Data-Analyst as secondary; name subsystems and competencies per
  role, and what each cannot assess.
  → `manifest.json`, validated by `manifestcheck` (5 roles; Frontend held back
  honestly until the console exists).
- [x] **1.4 Reference tasks for P1.** For each primary role, at least one
  **feature** task and one **debug** task: brief, overlay (incl. planted-bug
  diff for debug), hidden acceptance tests, and a reference solution branch.
  → `tasks/debug-auto-assign` and `tasks/feature-ticket-merge`, both verified
  by the harness. (More per-role tasks to add as the bank grows.)
- [x] **1.5 Verification harness.** Given a product + task + a candidate branch,
  run suite + acceptance and report pass/fail per test. Reused by submission
  verification and by task generation (Phase 2).
  → `v2/api/internal/verify`; tests prove bug-fails/fix-passes and
  stub-fails/solution-passes on P1.

## Phase 2 — Selector + task engine + delivery wiring

- [x] **2.1 Selector.** `role + JD → product@commit` (ARCHITECTURE part 2):
  role filter + JD-similarity rank, cached by (role, jd_hash), score recorded.
  → `v2/api/internal/engine` (`Select`); payments JD → ledger product, support
  JD → support product, role-only pick when no tag matches, loud error on no
  coverage.
- [x] **2.2 Task engine.** `role + JD + product → task`, generating only a
  bounded subsystem diff + brief + acceptance tests; verify-once via 1.5; store
  in `product_tasks`. Context limited to the manifest's role subsystem.
  → `v2/api/internal/taskgen`. **Reliability-first:** task *code* comes from the
  pre-verified reference bank (selected + ranked by JD); the model only adapts
  the brief (graceful fallback). Fresh model-authored overlays (verified by 1.5
  before caching) plug into the same path later. No truncation risk.
- [x] **2.3 JD cache + variant pool.** Reuse by `(product, commit, role,
  jd_hash)`; a pool of N variants (default 3) generated once per JD, drawn
  round-robin per candidate; JD edit → new `jd_hash` → regenerate pool;
  in-flight sessions keep their pinned `task_version`.
  → `taskgen` cache + `MemCache`; tested: pool built once per JD, variant
  rotates per candidate, cosmetic JD edits reuse, real edits regenerate, stable
  `task_version`. (Postgres-backed `product_tasks` cache: wired with the server.)
- [x] **2.4 Delivery wiring (fake API first).** A V2 Task resolves to
  `product_id + commit + product_task_id`; `/session/bundle` serves
  product@commit + overlay (or the CLI clones the repo and applies the overlay)
  with `TASK.md` locked. Extend `v2/cli/fakeapi` and the contract.
  → `v2/api/internal/delivery` assembles the authoritative candidate workspace
  (product minus scaffolding + overlay, TASK.md=brief locked, check/runtimes),
  wired to `taskgen.ResolvedTask` via `FromResolved`; tests prove the delivered
  stub fails acceptance and the solved workspace passes, and that the answer key
  (acceptance/solution/tasks) never leaks. `v2/cli/fakeapi.ProductTask` mirrors
  it so the CLI serves a real P1 task. **Remaining:** the real `/session/bundle`
  signed-archive path + a full CLI subprocess e2e against P1 (the existing e2e
  already proves the flow against a served Task; delivery proves assembly).

## Phase 3 — The remaining products (P2–P5)

Reuse the Phase-1 pipeline. One product per PR set; fill role task banks to the
ANALYSIS coverage matrix.

- [x] **3.1 P2 — AI RAG/Agent platform** (AI-Eng, AI/ML-Researcher primary).
  → `v2/products/p2-rag-platform` — chunking, model-free embedder, vector
  store + search, ingest/query engine, tool-calling agent, eval harness
  (HitAtK/AnswerMatch), HTTP API, seeded KB, runnable server; builds/vets/tests
  green. Manifest covers AI-Engineer, AI/ML-Researcher, Backend (validated).
  Two harness-verified tasks: `debug-retrieval-order`, `feature-recall-at-k`.
  Registered via `db/migrations/0006_seed_p2_product.sql`. (Chat web UI for
  Frontend coverage deferred, as with P1.)
- [ ] **3.2 P3 — Quant research platform** (Quant, AI/ML-Researcher, Data-Analyst).
- [ ] **3.3 P4 — Fintech ledger/payments** (Backend, Full-Stack).
- [ ] **3.4 P5 — Product-analytics / warehouse** (Data-Analyst primary).
- *Done when (each):* `status=verified`, manifest validates, every covered role
  has ≥1 verified feature and ≥1 verified debug reference task, and the selector
  routes an appropriate JD to it.

## Phase 4 — V2 integration (real API + the V1 seam)

- [ ] **4.1 `v2/api` for real.** Selector + task engine with real model calls
  (server-side only), backed by `products`/`product_tasks`. CLI e2e passes
  against the real API as well as the fake.
- [ ] **4.2 Company portal engine choice** (`v1-seam`, switch off). Choosing
  `engine=v2` for a role stores its role + JD; the handoff resolves role+JD →
  product+task via the selector/engine.
  *Done when:* with the switch off the V1 regression suite is unchanged; with it
  on, a v2 assessment reaches a product workspace from the dashboard.
- [ ] **4.3 Role rubric + report.** Each product task carries the role rubric it
  was calibrated against; the report scores per competency with coverage/"not
  assessed" from the manifest ([SPEC §19](../SPEC.md)).

## Phase 5 — Calibration, integrity, scale

- [ ] **5.1 Recruiter preview / dry-run** ([SPEC §7.5](../SPEC.md)): preview the
  selected product, the task, acceptance expectations, and the report shape;
  replay the reference solution as the dry-run.
- [ ] **5.2 Rubric calibration** against reference solutions and expert-reviewed
  samples; keep evaluator/prompt versions auditable ([SPEC §14](../SPEC.md)).
- [ ] **5.3 Integrity pass:** variant-pool sizing, cross-candidate similarity
  checks, and confirm the CLI coding-agent signals ([../PLAN.md task 1.8])
  apply unchanged on a cloned product.
- [ ] **5.4 Cost + retention caps:** per-JD generation budget, pool size caps,
  product-storage retention, and metrics (generations/candidate should trend to
  ~0 as pools warm).

---

## Decisions (settled with the owner)

| Decision | Choice | Why |
|---|---|---|
| Target surface | **V2 engine** | Clean versioned-bundle substrate; no legacy generation to unwind; V1 stays live and untouched |
| Codebase source | **We generate & own them** (not OSS) | Need a trusted test suite + known ground-truth to verify feature-addition and debugging; OSS gives neither |
| Number of products | **5** (P1–P5) | All 8 roles covered, with a second product for high-volume roles (integrity + variants) |
| Per-candidate tasks | **Generate from role+JD, cache by JD**; pool of variants; **regenerate on JD edit** | Company owns the JD; same JD reuses the cached pool → cost flat in candidate count |

## Open questions for later (not blocking Phase 0–1)

- Product stacks per product (confirm P1's client/API languages to match CLI
  preflight runtimes).
- Variant-pool size N per JD (default 3) — tune against integrity vs cost.
- Whether P5 is built in the first wave or deferred until P1–P4 are live.
- Clone-vs-bundle threshold per product size.
