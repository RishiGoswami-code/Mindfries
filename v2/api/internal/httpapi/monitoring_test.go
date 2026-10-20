package httpapi

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/policy"
)

var jpeg = []byte("\xff\xd8\xff\xe0fake-jpeg-bytes")

func TestPanelTokenIsForTheCandidateWhoOwnsTheSession(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	_, sid := e.startSession(g)

	other := newGrant()
	if r := e.portal("POST", "/v2/portal/sessions/"+sid+"/panel-token", map[string]any{"candidate_id": other["candidate_id"]}); r.Status != http.StatusForbidden {
		t.Fatalf("someone else's session: %d", r.Status)
	}
	if r := e.portal("POST", "/v2/portal/sessions/ses_nope/panel-token", map[string]any{"candidate_id": g["candidate_id"]}); r.Status != http.StatusNotFound {
		t.Fatalf("unknown session: %d", r.Status)
	}
	if r := e.do("POST", "/v2/portal/sessions/"+sid+"/panel-token", nil, map[string]any{"candidate_id": g["candidate_id"]}); r.Status != http.StatusForbidden {
		t.Fatalf("a browser can't ask for its own token: %d", r.Status)
	}

	tok := e.panelToken(g, sid)
	if r := e.panel("POST", "/v2/panel/status", "", map[string]any{}); r.Status != http.StatusUnauthorized {
		t.Fatalf("no token: %d", r.Status)
	}
	if r := e.panel("POST", "/v2/panel/status", "garbage", map[string]any{}); r.Status != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", r.Status)
	}
	if r := e.shareAll(tok); r.Status != 200 {
		t.Fatalf("good token: %d %s", r.Status, r.Raw)
	}
	e.advance(panelTokenTTL + time.Minute)
	if r := e.shareAll(tok); r.Status != http.StatusUnauthorized {
		t.Fatalf("an expired token: %d", r.Status)
	}
}

func TestBeginWaitsForTheRequiredMonitoring(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)

	begin := func() reply { return e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"}) }

	// The panel isn't even open.
	if r := begin(); r.Status != http.StatusConflict || r.str("code") != "monitoring_not_ready" || !strings.Contains(r.str("detail"), "panel isn't open") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// Open, but only the camera is on.
	e.panel("POST", "/v2/panel/status", tok, map[string]any{"camera": true, "screen": false, "audio": false})
	r := begin()
	if r.Status != http.StatusConflict || !strings.Contains(r.str("detail"), "screen isn't shared") || !strings.Contains(r.str("detail"), "microphone isn't on") ||
		strings.Contains(r.str("detail"), "camera") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if st := e.cli("GET", "/session", cli, nil).str("status"); st != "ready" {
		t.Fatalf("the clock must not have started: %s", st)
	}

	// Everything on: begin starts the clock for the full duration.
	e.shareAll(tok)
	r = begin()
	if r.Status != 200 || r.str("status") != "active" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	want := e.clock().Add(90 * time.Minute)
	if got := r.str("ends_at"); got != want.Format(time.RFC3339Nano) && got != want.Format(time.RFC3339) {
		t.Fatalf("ends_at %s, want %s", got, want.Format(time.RFC3339))
	}
	// Beginning again answers it as it is: the clock doesn't restart.
	e.advance(10 * time.Minute)
	if r2 := begin(); r2.Status != 200 || r2.str("ends_at") != r.str("ends_at") {
		t.Fatalf("a second begin must not move the end: %s vs %s", r2.str("ends_at"), r.str("ends_at"))
	}
}

func TestAssessmentThatDoesNotNeedAudioDoesNotWaitForIt(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	c := policy.Default()
	c.AssessmentID = g["assessment_id"].(string)
	c.RequireAudio = false
	c.DurationMinutes = 60
	if err := e.db.SaveAssessmentConfig(t.Context(), c, "test"); err != nil {
		t.Fatal(err)
	}
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	e.panel("POST", "/v2/panel/status", tok, map[string]any{"camera": true, "screen": true, "audio": false})
	if r := e.cli("POST", "/session/begin", cli, map[string]any{"ide": "manual"}); r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// And it won't take microphone clips it didn't ask for.
	if r := e.upload("audio", tok, "audio/webm", []byte("clip")); r.Status != http.StatusForbidden {
		t.Fatalf("audio not collected: %d %s", r.Status, r.Raw)
	}
}

