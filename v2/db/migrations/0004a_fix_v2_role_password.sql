-- FIX: 0004 was run without replacing the <V2_PASSWORD> placeholder, so the
-- v2_api role currently has the literal password '<V2_PASSWORD>' — insecure and
-- awkward (it contains angle brackets). Set a real secret before go-live.
--
-- Replace the value below with a strong secret, run it, and use that same
-- secret as the password in V2_DATABASE_URL for the deployed v2/api.

alter role v2_api with password 'REPLACE_WITH_A_STRONG_SECRET';

-- Verify the role exists and has the grants from 0004:
--   select rolname, rolcanlogin from pg_roles where rolname = 'v2_api';
--   select table_schema, table_name, privilege_type
--     from information_schema.role_table_grants where grantee = 'v2_api'
--     order by table_schema, table_name;
