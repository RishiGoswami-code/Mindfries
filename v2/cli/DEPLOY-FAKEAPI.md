# Throwaway deployment of the fake API (Render)

This puts `cmd/fakeapi` on the internet so the `mindfries` CLI can be tried
against a real URL. **It isn't the V2 API and isn't for candidates.** It
holds everything in memory: sessions, events and submissions disappear
whenever the service restarts or sleeps. Two demo invitations are seeded on
every start, so signing in always works.

It also hosts dev builds of the CLI for every platform, with the service's
address and the fake's development signing key baked in, plus the two
installers. A tester needs no environment variables, keys or codes.

## Deploy on Render (native Go)

1. In Render: **New → Web Service**, connect the repository, pick the
   branch, and choose **Go** as the language.
2. Settings:

   | Setting | Value |
   |---|---|
   | Root Directory | `v2/cli` |
   | Build Command | the line below |
   | Start Command | `./bin/fakeapi` |
   | Health Check Path | `/healthz` |
   | Instance type | Free is fine |

   Build Command (it builds the fake API, then the CLI for six platforms
   with the service's URL baked in):

   ```bash
   go build -o bin/fakeapi ./cmd/fakeapi && for t in windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do os=${t%/*}; arch=${t#*/}; ext=""; [ "$os" = windows ] && ext=".exe"; GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/mindfries/v2/cli/internal/buildinfo.DefaultAPI=https://mindfries-cli.onrender.com/v2/cli -X github.com/mindfries/v2/cli/internal/buildinfo.TrustDevKey=1" -o "dist/mindfries_${os}_${arch}${ext}" ./cmd/mindfries; done
   ```

   The output is `bin/fakeapi`, not `fakeapi`: `v2/cli/fakeapi` is already a
   folder (the fake's source), and `go build -o fakeapi` would put the
   program inside it. If the service has another address, change the URL in
   the build command.

3. Environment variables:

   | Variable | Value |
   |---|---|
   | `FAKEAPI_DIST` | `dist` (the folder the build command fills) |
   | `FAKEAPI_ADMIN_TOKEN` | a long random string |
   | `FAKEAPI_GITHUB_CLIENT_ID` | the Mindfries OAuth app's Client ID (below); leave it out to keep work local |
   | `FAKEAPI_GITHUB_CLIENT_SECRET` | that app's client secret; keep it only here |
   | `FAKEAPI_GITHUB_OPTIONAL` | `1` to let candidates skip GitHub; by default it's required |

   Render sets `PORT` and `RENDER_EXTERNAL_URL` itself.
4. Deploy. The log prints the addresses:

   ```text
   fake V2 API on https://<service>.onrender.com/v2/cli
   sign-in page: https://<service>.onrender.com/device (a fake browser session; two demo assessments)
   GitHub: the work goes to the candidate's own GitHub (OAuth app Ov23li…); set each assessment's reviewers with PUT /__admin/invitations/<id>/reviewers
   install: irm https://<service>.onrender.com/install.ps1 | iex      (Windows PowerShell)
   install: curl -fsSL https://<service>.onrender.com/install.sh | sh   (macOS, Linux)
   ```

Docker works too: Runtime **Docker**, Root Directory `v2` and Dockerfile Path `cli/Dockerfile.fakeapi` (the build needs `../contracts`),
the same environment variables (`FAKEAPI_DIST` is set by the image), and
`CLI_API` if the address isn't `https://mindfries-cli.onrender.com/v2/cli`.

## The GitHub OAuth app (once)

The candidate's work goes to a private repository in their own GitHub, so
deployment can be assessed. They connect it in one click from the sign-in
page, which needs an OAuth app (an **OAuth App**, at
`github.com/settings/applications/new`, not a GitHub App):

1. On GitHub, under the account or organisation that should own it:
   **Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Application name `Mindfries`, Homepage URL your site.
3. **Authorization callback URL** (Redirect URI):
   `https://mindfries-cli.onrender.com/github/callback`, the fake API's
   address plus `/github/callback`. Leave wildcard matching off. Device
   Flow isn't needed. "Expire user access tokens" can stay on.
4. **Register application**, then **Generate a new client secret**.
5. Put the **Client ID** in `FAKEAPI_GITHUB_CLIENT_ID` and the secret in
   `FAKEAPI_GITHUB_CLIENT_SECRET` on Render. The secret stays on the
   server; candidates never see it.

Candidates see "Authorize Mindfries" with the `repo` scope the first time
only. The token is handed to their CLI once, kept in their OS keychain and
deleted when the session ends.

## Try it as a candidate would

The quickest way: open `https://mindfries-cli.onrender.com/dashboard`,
press **Start assessment** on a demo, and run the command it shows. Setup
downloads the task, connects GitHub (if configured) and pushes; then run
`mindfries start`: pick your editor, let it check the computer, allow the
camera, microphone and screen share in the panel it opens, and the
assessment begins in your editor. The dashboard page follows along.

Or, step by step:

Install once. Windows PowerShell:

```powershell
irm https://mindfries-cli.onrender.com/install.ps1 | iex
```

macOS / Linux:

```bash
curl -fsSL https://mindfries-cli.onrender.com/install.sh | sh
```

Then, in a new terminal, `mindfries start`. It opens the sign-in page; check
the code matches, pick a demo assessment and press **Connect**. If GitHub is
on, the browser goes on to GitHub: press **Authorize** (the first time
only) and it comes back to the page. The terminal creates the repository,
pushes, and opens VS Code. From other terminals: `mindfries agent`, `mindfries status`,
`mindfries submit`, `mindfries stop`, `mindfries doctor`.

## A build from your own checkout

To try local changes against the hosted fake, build and install from the
checkout instead (needs Go):

```powershell
powershell -ExecutionPolicy Bypass -File v2/cli/install/dev-install.ps1
```

```bash
sh v2/cli/install/dev-install.sh
```

Both take another API address as an argument (`-Api <url>` on Windows).
`mindfries --version` shows which API a build talks to.

## Invitations and their reviewers

Reviewers are the hiring team for an assessment, so they're set on each
invitation (in the real product, the company portal does this), never in
the server's environment. Any number of GitHub usernames:

```bash
# A new assessment with its reviewers
curl -X POST -H "Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>" https://<service>.onrender.com/__admin/invitations -d '{"title":"Backend task","duration_minutes":45,"reviewers":["alice","bob","carol"]}'

# The invitations, with their IDs and reviewers (including the two demos)
curl -H "Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>" https://<service>.onrender.com/__admin/invitations

# Replace an assessment's reviewers
curl -X PUT -H "Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>" https://<service>.onrender.com/__admin/invitations/<invitation_id>/reviewers -d '{"reviewers":["alice","dave"]}'
```

A change reaches candidates already working on that assessment: their CLI
invites the new reviewers on its next `mindfries start` and at submit. It
never removes anyone, since the repository is the candidate's. Like
everything else in the fake, invitations are lost when the service
restarts; the two demos come back without reviewers.

The old setup codes still work, for testing `mindfries start <code>`:
`POST /__admin/setup-codes` the same way.

## Things to expect on the free plan

- **Sleep.** The service sleeps after about 15 minutes without requests.
  The first request after that takes up to a minute; `mindfries start` says
  it's still trying and waits. A running session's heartbeats keep the
  service awake.
- **Lost sessions.** A restart loses every session. The CLI then reports
  that the session has ended or that the token isn't recognised; run
  `mindfries stop --yes`, then `mindfries start` to sign in again. The
  GitHub repository stays in the candidate's account.
- **The assistant.** `mindfries agent` talks to the fake API's scripted model
  (read, list, create, replace, run, research), not a real one.
