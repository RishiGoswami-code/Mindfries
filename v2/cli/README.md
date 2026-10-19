# `v2/cli` — the `mindfries` CLI

The command the candidate runs on their own laptop. It sets up the
assessment workspace, runs the session watcher, opens `mindfries agent`, and
hands in the work. See [PLAN, Stage 1 and 1b](../PLAN.md#stage-1--the-cli-9-17)
and [SPEC §9](../SPEC.md#9-local-cli-and-assessment-agent-no-ide-extension).

**Status: built against the fake API (tasks 1.2–1.17); not merged yet.** The
real V2 API (Stage 2) doesn't exist yet. Every command talks to it only
through [`../contracts/cli-api.yaml`](../contracts/cli-api.yaml), which
`fakeapi/` serves today.

## The candidate's flow

1. On the dashboard they start a V2 assessment and get one command, unique
   to them and the assessment:
   ```sh
   curl -fsSL https://<v2-domain>/install.sh | sh -s -- K7QM-2XRP-9WDA
   ```
   ```powershell
   powershell -ExecutionPolicy Bypass -c "& ([scriptblock]::Create((irm https://<v2-domain>/install.ps1))) K7QM-2XRP-9WDA"
   ```
   The installer checks the binary against the release checksum, refuses a
   tampered one, and runs `mindfries start <code>`.
2. `start` shows the rules and what it records, asks them to close any
   coding agent, trades the single-use code for a session token (kept in the
   OS keychain), downloads and verifies the task, creates the workspace with
   a locked `TASK.md`, opens their editor, and runs the session watcher until
   the session ends.
3. They work in their editor, with `mindfries agent` in another terminal.
4. `mindfries submit` runs the checks, shows the diff, commits and hands in.
   `mindfries stop` ends the session without submitting.

## Commands

| Command | What it does |
|---|---|
| `mindfries start <setup-code>` | Sets up the workspace and runs the session watcher. Run it again without a code to carry on after a restart, or to reopen the editor. |
| `mindfries agent` | The coding and research assistant, in the terminal |
| `mindfries status` | The session, watcher, sync and Git state, or the last submission |
| `mindfries submit` | Runs the task's checks, shows the diff, commits and hands in |
| `mindfries stop` | Ends the session without submitting and removes this computer's credentials; the code stays |
| `mindfries doctor` | Preflight checks only; changes nothing |

## Try it locally

```sh
cd v2/cli
go run ./cmd/fakeapi            # prints a setup code
go run ./cmd/mindfries start <code> --api http://localhost:8787/v2/cli
```

`MINDFRIES_HOME` moves the CLI's own state (default: the user config
folder), and `MINDFRIES_TOKEN_STORE=file` skips the keychain.

To try it against a URL on the internet, deploy the fake API as a
throwaway service: see [DEPLOY-FAKEAPI.md](DEPLOY-FAKEAPI.md).

## Layout

| Path | What's there |
|---|---|
| `cmd/mindfries` | The binary; `main` only calls `internal/cli` |
| `cmd/fakeapi` | The fake V2 API, to run by hand |
| `cmd/mfsign` | Ed25519 keys, signing and checking for releases |
| `fakeapi/` | The fake API: setup codes, bundles, events, snapshots, the assistant gateway with a scripted model, submissions. Tests drive it from Go. |
| `internal/cli` | Argument parsing and the six commands |
| `internal/api` | The contract's types and a client, including the assistant's event stream |
| `internal/preflight` | `doctor`'s checks |
| `internal/workspace` | Download, verification, Git setup, locked files |
| `internal/watcher` | The session watcher: offline queue, sending, heartbeat, snapshots, file changes, signals, attribution |
| `internal/signals` | The coding-agent checks: processes, editor extensions, agent files, commit trailers, bursts |
| `internal/assistant` | `mindfries agent`: the chat, the tools and their limits |
| `internal/ledger` | The record of every edit the assistant applies |
| `internal/secret`, `internal/state`, `internal/trust` | Token storage, the local session record, manifest signatures |
| `install/` | The one-line installers and their tests |
| `e2e/` | The real binary through whole sessions against the fake API |

## What it records, and what it doesn't

Inside the workspace: Git state, file changes (sizes and line counts),
diff snapshots against the baseline, and which changes came from the
assistant. Outside it: only the names of running programs that match known
coding agents, and the AI extensions installed in the editor. No
keystrokes, no screen, no other files. Everything it finds is a review
signal for a person, never a verdict, and a normal CLI can't see a second
device ([SPEC §9](../SPEC.md#what-the-cli-cannot-promise)).

## Rules

- Talks to the server only through the contract in `v2/contracts`.
- Imports nothing from V1 (`candidate/`, `company/`, `internal-admin/`).
- Never holds a model key; `mindfries agent` calls the server.
- Runs in the foreground only: never a service, never at login.
- Standard library only.

## Verified, and not yet

Verified: `go test ./...` passes on Windows. That covers every task's
"done when", the failure table, both installers, and the end-to-end runs of
the real binary. The Windows Credential Manager was checked for real
(`MINDFRIES_TEST_KEYCHAIN=1`). CI runs the same tests on Linux, macOS and
Windows.

Not yet:
- CI hasn't run on GitHub, because nothing is pushed.
- The macOS and Linux keychain paths are only exercised in CI, with the file
  store.
- `mindfries agent` hasn't been tried by hand in each terminal.
- Code-signing certificates for Windows and macOS don't exist yet.
- The production manifest signing key doesn't exist yet.
- The V2 API's domain is still a placeholder.