func TestMonitoringStatusGoesStale(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	if m := e.cli("GET", "/session/monitoring", cli, nil).JSON(); m["panel_open"] != false || m["camera"] != false {
		t.Fatalf("before the panel opens: %v", m)
	}
	e.shareAll(tok)
	if m := e.cli("GET", "/session/monitoring", cli, nil).JSON(); m["panel_open"] != true || m["camera"] != true || m["audio"] != true || m["last_seen"] == nil {
		t.Fatalf("%v", m)
	}
	e.advance(25 * time.Second)
	if m := e.cli("GET", "/session/monitoring", cli, nil).JSON(); m["panel_open"] != false || m["screen"] != false {
		t.Fatalf("a panel that went quiet is not open and shares nothing: %v", m)
	}
}

func TestEvidenceUploads(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	e.shareAll(tok)

	// Not while the clock isn't running.
	if r := e.upload("camera", tok, "image/jpeg", jpeg); r.Status != http.StatusConflict || r.str("code") != "session_not_active" {
		t.Fatalf("before begin: %d %s", r.Status, r.Raw)
	}
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})

	r := e.upload("camera", tok, "image/jpeg", jpeg)
	if r.Status != http.StatusCreated || r.str("kind") != "camera_still" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if e.blobs.Count() != 1 {
		t.Fatal("the bytes go to blob storage")
	}
	if r := e.upload("camera", tok, "image/jpeg", jpeg); r.Status != http.StatusTooManyRequests {
		t.Fatalf("the panel sends at its pace, not faster: %d", r.Status)
	}
	e.advance(6 * time.Second)
	e.shareAll(tok)
	if r := e.upload("camera", tok, "image/jpeg", jpeg); r.Status != http.StatusCreated {
		t.Fatalf("after the interval: %d %s", r.Status, r.Raw)
	}
	if r := e.upload("screen", tok, "image/png", []byte("\x89PNGscreenshot")); r.Status != http.StatusCreated || r.str("kind") != "screenshot" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.upload("audio", tok, "audio/webm;codecs=opus", []byte("clip")); r.Status != http.StatusCreated || r.str("kind") != "audio_clip" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// Wrong type, wrong place, empty, too big.
	if r := e.upload("camera", tok, "text/html", []byte("<script>")); r.Status != http.StatusBadRequest {
		t.Fatalf("an upload that isn't an image: %d", r.Status)
	}
	if r := e.upload("video", tok, "video/mp4", []byte("x")); r.Status != http.StatusBadRequest {
		t.Fatalf("%d", r.Status)
	}
	e.advance(6 * time.Second)
	e.shareAll(tok)
	if r := e.upload("camera", tok, "image/jpeg", nil); r.Status != http.StatusBadRequest {
		t.Fatalf("empty: %d", r.Status)
	}
	if r := e.upload("camera", tok, "image/jpeg", bytes.Repeat([]byte{1}, 2<<20+1)); r.Status != http.StatusRequestEntityTooLarge || r.str("code") != "too_large" {
		t.Fatalf("too big: %d %s", r.Status, r.Raw)
	}

	evs, _ := e.db.EvidenceFor(t.Context(), sid, "", 100)
	if len(evs) != 4 {
		t.Fatalf("stored evidence: %d", len(evs))
	}
	for _, ev := range evs {
		if !strings.HasPrefix(ev.Path, "sessions/"+sid+"/") || ev.SHA256 == "" || ev.ExpiresAt.Before(e.clock().Add(89*24*time.Hour)) {
			t.Fatalf("%+v", ev)
		}
	}
}

