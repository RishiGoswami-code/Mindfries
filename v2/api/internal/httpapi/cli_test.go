package httpapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/bundle"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/taskgen"
	"github.com/mindfries/v2/contracts/canonical"
)

func cliServer(t *testing.T) (*httptest.Server, *session.Mem) {
	root := productsRoot(t)
	reg := loadReg(t, root, "p1-support-saas", "p2-rag-platform", "p3-quant-platform")
	mem := session.NewMem()
	return httptest.NewServer(New(reg, root, taskgen.NewMemCache(), nil, 3, mem)), mem
}

// req makes a CLI request with the version header and optional bearer token.
func req(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = bytes.NewBufferString(body)
	}
	hr, _ := http.NewRequest(method, url, r)
	hr.Header.Set(versionHeader, "0.1.0")
	if token != "" {
		hr.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestCLIHappyPath(t *testing.T) {
	srv, mem := cliServer(t)
	defer srv.Close()
	base := srv.URL + cliPrefix

	// Dashboard mints a setup code for a backend/support assessment.
	mem.CreateSetupCode(context.Background(), session.SetupCode{
		Code: "ABCD-1234-WXYZ", AssessmentTitle: "Support backend",
		Role: "backend", JD: "backend engineer for our customer support helpdesk saas, tickets and sla",
		DurationMinutes: 90, ExpiresAt: time.Now().Add(time.Hour),
	})

	// 1. version
	res := req(t, "GET", base+"/version", "", "")
	if res.StatusCode != 200 {
		t.Fatalf("version = %d", res.StatusCode)
	}
	res.Body.Close()

	// 2. exchange the setup code for a session token
	res = req(t, "POST", base+"/sessions/exchange", "", `{"setup_code":"ABCD-1234-WXYZ","device":{"os":"linux","arch":"amd64"}}`)
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("exchange = %d: %s", res.StatusCode, b)
	}
	var ex struct {
		SessionToken string         `json:"session_token"`
		Session      map[string]any `json:"session"`
	}
	json.NewDecoder(res.Body).Decode(&ex)
	res.Body.Close()
	if ex.SessionToken == "" {
		t.Fatal("no session token")
	}
	if ex.Session["status"] != "active" {
		t.Fatalf("session not active: %v", ex.Session["status"])
	}
	tok := ex.SessionToken

	// Reusing a setup code must fail.
	res = req(t, "POST", base+"/sessions/exchange", "", `{"setup_code":"ABCD-1234-WXYZ"}`)
	if res.StatusCode != http.StatusGone {
		t.Fatalf("reused code should be 410, got %d", res.StatusCode)
	}
	res.Body.Close()

	// 3. GET /session
	res = req(t, "GET", base+"/session", tok, "")
	if res.StatusCode != 200 {
		t.Fatalf("session = %d", res.StatusCode)
	}
	res.Body.Close()

	// 4. bundle manifest — signed, includes the real P1 task
	res = req(t, "GET", base+"/session/bundle", tok, "")
	if res.StatusCode != 200 {
		t.Fatalf("bundle = %d", res.StatusCode)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	var m bundle.Manifest
	json.Unmarshal(raw, &m)
	if m.Signature == nil || m.Signature.KeyID != bundle.DevKeyID {
		t.Fatalf("manifest not signed: %+v", m.Signature)
	}
	hasTask, hasSrc := false, false
	for _, f := range m.Files {
		if f.Path == "TASK.md" && f.Locked {
			hasTask = true
		}
		if f.Path == "internal/support/tickets.go" {
			hasSrc = true
		}
	}
	if !hasTask || !hasSrc {
		t.Fatal("bundle should contain a locked TASK.md and the P1 product source")
	}
	// The signature must verify the way the CLI does.
	if !manifestVerifies(raw) {
		t.Fatal("manifest signature would be rejected by the CLI")
	}

	// 5. archive — bytes hash to the manifest's archive_sha256
	res = req(t, "GET", base+"/session/bundle/archive", tok, "")
	archive, _ := io.ReadAll(res.Body)
	res.Body.Close()
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != m.ArchiveSHA256 {
		t.Fatal("archive bytes don't match the manifest's archive_sha256")
	}

	// 6. heartbeat
	res = req(t, "POST", base+"/session/heartbeat", tok, `{"sent_at":"2026-01-01T00:00:00Z","watcher":{"queue_depth":0,"last_sent_seq":0}}`)
	var hb map[string]any
	json.NewDecoder(res.Body).Decode(&hb)
	res.Body.Close()
	if hb["session_status"] != "active" {
		t.Fatalf("heartbeat status = %v", hb["session_status"])
	}

	// 7. events ack
	res = req(t, "POST", base+"/session/events", tok, `{"events":[{"seq":1},{"seq":2}]}`)
	var ack map[string]any
	json.NewDecoder(res.Body).Decode(&ack)
	res.Body.Close()
	if ack["acked_through_seq"].(float64) != 2 {
		t.Fatalf("acked_through_seq = %v", ack["acked_through_seq"])
	}
}

func TestCLIAuthAndVersionGuards(t *testing.T) {
	srv, _ := cliServer(t)
	defer srv.Close()
	base := srv.URL + cliPrefix

	// Missing version header -> 400.
	hr, _ := http.NewRequest("GET", base+"/session", nil)
	hr.Header.Set("Authorization", "Bearer x")
	res, _ := http.DefaultClient.Do(hr)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing version header should be 400, got %d", res.StatusCode)
	}
	res.Body.Close()

	// Bad token -> 401.
	res = req(t, "GET", base+"/session", "nope", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token should be 401, got %d", res.StatusCode)
	}
	res.Body.Close()
}

func manifestVerifies(raw []byte) bool {
	var env struct {
		Signature *bundle.Signature `json:"signature"`
	}
	json.Unmarshal(raw, &env)
	if env.Signature == nil {
		return false
	}
	var asMap map[string]json.RawMessage
	json.Unmarshal(raw, &asMap)
	delete(asMap, "signature")
	msg, err := canonical.Marshal(asMap)
	if err != nil {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature.Value)
	if err != nil {
		return false
	}
	return ed25519.Verify(bundle.DevKey().Public().(ed25519.PublicKey), msg, sig)
}
