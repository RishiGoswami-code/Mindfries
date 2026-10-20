package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/mindfries/v2/api/internal/session"
)

// Read + grade methods backing the admin review surface (GRADE step).

func (d *DB) RecentSessions(ctx context.Context, limit int) ([]session.Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := d.pool.Query(ctx, `select `+sessionCols+` from v2.sessions order by started_at desc limit $1`, limit)
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

func (d *DB) Events(ctx context.Context, sessionID string) ([]session.Event, error) {
	rows, err := d.pool.Query(ctx,
		`select seq, occurred_at, type, data, prev_hash, hash from v2.events where session_id = $1 order by seq`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []session.Event
	for rows.Next() {
		var e session.Event
		var data []byte
		if err := rows.Scan(&e.Seq, &e.OccurredAt, &e.Type, &data, &e.PrevHash, &e.Hash); err != nil {
			return nil, err
		}
		e.Data = data
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) SaveGrade(ctx context.Context, g session.Grade) error {
	check, _ := json.Marshal(g.Check)
	_, err := d.pool.Exec(ctx,
		`insert into v2.grades (submission_id, passed, check_cmd, output, graded_at)
		 values ($1,$2,$3,$4,$5)
		 on conflict (submission_id) do update set
		   passed = excluded.passed, check_cmd = excluded.check_cmd,
		   output = excluded.output, graded_at = excluded.graded_at`,
		g.SubmissionID, g.Passed, check, g.Output, g.GradedAt)
	return err
}

func (d *DB) GradeBySubmission(ctx context.Context, submissionID string) (session.Grade, error) {
	var g session.Grade
	var check []byte
	err := d.pool.QueryRow(ctx,
		`select submission_id, passed, check_cmd, output, graded_at from v2.grades where submission_id = $1`, submissionID).
		Scan(&g.SubmissionID, &g.Passed, &check, &g.Output, &g.GradedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session.Grade{}, session.ErrNotFound
	}
	if err != nil {
		return session.Grade{}, err
	}
	_ = json.Unmarshal(check, &g.Check)
	return g, nil
}

// --- grading-runner support (used by cmd/grade, not the engine handlers) ---

// UngradedSubmissions lists verified submissions that don't have a grade yet.
func (d *DB) UngradedSubmissions(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := d.pool.Query(ctx,
		`select s.id from v2.submissions s
		   left join v2.grades g on g.submission_id = s.id
		  where s.status = 'verified' and g.submission_id is null
		  order by s.created_at
		  limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SubmissionBundle returns the stored git bundle bytes for a submission.
func (d *DB) SubmissionBundle(ctx context.Context, id string) ([]byte, error) {
	var b []byte
	err := d.pool.QueryRow(ctx, `select bundle from v2.submissions where id = $1`, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, session.ErrNotFound
	}
	return b, err
}
