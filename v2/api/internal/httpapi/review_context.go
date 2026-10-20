package httpapi

import (
	"context"

	"github.com/mindfries/v2/api/internal/report"
	"github.com/mindfries/v2/api/internal/session"
)

// reportContext gathers what a session recorded beyond the CLI's events (the
// panel's gaps, checks, saved work, how the assistant and avatar were used,
// the repository) for the evidence report. Whatever a store can't give is
// simply left out.
func (s *Server) reportContext(ctx context.Context, sess session.Session) report.Context {
	var c report.Context
	if s.monitor != nil {
		if gaps, err := s.monitor.Gaps(ctx, sess.ID); err == nil {
			for _, g := range gaps {
				c.Gaps = append(c.Gaps, report.Gap{Source: g.Source, Started: g.StartedAt, Ended: g.EndedAt})
			}
		}
	}
	if s.panelStore != nil {
		if run, ok, _ := s.panelStore.LatestCheckRun(ctx, sess.ID); ok {
			c.LatestCheck = &report.CheckRun{Command: run.Command, Passed: run.Passed, Failing: len(run.Failing), Passing: len(run.Passing)}
		}
		if arts, err := s.panelStore.Artifacts(ctx, sess.ID); err == nil {
			for _, kind := range []string{"diagram", "notes", "review", "hypotheses"} {
				if _, ok := arts[kind]; ok {
					c.SavedArtifacts = append(c.SavedArtifacts, kind)
				}
			}
		}
	}
	if s.staff != nil {
		if rows, err := s.staff.SessionTranscript(ctx, sess.ID); err == nil {
			for _, row := range rows {
				switch {
				case row.Channel == "agent" && row.Kind == "transcript_in":
					c.AssistantTurns++
				case row.Channel == "avatar":
					c.AvatarUtterances++
				}
			}
		}
	}
	if s.github != nil {
		if repo, ok, _ := s.github.RepoFor(ctx, sess.ID); ok {
			c.Repo = &report.Repo{FullName: repo.Owner + "/" + repo.Name, URL: repo.HTMLURL, Reviewers: len(repo.Invited)}
		}
	}
	return c
}
