package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/ghub"
)

// The candidate's GitHub: connections, OAuth round trips, the one-time
// handoff, and the repository each session reports.

func (d *DB) SaveOAuthState(ctx context.Context, st ghub.State) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.github_oauth_states (state, candidate_id, session_id, return_to, expires_at)
		 values ($1, nullif($2, '')::uuid, nullif($3, ''), $4, $5)`,
		st.State, st.CandidateID, st.SessionID, st.ReturnTo, st.ExpiresAt)
	return err
}

// TakeOAuthState spends a state exactly once. It returns pgx.ErrNoRows-like
// ErrStateInvalid when the state is unknown, used or expired.
var ErrStateInvalid = errors.New("github state is unknown, used or expired")

func (d *DB) TakeOAuthState(ctx context.Context, state string, now time.Time) (ghub.State, error) {
	var st ghub.State
	err := d.pool.QueryRow(ctx,
		`update v2.github_oauth_states set used = true
		  where state = $1 and used = false and expires_at > $2
		returning state, coalesce(candidate_id::text, ''), coalesce(session_id, ''), return_to, expires_at`,
		state, now).Scan(&st.State, &st.CandidateID, &st.SessionID, &st.ReturnTo, &st.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ghub.State{}, ErrStateInvalid
	}
	st.Used = true
	return st, err
}

func (d *DB) FinishOAuthState(ctx context.Context, state, result, errText string) error {
	_, err := d.pool.Exec(ctx, `update v2.github_oauth_states set result = $2, error = $3 where state = $1`, state, result, errText)
	return err
}

// LatestOAuthState is the newest round trip started for a session (the CLI's
// connect link), so the token endpoint can say "failed: you didn't authorise".
func (d *DB) LatestOAuthState(ctx context.Context, sessionID string) (ghub.State, bool, error) {
	var st ghub.State
	err := d.pool.QueryRow(ctx,
		`select state, coalesce(candidate_id::text, ''), coalesce(session_id, ''), return_to, expires_at, used, result, error
		   from v2.github_oauth_states where session_id = $1 order by created_at desc limit 1`, sessionID,
	).Scan(&st.State, &st.CandidateID, &st.SessionID, &st.ReturnTo, &st.ExpiresAt, &st.Used, &st.Result, &st.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return ghub.State{}, false, nil
	}
	return st, err == nil, err
}

func (d *DB) SaveConnection(ctx context.Context, c ghub.Connection) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.github_connections (candidate_id, github_login, github_id, token_encrypted, scopes, connected_at, revoked_at)
		 values ($1::uuid, $2, $3, $4, 'repo', $5, null)
		 on conflict (candidate_id) do update set github_login = excluded.github_login, github_id = excluded.github_id,
		   token_encrypted = excluded.token_encrypted, connected_at = excluded.connected_at, revoked_at = null`,
		c.CandidateID, c.Login, c.GitHubID, c.Sealed, c.ConnectedAt)
	return err
}

// ConnectionFor returns the candidate's connection and whether it still
// holds an uncollected token.
func (d *DB) ConnectionFor(ctx context.Context, candidateID string) (ghub.Connection, bool, error) {
	var c ghub.Connection
	err := d.pool.QueryRow(ctx,
		`select candidate_id::text, github_login, github_id, token_encrypted, connected_at
		   from v2.github_connections where candidate_id = $1::uuid and revoked_at is null`, candidateID,
	).Scan(&c.CandidateID, &c.Login, &c.GitHubID, &c.Sealed, &c.ConnectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ghub.Connection{}, false, nil
	}
	return c, err == nil, err
}

// WipeToken forgets the candidate's token once it has been handed over.
func (d *DB) WipeToken(ctx context.Context, candidateID string) error {
	_, err := d.pool.Exec(ctx, `update v2.github_connections set token_encrypted = null where candidate_id = $1::uuid`, candidateID)
	return err
}

// RecordHandoff notes that the session's CLI has the token. It reports false
// when it had already been collected.
func (d *DB) RecordHandoff(ctx context.Context, sessionID string) (bool, error) {
	ct, err := d.pool.Exec(ctx, `insert into v2.github_handoffs (session_id) values ($1) on conflict do nothing`, sessionID)
	return ct.RowsAffected() == 1, err
}

func (d *DB) HandedOff(ctx context.Context, sessionID string) (bool, error) {
	var ok bool
	err := d.pool.QueryRow(ctx, `select exists(select 1 from v2.github_handoffs where session_id = $1)`, sessionID).Scan(&ok)
	return ok, err
}

func (d *DB) SaveRepo(ctx context.Context, r ghub.Repo) error {
	inv, _ := json.Marshal(r.Invited)
	_, err := d.pool.Exec(ctx,
		`insert into v2.session_repos (session_id, owner, name, html_url, private, invited)
		 values ($1, $2, $3, $4, $5, $6::jsonb)
		 on conflict (session_id) do update set owner = excluded.owner, name = excluded.name,
		   html_url = excluded.html_url, private = excluded.private, invited = excluded.invited`,
		r.SessionID, r.Owner, r.Name, r.HTMLURL, r.Private, string(inv))
	return err
}

func (d *DB) RepoFor(ctx context.Context, sessionID string) (ghub.Repo, bool, error) {
	var r ghub.Repo
	var inv []byte
	err := d.pool.QueryRow(ctx,
		`select session_id, owner, name, html_url, private, invited from v2.session_repos where session_id = $1`, sessionID,
	).Scan(&r.SessionID, &r.Owner, &r.Name, &r.HTMLURL, &r.Private, &inv)
	if errors.Is(err, pgx.ErrNoRows) {
		return ghub.Repo{}, false, nil
	}
	_ = json.Unmarshal(inv, &r.Invited)
	return r, err == nil, err
}
