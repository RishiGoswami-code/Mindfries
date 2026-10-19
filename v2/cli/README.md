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

1. They install the CLI once, with a command the dashboard shows:
   ```powershell
   irm https://<v2-domain>/install.ps1 | iex
   ```
   ```sh
   curl -fsSL https://<v2-domain>/install.sh | sh
   ```
   The installer checks the binary against the release checksum, refuses a
   tampered one, puts `mindfries` in a per-user folder
   (`%LOCALAPPDATA%\Mindfries\bin`, or `~/.local/bin`) and adds it to
   their PATH without admin rights. It says when a new terminal is needed.
2. In any terminal they type `mindfries start`. It shows the rules and what
   it records, asks them to close any coding agent, and opens a Mindfries
   sign-in page in the browser with a short code. They sign in, check the
   page shows the same code, and pick the assessment they were invited to.
   The CLI receives that session's token (kept in the OS keychain), downloads
   and verifies the task, creates the workspace with a locked `TASK.md`,
   opens their editor, and runs the session watcher until the session ends.
   When the assessment uses GitHub, Connect takes them on to GitHub's
   "Authorize Mindfries" page (one click, only the first time) and back,
   and `start` creates a private repository in their account, invites the
   hiring team, and pushes. They deploy from it (AWS, Supabase, Render, Actions) as the task
   asks.
3. They work in their editor (VS Code opens in a new window on
   `TASK.md`), commit and `git push` as usual, with `mindfries agent` in
   another terminal.
4. `mindfries submit` runs the checks, shows the diff, commits and hands in.
   `mindfries stop` ends the session without submitting.

Nothing to configure: the API address is baked into each build
(`buildinfo.DefaultAPI`, set by the release workflow). A dashboard command
with a single-use setup code, `mindfries start <code>`, also still works.

## Commands

| Command | What it does |
|---|---|
| `mindfries start` | Signs in through the browser, sets up the workspace and runs the session watcher. Run it again to carry on after a restart, or to reopen the editor. `--no-browser` prints the sign-in address instead. |
| `mindfries agent` | The coding and research assistant, in a rich terminal interface (plain line mode when piped, or with `MINDFRIES_PLAIN=1`) |
| `mindfries status` | The session, watcher, sync and Git state, or the last submission |
| `mindfries submit` | Runs the task's checks, shows the diff, commits, pushes to the candidate's GitHub if the session uses it, takes the deployed URL (`--url`) when the task asks, and hands in |
| `mindfries stop` | Ends the session without submitting and removes this computer's credentials; the code stays |
| `mindfries doctor` | Preflight checks only; changes nothing |

## Try it locally

One command builds this checkout and installs it on your PATH, the same way
a candidate's install does (needs Go):

```powershell
powershell -ExecutionPolicy Bypass -File v2/cli/install/dev-install.ps1 -Api http://localhost:8787/v2/cli
```

```sh
sh v2/cli/install/dev-install.sh http://localhost:8787/v2/cli
```

Then run the fake API and sign in:

```sh
cd v2/cli && go run ./cmd/fakeapi   # serves the sign-in page; any email works
mindfries start                      # in another terminal
```

Without an argument the dev install talks to the fake API hosted on Render,
which also serves its own installers and builds: see
[DEPLOY-FAKEAPI.md](DEPLOY-FAKEAPI.md). `mindfries --version` shows which API
a dev build talks to.

Build-time settings (`-ldflags -X github.com/mindfries/v2/cli/internal/...`):
`buildinfo.DefaultAPI` is the API address; `buildinfo.TrustDevKey=1` lets a
dev build trust the fake API's signing key from that address, not only from
localhost (release builds ignore it); `trust.ManifestKeys` holds the release
signing keys.

Developer overrides, never needed by a candidate: `--api` or `MINDFRIES_API`
(another API), `MINDFRIES_HOME` (the CLI's own state; default: the user
config folder), `MINDFRIES_TOKEN_STORE=file` (skip the keychain),
`MINDFRIES_TRUST_DEV_KEY=1` (dev builds only), `MINDFRIES_NO_BROWSER=1`.

## Layout

| Path | What's there |
|---|---|
| `cmd/mindfries` | The binary; `main` only calls `internal/cli` |
| `cmd/fakeapi` | The fake V2 API, to run by hand |
| `cmd/mfsign` | Ed25519 keys, signing and checking for releases |
| `fakeapi/fakegithub` | A fake GitHub for tests, with a real `git http-backend` to push to |
| `fakeapi/` | The fake API: browser sign-in with its `/device` page, invitations, setup codes, bundles, events, snapshots, the assistant gateway with a scripted model, submissions, and (with `-dist`) hosted installers and builds. Tests drive it from Go. |
| `internal/cli` | Argument parsing, the six commands, and the hidden `git-credential` helper |
| `internal/github` | GitHub's API with the candidate's token: the user, creating the repo, inviting reviewers |
| `internal/term`, `internal/tui` | Raw terminal mode, keys, the input box, spinner, menus and streamed Markdown for `mindfries agent` |
| `internal/api` | The contract's types and a client, including the assistant's event stream |
| `internal/preflight` | `doctor`'s checks |
| `internal/workspace` | Download, verification, Git setup, locked files |
| `internal/watcher` | The session watcher: offline queue, sending, heartbeat, snapshots, file changes, signals, attribution |
| `internal/signals` | The coding-agent checks: processes, editor extensions, agent files, commit trailers, bursts |
| `internal/assistant` | `mindfries agent`: the chat, the tools and their limits |
| `internal/ledger` | The record of every edit the assistant applies |
| `internal/secret`, `internal/state`, `internal/trust` | Token storage, the local session record, manifest signatures |
| `install/` | The one-line installers, the dev installers, and their tests |
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

Also verified by hand on Windows (2026-10-19): a fake API serving its own
builds, then `irm .../install.ps1 | iex` in a clean PowerShell, then
`mindfries start` with no variables, codes or flags beyond the sandbox's
own folders: the sign-in page showed the same code, Connect let the
terminal carry on, and `status` and `stop` worked by name.

Not yet:
- The hosted fake API on Render still runs the build from before the
  browser sign-in; it has to be redeployed from this branch.
- The installers' PATH changes were checked in the registry and in a fresh
  `HOME`, not in a newly opened Windows Terminal or macOS/Linux shell.
- CI hasn't run on GitHub, because nothing is pushed.
- The macOS and Linux keychain paths are only exercised in CI, with the file
  store.
- `mindfries agent` hasn't been tried by hand in each terminal.
- Code-signing certificates for Windows and macOS don't exist yet.
- The production manifest signing key doesn't exist yet.
- The V2 API's domain is still a placeholder.
