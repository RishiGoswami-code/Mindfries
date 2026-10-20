# `v2/api`: the V2 API

The server the `mindfries` CLI, the candidate portal's floating panel, and the
company and admin portals talk to. It owns V2's data (the `v2` schema), serves
the CLI contract ([`../contracts/cli-api.yaml`](../contracts/cli-api.yaml)),
runs the engine that picks a product and a task for a role and job
description, and keeps the evidence of each session.

What it does, by area (the plan is in [`../BACKEND.md`](../BACKEND.md)):

| Area | What |
|---|---|
| Engine | Select a product for a role + JD, resolve a JD-keyed task, assemble and sign the workspace bundle |
| Entry | Setup codes (the Start assessment command), the install scripts, the browser sign-in (RFC 8628) |
| GitHub | The candidate's own private repo: OAuth, one-time token handoff, the repo record |
| Clock and monitoring | `begin` waits for camera, screen and microphone in the panel; uploads; gaps |
| Tracker | Every notification, flag and activity, from the CLI's events, the panel and the server; SSE feed |
| Panel | Its configuration per candidate, and the diagram, notes, review comments and hypotheses it saves |
| Assistant | `mindfries agent`'s gateway: Gemini, policy-enforced, tools through the CLI, server-side research |
| Avatar | A locked Gemini Live token, and the transcripts kept as evidence |
| Staff | Assessment configuration, sessions, the evidence a reviewer reads, end and extend |
| Privacy and housekeeping | Retention, expiry, a candidate's export and deletion |

Evaluation, the report and the grade runner (`cmd/grade`, `internal/grade`,
`internal/report`, the admin review routes) are separate work and are not
described here.

## Run

```sh
V2_DATABASE_URL=postgres://v2_api:...@.../postgres \
V2_PRODUCTS_ROOT=../products \
go run ./cmd/server          # :8080
```

On Render: Root Directory `v2/api`, Build `bash render-build.sh`, Start
`./bin/server`. The database needs `v2/db/migrations` applied in order
(0001-0025; each is safe to run twice).

## Settings

Everything is read once, by `internal/config`. Settings marked **feature**
turn a feature on; without them its routes answer `not_configured` instead of
failing later.

| Variable | Meaning |
|---|---|
| `V2_DATABASE_URL` | Postgres URL for the `v2_api` role (**required**) |
| `PORT` | Listen port (default 8080) |
| `V2_PRODUCTS_ROOT` | Folder holding the product sources (default `../products`) |
| `V2_POOL_SIZE` | Task variants generated per JD (default 3) |
| `V2_SIGNING_KEY_ID`, `V2_SIGNING_PRIVATE_KEY` | The production bundle signing key (base64 of a 32-byte seed). Without it, the dev key signs and only dev CLIs trust it |
| `V2_PUBLIC_URL` | This API's own address (falls back to `RENDER_EXTERNAL_URL`) |
| `V2_PORTAL_URL` | The candidate portal: where the sign-in page and the panel live |
| `V2_PORTAL_SECRET` | **Portals.** The portals' servers send it as `X-Portal-Secret` on `/v2/portal/*` |
| `V2_ADMIN_TOKEN` | **Admin.** Mindfries ops send it as `X-Admin-Token` on `/v2/admin/*` and the staff routes |
| `V2_ENCRYPTION_KEY` | **Panel and GitHub.** 32 bytes, base64. Seals GitHub tokens and signs panel tokens |
| `V2_ALLOWED_ORIGINS` | Comma-separated portal origins allowed to call `/v2/panel/*` from a browser |
| `V2_GITHUB_CLIENT_ID`, `V2_GITHUB_CLIENT_SECRET` | **GitHub.** The OAuth app. Its callback is `<V2_PUBLIC_URL>/github/callback` |
| `V2_GITHUB_WEB_URL`, `V2_GITHUB_API_URL` | Default github.com / api.github.com (tests point them at a fake) |
| `V2_GEMINI_API_KEY` (or `GEMINI_API_KEY`) | **Assistant and avatar.** Without it the assistant is a scripted stand-in and the avatar is off |
| `V2_GEMINI_MODEL`, `V2_GEMINI_LIVE_MODEL`, `V2_GEMINI_BASE_URL` | The text model, the Live model, the API root |
| `V2_STORAGE_URL`, `V2_STORAGE_KEY`, `V2_STORAGE_BUCKET` | **Evidence uploads.** The Supabase project, its service key, and a *private* bucket (default `v2-evidence`) |
| `V2_DIST_DIR` | A folder of CLI builds this server hosts at `/install.*` and `/cli/latest/*` |
| `V2_CLI_DOWNLOAD_URL` | Or: where the installers fetch builds from (GitHub Releases, say) |
| `V2_MIN_CLI_VERSION`, `V2_LATEST_CLI_VERSION` | Older CLIs get `cli_too_old` |
| `V2_RETENTION_DAYS` | How long evidence media and transcripts are kept (default 90) |
| `V2_MAX_TOKENS_PER_SESSION`, `V2_MAX_TOKENS_PER_DAY` | Model budgets (default 400000 per session, 20,000,000 a day; 0 means no cap) |
| `V2_TRUST_PROXY` | `1` behind a proxy that sets `X-Forwarded-For` / `-Proto` |

