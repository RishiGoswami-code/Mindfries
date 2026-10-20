package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/policy"
)

func tabIDs(cfg map[string]any) []string {
	var out []string
	for _, t := range cfg["tabs"].([]any) {
		out = append(out, t.(map[string]any)["id"].(string))
	}
	return out
}

func has(list []string, id string) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func TestPanelConfigForABackendCandidate(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, sid, panel := startActive(t, e)
	e.sendEvents(cli, event(1, "task_file.modified", map[string]any{"path": "TASK.md", "restored": true}))

	r := e.panel("GET", "/v2/panel/config", panel, nil)
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	cfg := r.JSON()
	if cfg["sessionId"] != sid || cfg["role"] != "backend" || cfg["status"] != "active" || cfg["endsAt"] == nil || cfg["product"] == "" {
		t.Fatalf("%v", cfg)
	}
	tabs := tabIDs(cfg)
	for _, want := range []string{"notes", "resources", "activity", "debug", "test"} {
		if !has(tabs, want) {
			t.Errorf("a backend candidate on P1 needs the %s tab: %v", want, tabs)
		}
	}
	if has(tabs, "design") {
		t.Errorf("no design mode in the manifest, no design tab: %v", tabs)
	}
	// The Activity tab's badge is the flags the tracker holds.
	for _, tb := range cfg["tabs"].([]any) {
		m := tb.(map[string]any)
		if m["id"] == "activity" && m["badge"] != float64(1) {
			t.Errorf("the activity badge counts flags: %v", m)
		}
	}
	mon := cfg["monitoring"].(map[string]any)
	if mon["camera"] != true || mon["audio"] != true {
		t.Errorf("%v", mon)
	}
	if av := cfg["avatar"].(map[string]any); av["enabled"] != false {
		t.Errorf("the avatar is off until a Gemini key is configured: %v", av)
	}
	res := cfg["resources"].([]any)
	if res[0].(map[string]any)["title"] != "TASK.md" {
		t.Errorf("%v", res)
	}
	if n := len(cfg["activity"].([]any)); n < 2 {
		t.Errorf("the config carries the tracker so far: %d", n)
	}
	if cfg["diagram"].(map[string]any)["title"] != "Untitled Diagram" || cfg["notes"] != "" {
		t.Errorf("a fresh session starts empty: %v %v", cfg["diagram"], cfg["notes"])
	}
	if cfg["test"].(map[string]any)["title"] != "No checks run yet" {
		t.Errorf("%v", cfg["test"])
	}
	if cfg["firstTab"] == "" {
		t.Error("the panel needs to know which tab to open")
	}
}

func TestAnArchitectGetsTheDesignTabFromTheAssessmentsConfig(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	c := policy.Default()
	c.AssessmentID = g["assessment_id"].(string)
	c.TabsOverride = []string{"design", "review"}
	if err := e.db.SaveAssessmentConfig(t.Context(), c, "test"); err != nil {
		t.Fatal(err)
	}
	cli, sid := e.startSession(g)
	panel := e.panelToken(g, sid)
	e.shareAll(panel)
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})
	tabs := tabIDs(e.panel("GET", "/v2/panel/config", panel, nil).JSON())
	if strings.Join(tabs, ",") != "design,notes,resources,review,activity" {
		t.Fatalf("%v", tabs)
	}
}

