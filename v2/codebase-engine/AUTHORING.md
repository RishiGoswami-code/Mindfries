# Authoring a product (the repeatable pipeline)

How an owned product codebase is built so we can trust it and verify tasks
against it (PLAN task 1.1). P1 (`v2/products/p1-support-saas`) was built this
way; P2–P5 follow the same steps. The rule from the Q2 decision: we generate
and own the code *because* we need a trusted test suite and a known
ground-truth — neither of which an arbitrary OSS repo gives us.

## Steps

1. **Pick the domain and subsystems** from the coverage matrix
   (`ANALYSIS.md`): which roles this product serves and which subsystem each
   role works in. Write them down first — they become the manifest.
2. **Author in pieces, never one giant generation.** Scaffold the module, then
   build one subsystem at a time: domain model → store (interface +
   in-memory) → service rules → thin HTTP/UI layer → seed data. Each piece
   lands with its own tests. (This is why the V1 engine truncated: it tried to
   emit a whole codebase in one reply. Here, nothing is generated whole.)
3. **Keep the suite green and dependency-light.** Tests run with no database
   (in-memory store behind an interface) and no network, so the verification
   harness is fast and hermetic. `go test ./...` and `go vet ./...` clean.
4. **Make it runnable.** A `cmd/server` (or equivalent) that seeds realistic
   data and serves, so a candidate sees a live product, not a skeleton.
5. **Write the manifest** (`manifest.json`) and validate it:
   `go run ./cmd/manifestcheck <product>/manifest.json` (from `v2/api`). Only
   claim roles the code can actually assess; list `cannot_assess` honestly.
6. **Author reference tasks** under `<product>/_tasks/<id>/` (the leading
   underscore makes the Go tool ignore the bank, since overlay/solution files
   only compile once applied over the product):
   - `task.json` — id, kind, role, title, check command.
   - `TASK.md` — the candidate-facing brief (becomes the locked TASK.md).
   - `overlay/` — files applied over the product for the candidate (a feature
     stub, or a planted bug for a debug task).
   - `acceptance/` — hidden tests applied only at verification (feature tasks).
   - `solution/` — the reference solution, applied to prove the task is solvable.
7. **Verify the task is real** with the harness (`v2/api/internal/verify`):
   - feature: `overlay+acceptance` fails; `overlay+solution+acceptance` passes.
   - debug: `overlay` fails the suite; `overlay+solution` greens it.
   The `verify` package tests do exactly this for P1 — copy them per product.
8. **Freeze and register.** Commit; that commit is the product's
   `golden_commit`. Insert the product + manifest into `v2.products`
   (see `v2/db/migrations/0005_seed_p1_product.sql` for the shape).

## Task layer format (what the engine's `product_tasks` rows point at)

A task directory maps onto the `product_tasks` columns: `overlay/` →
`overlay_ref`, `acceptance/` → `acceptance_ref`, `solution/` → `solution_ref`,
`TASK.md` → `brief_md`, `task.json.kind/role` → `kind`/`role`. The JD-keyed
task engine (Phase 2) generates new task directories in this same shape and
runs them through the same harness before caching them.
