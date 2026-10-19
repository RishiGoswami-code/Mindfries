# Throwaway deployment of the fake API (Render)

This puts `cmd/fakeapi` on the internet so the `mindfries` CLI can be tried
against a real URL. **It isn't the V2 API and isn't for candidates.** It
holds everything in memory: sessions, events and submissions disappear
whenever the service restarts or sleeps.

## Deploy on Render

The branch has to be on GitHub first; Render deploys from a branch.

1. In Render: **New → Web Service**, connect the repository, and pick the
   branch (`feat/v2-development`).
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
3. Deploy. The service's log prints the base URL and a first setup code:

   ```text
   fake V2 API on https://<service>.onrender.com/v2/cli
   setup code: ABCD-EFGH-JKLM
   ```

Git is in the image because the fake API checks submitted Git bundles with it.

## Get more setup codes

Each code works once and lasts 10 minutes:

```bash
curl -X POST -H "Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>" https://<service>.onrender.com/__admin/setup-codes
```

## Run the CLI against it

Build a dev binary from `v2/cli`:

```bash
go build -o mindfries.exe ./cmd/mindfries
```

The fake API signs tasks with a development key. The CLI trusts that key
only from `localhost`, unless a **dev build** is told otherwise with
`MINDFRIES_TRUST_DEV_KEY=1`. Release builds ignore the variable.

PowerShell:

```powershell
$env:MINDFRIES_TRUST_DEV_KEY = '1'
$env:MINDFRIES_API = 'https://<service>.onrender.com/v2/cli'
.\mindfries.exe start ABCD-EFGH-JKLM
```

macOS / Linux:

```bash
MINDFRIES_TRUST_DEV_KEY=1 MINDFRIES_API=https://<service>.onrender.com/v2/cli ./mindfries start ABCD-EFGH-JKLM
```

Then, from other terminals with the same two variables set, run
`mindfries status`, `mindfries agent`, `mindfries submit` or `mindfries stop`.

Without `MINDFRIES_TRUST_DEV_KEY=1`, start connects and then refuses the
task ("signed with the development key"). That's the protection working,
not a bug. Running `mindfries stop --yes` clears the half-started session.

## Things to expect on the free plan

- **Sleep.** The service sleeps after about 15 minutes without requests.
  The first request after that takes up to a minute, and everything in
  memory is gone, including any active session. Start a fresh one.
- **Lost sessions.** The CLI then reports that the session has ended or that
  the token isn't recognised; `mindfries stop --yes` tidies up locally.
- **The assistant.** `mindfries agent` talks to the fake API's scripted model
  (read, list, create, replace, run, research), not a real one.
