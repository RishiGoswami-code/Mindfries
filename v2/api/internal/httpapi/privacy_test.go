package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/jobs"
	"github.com/mindfries/v2/api/internal/session"
)

func TestACandidateCanSeeAndDeleteTheirData(t *testing.T) {
	e := newEnv(t, nil)
	g, cli, sid, panel := startActive(t, e)
	cand := g["candidate_id"].(string)
	e.sendEvents(cli, event(1, "git.commit", map[string]any{"commit": "abcdef1234", "files_changed": 1}))
	e.panel("PUT", "/v2/panel/artifacts/notes", panel, map[string]any{"body": map[string]any{"text": "private thoughts"}})
	e.upload("camera", panel, "image/jpeg", jpeg)
	if e.blobs.Count() != 1 {
		t.Fatal("a still should be stored")
	}

	if r := e.do("GET", "/v2/portal/candidates/"+cand+"/export", nil, nil); r.Status != http.StatusForbidden {
		t.Fatalf("the portal's server only: %d", r.Status)
	}
	r := e.portal("GET", "/v2/portal/candidates/"+cand+"/export", nil)
	if r.Status != 200 || !strings.Contains(r.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	exp := r.JSON()["sessions"].([]any)
	if len(exp) != 1 {
		t.Fatalf("%v", exp)
	}
	one := exp[0].(map[string]any)
	if one["id"] != sid || one["activity"] == nil || one["artifacts"] == nil || one["media"] == nil {
		t.Fatalf("an export has the lot: %v", one)
	}
	if !strings.Contains(string(r.Raw), "private thoughts") {
		t.Fatal("their own notes are theirs to see")
	}
	if other := e.portal("GET", "/v2/portal/candidates/"+newGrant()["candidate_id"].(string)+"/export", nil).JSON()["sessions"].([]any); len(other) != 0 {
		t.Fatal("someone else's export must be empty")
	}

	// Deletion must be asked for plainly.
	if r := e.portal("POST", "/v2/portal/candidates/"+cand+"/delete", map[string]any{}); r.Status != http.StatusBadRequest {
		t.Fatalf("no confirmation: %d", r.Status)
	}
	if e.blobs.Count() != 1 {
		t.Fatal("nothing is deleted without the confirmation")
	}
	r = e.portal("POST", "/v2/portal/candidates/"+cand+"/delete", map[string]any{"confirm": "delete"})
	if r.Status != 200 || r.JSON()["sessions_deleted"] != float64(1) || r.JSON()["files_deleted"] != float64(1) {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if e.blobs.Count() != 0 {
		t.Fatal("the stored files must be gone")
	}
	if _, err := e.db.SessionByID(t.Context(), sid); err != session.ErrNotFound {
		t.Fatalf("the session is gone: %v", err)
	}
	if items, _ := e.db.ActivityAfter(t.Context(), sid, 0, 10); len(items) != 0 {
		t.Fatal("and what hung off it")
	}
	if r := e.cli("GET", "/session", cli, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("the token is dead: %d", r.Status)
	}
	if after := e.portal("GET", "/v2/portal/candidates/"+cand+"/export", nil).JSON()["sessions"].([]any); len(after) != 0 {
		t.Fatal("nothing left to export")
	}
}

func TestHousekeeping(t *testing.T) {
	e := newEnv(t, nil)
	g, cli, sid, panel := startActive(t, e)
	e.upload("camera", panel, "image/jpeg", jpeg)

	r := jobs.Runner{Store: e.db, Blobs: e.blobs, Now: e.clock, RetentionDays: 90}
	var expired []string
	r.OnExpire = func(ctx context.Context, s session.Session) {
		expired = append(expired, s.ID)
		e.db.AddActivity(ctx, s.ID, activity.Item{At: e.clock(), Severity: activity.Info, Source: activity.Server, Text: "Time ran out. The assessment ended.", DedupeKey: "expired"})
	}

	// Nothing of this session is due yet. (The test database is shared, so other
	// tests' old rows may be, which is why this checks only what is ours.)
	if _, err := r.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, id := range expired {
		if id == sid {
			t.Fatal("a running session must not be expired early")
		}
	}
	if left, _ := e.db.EvidenceFor(t.Context(), sid, "", 10); len(left) != 1 {
		t.Fatal("fresh evidence stays")
	}
	// Time runs out with the computer gone: the session is ended and noted.
	e.advance(91 * time.Minute)
	if _, err := r.Once(t.Context()); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range expired {
		found = found || id == sid
	}
	if !found {
		t.Fatalf("session %s should have been expired (%v)", sid, expired)
	}
	s, _ := e.db.SessionByID(t.Context(), sid)
	if s.Status != "expired" {
		t.Fatalf("%s", s.Status)
	}
	// A session that was set up and abandoned is ended after a day.
	g2 := newGrant()
	_, sid2 := e.startSession(g2)
	e.advance(25 * time.Hour)
	if rep, _ := r.Once(t.Context()); rep.ReadyAbandoned < 1 {
		t.Fatalf("%+v", rep)
	}
	if s2, _ := e.db.SessionByID(t.Context(), sid2); s2.Status != "expired" {
		t.Fatalf("%s", s2.Status)
	}

	// Evidence past 90 days goes: bytes first, then the row, and it's logged.
	e.advance(91 * 24 * time.Hour)
	rep, err := r.Once(t.Context())
	if err != nil || rep.MediaDeleted < 1 {
		t.Fatalf("%+v %v", rep, err)
	}
	if e.blobs.Count() != 0 {
		t.Fatalf("the stored still must be deleted (%d left)", e.blobs.Count())
	}
	if left, _ := e.db.EvidenceFor(t.Context(), sid, "", 10); len(left) != 0 {
		t.Fatalf("the row says it's gone: %v", left)
	}
	log, _ := e.db.DeletionsFor(t.Context(), sid)
	if len(log) != 1 || log[0].What != "evidence_media" || log[0].Reason != "retention" || log[0].Count != 1 {
		t.Fatalf("%+v", log)
	}
	// Running it again finds nothing new.
	if rep, _ := r.Once(t.Context()); rep.MediaDeleted != 0 || rep.SessionsExpired != 0 {
		t.Fatalf("%+v", rep)
	}
	_ = g
	_ = cli
}

func TestOldTranscriptsAreBlankedButKept(t *testing.T) {
	e, g, cli, sid := assistantEnv(t, "explain_only", nil, nil)
	e.ask(cli, "my secret question about the design", "")
	e.cli("POST", "/session/revoke", cli, map[string]any{"reason": "stop"})
	_ = g
	r := jobs.Runner{Store: e.db, Blobs: e.blobs, Now: e.clock, RetentionDays: 90}
	e.advance(100 * 24 * time.Hour)
	rep, err := r.Once(t.Context())
	if err != nil || rep.TranscriptsBlank < 2 {
		t.Fatalf("%+v %v", rep, err)
	}
	rows, _ := e.db.SessionTranscript(t.Context(), sid)
	if len(rows) < 2 {
		t.Fatalf("the rows (and so the counts and timing) survive: %d", len(rows))
	}
	for _, row := range rows {
		if strings.Contains(string(row.Body), "secret question") {
			t.Fatalf("what was said must be gone: %s", row.Body)
		}
	}
}
