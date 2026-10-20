-- The v2 schema is internal: only the server-side v2_api role reads it, and it
-- is NOT exposed to Supabase's public API (PostgREST serves `public` only). So
-- Row Level Security adds no protection here — but when it is ON, the v2_api
-- role (a plain login role, not the table owner) sees ZERO rows, and the
-- engine's product registry comes back empty.
--
-- Something enabled RLS after 0002/0003 (the Supabase dashboard's "Enable RLS"
-- prompt or a security advisor). These statements make the intended state
-- explicit and re-runnable.

alter table v2.products      disable row level security;
alter table v2.product_tasks disable row level security;
