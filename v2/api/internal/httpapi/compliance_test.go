package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/policy"
)

func TestTheNoticeSaysWhatThisAssessmentCollects(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	c := policy.Default()
	c.AssessmentID = g["assessment_id"].(string)
	c.RequireAudio = false
	c.AssistantMode = policy.Off
	c.AvatarEnabled = false
	if err := e.db.SaveAssessmentConfig(t.Context(), c, "test"); err != nil {
		t.Fatal(err)
	}
	r := e.portal("GET", "/v2/portal/assessments/"+c.AssessmentID+"/notice", nil)
	if r.Status != 200 || r.str("version") != NoticeVersion {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	collected := map[string]bool{}
	for _, it := range r.JSON()["items"].([]any) {
		m := it.(map[string]any)
		collected[m["id"].(string)] = m["collected"] == true
		if m["label"] == "" || m["description"] == "" {
			t.Errorf("every line needs words: %v", m)
		}
	}
	if !collected["camera"] || !collected["screen"] || collected["audio"] || collected["assistant"] || !collected["github"] || collected["not"] {
		t.Fatalf("the notice follows the assessment's settings: %v", collected)
	}
}

func TestTheClockWaitsForTheCandidateToAcceptTheNotice(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSessionUnaccepted(g)
	panel := e.panelToken(g, sid)
	rep := e.shareAll(panel)
	if m := rep.JSON()["missing"].([]any); len(m) != 1 || !strings.Contains(m[0].(string), "accepted the notice") {
		t.Fatalf("the panel is told what's missing: %v", m)
	}
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}); r.Status != http.StatusConflict || r.str("code") != "monitoring_not_ready" || !strings.Contains(r.str("detail"), "accepted the notice") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}

	// A stale or incomplete acceptance doesn't count.
	cand, asmt := g["candidate_id"], g["assessment_id"]
	if r := e.portal("POST", "/v2/portal/consents", map[string]any{"candidate_id": cand, "assessment_id": asmt, "version": "2020-01-01", "items": []string{"camera"}}); r.Status != http.StatusConflict {
		t.Fatalf("an old notice version: %d", r.Status)
	}
	if r := e.portal("POST", "/v2/portal/consents", map[string]any{"candidate_id": cand, "assessment_id": asmt, "version": NoticeVersion, "items": []string{"camera"}}); r.Status != http.StatusBadRequest || !strings.Contains(r.str("detail"), "missing") {
		t.Fatalf("accepting only part: %d %s", r.Status, r.Raw)
	}
	if got := e.portal("GET", "/v2/portal/consents?candidate_id="+cand.(string)+"&assessment_id="+asmt.(string), nil).JSON(); got["accepted"] != false {
		t.Fatalf("%v", got)
	}

	e.acceptNotice(g)
	if got := e.portal("GET", "/v2/portal/consents?candidate_id="+cand.(string)+"&assessment_id="+asmt.(string), nil).JSON(); got["accepted"] != true {
		t.Fatalf("%v", got)
	}
	e.shareAll(panel)
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}); r.Status != 200 {
		t.Fatalf("after accepting: %d %s", r.Status, r.Raw)
	}
}

func TestStaffReadsOfEvidenceNameWhoAndAreLogged(t *testing.T) {
	e := newEnv(t, nil)
	g, _, sid, panel := startActive(t, e)
	e.upload("camera", panel, "image/jpeg", jpeg)

	// Without saying who is reading, there is no evidence.
	bare := map[string]string{"X-Portal-Secret": testPortalSecret}
	for _, path := range []string{"/evidence", "/report", "/media/evi_x"} {
		if r := e.do("GET", "/v2/portal/sessions/"+sid+path, bare, nil); r.Status != http.StatusBadRequest || !strings.Contains(r.str("detail"), "X-Actor") {
			t.Errorf("%s: %d %s", path, r.Status, r.Raw)
		}
	}
	ev := e.staff("GET", "/v2/portal/sessions/"+sid+"/evidence", nil).JSON()
	id := ev["media"].([]any)[0].(map[string]any)["id"].(string)
	e.staff("GET", "/v2/portal/sessions/"+sid+"/media/"+id, nil)
	rep := e.staff("GET", "/v2/portal/sessions/"+sid+"/report", nil)
	if rep.Status != 200 || rep.JSON()["events"] != nil || rep.JSON()["report"] == nil {
		t.Fatalf("the company's report has no raw events: %d %s", rep.Status, rep.Raw)
	}
	if c := ev["consent"].(map[string]any); c["version"] != NoticeVersion {
		t.Fatalf("the evidence shows what the candidate accepted: %v", c)
	}
	_ = g

	log := e.staff("GET", "/v2/portal/sessions/"+sid+"/access-log", nil).JSON()["access"].([]any)
	var actions []string
	for _, a := range log {
		m := a.(map[string]any)
		if m["actor"] != "company:pat@acme.example" {
			t.Errorf("%v", m)
		}
		actions = append(actions, m["action"].(string))
	}
	if strings.Join(actions, ",") != "evidence,media,report" {
		t.Fatalf("every read is logged, in order: %v", actions)
	}
	// The access log itself is staff-only.
	if r := e.do("GET", "/v2/portal/sessions/"+sid+"/access-log", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("%d", r.Status)
	}
}

func TestACandidateCanSayWhatAFlagWas(t *testing.T) {
	e := newEnv(t, nil)
	_, cli, sid, panel := startActive(t, e)
	e.sendEvents(cli, event(1, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "c", "process_name": "claude.exe"}}}))
	items := e.panel("GET", "/v2/panel/activity", panel, nil).JSON()["items"].([]any)
	var flagID, infoID float64
	for _, it := range items {
		m := it.(map[string]any)
		if m["severity"] == "flag" {
			flagID = m["id"].(float64)
		}
		if m["severity"] == "info" {
			infoID = m["id"].(float64)
		}
	}
	respond := func(id float64, text string) reply {
		return e.panel("POST", "/v2/panel/activity/"+itoa(int64(id))+"/response", panel, map[string]any{"text": text})
	}
	if r := respond(flagID, "That was a leftover window from earlier. I closed it."); r.Status != http.StatusNoContent {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := respond(flagID, "   "); r.Status != http.StatusBadRequest {
		t.Fatalf("an empty answer: %d", r.Status)
	}
	if r := respond(infoID, "x"); r.Status != http.StatusNotFound {
		t.Fatalf("only a flag or notice can be answered: %d", r.Status)
	}
	if r := respond(99999999, "x"); r.Status != http.StatusNotFound {
		t.Fatalf("%d", r.Status)
	}
	// Another candidate can't answer for them.
	_, _, _, otherPanel := startActive(t, e)
	if r := e.panel("POST", "/v2/panel/activity/"+itoa(int64(flagID))+"/response", otherPanel, map[string]any{"text": "mine"}); r.Status != http.StatusNotFound {
		t.Fatalf("%d", r.Status)
	}
	// The reviewer sees it beside the flag.
	ev := e.staff("GET", "/v2/portal/sessions/"+sid+"/evidence", nil).JSON()
	fr := ev["flag_responses"].([]any)
	if len(fr) != 1 || !strings.Contains(fr[0].(map[string]any)["text"].(string), "leftover window") {
		t.Fatalf("%v", fr)
	}
}
