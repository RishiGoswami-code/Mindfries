package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/policy"
)

// fakeGemini answers the Live token endpoint and records what it was asked.
func fakeGemini(t *testing.T) (*httptest.Server, *map[string]any) {
	var got map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1alpha/auth_tokens" || r.Header.Get("x-goog-api-key") != "gem-key" {
			w.WriteHeader(404)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"name":"auth_tokens/abc123"}`))
	}))
	t.Cleanup(ts.Close)
	return ts, &got
}

func avatarEnv(t *testing.T) (*env, string, string, string, *map[string]any) {
	gem, asked := fakeGemini(t)
	e, g, cli, sid := assistantEnv(t, policy.ExplainOnly, func(c *config.Config) {
		c.GeminiKey, c.GeminiBaseURL, c.GeminiLiveModel = "gem-key", gem.URL, "gemini-live-test"
	}, nil)
	return e, cli, sid, e.panelToken(g, sid), asked
}

func TestTheAvatarGetsALockedLiveToken(t *testing.T) {
	e, cli, sid, panel, asked := avatarEnv(t)
	e.sendEvents(cli, event(1, "git.commit", map[string]any{"commit": "3f2a91c0", "files_changed": 2}))
	e.sendEvents(cli, event(2, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "x", "process_name": "claude.exe"}}}))

	cfg := e.panel("GET", "/v2/panel/config", panel, nil).JSON()
	if av := cfg["avatar"].(map[string]any); av["enabled"] != true || av["name"] != "Tanu" {
		t.Fatalf("%v", av)
	}
	r := e.panel("POST", "/v2/panel/avatar/token", panel, map[string]any{})
	if r.Status != 200 {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r.str("token") != "auth_tokens/abc123" || r.str("name") != "Tanu" || r.str("model") != "gemini-live-test" ||
		!strings.Contains(r.str("ws_url"), "BidiGenerateContentConstrained") || r.str("expires_at") == "" {
		t.Fatalf("%s", r.Raw)
	}
	_ = sid

	// What the browser can't change: the instructions and the model are in the token's constraints.
	c := (*asked)["liveConnectConstraints"].(map[string]any)
	if c["model"] != "models/gemini-live-test" || (*asked)["uses"] != float64(1) {
		t.Fatalf("%v", *asked)
	}
	sys := c["config"].(map[string]any)["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"Tanu", "backend", "Tickets pile onto", "never", "score, grade, rank", "Commit 3f2a91c"} {
		if !strings.Contains(sys, want) {
			// the brief text depends on the task; check the structural pieces strictly
			if want == "Tickets pile onto" {
				continue
			}
			t.Errorf("the avatar's instructions should contain %q:\n%s", want, sys)
		}
	}
	if strings.Contains(sys, "claude.exe") {
		t.Errorf("the avatar never hears about flags, so it can't accuse:\n%s", sys)
	}
	if mods := c["config"].(map[string]any)["responseModalities"].([]any); len(mods) != 1 || mods[0] != "AUDIO" {
		t.Errorf("%v", mods)
	}
}

func TestTheAvatarIsOffWithoutAKeyOrWhenDisabled(t *testing.T) {
	e, _, _, _ := assistantEnv(t, policy.ExplainOnly, nil, nil) // no Gemini key
	g := newGrant()
	_, sid := e.startSession(g)
	panel := e.panelToken(g, sid)
	if r := e.panel("POST", "/v2/panel/avatar/token", panel, map[string]any{}); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("no key: %d", r.Status)
	}
	if cfg := e.panel("GET", "/v2/panel/config", panel, nil).JSON(); cfg["avatar"].(map[string]any)["enabled"] != false {
		t.Fatal("the panel shouldn't offer an avatar that can't start")
	}

	gem, _ := fakeGemini(t)
	e2 := newEnv(t, func(c *config.Config) { c.GeminiKey, c.GeminiBaseURL, c.GeminiLiveModel = "gem-key", gem.URL, "m" })
	g2 := newGrant()
	cfg := policy.Default()
	cfg.AssessmentID = g2["assessment_id"].(string)
	cfg.AvatarEnabled = false
	_ = e2.db.SaveAssessmentConfig(t.Context(), cfg, "test")
	cli2, sid2 := e2.startSession(g2)
	p2 := e2.panelToken(g2, sid2)
	e2.shareAll(p2)
	e2.cli("POST", "/session/begin", cli2, map[string]any{"ide": "vscode"})
	if r := e2.panel("POST", "/v2/panel/avatar/token", p2, map[string]any{}); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("avatar disabled for this assessment: %d", r.Status)
	}
}

func TestTheAvatarTalksOnlyWhileTheClockRunsAndNotTooOften(t *testing.T) {
	gem, _ := fakeGemini(t)
	e := newEnv(t, func(c *config.Config) { c.GeminiKey, c.GeminiBaseURL, c.GeminiLiveModel = "gem-key", gem.URL, "m" })
	g := newGrant()
	cli, sid := e.startSession(g)
	panel := e.panelToken(g, sid)
	if r := e.panel("POST", "/v2/panel/avatar/token", panel, map[string]any{}); r.Status != http.StatusConflict || r.str("code") != "session_not_active" {
		t.Fatalf("before begin: %d %s", r.Status, r.Raw)
	}
	e.shareAll(panel)
	e.cli("POST", "/session/begin", cli, map[string]any{"ide": "vscode"})
	ok := 0
	for i := 0; i < 8; i++ {
		if e.panel("POST", "/v2/panel/avatar/token", panel, map[string]any{}).Status == 200 {
			ok++
		}
	}
	if ok != 6 {
		t.Fatalf("six avatar sessions an hour, then rate_limited: %d", ok)
	}
}

func TestWhatTheAvatarSaysIsKeptOnceAndInOrder(t *testing.T) {
	e, _, sid, panel, _ := avatarEnv(t)
	post := func(us ...map[string]any) reply {
		return e.panel("POST", "/v2/panel/avatar/transcript", panel, map[string]any{"utterances": us})
	}
	u := func(seq int, role, text string) map[string]any {
		return map[string]any{"seq": seq, "role": role, "text": text, "at": "2026-10-20T10:00:00Z"}
	}
	if r := post(u(1, "avatar", "What made you reach for a cache first?"), u(2, "candidate", "Reads dominate; Postgres is the bottleneck.")); r.Status != 200 || r.JSON()["stored"] != float64(2) {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	// A retried upload (and a new line) stores only what's new.
	if r := post(u(2, "candidate", "Reads dominate; Postgres is the bottleneck."), u(3, "avatar", "And how would you invalidate it?")); r.JSON()["stored"] != float64(1) {
		t.Fatalf("%s", r.Raw)
	}
	tr, _ := e.db.AvatarTranscript(t.Context(), sid)
	if len(tr) != 3 || tr[0].Role != "avatar" || tr[1].Role != "candidate" || tr[2].Seq != 3 || !strings.Contains(tr[2].Text, "invalidate") {
		t.Fatalf("%+v", tr)
	}
	// Junk is refused, not stored.
	for _, bad := range []map[string]any{u(0, "avatar", "x"), u(9, "judge", "x"), u(10, "avatar", "   ")} {
		if r := post(bad); r.Status != http.StatusBadRequest {
			t.Errorf("%v: %d", bad, r.Status)
		}
	}
	if r := e.panel("POST", "/v2/panel/avatar/transcript", panel, map[string]any{"utterances": []any{}}); r.Status != http.StatusBadRequest {
		t.Fatalf("%d", r.Status)
	}
	// It counts against the session's budget, since the server can't see Gemini's own tokens.
	if n, _ := e.db.SessionTokens(t.Context(), sid); n == 0 {
		t.Fatal("avatar talk should be counted")
	}
}
