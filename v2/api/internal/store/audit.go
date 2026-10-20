package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/session"
)

// Consent, the evidence-access audit, and the candidate's words about a flag.

// Consent is a candidate's acceptance of what an assessment collects.
type Consent struct {
	CandidateID  string    `json:"candidate_id"`
	AssessmentID string    `json:"assessment_id"`
	Version      string    `json:"version"`
	Items        []string  `json:"items"`
	AcceptedAt   time.Time `json:"accepted_at"`
}

// SaveConsent records (or re-records) an acceptance.
func (d *DB) SaveConsent(ctx context.Context, c Consent) error {
	items, _ := json.Marshal(nonNil(c.Items))
	_, err := d.pool.Exec(ctx,
		`insert into v2.consents (candidate_id, assessment_id, version, items, accepted_at)
		 values ($1::uuid, $2, $3, $4::jsonb, $5)
		 on conflict (candidate_id, assessment_id, version) do update set items = excluded.items, accepted_at = excluded.accepted_at`,
		c.CandidateID, c.AssessmentID, c.Version, string(items), c.AcceptedAt)
	return err
}

// ConsentFor is the candidate's latest acceptance for an assessment.
func (d *DB) ConsentFor(ctx context.Context, candidateID, assessmentID string) (Consent, bool, error) {
	var c Consent
	var items []byte
	err := d.pool.QueryRow(ctx,
		`select candidate_id::text, assessment_id, version, items, accepted_at from v2.consents
		  where candidate_id = $1::uuid and assessment_id = $2 order by accepted_at desc limit 1`, candidateID, assessmentID,
	).Scan(&c.CandidateID, &c.AssessmentID, &c.Version, &items, &c.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Consent{}, false, nil
	}
	_ = json.Unmarshal(items, &c.Items)
	return c, err == nil, err
}

// Access is one staff read of sensitive evidence.
type Access struct {
	SessionID string    `json:"session_id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail,omitempty"`
	At        time.Time `json:"at"`
}

func (d *DB) LogAccess(ctx context.Context, a Access) error {
	_, err := d.pool.Exec(ctx, `insert into v2.evidence_access (session_id, actor, action, detail, at) values ($1, $2, $3, $4, $5)`,
		a.SessionID, a.Actor, a.Action, a.Detail, a.At)
	return err
}

func (d *DB) AccessLog(ctx context.Context, sessionID string) ([]Access, error) {
	rows, err := d.pool.Query(ctx, `select session_id, actor, action, detail, at from v2.evidence_access where session_id = $1 order by at, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Access{}
	for rows.Next() {
		var a Access
		if err := rows.Scan(&a.SessionID, &a.Actor, &a.Action, &a.Detail, &a.At); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// FlagResponse is what the candidate said about a flag or notice.
type FlagResponse struct {
	ActivityID int64     `json:"activity_id"`
	Text       string    `json:"text"`
	At         time.Time `json:"at"`
}

// RespondToFlag stores the candidate's explanation (one per item; a new one
// replaces it). The item must belong to the session and be a flag or notice.
func (d *DB) RespondToFlag(ctx context.Context, sessionID string, r FlagResponse) error {
	ct, err := d.pool.Exec(ctx,
		`insert into v2.flag_responses (activity_id, text, at)
		 select id, $3, $4 from v2.activity where id = $1 and session_id = $2 and severity in ('flag', 'notice')
		 on conflict (activity_id) do update set text = excluded.text, at = excluded.at`,
		r.ActivityID, sessionID, r.Text, r.At)
	if err == nil && ct.RowsAffected() == 0 {
		return session.ErrNotFound
	}
	return err
}

func (d *DB) FlagResponses(ctx context.Context, sessionID string) ([]FlagResponse, error) {
	rows, err := d.pool.Query(ctx,
		`select f.activity_id, f.text, f.at from v2.flag_responses f join v2.activity a on a.id = f.activity_id
		  where a.session_id = $1 order by f.activity_id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FlagResponse{}
	for rows.Next() {
		var f FlagResponse
		if err := rows.Scan(&f.ActivityID, &f.Text, &f.At); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
