# Codebase engine — role & market analysis

Why the assessment engine should stop generating a throwaway mini-codebase per
candidate, and instead stand up a small set of large, owned, production-like
**products** that many roles are assessed on. This file is the *coverage*
argument. The engine design is in [`ARCHITECTURE.md`](ARCHITECTURE.md); the
build order is in [`PLAN.md`](PLAN.md).

See [`../SPEC.md`](../SPEC.md) for the product principles this serves, and
[`../ROLE_FLOWS.md`](../ROLE_FLOWS.md) for the FDE/DevOps worked examples the
products must be able to host.

## What's wrong with the current engine (V1)

The live V1 engine (`candidate/backend/internal/llm/conversation.go`,
`GenerateTask`) authors a **whole mini-codebase per candidate**: 4–7 files,
each under 80 lines, plus a hidden solution and a verification run. One
generation per invitation, ~6 min each, a 20k-token reply ceiling, a single
retry, and **no caching**.

Consequences, all of which the redesign targets:

- **Cost scales with candidates.** Every invite is a fresh ~6-minute LLM job.
- **Fragile.** Truncation (`ErrTruncated` at `openrouter.go:43`) and
  verification failure kill the invitation and send an alert email; the
  candidate is stuck.
- **Not production-like.** 4–7 tiny files can't show how someone works in a
  real product with history, tests, dependencies, and other people's code.
- **Thin role coverage.** A toy task can't exercise FDE customer-integration,
  quant data-pipeline reasoning, or AI-engineering eval work.

## The core insight: one product serves many roles

A real product already contains a backend, a frontend, an API, usually an AI
feature, and an analytics surface. So Backend, Frontend, Full-Stack, FDE,
AI-Engineer and Data-Analyst can all be assessed **on the same product**, each
touching a different subsystem — exactly like several engineers collaborating
on one codebase. Only the numerical roles (Quant, AI/ML Researcher) need a
distinct data/compute substrate.

That collapses "one codebase per candidate" into "a handful of products, each
reused across many roles and many candidates."

## The eight roles and what each needs to be shown

| Role | What the task must let them demonstrate | Substrate it needs |
|---|---|---|
| Backend Engineer | APIs, data models, services, correctness, reliability, tests | A real service + DB + API |
| Frontend Engineer | UI, state, components, a11y, API integration | A real web client |
| Full-Stack SDE | A vertical slice across both | A web product (client + server) |
| Forward Deployed Engineer | Integrate with a customer system, investigate unfamiliar data/API, ship a slice, handover | A product with external integrations + sample data + a customer scenario |
| Quantitative Engineer | Data pipelines, backtesting, risk, numerical correctness, statistical discipline | A quant/market-data research codebase + datasets |
| Data Analyst | SQL/dataframes, cleaning, metrics, exploratory analysis | Datasets + an analytics/metrics surface |
| AI/ML Researcher | Experiment design, time-aware validation, leakage, evaluation, reproducibility | An ML pipeline / eval harness + datasets |
| AI Engineer | LLM apps, RAG, agents, prompt/eval, serving, integration | An AI-powered application |

## Proposed products (5) and the coverage matrix

Five owned products. ✓ = a *primary* assessment surface for the role; ∘ = a
usable *secondary* surface.

| Product | Backend | Frontend | Full-Stack | FDE | Quant | Data Analyst | AI/ML Researcher | AI Engineer |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| **P1 — Support SaaS** | ✓ | ✓ | ✓ | ✓ | | ∘ | | ∘ |
| **P2 — AI RAG/Agent platform** | ∘ | ✓ | ✓ | | | | ✓ | ✓ |
| **P3 — Quant research platform** | ∘ | | | | ✓ | ✓ | ✓ | |
| **P4 — Fintech ledger/payments** | ✓ | ∘ | ✓ | ∘ | | | | |
| **P5 — Product-analytics / warehouse** | ∘ | | | | | ✓ | ∘ | |

- **P1–P3 cover all eight roles at least once.** P4 and P5 give the
  high-volume roles (Backend, Frontend, Full-Stack, AI Engineer, Data Analyst)
  a **second product**, so not every candidate for a role sees the same
  codebase — required for integrity and for calibrating task variants
  ([SPEC §11](../SPEC.md)).
- **P1** maps straight onto the existing FDE flow (a support team that wants to
  triage tickets and suggest responses — [ROLE_FLOWS Flow A](../ROLE_FLOWS.md)).
- **P3** maps onto the [SPEC §15 quant scenarios](../SPEC.md) (Sharpe,
  leakage, unit mismatch, missing-data-as-signal).

### Per-product sketch

Each product is a believable, owned codebase with real history, a green test
suite, seed data, and run instructions. Stacks are indicative, not fixed.

- **P1 — Support SaaS** (B2B multi-tenant). Next.js client + Go/Node API +
  Postgres + a queue + webhooks; an **AI "suggested reply"** feature over a
  knowledge base; an **analytics** module (SLA, CSAT, ticket volume).
  *Hosts:* Backend (API/queue/webhooks), Frontend (agent console), Full-Stack
  (a ticket-workflow slice), FDE (triage + suggested-reply integration),
  AI-Eng (the reply/RAG feature), Data-Analyst (support metrics).
- **P2 — AI RAG/Agent platform.** Document ingestion → chunking → vector store
  → retrieval → tool-calling agent → an **eval harness** → a chat UI + serving
  API. *Hosts:* AI-Eng (retrieval/agent/serving), AI/ML-Researcher (the eval
  harness, prompt/model comparison, offline eval), Backend (ingestion/serving),
  Frontend (chat UI).
- **P3 — Quant research platform** (Python). Market/time-series ingestion →
  feature pipeline → backtesting engine → portfolio construction → risk
  metrics → notebooks. *Hosts:* Quant (primary), AI/ML-Researcher (time-aware
  validation, leakage), Data-Analyst (cleaning, metrics on the same datasets),
  Backend (the ingestion service).
- **P4 — Fintech ledger/payments.** Double-entry ledger, idempotent payment
  flows, reconciliation, a money-correctness test suite; a thin admin UI.
  *Hosts:* Backend (correctness, concurrency, idempotency — high-signal),
  Full-Stack (a payout slice), Frontend/FDE (secondary).
- **P5 — Product-analytics / warehouse** (optional-but-chosen). Event ingestion,
  a warehouse schema, SQL models/transforms, a metrics/dashboard layer.
  *Hosts:* Data-Analyst (primary: SQL, modelling, metrics), AI/ML-Researcher
  (feature/experiment data), Backend (pipelines).

## Market-trend rationale (2026)

The chosen domains track where hiring volume and signal concentrate:

- **AI-native SaaS** and **RAG/agent + LLM-eval** work are the fastest-growing
  engineering hiring areas → P1's AI feature and the whole of P2.
- **Quant / ML research** remains high-value and hard to assess well with toy
  tasks → P3.
- **Data-analyst roles are shifting toward analytics-engineering** (SQL +
  Python + modelling) rather than spreadsheet work → P3/P5.
- **Money/correctness backends** (payments, ledgers) are a durable,
  high-signal backend domain that rewards real engineering discipline → P4.

## How this maps to assessment quality

- **Realism:** candidates work in a product with other people's code, tests,
  and history — the thing the PRD actually wants to observe (*how* they work).
- **Comparability:** the same product + a calibrated task variant across
  candidates keeps the rubric stable ([SPEC §11, §14](../SPEC.md)).
- **Coverage honesty:** each product's role manifest states which competencies
  a given area can and cannot exercise, so the report can mark "not assessed"
  truthfully rather than inventing a signal.
