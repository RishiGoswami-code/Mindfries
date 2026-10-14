package httpapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mindfries/candidate-backend/internal/db"
)

func TestTechStackTagsCapsAtTwoEntriesPlusDuration(t *testing.T) {
	cases := []struct {
		stack []string
		min   int
		want  []string
	}{
		{[]string{"React", "Go", "Postgres", "Redis"}, 90, []string{"React", "Go", "90 min"}},
		{[]string{"Rust"}, 60, []string{"Rust", "60 min"}},
		{nil, 45, []string{"45 min"}},
	}
	for _, c := range cases {
		if got := techStackTags(c.stack, c.min); !reflect.DeepEqual(got, c.want) {
			t.Errorf("techStackTags(%v, %d) = %v, want %v", c.stack, c.min, got, c.want)
		}
	}
}

// An invitation says whether its task exists yet, and is named after its role.
func TestToInvitationView(t *testing.T) {
	role := "Backend Engineer"
	got := toInvitationView(db.Invitation{
		ID: "a1", CompanyName: "Acme", TemplateName: "Assessment", Role: &role,
		TechStack: []string{"Go"}, DurationMin: 45, Status: "invited", TaskStatus: "generating",
	})
	if got.Role != "Backend Engineer" || got.Company != "Acme" || got.TaskStatus != "generating" || got.Status != "invited" {
		t.Fatalf("unexpected view: %+v", got)
	}
}

func TestInvitationStatusMapsUnderscoreToHyphen(t *testing.T) {
	cases := map[string]string{
		"invited":     "invited",
		"in_progress": "in-progress",
		"submitted":   "submitted",
		"closed":      "closed",
	}
	for in, want := range cases {
		if got := invitationStatus(in); got != want {
			t.Errorf("invitationStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToInvitationViewPrefersExplicitRoleOverTemplateName(t *testing.T) {
	role := "Senior Backend Engineer"
	got := toInvitationView(db.Invitation{
		ID: "a1", CompanyName: "Northwind Labs", TemplateName: "Bug Fix Sprint",
		TechStack: []string{"Go"}, DurationMin: 90, Role: &role, Status: "invited",
	})
	if got.Role != role {
		t.Fatalf("Role = %q, want the explicit role %q", got.Role, role)
	}
	if got.Company != "Northwind Labs" {
		t.Fatalf("Company = %q", got.Company)
	}
	if got.Status != "invited" {
		t.Fatalf("Status = %q", got.Status)
	}
	if got.Due != "No due date set" {
		t.Fatalf("Due = %q, want the honest no-due-date fallback", got.Due)
	}
}

func TestToInvitationViewFallsBackToTemplateName(t *testing.T) {
	got := toInvitationView(db.Invitation{ID: "a2", TemplateName: "Refactor Challenge", Status: "in_progress", DurationMin: 60})
	if got.Role != "Refactor Challenge" {
		t.Fatalf("Role = %q, want the template name when no explicit role is set", got.Role)
	}
	if got.Status != "in-progress" {
		t.Fatalf("Status = %q, want the hyphenated form", got.Status)
	}
}

func TestToInvitationViewCarriesMatchScoreThrough(t *testing.T) {
	score := 92
	got := toInvitationView(db.Invitation{ID: "a3", TemplateName: "X", Status: "invited", MatchScore: &score})
	if got.Match == nil || *got.Match != 92 {
		t.Fatalf("Match = %v, want 92", got.Match)
	}
}

func TestToInvitationViewFormatsDueDate(t *testing.T) {
	due := "2026-09-30"
	got := toInvitationView(db.Invitation{ID: "a4", TemplateName: "X", Status: "invited", DueDate: &due})
	if got.Due != "Due 2026-09-30" {
		t.Fatalf("Due = %q", got.Due)
	}
}

func TestEventTypePatternAcceptsRealEventTypesAndRejectsGarbage(t *testing.T) {
	valid := []string{"workspace_output", "file_edit", "navigation", "git", "terminal", "test_run", "ai_usage", "a"}
	for _, s := range valid {
		if !eventTypePattern.MatchString(s) {
			t.Errorf("eventTypePattern rejected real event type %q", s)
		}
	}
	invalid := []string{
		"", "Workspace_Output", "1_starts_with_digit", "has space", "has-dash",
		"has.dot", "has/slash", "sql'; drop table activity_events; --",
		"a" + strings.Repeat("b", 64), // 65 chars, otherwise valid charset — length is the point
	}
	for _, s := range invalid {
		if eventTypePattern.MatchString(s) {
			t.Errorf("eventTypePattern accepted invalid event type %q", s)
		}
	}
}

func TestSessionIsLiveOnlyForTheLiveStatus(t *testing.T) {
	cases := map[string]bool{
		"live":       true,
		"submitted":  false,
		"evaluating": false,
		"completed":  false,
		"failed":     false,
		"stuck":      false, // an admin support-override target, not something a candidate action should write around
		"":           false,
	}
	for status, want := range cases {
		if got := sessionIsLive(status); got != want {
			t.Errorf("sessionIsLive(%q) = %v, want %v", status, got, want)
		}
	}
}
