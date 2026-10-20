package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/session"
)

func ev(seq int64, min int, typ, data string) session.Event {
	return session.Event{Seq: seq, OccurredAt: time.Date(2026, 1, 1, 12, min, 0, 0, time.UTC), Type: typ, Data: json.RawMessage(data)}
}

// Events shaped exactly like the real captured run (feature-sla-countdown).
func TestBuildFromRealShapedEvents(t *testing.T) {
	events := []session.Event{
		ev(1, 0, "preflight.result", `{"checks":[]}`),
		ev(2, 0, "workspace.ready", `{"resumed":true}`),
		ev(3, 0, "ide.launched", `{"ide":"vscode"}`),
		ev(4, 0, "agent_signal.process", `{"phase":"start","matches":[{"agent":"claude-code","process_name":"claude.exe"}],"closed_by_candidate":false}`),
		ev(6, 0, "agent_signal.extension", `{"ide":"vscode","extensions":["github.copilot-chat"]}`),
		ev(8, 5, "file.changed", `{"path":"console/src/sla.ts","change":"modified","lines_added":6,"lines_removed":3}`),
		ev(9, 5, "change.attributed", `{"path":"console/src/sla.ts","lines":6,"origin":"candidate"}`),
		ev(10, 6, "file.changed", `{"path":"console/src/components/SlaBadge.tsx","lines_added":11,"lines_removed":6}`),
		ev(11, 6, "change.attributed", `{"path":"console/src/components/SlaBadge.tsx","lines":11,"origin":"assistant"}`),
		ev(12, 8, "git.commit", `{"commit":"abc","files_changed":2}`),
	}
	sess := session.Session{DurationMinutes: 90}
	sub := session.Submission{ID: "sub_1", TaskFilesIntact: true}
	cov := manifest.RoleCoverage{Competencies: []string{"React component design"}, CannotAssess: []string{"visual polish"}}
	grade := &session.Grade{Passed: true, Check: []string{"npm", "test"}}

	r := Build(sess, sub, events, cov, grade)

	if !r.Outcome.Graded || !r.Outcome.Passed {
		t.Fatal("outcome should be graded+passed")
	}
	if r.Effort.FilesChanged != 2 || r.Effort.LinesAdded != 17 || r.Effort.Commits != 1 {
		t.Fatalf("effort wrong: %+v", r.Effort)
	}
	if r.Effort.ActiveMinutes != 3 { // first edit min 5 → commit min 8
		t.Fatalf("active minutes = %d, want 3", r.Effort.ActiveMinutes)
	}
	if len(r.AI.AgentsDetected) != 1 || r.AI.AgentsDetected[0] != "claude-code" {
		t.Fatalf("agents = %v", r.AI.AgentsDetected)
	}
	if !r.AI.AgentsLeftRunning {
		t.Fatal("should flag agent left running")
	}
	if r.AI.AgentAttributedLines != 11 || r.AI.CandidateAttributedLines != 6 {
		t.Fatalf("attribution wrong: %+v", r.AI)
	}
	if len(r.AI.AIExtensions) != 1 {
		t.Fatalf("extensions = %v", r.AI.AIExtensions)
	}
	// The AI signals must surface as reviewer flags.
	if len(r.Flags) < 3 {
		t.Fatalf("expected several flags, got %v", r.Flags)
	}
	if len(r.Coverage.Competencies) != 1 {
		t.Fatal("coverage not carried through")
	}
}

func TestUngradedOutcome(t *testing.T) {
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{TaskFilesIntact: true}, nil, manifest.RoleCoverage{}, nil)
	if r.Outcome.Graded {
		t.Fatal("outcome should be ungraded when grade is nil")
	}
}

// The CLI's attribution values are assistant, candidate and unknown. An edit
// the assistant made must not be credited to the candidate, and a change of
// unknown origin must not be credited to anyone.
func TestAttributionUsesTheCLIsOrigins(t *testing.T) {
	events := []session.Event{
		ev(1, 1, "file.changed", `{"path":"a.go","lines_added":10,"lines_removed":0}`),
		ev(2, 1, "change.attributed", `{"path":"a.go","lines":10,"origin":"assistant","edit_id":"e1"}`),
		ev(3, 2, "file.changed", `{"path":"b.go","lines_added":7,"lines_removed":0}`),
		ev(4, 2, "change.attributed", `{"path":"b.go","lines":7,"origin":"candidate"}`),
		ev(5, 3, "file.changed", `{"path":"c.go","lines_added":90,"lines_removed":0}`),
		ev(6, 3, "change.attributed", `{"path":"c.go","lines":90,"origin":"unknown"}`),
	}
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{ID: "sub_1", TaskFilesIntact: true}, events, manifest.RoleCoverage{}, nil)
	if r.AI.AgentAttributedLines != 10 || r.AI.CandidateAttributedLines != 7 || r.AI.UnknownAttributedLines != 90 {
		t.Fatalf("%+v", r.AI)
	}
}

func TestNoSubmissionMeansNoTaskFilesFlag(t *testing.T) {
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{}, nil, manifest.RoleCoverage{}, nil)
	if r.Submitted {
		t.Fatal("nothing was submitted")
	}
	for _, f := range r.Flags {
		if strings.Contains(f, "Locked task files") {
			t.Fatalf("an unsubmitted session must not be flagged for task files: %v", r.Flags)
		}
	}
	// A real submission whose locked files changed is flagged.
	r = Build(session.Session{DurationMinutes: 90}, session.Submission{ID: "sub_1", TaskFilesIntact: false}, nil, manifest.RoleCoverage{}, nil)
	found := false
	for _, f := range r.Flags {
		found = found || strings.Contains(f, "Locked task files")
	}
	if !r.Submitted || !found {
		t.Fatalf("%v", r.Flags)
	}
}
