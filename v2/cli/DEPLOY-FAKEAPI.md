# Throwaway deployment of the fake API (Render)

This puts `cmd/fakeapi` on the internet so the `mindfries` CLI can be tried
against a real URL. **It isn't the V2 API and isn't for candidates.** It
holds everything in memory: sessions, events and submissions disappear
whenever the service restarts or sleeps. Two demo invitations are seeded on
every start, so signing in always works.

The image also hosts dev builds of the CLI for every platform, with the
service's address and the fake's development signing key baked in, plus the
two installers. A tester needs no environment variables, keys or codes.

## Deploy on Render

The branch has to be on GitHub first; Render deploys from a branch.

1. In Render: **New → Web Service**, connect the repository, and pick the
   branch.
2. Settings:

   | Setting | Value |
   |---|---|
   | Root Directory | `v2/cli` |
   | Runtime | **Docker** |
   | Dockerfile Path | `./Dockerfile.fakeapi` |
   | Instance type | Free is fine |
   | Health Check Path | `/healthz` |
   | Environment variable | `FAKEAPI_ADMIN_TOKEN` = a long random string |

   Render sets `PORT` and `RENDER_EXTERNAL_URL` itself, and the fake API
   uses both.
3. If the service isn't `https://mindfries-cli.onrender.com`, add an
   environment variable `CLI_API` = `https://<service>.onrender.com/v2/cli`
   (Render hands environment variables to the Docker build as build
   arguments) so the hosted CLI builds talk to it.
4. Deploy. The log prints the addresses:

   ```text
   fake V2 API on https://<service>.onrender.com/v2/cli
   sign-in page: https://<service>.onrender.com/device (any email; two demo assessments)
   install: irm https://<service>.onrender.com/install.ps1 | iex      (Windows PowerShell)
   install: curl -fsSL https://<service>.onrender.com/install.sh | sh   (macOS, Linux)
   ```

Git is in the image because the fake API checks submitted Git bundles with it.

## Try it as a candidate would

Install once. Windows PowerShell:

```powershell
irm https://mindfries-cli.onrender.com/install.ps1 | iex
```

macOS / Linux:

```bash
curl -fsSL https://mindfries-cli.onrender.com/install.sh | sh
```

The installer puts `mindfries` on your PATH (`%LOCALAPPDATA%\Mindfries\bin`
or `~/.local/bin`). Then, in any terminal (a new one, if `mindfries` isn't
found):

```text
mindfries start
```

It opens the fake's sign-in page in your browser and shows a code. Check the
page shows the same code, enter any email, pick a demo assessment and press
**Connect**. The terminal carries on by itself. From other terminals:
`mindfries agent`, `mindfries status`, `mindfries submit`, `mindfries stop`,
`mindfries doctor`.

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
  `mindfries stop --yes`, then `mindfries start` to sign in again.
- **The assistant.** `mindfries agent` talks to the fake API's scripted model
  (read, list, create, replace, run, research), not a real one.
