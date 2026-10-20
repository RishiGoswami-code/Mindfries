-- B1.1: sessions get links to the V1 candidate, company and invitation they
-- belong to (soft links: no foreign key into V1; the assessment link is
-- 0016's text assessment_id), and session tokens stop being stored in plain
-- text. Run after 0016.

-- The clock starts at begin; until then started_at/ends_at are provisional.
alter table v2.sessions add column if not exists begun_at timestamptz;

-- Who and what the session is for (V1 ids, uuid, no foreign key: v2_api may
-- only read V1 tables).
alter table v2.sessions add column if not exists candidate_id  uuid;
alter table v2.sessions add column if not exists company_id    uuid;
alter table v2.sessions add column if not exists invitation_id uuid;
create index if not exists sessions_candidate_idx  on v2.sessions (candidate_id);
create index if not exists sessions_assessment_idx on v2.sessions (assessment_id); -- assessment_id is 0016's text column

-- Tokens: store SHA-256(token) only. Existing rows are hashed in place and
-- their plain text removed; the CLI keeps the token it was given.
alter table v2.sessions add column if not exists token_hash text;
update v2.sessions set token_hash = encode(sha256(convert_to(token, 'UTF8')), 'hex')
  where token_hash is null and token is not null;
alter table v2.sessions alter column token drop not null;
update v2.sessions set token = null where token is not null;
create unique index if not exists sessions_token_hash_idx on v2.sessions (token_hash);

-- Setup codes carry the same links, so a code minted for an invitation
-- produces a session for that candidate.
alter table v2.setup_codes add column if not exists candidate_id  uuid;
alter table v2.setup_codes add column if not exists company_id    uuid;
alter table v2.setup_codes add column if not exists invitation_id uuid;
alter table v2.setup_codes add column if not exists session_id    text;

insert into v2.schema_migrations (version) values ('0017_session_security')
  on conflict (version) do nothing;
