# V2 setup & deploy runbook

Everything you do on the cloud consoles to stand V2 up for a real
candidate-evaluation launch. Your Supabase/Vercel/Railway accounts aren't
connected yet, so this is written as console steps you run by hand. Each step
says what to create and which value flows where.

Three services only: **Supabase** (database + file storage), **Railway** (the
Go API, `v2/api`), **Vercel** (the web app, `v2/web`). Plus one **model
provider** key for task generation and the assistant.

Status tags: ✅ = do it now (code/migrations ready) · 🟡 = do it when that
component lands (I'll flag it in the build).

---

## 0. Accounts you need

| Service | Why | Plan |
|---|---|---|
| Supabase | Postgres (`v2` schema) + Storage (product source bundles) | Free tier is fine to start |
| Railway | Hosts `v2/api` (Go) | Hobby plan |
| Vercel | Hosts `v2/web` (candidate screens) | Hobby/Pro |
| Model provider | Task generation + `mindfries agent` (server-side calls) | Gemini API key *or* OpenRouter key — same as V1 uses |

---

## 1. Supabase ✅ (do this first — migrations are ready)

1. **Create a project.** supabase.com → New project. Pick a region close to
   your candidates. Save the database password.
2. **Run the migrations.** Project → SQL Editor → paste and run, in order:
   - `v2/db/migrations/0001_v2_schema.sql`
   - `v2/db/migrations/0002_products.sql`
   - `v2/db/migrations/0003_product_tasks.sql`
   Confirm with `select * from v2.schema_migrations;` — you should see three rows.
3. **Create the storage bucket** for product source bundles:
   Storage → New bucket → name `products` → **Private**. (Product tarballs the
   API serves to the CLI live here.)
4. **Create the V2 API database role.** SQL Editor:
   - Open `v2/db/migrations/0004_v2_role_grants.sql`, replace `<V2_PASSWORD>`
     with a strong secret, run it. This role can do everything in `v2` and only
     **read** the five V1 tables — the DB-level guarantee a V2 bug can't touch
     V1 data.
5. **Collect these values** (Project → Settings): you'll paste them into
   Railway in step 3.
   - `V2_DATABASE_URL` — the connection string **for the `v2_api` role** you
     just made (Settings → Database → Connection string, swap in user
     `v2_api` and its password). Use the **pooled** connection string.
   - `V2_SUPABASE_URL` — Project URL (Settings → API).
   - `V2_SUPABASE_SERVICE_ROLE_KEY` — service role key (Settings → API) — used
     only for Storage uploads/downloads, kept on the API, never shipped.

> V1 note: this is a *new* Supabase project, separate from V1's. V2 reuses V1
> accounts later through the signed handoff, not by sharing a database now.
> (If you'd rather put the `v2` schema inside V1's existing Supabase project —
> the original plan — tell me and I'll adjust; it changes nothing above except
> which project you run the migrations in.)

---

## 2. Model provider ✅

Get **one** of:
- A **Gemini API key** (ai.google.dev) → env `V2_GEMINI_API_KEY`, or
- An **OpenRouter key** (openrouter.ai) → env `V2_OPENROUTER_API_KEY`.

The API calls the model **server-side only** (SPEC): no key ever reaches a
candidate's machine. Set a spend limit on the provider dashboard now — task
generation is the main cost and this caps it.

---

## 3. Railway — the API 🟡 (when `v2/api` lands)

1. New project → Deploy from GitHub repo → point at this repo, **root
   directory `v2/api`**.
2. Set environment variables (all prefixed `V2_`):
   - `V2_DATABASE_URL`, `V2_SUPABASE_URL`, `V2_SUPABASE_SERVICE_ROLE_KEY` (from
     step 1.5)
   - `V2_GEMINI_API_KEY` **or** `V2_OPENROUTER_API_KEY` (from step 2)
   - `V2_PRODUCTS_BUCKET=products`
   - `V2_PUBLIC_URL` — Railway gives you a public domain after first deploy;
     set it to that (the CLI builds bake this in so candidates set nothing).
3. Deploy. Note the public URL — it goes into the CLI build (step 5) and the
   web app (step 4).

---

## 4. Vercel — the web app 🟡 (when `v2/web` lands)

1. New project → import this repo → **root directory `v2/web`**.
2. Environment variables:
   - `NEXT_PUBLIC_V2_API_URL` — the Railway public URL from step 3.
   - (The sign-in handoff secret and any others get added when the handoff
     lands; I'll flag it.)
3. Deploy. This is where candidates land from their invite link.

---

## 5. CLI distribution 🟡 (already built; needs your API URL baked in)

The CLI (`v2/cli`) is built. For a real launch:
1. Set the build's API address to your Railway `V2_PUBLIC_URL` (it's baked in,
   so candidates set no env vars).
2. Produce signed release builds for Windows/macOS/Linux and host them (GitHub
   Releases on this repo is fine) behind the one-line installer. See
   `v2/PLAN.md` tasks 1.12–1.13; I'll wire the release step when we get there.

---

## What's live after each step

- After **step 1**: the database is ready; I can seed products and the engine
  can read/write `v2.products` / `v2.product_tasks`.
- After **steps 3–4**: candidates can be invited and work end-to-end.
- After **step 5**: candidates install the CLI with one command and start.

I'll tell you the moment each 🟡 becomes ✅ as the code lands.
