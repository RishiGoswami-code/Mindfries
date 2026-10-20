package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/session"
)

// DB implements session.Store against v2.setup_codes / v2.sessions.

func (d *DB) CreateSetupCode(ctx context.Context, c session.SetupCode) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.setup_codes (code, assessment_title, role, jd, duration_minutes, expires_at, used)
		 values ($1, $2, $3, $4, $5, $6, false)`,
		c.Code, c.AssessmentTitle, c.Role, c.JD, c.DurationMinutes, c.ExpiresAt)
	return err
}

func (d *DB) TakeSetupCode(ctx context.Context, code string, now time.Time) (session.SetupCode, error) {
	// Atomically mark unused, unexpired codes used and return them.
	var c session.SetupCode
	err := d.pool.QueryRow(ctx,
		`update v2.setup_codes set used = true
		   where code = $1 and used = false and expires_at > $2
		 returning code, assessment_title, role, jd, duration_minutes, expires_at`,
		code, now,
	).Scan(&c.Code, &c.AssessmentTitle, &c.Role, &c.JD, &c.DurationMinutes, &c.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		_ = d.pool.QueryRow(ctx, `select exists(select 1 from v2.setup_codes where code = $1)`, code).Scan(&exists)
		if exists {
			return session.SetupCode{}, session.ErrCodeUnusable
		}
		return session.SetupCode{}, session.ErrNotFound
	}
	if err != nil {
		return session.SetupCode{}, err
	}
	c.Used = true
	return c, nil
}

func (d *DB) CreateSession(ctx context.Context, s session.Session) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.sessions
		   (id, token, assessment_title, role, jd_hash, product_slug, bank_task_id, task_version, brief, status, started_at, ends_at)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		s.ID, s.Token, s.AssessmentTitle, s.Role, s.JDHash, s.ProductSlug, s.BankTaskID,
		s.TaskVersion, s.Brief, s.Status, s.StartedAt, s.EndsAt)
	return err
}

func (d *DB) SessionByToken(ctx context.Context, token string) (session.Session, error) {
	var s session.Session
	err := d.pool.QueryRow(ctx,
		`select id, token, assessment_title, role, jd_hash, product_slug, bank_task_id,
		        task_version, brief, status, started_at, ends_at
		   from v2.sessions where token = $1`, token,
	).Scan(&s.ID, &s.Token, &s.AssessmentTitle, &s.Role, &s.JDHash, &s.ProductSlug, &s.BankTaskID,
		&s.TaskVersion, &s.Brief, &s.Status, &s.StartedAt, &s.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return session.Session{}, session.ErrNotFound
	}
	return s, err
}

func (d *DB) SetStatus(ctx context.Context, id, status string) error {
	_, err := d.pool.Exec(ctx, `update v2.sessions set status = $2 where id = $1`, id, status)
	return err
}

func (d *DB) AppendEvents(ctx context.Context, sessionID string, events []session.Event) (int64, int, int, error) {
	stored := 0
	for _, e := range events {
		data := string(e.Data)
		if data == "" {
			data = "null"
		}
		ct, err := d.pool.Exec(ctx,
			`insert into v2.events (session_id, seq, occurred_at, type, data, prev_hash, hash)
			 values ($1, $2, $3, $4, $5::jsonb, $6, $7)
			 on conflict (session_id, seq) do nothing`,
			sessionID, e.Seq, e.OccurredAt, e.Type, data, e.PrevHash, e.Hash)
		if err != nil {
			return 0, 0, 0, err
		}
		if ct.RowsAffected() == 1 {
			stored++
		}
	}
	acked, err := d.LastAckedSeq(ctx, sessionID)
	if err != nil {
		return 0, 0, 0, err
	}
	return acked, stored, len(events) - stored, nil
}

func (d *DB) LastAckedSeq(ctx context.Context, sessionID string) (int64, error) {
	var acked int64
	err := d.pool.QueryRow(ctx,
		`select coalesce(max(seq), 0) from v2.events where session_id = $1`, sessionID).Scan(&acked)
	return acked, err
}

func (d *DB) SaveSnapshot(ctx context.Context, sessionID string, s session.Snapshot) error {
	_, err := d.pool.Exec(ctx,
		`insert into v2.snapshots (session_id, seq, taken_at, base_commit, head_commit, patch, patch_sha256)
		 values ($1, $2, $3, $4, $5, $6, $7)
		 on conflict (session_id, seq) do nothing`,
		sessionID, s.Seq, s.TakenAt, s.BaseCommit, s.HeadCommit, s.Patch, s.PatchSHA256)
	return err
}
