package store

import (
	"context"
	"time"

	"github.com/mindfries/v2/api/internal/session"
)

// Retention, clean-up and a candidate's right to their data.

// ExpireSessions ends active sessions whose time has run out and returns them.
// (The watcher's heartbeat also notices; this catches sessions whose computer
// went away.)
func (d *DB) ExpireSessions(ctx context.Context, now time.Time) ([]session.Session, error) {
	rows, err := d.pool.Query(ctx,
		`update v2.sessions set status = 'expired' where status = 'active' and ends_at < $1 returning `+sessionCols, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []session.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ExpireReady ends sessions that were set up but never begun, after a long
// time, so abandoned setups don't linger as "live" (and block a fresh start).
func (d *DB) ExpireReady(ctx context.Context, olderThan time.Time) (int64, error) {
	ct, err := d.pool.Exec(ctx, `update v2.sessions set status = 'expired' where status = 'ready' and created_at < $1`, olderThan)
	return ct.RowsAffected(), err
}

// PurgeStale removes spent short-lived records: setup codes and GitHub round
// trips and device sign-ins that expired before cutoff.
func (d *DB) PurgeStale(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for _, q := range []string{
		`delete from v2.setup_codes where expires_at < $1`,
		`delete from v2.github_oauth_states where expires_at < $1`,
		`delete from v2.device_codes where expires_at < $1`,
	} {
		ct, err := d.pool.Exec(ctx, q, cutoff)
		if err != nil {
			return total, err
		}
		total += ct.RowsAffected()
	}
	return total, nil
}

// LogDeletion records that something was deleted, and why.
func (d *DB) LogDeletion(ctx context.Context, sessionID, what, reason string, count int, at time.Time) error {
	_, err := d.pool.Exec(ctx, `insert into v2.deletion_log (session_id, what, reason, count, at) values (nullif($1, ''), $2, $3, $4, $5)`,
		sessionID, what, reason, count, at)
	return err
}

// DeletionLog lists what was deleted for a session.
type Deletion struct {
	SessionID string    `json:"session_id,omitempty"`
	What      string    `json:"what"`
	Reason    string    `json:"reason"`
	Count     int       `json:"count"`
	At        time.Time `json:"at"`
}

func (d *DB) DeletionsFor(ctx context.Context, sessionID string) ([]Deletion, error) {
	rows, err := d.pool.Query(ctx, `select coalesce(session_id, ''), what, reason, count, at from v2.deletion_log where session_id = $1 order by at`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deletion{}
	for rows.Next() {
		var x Deletion
		if err := rows.Scan(&x.SessionID, &x.What, &x.Reason, &x.Count, &x.At); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// SessionsOfCandidate lists every session a candidate has.
func (d *DB) SessionsOfCandidate(ctx context.Context, candidateID string) ([]session.Session, error) {
	rows, err := d.pool.Query(ctx, `select `+sessionCols+` from v2.sessions where candidate_id = $1::uuid order by created_at`, candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []session.Session{}
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// EvidencePathsOfSession lists every stored file path for a session, deleted or not,
// so none is left behind when the session goes.
func (d *DB) EvidencePathsOfSession(ctx context.Context, sessionID string) ([]string, error) {
	rows, err := d.pool.Query(ctx, `select storage_path from v2.evidence_objects where session_id = $1 and deleted_at is null`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteSession removes a session and everything hung off it (events,
// snapshots, activity, artifacts, transcripts, evidence rows, the submission)
// by cascade.
func (d *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := d.pool.Exec(ctx, `delete from v2.sessions where id = $1`, id)
	return err
}

// DeleteCandidateConnections forgets a candidate's GitHub link and sign-ins.
func (d *DB) DeleteCandidateConnections(ctx context.Context, candidateID string) error {
	for _, q := range []string{
		`delete from v2.github_connections where candidate_id = $1::uuid`,
		`delete from v2.github_oauth_states where candidate_id = $1::uuid`,
		`delete from v2.device_codes where candidate_id = $1::uuid`,
		`delete from v2.setup_codes where candidate_id = $1::uuid`,
	} {
		if _, err := d.pool.Exec(ctx, q, candidateID); err != nil {
			return err
		}
	}
	return nil
}

// TranscriptCutoff blanks assistant and avatar transcripts of sessions that
// ended before cutoff, keeping the rows (so counts and timing survive) but
// not what was said.
func (d *DB) BlankOldTranscripts(ctx context.Context, cutoff time.Time) (int64, error) {
	ct, err := d.pool.Exec(ctx,
		`update v2.assistant_steps s set body = '{"redacted": true}'::jsonb
		   from v2.assistant_turns t join v2.sessions x on x.id = t.session_id
		  where s.turn_id = t.id and x.status in ('submitted', 'expired', 'revoked') and x.ends_at < $1
		    and s.kind in ('transcript_in', 'transcript_out', 'tool_result') and s.body->>'redacted' is null`, cutoff)
	return ct.RowsAffected(), err
}
