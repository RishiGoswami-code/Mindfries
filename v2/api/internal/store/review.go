package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/mindfries/v2/api/internal/session"
)

// Read + grade methods backing the admin review surface (GRADE step).

const sessionCols = `id, token, coalesce(assessment_id, ''), assessment_title, role, jd_hash, product_slug, bank_task_id,
	task_version, brief, duration_minutes, status, started_at, ends_at`

func scanSession(row pgx.Row) (session.Session, error) {
	var s session.Session
	err := row.Scan(&s.ID, &s.Token, &s.AssessmentID, &s.AssessmentTitle, &s.Role, &s.JDHash, &s.ProductSlug, &s.BankTaskID,
		&s.TaskVersion, &s.Brief, &s.DurationMinutes, &s.Status, &s.StartedAt, &s.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session.Session{}, session.ErrNotFound
	}
	return s, err
}

func (d *DB) SessionByID(ctx context.Context, id string) (session.Session, error) {
	return scanSession(d.pool.QueryRow(ctx, `select `+sessionCols+` from v2.sessions where id = $1`, id))
}

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