func TestWhatTheCandidateMakesIsSavedAndComesBack(t *testing.T) {
	e := newEnv(t, nil)
	_, _, sid, panel := startActive(t, e)
	save := func(kind string, version any, body any) reply {
		req := map[string]any{"body": body}
		if version != nil {
			req["version"] = version
		}
		return e.panel("PUT", "/v2/panel/artifacts/"+kind, panel, req)
	}
	diagram := map[string]any{"title": "Content platform", "nodes": []map[string]any{
		{"id": "lb", "x": 10, "y": 20, "w": 160, "h": 64, "title": "Load Balancer", "tone": "purple", "icon": "balancer"},
		{"id": "db", "x": 300, "y": 20, "w": 160, "h": 64, "title": "Database", "tone": "red", "icon": "database"},
	}, "edges": []map[string]any{{"id": "e1", "from": "lb", "to": "db"}}}

	r := save("diagram", 0, diagram)
	if r.Status != 200 || r.JSON()["version"] != float64(1) {
		t.Fatalf("first save: %d %s", r.Status, r.Raw)
	}
	if r := save("diagram", 1, diagram); r.JSON()["version"] != float64(1) {
		t.Fatalf("saving what is already saved makes no new version: %s", r.Raw)
	}
	diagram["title"] = "Content platform v2"
	if r := save("diagram", 1, diagram); r.JSON()["version"] != float64(2) {
		t.Fatalf("%s", r.Raw)
	}
	if r := save("diagram", 1, diagram); r.Status != http.StatusConflict || r.str("code") != "conflict" || r.JSON()["current_version"] != float64(2) {
		t.Fatalf("a stale save from another tab: %d %s", r.Status, r.Raw)
	}
	if r := save("notes", nil, map[string]any{"text": "## Decisions\n- Redis for hot reads"}); r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := save("review", nil, map[string]any{"comments": []map[string]any{{"file": "a.go", "line": 4, "text": "stampede?"}}}); r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := save("hypotheses", nil, map[string]any{"items": []map[string]any{{"text": "ordering bug", "status": "open"}}}); r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	for _, bad := range []struct {
		kind string
		body any
	}{
		{"diagram", map[string]any{"title": "t", "nodes": []map[string]any{{"id": "a", "w": 1, "h": 1}}, "edges": []map[string]any{{"id": "e", "from": "a", "to": "ghost"}}}},
		{"notes", []string{"not", "an", "object"}},
		{"poster", map[string]any{}},
	} {
		if r := save(bad.kind, nil, bad.body); r.Status != http.StatusBadRequest && r.Status != http.StatusNotFound {
			t.Errorf("%s should be refused: %d", bad.kind, r.Status)
		}
	}

	// A reload (or another device) gets it all back.
	cfg := e.panel("GET", "/v2/panel/config", panel, nil).JSON()
	if cfg["diagram"].(map[string]any)["title"] != "Content platform v2" || len(cfg["diagram"].(map[string]any)["nodes"].([]any)) != 2 {
		t.Fatalf("diagram: %v", cfg["diagram"])
	}
	if !strings.Contains(cfg["notes"].(string), "Redis for hot reads") {
		t.Fatalf("notes: %v", cfg["notes"])
	}
	if c := cfg["review"].(map[string]any)["comments"].([]any); len(c) != 1 {
		t.Fatalf("review comments: %v", c)
	}
	if h := cfg["debug"].(map[string]any)["hypotheses"].([]any); len(h) != 1 {
		t.Fatalf("hypotheses: %v", h)
	}
	v := cfg["versions"].(map[string]any)
	if v["diagram"] != float64(2) || v["notes"] != float64(1) {
		t.Fatalf("versions: %v", v)
	}
	// Every version is kept as evidence.
	vs, _ := e.db.ArtifactVersions(t.Context(), sid, "diagram")
	if len(vs) != 2 {
		t.Fatalf("history: %d", len(vs))
	}
	// And the tracker noted that work began, once.
	items := e.panel("GET", "/v2/panel/activity", panel, nil).JSON()["items"].([]any)
	n := 0
	for _, it := range items {
		if strings.Contains(it.(map[string]any)["text"].(string), "Started the system design diagram") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the diagram's start is noted once: %d", n)
	}
}

func TestNothingIsSavedOnceTheSessionHasEnded(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, _, panel := startActive(t, e)
	e.cli("POST", "/session/revoke", cli, map[string]any{"reason": "stop"})
	// A revoked session's token no longer works for the CLI, and the panel can't add to the record.
	r := e.panel("PUT", "/v2/panel/artifacts/notes", panel, map[string]any{"body": map[string]any{"text": "late"}})
	if r.Status != http.StatusConflict || r.str("code") != "session_not_active" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
}

func TestCheckRunsReachTheTestingTab(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, sid, panel := startActive(t, e)
	e.sendEvents(cli, event(1, "check.run", map[string]any{
		"command": "go test ./internal/support/", "passed": false,
		"failing": []string{"TestAutoAssignBalances"}, "passing": []string{"TestTicketCreate", "TestSLABreach"},
		"output_tail": "--- FAIL: TestAutoAssignBalances",
	}))
	e.sendEvents(cli, event(1, "check.run", map[string]any{"command": "go test ./internal/support/", "passed": false})) // re-sent

	test := e.panel("GET", "/v2/panel/config", panel, nil).JSON()["test"].(map[string]any)
	if test["title"] != "go test ./internal/support/" || len(test["failing"].([]any)) != 1 || len(test["passing"].([]any)) != 2 || test["passed"] != false {
		t.Fatalf("%v", test)
	}
	run, ok, _ := e.db.LatestCheckRun(t.Context(), sid)
	if !ok || run.OutputTail == "" {
		t.Fatalf("%+v", run)
	}
	items := e.panel("GET", "/v2/panel/activity", panel, nil).JSON()["items"].([]any)
	found := 0
	for _, it := range items {
		if strings.Contains(it.(map[string]any)["text"].(string), "2 passed, 1 failed") {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("one tracker line per run: %d", found)
	}
}
