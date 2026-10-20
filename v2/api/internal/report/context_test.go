package report

import (
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/session"
)

func TestContextSaysWhatElseWasRecordedWithoutScoring(t *testing.T) {
	start := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	end := start.Add(40 * time.Second)
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{ID: "s", TaskFilesIntact: true}, nil, manifest.RoleCoverage{}, nil)
	r = r.WithContext(Context{
		Gaps:             []Gap{{Source: "screen", Started: start, Ended: &end}, {Source: "panel", Started: start}},
		LatestCheck:      &CheckRun{Command: "go test ./...", Failing: 2, Passing: 41},
		SavedArtifacts:   []string{"diagram", "notes"},
		AssistantTurns:   3,
		AvatarUtterances: 8,
		Repo:             &Repo{FullName: "octo/mindfries-x", URL: "https://github.com/octo/mindfries-x", Reviewers: 2},
	})
	all := strings.Join(r.Context.Notes, " | ")
	for _, want := range []string{
		"The screen wasn't recorded for 40s (a gap in the record)",
		"The monitoring panel wasn't being recorded at the end",
		"41 passed, 2 failed",
		"saved work in the panel: [diagram notes]",
		"Asked the assistant 3 time",
		"Spoke with the avatar (8 line",
		"octo/mindfries-x",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("notes should contain %q:\n%s", want, all)
		}
	}
	// Gaps are context, never flags.
	for _, f := range r.Flags {
		if strings.Contains(strings.ToLower(f), "gap") || strings.Contains(strings.ToLower(f), "screen") {
			t.Errorf("a gap in the record is not a flag: %s", f)
		}
	}
	if len(r.Context.Gaps) != 2 || r.Context.Gaps[1].Ended != nil {
		t.Errorf("%+v", r.Context.Gaps)
	}
}

func TestContextOfNothingIsEmptyNotNull(t *testing.T) {
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{}, nil, manifest.RoleCoverage{}, nil).WithContext(Context{})
	if r.Context == nil || r.Context.Gaps == nil || r.Context.SavedArtifacts == nil || len(r.Context.Notes) != 0 {
		t.Fatalf("%+v", r.Context)
	}
}
