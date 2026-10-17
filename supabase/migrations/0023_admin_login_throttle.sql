-- Mindfries — counting failed admin sign-ins by network, and failed root
-- admin sign-ins from anywhere (issue #66).
--
-- Apply after 0022_invite_only_assessments.sql.
--
-- candidate_auth_attempts (0020) already counts the candidate site's public
-- forms by a keyed hash of the caller's address. The admin portal's sign-in
-- is counted in the same table under two more kinds:
--   admin_login  a failed admin sign-in, by network — past the limit that
--                network is refused for a while, whichever account it tries
--   root_login   a failed sign-in as the root admin (ROOT_ADMIN_EMAIL), from
--                any network. The root account can't be locked — it is how
--                the operator gets back in — so these are counted to slow
--                guessing down and to raise an alert, not to lock it.

do $$
begin
  if exists (select 1 from pg_constraint where conname = 'candidate_auth_attempts_kind_check') then
    alter table candidate_auth_attempts drop constraint candidate_auth_attempts_kind_check;
  end if;
  alter table candidate_auth_attempts add constraint candidate_auth_attempts_kind_check
    check (kind in ('login', 'signup', 'reset', 'admin_login', 'root_login'));
end $$;
