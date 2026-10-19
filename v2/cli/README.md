# `v2/cli` — the `mindfries` CLI

The command the candidate runs on their own laptop. It sets up the
assessment workspace, runs the session watcher, opens `mindfries agent`, and
hands in the work. See [PLAN, Stage 1 and 1b](../PLAN.md#stage-1--the-cli-9-17)
and [SPEC §9](../SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension).

**Status: empty.** The Go module arrives with task 1.2.

## What goes here

- A Go module, built as a single static binary for Windows, macOS and Linux
  (amd64 and arm64).
- The commands: `start`, `agent`, `status`, `submit`, `stop`, `doctor`.
- `fakeapi/`: a scriptable local server that serves
  [`../contracts/cli-api.yaml`](../contracts), so the CLI can be built and
  tested before `v2/api` exists.
- The installer script that the dashboard's setup command runs (task 1.12).

## Rules

- Talks to the server only through the contract in `v2/contracts`.
- Imports nothing from V1 (`candidate/`, `company/`, `internal-admin/`).
- Never holds a model key; `mindfries agent` calls the server.
- Runs in the foreground only: never a service, never at login.
- Reads only what [PLAN task 1.8](../PLAN.md#stage-1--the-cli-9-17) lists.
  Nothing outside the workspace beyond process names and IDE extension lists.