func TestAShareThatStopsIsAGapNotAVerdict(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	e.shareAll(tok)
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})

	sub, cancel := e.hub.Subscribe(sid)
	defer cancel()

	// The screen share stops for a few seconds, then comes back: a notice, not a flag.
	e.advance(5 * time.Second)
	e.panel("POST", "/v2/panel/status", tok, map[string]any{"camera": true, "screen": false, "audio": true})
	e.advance(8 * time.Second)
	e.shareAll(tok)

	// Later it stops again, this time for a minute: a flag, worded as a gap.
	e.advance(time.Minute)
	e.panel("POST", "/v2/panel/status", tok, map[string]any{"camera": true, "screen": false, "audio": true})
	e.advance(time.Minute)
	e.shareAll(tok)

	gaps, _ := e.db.Gaps(t.Context(), sid)
	if len(gaps) != 2 || gaps[0].Source != "screen" || gaps[0].EndedAt == nil {
		t.Fatalf("gaps: %+v", gaps)
	}
	items, _ := e.db.ActivityAfter(t.Context(), sid, 0, 100)
	var texts []string
	flags := 0
	for _, it := range items {
		texts = append(texts, it.Severity+": "+it.Text)
		if it.Severity == "flag" {
			flags++
			if !strings.Contains(it.Text, "Recorded as a gap") || it.Category != "signal" {
				t.Errorf("a flag must say what was recorded, as a signal: %+v", it)
			}
		}
	}
	if flags != 1 {
		t.Fatalf("only the long gap is a flag:\n%s", strings.Join(texts, "\n"))
	}
	want := "Screen was off for 1m0s. Recorded as a gap."
	found := false
	for _, it := range items {
		found = found || it.Text == want
	}
	if !found {
		t.Fatalf("expected %q in:\n%s", want, strings.Join(texts, "\n"))
	}
	// The open feed heard about it live.
	heard := 0
	for len(sub) > 0 {
		<-sub
		heard++
	}
	if heard < 4 {
		t.Fatalf("the live feed should have had the gap items: %d", heard)
	}
}

func TestSilentPanelIsNoticedByTheHeartbeat(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	e.shareAll(tok)
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})

	beat := func() reply {
		return e.cli("POST", "/session/heartbeat", cli, map[string]any{"sent_at": e.clock(), "watcher": map[string]any{"queue_depth": 0, "last_sent_seq": 0}})
	}
	e.advance(30 * time.Second)
	if r := beat(); r.Status != 200 || r.JSON()["stop"] != false {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if gaps, _ := e.db.Gaps(t.Context(), sid); len(gaps) != 0 {
		t.Fatalf("30 seconds of quiet is not a gap: %+v", gaps)
	}
	e.advance(61 * time.Second)
	r := beat()
	if m := r.JSON()["monitoring"].(map[string]any); m["panel_open"] != false {
		t.Fatalf("the reply says what the panel has on: %v", m)
	}
	gaps, _ := e.db.Gaps(t.Context(), sid)
	if len(gaps) != 1 || gaps[0].Source != "panel" || gaps[0].EndedAt != nil {
		t.Fatalf("%+v", gaps)
	}
	beat() // another beat doesn't make a second gap
	if gaps, _ := e.db.Gaps(t.Context(), sid); len(gaps) != 1 {
		t.Fatalf("one gap per silence: %+v", gaps)
	}
	// The panel comes back and the gap closes.
	e.shareAll(tok)
	gaps, _ = e.db.Gaps(t.Context(), sid)
	if gaps[0].EndedAt == nil {
		t.Fatal("the panel is back; the gap should have ended")
	}
}

func TestHeartbeatTellsAFinishedSessionToStop(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	cli, sid := e.startSession(g)
	tok := e.panelToken(g, sid)
	e.shareAll(tok)
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})
	e.advance(91 * time.Minute)
	r := e.cli("POST", "/session/heartbeat", cli, map[string]any{"sent_at": e.clock(), "watcher": map[string]any{"queue_depth": 0, "last_sent_seq": 0}})
	if r.str("session_status") != "expired" || r.JSON()["stop"] != true {
		t.Fatalf("%s", r.Raw)
	}
}
