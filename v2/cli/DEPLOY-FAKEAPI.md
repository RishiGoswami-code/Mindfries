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
   | `FAKEAPI_GITHUB_REVIEWERS` | GitHub usernames to invite with read access, comma-separated |
   | `FAKEAPI_GITHUB_OPTIONAL` | `1` to let candidates skip GitHub; by default it's required |

   Render sets `PORT` and `RENDER_EXTERNAL_URL` itself.
4. Deploy. The log prints the addresses:

   ```text
   fake V2 API on https://<service>.onrender.com/v2/cli
   sign-in page: https://<service>.onrender.com/device (a fake browser session; two demo assessments)
   GitHub: the work goes to the candidate's own GitHub (OAuth app Ov23li…; reviewers [hiring-team])
   install: irm https://<service>.onrender.com/install.ps1 | iex      (Windows PowerShell)
   install: curl -fsSL https://<service>.onrender.com/install.sh | sh   (macOS, Linux)
   ```

Docker works too: Runtime **Docker**, Dockerfile Path `./Dockerfile.fakeapi`,
the same environment variables (`FAKEAPI_DIST` is set by the image), and
`CLI_API` if the address isn't `https://mindfries-cli.onrender.com/v2/cli`.

## The GitHub OAuth app (once)

The candidate's work goes to a private repository in their own GitHub, so
deployment can be assessed. The CLI signs them in with GitHub's own device
login, which needs an OAuth app:

1. On GitHub, under the account or organisation that should own it:
   **Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Application name `Mindfries`, Homepage URL your site, Authorization
   callback URL the same (device login doesn't use it).
3. **Tick "Enable Device Flow"**, then **Register application**.
4. Copy the **Client ID** into `FAKEAPI_GITHUB_CLIENT_ID`. No client secret
   is needed or used.

Candidates see "Mindfries wants to access your account" with the `repo`
scope; the token stays in their OS keychain and is deleted when the session
ends.

## Try it as a candidate would

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
on, the same page then shows GitHub's code: press **Open GitHub**, enter it
and authorise. The terminal creates the repository, pushes, and opens VS
Code. From other terminals: `mindfries agent`, `mindfries status`,
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

## More invitations, or setup codes

```bash
curl -X POST -H "Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>" https://<service>.onrender.com/__admin/invitations -d '{"title":"My task","duration_minutes":45}'
```

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
