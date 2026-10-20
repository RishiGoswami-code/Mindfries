package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// What the candidate makes in the panel (v2.artifacts, with every saved
// version kept in v2.artifact_versions as evidence) and the task check runs
// the CLI reports (v2.check_runs).

// ErrVersionConflict means the artifact was saved by someone else (another
// tab) since the version the caller read.
var ErrVersionConflict = errors.New("the artifact changed since you loaded it")

// Artifact is one saved thing.
type Artifact struct {
	Kind      string          `json:"kind"`
	Version   int             `json:"version"`
	Body      json.RawMessage `json:"body"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SaveArtifact stores a new version of a session's artifact. expect, when
// non-nil, is the version the caller last saw: a different current version is
// ErrVersionConflict (a second tab must not silently overwrite the first).
// Saving a body identical to the current one changes nothing.
func (d *DB) SaveArtifact(ctx context.Context, sessionID, kind string, body []byte, expect *int, at time.Time) (int, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	cur, same := 0, false
	var curBody []byte
	err = tx.QueryRow(ctx, `select version, body from v2.artifacts where session_id = $1 and kind = $2 for update`, sessionID, kind).Scan(&cur, &curBody)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	if err == nil {
		same = jsonEqual(curBody, body)
	}
	if expect != nil && *expect != cur {
		return cur, ErrVersionConflict
	}
	if same {
		return cur, nil
	}
	next := cur + 1
	if _, err := tx.Exec(ctx,
		`insert into v2.artifacts (session_id, kind, version, body, updated_at) values ($1, $2, $3, $4::jsonb, $5)
		 on conflict (session_id, kind) do update set version = excluded.version, body = excluded.body, updated_at = excluded.updated_at`,
		sessionID, kind, next, string(body), at); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx,
		`insert into v2.artifact_versions (session_id, kind, version, body, saved_at) values ($1, $2, $3, $4::jsonb, $5)`,
		sessionID, kind, next, string(body), at); err != nil {
		return 0, err
	}
	return next, tx.Commit(ctx)
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}

// Artifacts returns the current version of each artifact a session has.
func (d *DB) Artifacts(ctx context.Context, sessionID string) (map[string]Artifact, error) {
	rows, err := d.pool.Query(ctx, `select kind, version, body, updated_at from v2.artifacts where session_id = $1`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Artifact{}
	for rows.Next() {
		var a Artifact
		var body []byte
		if err := rows.Scan(&a.Kind, &a.Version, &body, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Body = json.RawMessage(body)
		out[a.Kind] = a
	}
	return out, rows.Err()
}

// ArtifactVersions lists every saved version of one artifact, oldest first.
func (d *DB) ArtifactVersions(ctx context.Context, sessionID, kind string) ([]Artifact, error) {
	rows, err := d.pool.Query(ctx,
		`select kind, version, body, saved_at from v2.artifact_versions where session_id = $1 and kind = $2 order by version`, sessionID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artifact{}
	for rows.Next() {
		var a Artifact
		var body []byte
		if err := rows.Scan(&a.Kind, &a.Version, &body, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Body = json.RawMessage(body)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CheckRun is one run of the task's checks on the candidate's computer.
type CheckRun struct {
	SessionID  string    `json:"session_id"`
	EventSeq   *int64    `json:"event_seq,omitempty"`
	RanAt      time.Time `json:"ran_at"`
	Command    string    `json:"command"`
	Passed     bool      `json:"passed"`
	Failing    []string  `json:"failing"`
	Passing    []string  `json:"passing"`
	OutputTail string    `json:"output_tail"`
}

// SaveCheckRun stores a run; the same event twice is stored once.
func (d *DB) SaveCheckRun(ctx context.Context, c CheckRun) (bool, error) {
	f, _ := json.Marshal(nonNil(c.Failing))
	p, _ := json.Marshal(nonNil(c.Passing))
	ct, err := d.pool.Exec(ctx,
		`insert into v2.check_runs (session_id, event_seq, ran_at, command, passed, failing, passing, output_tail)
		 values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8)
		 on conflict (session_id, event_seq) where event_seq is not null do nothing`,
		c.SessionID, c.EventSeq, c.RanAt, c.Command, c.Passed, string(f), string(p), c.OutputTail)
	return ct.RowsAffected() == 1, err
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// LatestCheckRun is the session's most recent run (false if none).
func (d *DB) LatestCheckRun(ctx context.Context, sessionID string) (CheckRun, bool, error) {
	var c CheckRun
	var f, p []byte
	err := d.pool.QueryRow(ctx,
		`select session_id, event_seq, ran_at, command, passed, failing, passing, output_tail
		   from v2.check_runs where session_id = $1 order by ran_at desc, id desc limit 1`, sessionID,
	).Scan(&c.SessionID, &c.EventSeq, &c.RanAt, &c.Command, &c.Passed, &f, &p, &c.OutputTail)
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckRun{}, false, nil
	}
	_ = json.Unmarshal(f, &c.Failing)
	_ = json.Unmarshal(p, &c.Passing)
	return c, err == nil, err
}
