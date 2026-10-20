package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/policy"
)

func (e *env) staff(method, path string, body any) reply {
	e.t.Helper()
	return e.do(method, path, map[string]string{"X-Portal-Secret": testPortalSecret, "X-Actor": "company:pat@acme.example"}, body)
}

func TestStaffRoutesNeedTheSecretOrTheAdminToken(t *testing.T) {
	e := newEnv(t, nil)
	asmt := newGrant()["assessment_id"].(string)
	path := "/v2/portal/assessments/" + asmt + "/config"
	if r := e.do("GET", path, nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("anonymous: %d", r.Status)
	}
	if r := e.do("GET", path, map[string]string{"X-Portal-Secret": "wrong"}, nil); r.Status != http.StatusForbidden {
		t.Fatalf("wrong secret: %d", r.Status)
	}
	if r := e.do("GET", path, map[string]string{"X-Portal-Secret": testPortalSecret}, nil); r.Status != 200 {
		t.Fatalf("portal: %d", r.Status)
	}
	if r := e.do("GET", path, map[string]string{"X-Admin-Token": testAdminToken}, nil); r.Status != 200 {
		t.Fatalf("admin: %d", r.Status)
	}
	// The tasks listing is the admin's alone.
	if r := e.staff("GET", "/v2/admin/products/p1-support-saas/tasks", nil); r.Status != http.StatusForbidden {
		t.Fatalf("a portal can't list the registry: %d", r.Status)
	}
	r := e.do("GET", "/v2/admin/products/p1-support-saas/tasks", map[string]string{"X-Admin-Token": testAdminToken}, nil)
	if r.Status != 200 || len(r.JSON()["tasks"].([]any)) < 2 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
}

func TestACompanySetsAnAssessmentUp(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	asmt := g["assessment_id"].(string)
	path := "/v2/portal/assessments/" + asmt + "/config"

	// Only what's sent changes; the rest keeps its default.
	r := e.staff("PUT", path, map[string]any{"reviewers": []string{"@Lead", "lead", "second-reviewer"}, "duration_minutes": 120, "assistant_mode": "code_allowed", "updated_by": "company:acme"})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	c := e.staff("GET", path, nil).JSON()
	cfg := c["config"].(map[string]any)
	if c["configured"] != true || cfg["duration_minutes"] != float64(120) || cfg["assistant_mode"] != "code_allowed" ||
		cfg["require_camera"] != true || len(cfg["reviewers"].([]any)) != 2 {
		t.Fatalf("%v", c)
	}
	// Change just the reviewers: the rest stays.
	e.staff("PUT", path, map[string]any{"reviewers": []string{"someone-new"}})
	cfg = e.staff("GET", path, nil).JSON()["config"].(map[string]any)
	if cfg["duration_minutes"] != float64(120) || cfg["reviewers"].([]any)[0] != "someone-new" {
		t.Fatalf("%v", cfg)
	}
	for _, bad := range []map[string]any{
		{"duration_minutes": 2}, {"assistant_mode": "sudo"}, {"reviewers": []string{"not a login!"}},
	} {
		if r := e.staff("PUT", path, bad); r.Status != http.StatusBadRequest {
			t.Errorf("%v: %d", bad, r.Status)
		}
	}
	if r := e.staff("PUT", "/v2/portal/assessments/not-a-uuid/config", map[string]any{}); r.Status != http.StatusBadRequest {
		t.Fatalf("%d", r.Status)
	}
	// And a session started afterwards sees it.
	cli, _ := e.startSession(g)
	s := e.cli("GET", "/session", cli, nil).JSON()
	if s["duration_minutes"] != float64(120) || s["policy"].(map[string]any)["assistant_mode"] != "code_allowed" {
		t.Fatalf("%v", s)
	}
}

func TestACompanySeesItsSessionsAndEndsOrExtendsOne(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	_, cli, sid, panel := startActiveFor(t, e, g)
	asmt, company := g["assessment_id"].(string), g["company_id"].(string)
	e.sendEvents(cli, event(1, "task_file.modified", map[string]any{"path": "TASK.md", "restored": true}))

	list := e.staff("GET", "/v2/portal/assessments/"+asmt+"/sessions?company_id="+company, nil).JSON()["sessions"].([]any)
	if len(list) != 1 {
		t.Fatalf("%v", list)
	}
	row := list[0].(map[string]any)
	if row["session_id"] != sid || row["status"] != "active" || row["flags"] != float64(1) || row["begun"] != true {
		t.Fatalf("%v", row)
	}
	// Another company sees nothing, and can't read, end or extend it.
	other := newGrant()["company_id"].(string)
	if l := e.staff("GET", "/v2/portal/assessments/"+asmt+"/sessions?company_id="+other, nil).JSON()["sessions"].([]any); len(l) != 0 {
		t.Fatalf("another company must not see it: %v", l)
	}
	for _, p := range []string{"/evidence", "/end", "/extend"} {
		method := "GET"
		if p != "/evidence" {
			method = "POST"
		}
		if r := e.staff(method, "/v2/portal/sessions/"+sid+p+"?company_id="+other, map[string]any{"minutes": 5}); r.Status != http.StatusNotFound {
			t.Errorf("%s as another company: %d", p, r.Status)
		}
	}

	before := e.cli("GET", "/session", cli, nil).str("ends_at")
	r := e.staff("POST", "/v2/portal/sessions/"+sid+"/extend?company_id="+company, map[string]any{"minutes": 15})
	if r.Status != 200 || r.str("ends_at") == before {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.staff("POST", "/v2/portal/sessions/"+sid+"/extend", map[string]any{"minutes": 500}); r.Status != http.StatusBadRequest {
		t.Fatalf("more than an hour at once: %d", r.Status)
	}
	_ = panel
	if r := e.staff("POST", "/v2/portal/sessions/"+sid+"/end?company_id="+company, nil); r.Status != http.StatusNoContent {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// Ended from the portal: the CLI's token stops, and the watcher is told.
	if r := e.cli("GET", "/session", cli, nil); r.Status != http.StatusUnauthorized || r.str("code") != "token_revoked" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.staff("POST", "/v2/portal/sessions/"+sid+"/end", nil); r.Status != http.StatusConflict {
		t.Fatalf("ending twice: %d", r.Status)
	}
}

// startActiveFor is startActive for a grant the test already made.
func startActiveFor(t *testing.T, e *env, g map[string]any) (map[string]any, string, string, string) {
	t.Helper()
	cli, sid := e.startSession(g)
	panel := e.panelToken(g, sid)
	e.shareAll(panel)
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}); r.Status != 200 {
		t.Fatalf("begin: %d %s", r.Status, r.Raw)
	}
	return g, cli, sid, panel
}