## Routes

CLI (bearer session token, `Mindfries-CLI-Version` header), under `/v2/cli`:
`/version`, `/sessions/exchange`, `/sessions/device`, `/sessions/device/token`,
`/session`, `/session/begin`, `/session/monitoring`, `/session/bundle`,
`/session/bundle/archive`, `/session/heartbeat`, `/session/events`,
`/session/snapshots`, `/session/github/token`, `/session/github/repo`,
`/session/assistant/turns` (+ `/tool-results`), `/session/submission`
(+ `/{id}/bundle`), `/session/revoke`.

Portals' servers (`X-Portal-Secret`): `POST /v2/portal/setup-codes`,
`/v2/portal/device/{user_code}` (+ `/approve`, `/deny`),
`/v2/portal/github/connect`, `/v2/portal/github/status`,
`/v2/portal/sessions/{id}/panel-token`, and (with the admin token too) the
staff routes: `/v2/portal/assessments/{id}/config`, `…/sessions`,
`/v2/portal/sessions/{id}/evidence`, `…/media/{evidence_id}`, `…/end`,
`…/extend`, `…/activity/{id}/review`; `/v2/portal/candidates/{id}/export` and
`/delete`.

Admin (`X-Admin-Token`): `POST /v2/admin/setup-codes`,
`GET /v2/admin/products/{slug}/tasks`, and the review routes.

The panel's browser (`Authorization: Bearer <panel token>`, 15 minutes):
`POST /v2/panel/status`, `/upload/{camera|screen|audio}`,
`GET /v2/panel/config`, `/feed` (Server-Sent Events, resumes with
`Last-Event-ID`), `/activity`, `PUT /v2/panel/artifacts/{kind}`,
`POST /v2/panel/avatar/token`, `/avatar/transcript`.

Open: `GET /healthz`, `/v2/products`, `/s/{code}.ps1|.sh`,
`/install.sh|ps1`, `/cli/latest/*`, `/github/start`, `/github/callback`.

## Tests

```sh
go test ./...                       # unit tests; database tests skip
```

Most behaviour is tested against a real Postgres. Point
`V2_TEST_DATABASE_URL` at a disposable database (a superuser URL):

```sh
V2_TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres go test ./...
```

That applies every migration (twice, in a throwaway database, to prove they
re-run cleanly and that `v2_api` can't write V1), and runs the real
`mindfries` binary through setup, start and submit against this server. CI does
the same in the "V2 api against Postgres" job. If a local database's schema has
drifted, run once with `V2_TEST_RESET=1 go test -p 1 ./...`.

## Rules

- Connects to Postgres as the V2 role: full access to `v2`, read-only on the V1
  tables the engine reads, nothing else
  ([`SECURITY.md`](SECURITY.md)).
- Environment variables are prefixed `V2_`. No V1 secret is reused.
- Imports nothing from `candidate/backend` or `company/backend`.
- A change to the CLI contract lands in `contracts/cli-api.yaml`, the fake API
  (`../cli/fakeapi`) and this server together.
