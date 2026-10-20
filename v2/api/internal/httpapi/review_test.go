package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/taskgen"
)

func adminGet(t *testing.T, url, token string) (*http.Response, []byte) {
	t.Helper()
	hr, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		hr.Header.Set("X-Admin-Token", token)
	}
	res, err := http.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	return res, b
}

func TestAdminReviewSurface(t *testing.T) {
	t.Setenv("V2_ADMIN_TOKEN", "tok")
	ctx := context.Background()
	root := productsRoot(t)
	reg := loadReg(t, root, "p1-support-saas")
	mem := session.NewMem()

	_ = mem.CreateSession(ctx, session.Session{
		ID: "ses_x", Token: "tok_x", Role: "frontend", ProductSlug: "p1-support-saas",
		BankTaskID: "feature-sla-countdown", TaskVersion: "tv", DurationMinutes: 90,
		Status: "submitted", StartedAt: time.Now(),
	})
	_, _, _, _ = mem.AppendEvents(ctx, "ses_x", []session.Event{
		{Seq: 1, OccurredAt: time.Now(), Type: "agent_signal.process",
			Data: json.RawMessage(`{"matches":[{"agent":"claude-code"}],"closed_by_candidate":false}`)},
	})
	_ = mem.CreateSubmission(ctx, session.Submission{ID: "sub_x", SessionID: "ses_x", TaskFilesIntact: true, Status: "verified", CreatedAt: time.Now()})
	_ = mem.SaveGrade(ctx, session.Grade{SubmissionID: "sub_x", Passed: true, Check: []string{"npm", "test"}, GradedAt: time.Now()})

	srv := httptest.NewServer(New(reg, root, taskgen.NewMemCache(), nil, 3, mem, "", nil))
	defer srv.Close()

	// Auth is required.
	if res, _ := adminGet(t, srv.URL+"/v2/admin/reviews", ""); res.StatusCode != http.StatusForbidden {
		t.Fatalf("no token should be 403, got %d", res.StatusCode)
	}

	// List shows the session as submitted + graded + passed.
	res, body := adminGet(t, srv.URL+"/v2/admin/reviews", "tok")
	if res.StatusCode != 200 {
		t.Fatalf("list status %d: %s", res.StatusCode, body)
	}
	var list []map[string]any
	_ = json.Unmarshal(body, &list)
	if len(list) != 1 || list[0]["submitted"] != true || list[0]["graded"] != true || list[0]["passed"] != true {
		t.Fatalf("list wrong: %s", body)
	}

	// Detail carries the report with the AI flag and the passed outcome.
	res, body = adminGet(t, srv.URL+"/v2/admin/reviews/ses_x", "tok")
	if res.StatusCode != 200 {
		t.Fatalf("detail status %d: %s", res.StatusCode, body)
	}
	var detail struct {
		Report struct {
			Outcome struct {
				Graded bool `json:"graded"`
				Passed bool `json:"passed"`
			} `json:"outcome"`
			AI struct {
				AgentsDetected []string `json:"agents_detected"`
			} `json:"ai"`
			Flags []string `json:"flags"`
		} `json:"report"`
	}
	_ = json.Unmarshal(body, &detail)
	if !detail.Report.Outcome.Graded || !detail.Report.Outcome.Passed {
		t.Fatalf("outcome not graded/passed: %s", body)
	}
	if len(detail.Report.AI.AgentsDetected) == 0 || len(detail.Report.Flags) == 0 {
		t.Fatalf("expected AI agent + flags in report: %s", body)
	}
}
