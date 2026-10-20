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
		`insert into v2.setup_codes
		   (code, assessment_title, role, jd, duration_minutes, expires_at, used,
		    candidate_id, company_id, assessment_id, invitation_id)
		 values ($1, $2, $3, $4, $5, $6, false,
		   nullif($7, '')::uuid, nullif($8, '')::uuid, nullif($9, ''), nullif($10, '')::uuid)`,
		c.Code, c.AssessmentTitle, c.Role, c.JD, c.DurationMinutes, c.ExpiresAt,
		c.Links.CandidateID, c.Links.CompanyID, c.AssessmentID, c.Links.InvitationID)
	return err
}

func (d *DB) TakeSetupCode(ctx context.Context, code string, now time.Time) (session.SetupCode, error) {
	// Atomically mark unused, unexpired codes used and return them.
	var c session.SetupCode
	err := d.pool.QueryRow(ctx,
		`update v2.setup_codes set used = true
		   where code = $1 and used = false and expires_at > $2
		 returning code, assessment_title, role, jd, duration_minutes, expires_at,
		   coalesce(candidate_id::text, ''), coalesce(company_id::text, ''),
		   coalesce(assessment_id, ''), coalesce(invitation_id::text, '')`,
		code, now,
	).Scan(&c.Code, &c.AssessmentTitle, &c.Role, &c.JD, &c.DurationMinutes, &c.ExpiresAt,
		&c.Links.CandidateID, &c.Links.CompanyID, &c.AssessmentID, &c.Links.InvitationID)
	if errors.Is(err, pgx.ErrNoRows) {
		var used bool
		var expires time.Time
		err := d.pool.QueryRow(ctx, `select used, expires_at from v2.setup_codes where code = $1`, code).Scan(&used, &expires)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return session.SetupCode{}, session.ErrNotFound
		case err != nil:
			return session.SetupCode{}, err
		case used:
			return session.SetupCode{}, session.ErrCodeUnusable
		}
		return session.SetupCode{}, session.ErrCodeExpired
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
		   (id, token_hash, assessment_title, role, jd_hash, product_slug, bank_task_id, task_version, brief,
		    duration_minutes, status, started_at, ends_at,
		    candidate_id, company_id, assessment_id, invitation_id)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		    nullif($14, '')::uuid, nullif($15, '')::uuid, nullif($16, ''), nullif($17, '')::uuid)`,
		s.ID, session.HashToken(s.Token), s.AssessmentTitle, s.Role, s.JDHash, s.ProductSlug, s.BankTaskID,
		s.TaskVersion, s.Brief, s.DurationMinutes, s.Status, s.StartedAt, s.EndsAt,
		s.Links.CandidateID, s.Links.CompanyID, s.AssessmentID, s.Links.InvitationID)
	return err
}

const sessionCols = `id, assessment_title, role, jd_hash, product_slug, bank_task_id,
	task_version, brief, duration_minutes, status, started_at, ends_at, begun_at,
	coalesce(candidate_id::text, ''), coalesce(company_id::text, ''),
	coalesce(assessment_id, ''), coalesce(invitation_id::text, '')`

func scanSession(row pgx.Row) (session.Session, error) {
	var s session.Session
	err := row.Scan(&s.ID, &s.AssessmentTitle, &s.Role, &s.JDHash, &s.ProductSlug, &s.BankTaskID,
		&s.TaskVersion, &s.Brief, &s.DurationMinutes, &s.Status, &s.StartedAt, &s.EndsAt, &s.BegunAt,
		&s.Links.CandidateID, &s.Links.CompanyID, &s.AssessmentID, &s.Links.InvitationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return session.Session{}, session.ErrNotFound
	}
	// Times go out as UTC whatever the server's zone is.
	s.StartedAt, s.EndsAt = s.StartedAt.UTC(), s.EndsAt.UTC()
	if s.BegunAt != nil {
		u := s.BegunAt.UTC()
		s.BegunAt = &u
	}
	return s, err
}

func (d *DB) SessionByToken(ctx context.Context, token string) (session.Session, error) {
	return scanSession(d.pool.QueryRow(ctx,
		`select `+sessionCols+` from v2.sessions where token_hash = $1`, session.HashToken(token)))
}

func (d *DB) SessionByID(ctx context.Context, id string) (session.Session, error) {
	return scanSession(d.pool.QueryRow(ctx, `select `+sessionCols+` from v2.sessions where id = $1`, id))
}

func (d *DB) SetStatus(ctx context.Context, id, status string) error {
	_, err := d.pool.Exec(ctx, `update v2.sessions set status = $2 where id = $1`, id, status)
	return err
}

func (d *DB) Begin(ctx context.Context, id string, startedAt, endsAt time.Time) error {
	_, err := d.pool.Exec(ctx,
		`update v2.sessions set status = 'active', started_at = $2, ends_at = $3, begun_at = $2 where id = $1`,
		id, startedAt, endsAt)
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

func (d *DB) CreateSubmission(ctx context.Context, sub session.Submission) error {
	manifest := string(sub.Manifest)
	if manifest == "" {
		manifest = "{}"
	}
	_, err := d.pool.Exec(ctx,
		`insert into v2.submissions
		   (id, session_id, task_version, branch, base_commit, head_commit, commit_message,
		    bundle_sha256, bundle_bytes, task_files_intact, manifest, status, created_at)
		 values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13)`,
		sub.ID, sub.SessionID, sub.TaskVersion, sub.Branch, sub.BaseCommit, sub.HeadCommit, sub.CommitMessage,
		sub.BundleSHA256, sub.BundleBytes, sub.TaskFilesIntact, manifest, sub.Status, sub.CreatedAt)
	return err
}

func scanSubmission(row pgx.Row) (session.Submission, error) {
	var s session.Submission
	var manifest []byte
	err := row.Scan(&s.ID, &s.SessionID, &s.TaskVersion, &s.Branch, &s.BaseCommit, &s.HeadCommit,
		&s.CommitMessage, &s.BundleSHA256, &s.BundleBytes, &s.TaskFilesIntact, &manifest, &s.Status,
		&s.CreatedAt, &s.VerifiedAt)
	s.Manifest = manifest
	if errors.Is(err, pgx.ErrNoRows) {
		return session.Submission{}, session.ErrNotFound
	}
	return s, err
}

const submissionCols = `id, session_id, task_version, branch, base_commit, head_commit, commit_message,
	bundle_sha256, bundle_bytes, task_files_intact, manifest, status, created_at, verified_at`

func (d *DB) SubmissionBySession(ctx context.Context, sessionID string) (session.Submission, error) {
	return scanSubmission(d.pool.QueryRow(ctx,
		`select `+submissionCols+` from v2.submissions where session_id = $1`, sessionID))
}

func (d *DB) SubmissionByID(ctx context.Context, id string) (session.Submission, error) {
	return scanSubmission(d.pool.QueryRow(ctx,
		`select `+submissionCols+` from v2.submissions where id = $1`, id))
}

func (d *DB) CompleteSubmission(ctx context.Context, id string, bundle []byte, status string, verifiedAt time.Time) error {
	_, err := d.pool.Exec(ctx,
		`update v2.submissions set bundle = $2, status = $3, verified_at = $4 where id = $1`,
		id, bundle, status, verifiedAt)
	return err
}

func (d *DB) LiveSessionFor(ctx context.Context, candidateID, assessmentID string) (session.Session, bool, error) {
	if candidateID == "" || assessmentID == "" {
		return session.Session{}, false, nil
	}
	s, err := scanSession(d.pool.QueryRow(ctx,
		`select `+sessionCols+` from v2.sessions
		  where candidate_id = $1::uuid and assessment_id = $2 and status in ('ready', 'active')
		  order by created_at desc limit 1`, candidateID, assessmentID))
	if errors.Is(err, session.ErrNotFound) {
		return session.Session{}, false, nil
	}
	return s, err == nil, err
}

func (d *DB) RotateToken(ctx context.Context, sessionID, newToken string) error {
	ct, err := d.pool.Exec(ctx, `update v2.sessions set token_hash = $2 where id = $1`, sessionID, session.HashToken(newToken))
	if err == nil && ct.RowsAffected() == 0 {
		return session.ErrNotFound
	}
	return err
}

func (d *DB) SetupCodeLive(ctx context.Context, code string, now time.Time) (bool, error) {
	var live bool
	err := d.pool.QueryRow(ctx,
		`select exists(select 1 from v2.setup_codes where code = $1 and used = false and expires_at > $2)`,
		code, now).Scan(&live)
	return live, err
}

// MarkAssessmentSubmitted flips the linked V1 assessments row to 'submitted'.
// It needs the v2_api role to hold UPDATE on public.assessments: the one V1
// write V2 makes, granted by the status write-back in the V1 seam.
func (d *DB) MarkAssessmentSubmitted(ctx context.Context, assessmentID string) error {
	if assessmentID == "" {
		return nil
	}
	_, err := d.pool.Exec(ctx, `update public.assessments set status = 'submitted' where id = $1`, assessmentID)
	return err
}