func TestTheEvidenceAReviewerReads(t *testing.T) {
	e, g, cli, sid := assistantEnv(t, policy.CodeAllowed, nil, nil)
	panel := e.panelToken(g, sid)
	// A session with a bit of everything.
	e.sendEvents(cli,
		event(1, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "c", "process_name": "claude.exe"}}}),
		event(2, "check.run", map[string]any{"command": "go test ./...", "passed": true, "passing": []string{"TestA"}}))
	e.panel("PUT", "/v2/panel/artifacts/notes", panel, map[string]any{"body": map[string]any{"text": "my notes"}})
	e.ask(cli, "why is this slow?", "")
	e.advance(6 * time.Second)
	e.shareAll(panel)
	e.upload("camera", panel, "image/jpeg", jpeg)
	e.do("PUT", cliPrefix+"/session/github/repo", nil, nil) // (not configured here; nothing to record)

	r := e.staff("GET", "/v2/portal/sessions/"+sid+"/evidence", nil)
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	ev := r.JSON()
	sess := ev["session"].(map[string]any)
	if sess["id"] != sid || sess["role"] != "backend" || sess["status"] != "active" {
		t.Fatalf("%v", sess)
	}
	for _, key := range []string{"monitoring", "media", "activity", "artifacts", "latest_check_run", "assistant_transcript", "config"} {
		if ev[key] == nil {
			t.Errorf("evidence is missing %q", key)
		}
	}
	if media := ev["media"].([]any); len(media) != 1 || media[0].(map[string]any)["kind"] != "camera_still" {
		t.Fatalf("media: %v", ev["media"])
	}
	if media := ev["media"].([]any); media[0].(map[string]any)["storage_path"] != nil || media[0].(map[string]any)["Path"] != nil {
		t.Fatalf("storage paths aren't handed out: %v", media[0])
	}
	if tr := ev["assistant_transcript"].([]any); len(tr) < 2 || tr[0].(map[string]any)["channel"] != "agent" {
		t.Fatalf("transcript: %v", tr)
	}

	// A still is fetched one at a time through a short-lived address.
	id := ev["media"].([]any)[0].(map[string]any)["id"].(string)
	m := e.staff("GET", "/v2/portal/sessions/"+sid+"/media/"+id, nil)
	if m.Status != 200 || !strings.HasPrefix(m.str("url"), "mem://sessions/"+sid+"/camera_still/") || m.str("kind") != "camera_still" {
		t.Fatalf("%d %s", m.Status, m.Raw)
	}
	if r := e.staff("GET", "/v2/portal/sessions/"+sid+"/media/evi_nope", nil); r.Status != http.StatusNotFound {
		t.Fatalf("%d", r.Status)
	}

	// A reviewer answers a flag.
	var flagID int64
	for _, it := range ev["activity"].([]any) {
		m := it.(map[string]any)
		if m["severity"] == "flag" {
			flagID = int64(m["id"].(float64))
		}
	}
	if flagID == 0 {
		t.Fatal("expected a flag for the agent process")
	}
	path := "/v2/portal/sessions/" + sid + "/activity/" + itoa(flagID) + "/review"
	if r := e.staff("POST", path, map[string]any{"decision": "explained", "note": "Candidate said it was a leftover window.", "reviewer": "pat@acme.example"}); r.Status != http.StatusNoContent {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.staff("POST", path, map[string]any{"decision": "guilty", "reviewer": "pat"}); r.Status != http.StatusBadRequest {
		t.Fatalf("a review's decision is one of a fixed few: %d", r.Status)
	}
	if r := e.staff("POST", path, map[string]any{"decision": "noted"}); r.Status != http.StatusBadRequest {
		t.Fatalf("a review names who made it: %d", r.Status)
	}
	if r := e.staff("POST", "/v2/portal/sessions/"+sid+"/activity/99999999/review", map[string]any{"decision": "noted", "reviewer": "x"}); r.Status != http.StatusNotFound {
		t.Fatalf("an item that isn't in this session: %d", r.Status)
	}
	rev := e.staff("GET", "/v2/portal/sessions/"+sid+"/evidence", nil).JSON()["flag_reviews"].([]any)
	if len(rev) != 1 || rev[0].(map[string]any)["decision"] != "explained" || rev[0].(map[string]any)["reviewer"] != "pat@acme.example" {
		t.Fatalf("%v", rev)
	}
}
