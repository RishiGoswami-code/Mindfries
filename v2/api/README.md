# `v2/api` — the V2 API

The server the CLI and `v2/web` talk to: setup codes and session tokens,
task bundles, the event and evidence store, the assistant gateway for
`mindfries agent`, submissions, and later evaluation. See
[PLAN, Stage 2](../PLAN.md#stage-2--the-v2-api-and-connecting-v2-to-v1-6-core-services)
and [SPEC §6](../SPEC.md#core-services).

**Status: the engine + the first real server are built.** The engine
(`internal/engine`, `taskgen`, `delivery`, `verify`, `manifest`) selects a
product for a role+JD, resolves a JD-cached task, and assembles the candidate
workspace. `cmd/server` is the real HTTP server, DB-connected. The full CLI
session contract (sessions, tokens, bundle, events, submit) still runs against
`v2/cli/fakeapi` until it is ported here.

## Run / deploy

`cmd/server` connects to the `v2` schema in Postgres (as the `v2_api` role),
reads the product registry, and resolves assessments.

```sh
V2_DATABASE_URL=postgres://v2_api:...@.../postgres \
V2_PRODUCTS_ROOT=../products \
go run ./cmd/server          # :8080
```

| Env | Meaning |
|---|---|
| `V2_DATABASE_URL` | Postgres URL for the `v2_api` role (**required**) |
| `V2_PRODUCTS_ROOT` | directory holding the product sources (default `../products`) |
| `V2_POOL_SIZE` | task variants per JD (default 3) |
| `PORT` | listen port (default 8080) |

Endpoints: `GET /healthz`, `GET /v2/products`, `POST /v2/assessments/resolve`
(`{"role":"backend","jd":"…"}` → selected product + resolved task + the
candidate workspace).

**On Render** (reusing the same project, replacing the fakeapi service): set the
service's **Root Directory** to `v2/api`, **Build** `go build -o bin/server
./cmd/server`, **Start** `./bin/server`, and add `V2_DATABASE_URL` (the
Supabase pooled connection string using `v2_api` + the password you set). The
product sources ship in the same repo checkout, so `../products` resolves at
runtime.

## What goes here

- A Go module (deployed in the existing backend project — see
  [`../INTEGRATION.md`](../INTEGRATION.md), not a separate Railway service).
- The `/v2/…` routes, implementing [`../contracts`](../contracts).
- Model calls for the assistant, evaluators and avatar. Keys stay here.

## Rules

- Connects to Postgres as the V2 role: full access to the `v2` schema,
  read-only on the V1 tables listed in [PLAN, Part 1 §3](../PLAN.md#3-its-own-data-beside-v1s),
  nothing else.
- Environment variables are prefixed `V2_`. No V1 secret is reused.
- Imports nothing from `candidate/backend` or `company/backend`. Copy a thin
  piece and own it instead.
