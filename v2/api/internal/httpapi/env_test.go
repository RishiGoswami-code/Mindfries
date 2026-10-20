package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/blob"
	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/store"
	"github.com/mindfries/v2/api/internal/taskgen"
	"github.com/mindfries/v2/api/internal/testdb"
)

// env is a server on a real (test) Postgres with a clock the test controls.
// The tests that use it skip when V2_TEST_DATABASE_URL isn't set.
type env struct {
	t   *testing.T
	ts  *httptest.Server
	db  *store.DB
	cfg config.Config

	blobs *blob.Mem
	hub   *activity.Hub

	mu  sync.Mutex
	now time.Time
}

const (
	testPortalSecret = "portal-secret-for-tests"
	testAdminToken   = "admin-token-for-tests"
)

func newEnv(t *testing.T, mod func(*config.Config)) *env {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	db, err := store.Open(ctx, testdb.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	root := productsRoot(t)
	e := &env{t: t, db: db, blobs: blob.NewMem(), hub: activity.NewHub(), now: time.Now().UTC().Truncate(time.Second)}
	e.cfg = config.Config{
		ProductsRoot: root, PoolSize: 3, MinCLIVersion: "0.0.0", LatestCLIVersion: "0.1.0",
		PortalSecret: testPortalSecret, AdminToken: testAdminToken,
		PublicURL: "https://api.test", PortalURL: "https://portal.test",
		EncryptionKey: bytes.Repeat([]byte{7}, 32),
	}
	if mod != nil {
		mod(&e.cfg)
	}
	reg := loadReg(t, root, "p1-support-saas", "p2-rag-platform", "p3-quant-platform")
	h := NewServer(Options{
		Config: e.cfg, Registry: reg, Cache: taskgen.NewMemCache(),
		Sessions: db, Devices: db, Configs: db, GitHub: db, Monitor: db, Activity: db, Blobs: e.blobs, Hub: e.hub,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: e.clock,
	})
	e.ts = httptest.NewServer(h)
	t.Cleanup(e.ts.Close)
	return e
}

func (e *env) clock() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now
}

func (e *env) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

// reply is a response with its body read.
type reply struct {
	Status int
	Header http.Header
	Raw    []byte
}

func (r reply) JSON() map[string]any {
	var m map[string]any
	_ = json.Unmarshal(r.Raw, &m)
	return m
}

func (r reply) str(key string) string {
	s, _ := r.JSON()[key].(string)
	return s
}

func (e *env) do(method, path string, hdr map[string]string, body any) reply {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = bytes.NewBufferString(b)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rd = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rd)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if rd != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return reply{Status: res.StatusCode, Header: res.Header, Raw: raw}
}

// portal calls a /v2/portal route as the portal's server.
func (e *env) portal(method, path string, body any) reply {
	e.t.Helper()
	return e.do(method, path, map[string]string{"X-Portal-Secret": testPortalSecret}, body)
}

// cli calls a CLI route; token may be empty.
func (e *env) cli(method, path, token string, body any) reply {
	e.t.Helper()
	h := map[string]string{versionHeader: "0.1.0"}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return e.do(method, cliPrefix+path, h, body)
}

// grant is a believable portal request for a fresh candidate and assessment.
func newGrant() map[string]any {
	return map[string]any{
		"candidate_id": auth.NewUUID(), "company_id": auth.NewUUID(),
		"assessment_id": auth.NewUUID(), "invitation_id": auth.NewUUID(),
		"role": "backend", "assessment_title": "Support backend",
		"jd": "backend engineer for our customer support helpdesk saas, tickets and sla",
	}
}

var linuxDevice = map[string]any{"os": "linux", "arch": "amd64"}

// startSession mints a code and exchanges it, returning the token and session.
func (e *env) startSession(g map[string]any) (token string, sessionID string) {
	e.t.Helper()
	m := e.portal("POST", "/v2/portal/setup-codes", g)
	if m.Status != http.StatusCreated {
		e.t.Fatalf("mint: %d %s", m.Status, m.Raw)
	}
	ex := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": m.str("setup_code"), "device": linuxDevice})
	if ex.Status != 200 {
		e.t.Fatalf("exchange: %d %s", ex.Status, ex.Raw)
	}
	sess, _ := ex.JSON()["session"].(map[string]any)
	return ex.str("session_token"), sess["id"].(string)
}

// panel returns a panel token for a session the candidate owns.
func (e *env) panelToken(g map[string]any, sessionID string) string {
	e.t.Helper()
	r := e.portal("POST", "/v2/portal/sessions/"+sessionID+"/panel-token", map[string]any{"candidate_id": g["candidate_id"]})
	if r.Status != 200 {
		e.t.Fatalf("panel token: %d %s", r.Status, r.Raw)
	}
	return r.str("token")
}

// panel calls a /v2/panel route as the browser.
func (e *env) panel(method, path, token string, body any) reply {
	e.t.Helper()
	return e.do(method, path, map[string]string{"Authorization": "Bearer " + token}, body)
}

// upload sends raw bytes to a panel upload route.
func (e *env) upload(what, token, contentType string, data []byte) reply {
	e.t.Helper()
	return e.do("POST", "/v2/panel/upload/"+what, map[string]string{"Authorization": "Bearer " + token, "Content-Type": contentType}, data)
}

// shareAll reports everything on, as the panel does once the candidate allows it.
func (e *env) shareAll(token string) reply {
	return e.panel("POST", "/v2/panel/status", token, map[string]any{"camera": true, "screen": true, "audio": true})
}
