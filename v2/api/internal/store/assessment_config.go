package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/mindfries/v2/api/internal/policy"
)

// Per-assessment configuration, in v2.assessment_config.

// AssessmentConfig returns an assessment's settings, or the defaults (and
// false) when nobody has configured it.
func (d *DB) AssessmentConfig(ctx context.Context, assessmentID string) (policy.Config, bool, error) {
	c := policy.Default()
	c.AssessmentID = assessmentID
	if assessmentID == "" {
		return c, false, nil
	}
	var reviewers, tabs []byte
	var mode string
	err := d.pool.QueryRow(ctx,
		`select duration_minutes, assistant_mode, outside_agents_allowed, reviewers,
		        require_camera, require_screen, require_audio, avatar_enabled, tabs_override, jd
		   from v2.assessment_config where assessment_id = $1::uuid`, assessmentID).Scan(
		&c.DurationMinutes, &mode, &c.OutsideAgentsAllowed, &reviewers,
		&c.RequireCamera, &c.RequireScreen, &c.RequireAudio, &c.AvatarEnabled, &tabs, &c.JD)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	c.AssistantMode = policy.AssistantMode(mode)
	c.Reviewers = []string{}
	_ = json.Unmarshal(reviewers, &c.Reviewers)
	if len(tabs) > 0 {
		_ = json.Unmarshal(tabs, &c.TabsOverride)
	}
	return c, true, nil
}

// SaveAssessmentConfig upserts an assessment's settings. The caller has
// already checked c.Problem().
func (d *DB) SaveAssessmentConfig(ctx context.Context, c policy.Config, by string) error {
	reviewers, _ := policy.NormaliseReviewers(c.Reviewers)
	rj, _ := json.Marshal(reviewers)
	var tabs any
	if c.TabsOverride != nil {
		b, _ := json.Marshal(c.TabsOverride)
		tabs = string(b)
	}
	_, err := d.pool.Exec(ctx,
		`insert into v2.assessment_config
		   (assessment_id, duration_minutes, assistant_mode, outside_agents_allowed, reviewers,
		    require_camera, require_screen, require_audio, avatar_enabled, tabs_override, jd, updated_by)
		 values ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10::jsonb, $11, $12)
		 on conflict (assessment_id) do update set
		   duration_minutes = excluded.duration_minutes, assistant_mode = excluded.assistant_mode,
		   outside_agents_allowed = excluded.outside_agents_allowed, reviewers = excluded.reviewers,
		   require_camera = excluded.require_camera, require_screen = excluded.require_screen,
		   require_audio = excluded.require_audio, avatar_enabled = excluded.avatar_enabled,
		   tabs_override = excluded.tabs_override, jd = excluded.jd,
		   updated_at = now(), updated_by = excluded.updated_by`,
		c.AssessmentID, c.DurationMinutes, string(c.AssistantMode), c.OutsideAgentsAllowed, string(rj),
		c.RequireCamera, c.RequireScreen, c.RequireAudio, c.AvatarEnabled, tabs, c.JD, by)
	return err
}
