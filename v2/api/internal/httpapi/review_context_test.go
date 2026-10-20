package httpapi

import (
	"strings"
	"testing"
	"time"
)

// The admin's evidence report carries the rest of what the session recorded.
func TestAdminReportCarriesThePanelsEvidence(t *testing.T) {
	t.Setenv("V2_ADMIN_TOKEN", testAdminToken)
	e := newEnv(t, nil)
	_, cli, sid, panel := startActive(t, e)

	// A gap, a check run, saved work and an assistant question.
	e.advance(5 * time.Second)
	e.panel("POST", "/v2/panel/status", panel, map[string]any{"camera": true, "screen": false, "audio": true})
	e.advance(20 * time.Second)
	e.shareAll(panel)
	e.sendEvents(cli, event(1, "check.run", map[string]any{"command": "go test ./...", "passed": false, "failing": []string{"TestA"}, "passing": []string{"TestB"}}))
	e.panel("PUT", "/v2/panel/artifacts/notes", panel, map[string]any{"body": map[string]any{"text": "notes"}})

	r := e.do("GET", "/v2/admin/reviews/"+sid, map[string]string{"X-Admin-Token": testAdminToken}, nil)
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	rep := r.JSON()["report"].(map[string]any)
	ctx, ok := rep["context"].(map[string]any)
	if !ok {
		t.Fatalf("the report has no context: %v", rep)
	}
	notes := ""
	for _, n := range ctx["notes"].([]any) {
		notes += n.(string) + " | "
	}
	for _, want := range []string{"The screen wasn't recorded for 20s", "1 passed, 1 failed", "saved work in the panel: [notes]"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes should contain %q: %s", want, notes)
		}
	}
	if rep["submitted"] != false {
		t.Errorf("nothing was submitted: %v", rep["submitted"])
	}
	for _, f := range rep["flags"].([]any) {
		if strings.Contains(f.(string), "Locked task files") {
			t.Errorf("an unsubmitted session isn't flagged for task files: %v", f)
		}
	}
}
