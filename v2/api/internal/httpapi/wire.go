package httpapi

import (
	"context"
	"regexp"
	"strings"

	"github.com/mindfries/v2/api/internal/assistant"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// repoName is the suggested name for the candidate's repository.
func repoName(title string) string {
	n := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if n == "" {
		n = "assessment"
	}
	if len(n) > 40 {
		n = strings.Trim(n[:40], "-")
	}
	return "mindfries-" + n
}

// configFor is the assessment's live configuration (the defaults when the
// session has no assessment or nobody configured it).
func (s *Server) configFor(ctx context.Context, sess session.Session) policy.Config {
	c := policy.Default()
	if s.configs == nil || sess.AssessmentID == "" {
		return c
	}
	if got, _, err := s.configs.AssessmentConfig(ctx, sess.AssessmentID); err == nil {
		c = got
	}
	return c
}

// wireSession is the contract's Session. Policy, reviewers and the monitoring
// the assessment requires are read live, so a reviewer the company adds
// mid-session is seen by the CLI at its next `start` and at submit.
func (s *Server) wireSession(ctx context.Context, sess session.Session, lastAcked int64) map[string]any {
	cfg := s.configFor(ctx, sess)
	out := map[string]any{
		"id":               sess.ID,
		"assessment_id":    assessmentID(sess),
		"assessment_title": sess.AssessmentTitle,
		"status":           sess.Status,
		"duration_minutes": sess.DurationMinutes,
		"started_at":       sess.StartedAt,
		"ends_at":          sess.EndsAt,
		"policy": map[string]any{
			"assistant_mode":         string(cfg.AssistantMode),
			"outside_agents_allowed": false, // always false in V2: outside agents are flagged
			"research_domains":       researchDomains(cfg),
		},
		"intervals": map[string]any{
			"heartbeat_seconds": 30, "snapshot_seconds": 120, "agent_scan_seconds": 180,
		},
		"limits": map[string]any{
			"max_events_per_batch": 500, "max_batch_bytes": 1 << 20,
			"max_snapshot_bytes": 2 << 20, "max_bundle_bytes": int64(50) << 20,
		},
		"last_acked_seq": lastAcked,
		"monitoring": map[string]any{
			"camera": cfg.RequireCamera, "screen": cfg.RequireScreen, "audio": cfg.RequireAudio,
			"panel_url": s.panelURL(sess),
		},
	}
	if s.assist != nil && cfg.AssistantMode != policy.Off {
		total, left := s.budget(ctx, sess)
		out["policy"].(map[string]any)["assistant_budget"] = map[string]any{"tokens_total": total, "tokens_remaining": left}
	}
	if s.cfg.HasGitHub() {
		out["github"] = map[string]any{
			"required":   true,
			"scopes":     []string{"repo"},
			"repo_name":  repoName(sess.AssessmentTitle),
			"reviewers":  cfg.Reviewers,
			"deployment": false,
		}
	}
	if sub, err := s.sessions.SubmissionBySession(ctx, sess.ID); err == nil {
		out["submission_id"] = sub.ID
	}
	return out
}

// assessmentID is the contract's assessment_id: V1's when the session came
// from an invitation, otherwise one derived from the session.
func assessmentID(sess session.Session) string {
	if sess.AssessmentID != "" {
		return sess.AssessmentID
	}
	return "asm_" + sess.ID
}

// panelURL is the page the candidate keeps open: the floating panel, in the
// candidate portal. The page asks this API for a panel token through the
// portal's server, after checking the signed-in candidate owns the session.
func (s *Server) panelURL(sess session.Session) string {
	base := s.cfg.PortalURL
	if base == "" {
		base = s.cfg.PublicURL
	}
	return base + "/v2/session/" + sess.ID
}

// researchDomains is where the assistant may research: the assessment's list,
// or the official documentation sites when it names none.
func researchDomains(c policy.Config) []string {
	if len(c.ResearchDomains) > 0 {
		return c.ResearchDomains
	}
	return assistant.DefaultResearch
}
