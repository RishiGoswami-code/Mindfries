# P1 — Support SaaS (Helpdesk)

One of the owned product codebases candidates are assessed on
(`v2/codebase-engine/ANALYSIS.md`). A believable B2B customer-support desk:
tickets and their conversation, agents and teams, SLA policies, a
knowledge-base suggested-reply feature, webhook delivery, and support
analytics. Candidates work on this real product; each assessment targets one
role's subsystem (`manifest.json`).

## Run it

```sh
go run ./cmd/server          # listens on :8080
PORT=9000 go run ./cmd/server
```

Then: `GET /healthz`, `GET /tickets`, `GET /sla/breaches`, `GET /metrics`,
`GET /tickets/{id}/suggestions`, and the `POST` operations (create, reply,
assign, status, rate). The server seeds realistic sample data on start, so
some tickets are already in SLA breach.

## Test it

```sh
go test ./...        # the suite that defines "correct" for this product
go vet ./...
```

The store is in-memory behind an interface, so the suite needs no database.

## Layout

```
cmd/server/            runnable entry point (seeds data, serves the API)
internal/support/      the domain + application layer (all business rules)
  model.go             tickets, messages, agents, teams, SLA policies, KB
  store.go             Store interface + in-memory implementation
  service.go           the application service (injectable clock)
  tickets.go           create / reply / assign / status / rate
  sla.go               due times, breach detection, the breach queue
  routing.go           load-balanced auto-assignment
  suggest.go           knowledge-base retrieval/ranking (suggested reply)
  metrics.go           support analytics (volume, SLA compliance, CSAT)
  webhooks.go          event delivery with bounded retries + backoff
internal/httpapi/      thin JSON HTTP layer over the domain
internal/seed/         realistic sample catalogue + tickets
manifest.json          role-coverage manifest (which roles, which subsystems)
tasks/                 reference assessment tasks (feature + debug)
```

## What it assesses

See `manifest.json` for the authoritative mapping. In this build the product
assesses **Backend, Full-Stack (server side), FDE, AI-Engineer (the
suggested-reply retrieval), and Data-Analyst (the metrics module)**.

> **Not yet in this build:** a browser console (Next.js). Frontend-role
> coverage for P1 is the next component; until it lands, Frontend candidates
> are served by another product per the coverage matrix. The manifest only
> claims what the code can actually assess (coverage honesty, SPEC §19).
