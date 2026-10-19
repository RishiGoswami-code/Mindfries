# `v2/api` — the V2 API

The server the CLI and `v2/web` talk to: setup codes and session tokens,
task bundles, the event and evidence store, the assistant gateway for
`mindfries agent`, submissions, and later evaluation. See
[PLAN, Stage 2](../PLAN.md#stage-2--the-v2-api-and-connecting-v2-to-v1-6-core-services)
and [SPEC §6](../SPEC.md#core-services).

**Status: empty.** Stage 2 builds it. Until then the CLI uses
`v2/cli/fakeapi`.

## What goes here

- A Go module, deployed as its own Railway service.
- The `/v2/…` routes, implementing [`../contracts`](../contracts).
- Model calls for the assistant, evaluators and avatar. Keys stay here.

## Rules

- Connects to Postgres as the V2 role: full access to the `v2` schema,
  read-only on the V1 tables listed in [PLAN, Part 1 §3](../PLAN.md#3-its-own-data-beside-v1s),
  nothing else.
- Environment variables are prefixed `V2_`. No V1 secret is reused.
- Imports nothing from `candidate/backend` or `company/backend`. Copy a thin
  piece and own it instead.
