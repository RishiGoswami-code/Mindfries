package report

import (
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/session"
)

func TestFlagsStateWhatWasRecordedAndNeverAccuse(t *testing.T) {
	events := []session.Event{
		ev(1, 0, "agent_signal.process", `{"phase":"during","matches":[{"agent":"claude-code","process_name":"claude.exe"}]}`),
		ev(2, 0, "agent_signal.extension", `{"ide":"vscode","extensions":["github.copilot"]}`),
		ev(3, 1, "file.changed", `{"path":"a.go","lines_added":120,"lines_removed":0}`),
		ev(4, 1, "change.attributed", `{"path":"a.go","lines":120,"origin":"unknown"}`),
		ev(5, 2, "file.changed", `{"path":"b.go","lines_added":9,"lines_removed":0}`),
		ev(6, 2, "change.attributed", `{"path":"b.go","lines":9,"origin":"assistant"}`),
	}
	r := Build(session.Session{DurationMinutes: 90}, session.Submission{ID: "s", TaskFilesIntact: false}, events, manifest.RoleCoverage{}, nil)
	all := strings.ToLower(strings.Join(r.Flags, " | "))
	for _, want := range []string{"claude-code", "github.copilot", "120 changed line", "locked task files were changed"} {
		if !strings.Contains(all, want) {
			t.Errorf("a flag should state %q: %s", want, all)
		}
	}
	for _, banned := range []string{"cheat", "fraud", "dishonest", "violat", "not the candidate", "guilty", "plagiar"} {
		if strings.Contains(all, banned) {
			t.Errorf("a flag must not accuse (%q): %s", banned, all)
		}
	}
	// Using the assessment's own assistant isn't a flag.
	if strings.Contains(all, "9 changed line") {
		t.Errorf("assistant edits are how the candidate worked, not a flag: %s", all)
	}
}
