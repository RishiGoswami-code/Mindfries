# V2 API: threat model and what is done about it

Who can reach what, what each party could try, and the control that stops it.
Written with the code (B12); a security review should start here and look for
what is missing.

## Who calls this server, and how they prove it

| Caller | Proof | Can do |
|---|---|---|
| The candidate's CLI | A session bearer token (256 random bits; only its SHA-256 is stored) | Their own session: bundle, events, submit, assistant, GitHub token. Nothing else |
| The candidate's browser (the panel) | A panel token: an HMAC over {session, scope, expiry}, 15 minutes | Their own session's panel routes |
| A portal's server | `X-Portal-Secret` | Mint codes, approve sign-ins, read evidence. A browser never holds it |
| Mindfries admin | `X-Admin-Token` | Admin routes and the staff routes |
| Anyone | Nothing | Health, products, install scripts, the GitHub redirect (state-checked) |

With no portal secret or admin token configured, those routes are off, never
open.

## What could go wrong

**Someone uses another candidate's session.** Tokens are random and unique.
A panel token is issued only by the portal's server, after it checks the signed-in
candidate owns the session (`sess.Links.CandidateID`). A leaked database
doesn't give working tokens (only hashes are stored).

**A session token or GitHub token leaks.** Neither is logged or put in an
error. GitHub tokens are sealed with AES-256-GCM (key derived from
`V2_ENCRYPTION_KEY`, bound to the candidate), held only until the CLI collects
them once, then wiped. `git` is given the token through environment variables,
never a URL.

**The panel (a browser) is hostile.** Everything it sends is validated: uploads
by type, size and rate; artifacts by shape and size, and re-serialised so only
known fields survive (`internal/panel`). It can add to the record of its own
session only while the clock runs, and only evidence the assessment collects.

**The AI is told something nasty, or tries something.** In `explain_only` the
model is never offered an editing or running tool, and if it asks anyway the
server refuses and says so. The research tool fetches only the assessment's
listed sites over https, never an address inside our network (checked at
connect time), with every redirect re-checked and a size cap. The avatar's
instructions and model are locked inside its Live token; it is told nothing
about flags, so it can't accuse. Every prompt, reply and tool call is stored.

**Cost runs away.** Per-session token budgets and a daily cap (counting the
avatar's talk by transcript, since Gemini's own counts are in the browser's
hands); six avatar sessions an hour per candidate; one upload per interval.

**An OAuth redirect is abused.** A state is random, single-use and expires in
15 minutes; a return address is honoured only on the candidate portal.

**A flag is taken for a verdict.** Items carry a category (observed, verified,
signal), the wording says what was recorded and never what it means, and a
test fails if the wording accuses. Reviewers answer flags with a fixed set of
decisions and a name.

**Evidence outlives its welcome.** Media is deleted after `V2_RETENTION_DAYS`
(bytes first, then the row, then a log line); old transcripts are blanked;
spent codes are purged. A candidate can export and delete what V2 holds on
them, and the deletion is logged without their identity.

**V2 reaches into V1.** `v2_api` can write only the `v2` schema and read a
short list of V1 tables; a test proves it is refused when it tries to write a
V1 table. The one V1 write (marking an assessment submitted) is a separate
grant, made by the V1 seam, not here.

## Known gaps

- **The grader is not a sandbox.** `cmd/grade` runs a candidate's tests with a
  stripped environment (none of the grader's variables), which stops them
  reading `V2_DATABASE_URL` from their own environment. It does not stop them
  reading it from the grader process itself (on Linux a child of the same user
  can read `/proc/<parent>/environ`), reaching the network, or filling the
  disk, and there is no time limit (by decision). Run it where that is
  harmless: a throwaway container or CI job as a different user from the one
  holding the credential, ideally with no network except the database.

- **Gemini Live token format.** `assistant.LiveTokens.Mint` follows Google's
  documented ephemeral-token API, but could only be tested against a stand-in.
  It needs a smoke test with a real key before the avatar is switched on.
- **Single instance.** The live feed's fan-out (`activity.Hub`) is in
  memory. With two instances a feed would miss items made on the other (it
  catches up from the database on reconnect, but not live). Fine on one
  Render instance; a Postgres `LISTEN` or Redis fan-out is the fix when it
  scales out.
- **Hash-chain checks.** The server stores the CLI's event hash chain but
  doesn't verify it yet.
- **Rate limits are per instance and in memory.**
- **Web Application Firewall and DDoS** are the platform's job (Render).
