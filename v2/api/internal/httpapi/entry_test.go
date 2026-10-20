package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/policy"
)

func TestPortalRoutesNeedTheSecret(t *testing.T) {
	e := newEnv(t, nil)
	if r := e.do("POST", "/v2/portal/setup-codes", nil, newGrant()); r.Status != http.StatusForbidden {
		t.Fatalf("no secret: %d", r.Status)
	}
	if r := e.do("POST", "/v2/portal/setup-codes", map[string]string{"X-Portal-Secret": "wrong"}, newGrant()); r.Status != http.StatusForbidden {
		t.Fatalf("wrong secret: %d", r.Status)
	}
	if r := e.do("POST", "/v2/admin/setup-codes", map[string]string{"X-Portal-Secret": testPortalSecret}, newGrant()); r.Status != http.StatusForbidden {
		t.Fatalf("the portal secret is not the admin token: %d", r.Status)
	}
	if r := e.do("POST", "/v2/admin/setup-codes", map[string]string{"X-Admin-Token": testAdminToken}, newGrant()); r.Status != http.StatusCreated {
		t.Fatalf("admin: %d %s", r.Status, r.Raw)
	}

	off := newEnv(t, func(c *config.Config) { c.PortalSecret = "" })
	if r := off.do("POST", "/v2/portal/setup-codes", map[string]string{"X-Portal-Secret": ""}, newGrant()); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("with no secret configured the routes are off, not open: %d", r.Status)
	}
}

func TestMintValidatesWhatThePortalSends(t *testing.T) {
	e := newEnv(t, nil)
	bad := []map[string]any{
		{"candidate_id": "not-a-uuid"},
		func() map[string]any { g := newGrant(); g["role"] = "astronaut"; return g }(),
		func() map[string]any { g := newGrant(); g["assessment_id"] = ""; return g }(),
		func() map[string]any { g := newGrant(); g["duration_minutes"] = 3; return g }(),
	}
	for i, g := range bad {
		if r := e.portal("POST", "/v2/portal/setup-codes", g); r.Status != http.StatusBadRequest || r.str("code") != "malformed" {
			t.Errorf("case %d: %d %s", i, r.Status, r.Raw)
		}
	}
}

func TestSetupCodeToSession(t *testing.T) {
	e := newEnv(t, func(c *config.Config) {
		c.GitHubClientID, c.GitHubClientSecret = "id", "secret"
		c.CLIDistDir = t.TempDir()
	})
	g := newGrant()
	asmt := g["assessment_id"].(string)

	// The company has configured this assessment: reviewers, time, policy.
	cfg := policy.Default()
	cfg.AssessmentID = asmt
	cfg.DurationMinutes = 75
	cfg.AssistantMode = policy.CodeAllowed
	cfg.Reviewers = []string{"@hiring-manager", "tech-lead"}
	cfg.RequireAudio = false
	if err := e.db.SaveAssessmentConfig(t.Context(), cfg, "test"); err != nil {
		t.Fatal(err)
	}

	m := e.portal("POST", "/v2/portal/setup-codes", g)
	if m.Status != http.StatusCreated {
		t.Fatalf("%d %s", m.Status, m.Raw)
	}
	code := m.str("setup_code")
	if !codeInPath.MatchString(code) {
		t.Fatalf("code %q", code)
	}
	if !strings.Contains(m.str("windows_command"), "https://api.test/s/"+code+".ps1") ||
		!strings.Contains(m.str("unix_command"), "https://api.test/s/"+code+".sh") ||
		m.str("cli_command") != "mindfries setup "+code {
		t.Fatalf("commands: %s", m.Raw)
	}

	// The install script is served with the code in it, but only while it's live.
	ps := e.do("GET", "/s/"+code+".ps1", nil, nil)
	sh := e.do("GET", "/s/"+code+".sh", nil, nil)
	if ps.Status != 200 || !strings.Contains(string(ps.Raw), `param([string]$Code = "`+code+`")`) {
		t.Fatalf("ps1: %d %.200s", ps.Status, ps.Raw)
	}
	if sh.Status != 200 || !strings.Contains(string(sh.Raw), `CODE="${1:-`+code+`}"`) {
		t.Fatalf("sh: %d %.200s", sh.Status, sh.Raw)
	}
	if strings.Contains(string(ps.Raw), "get.v2.mindfries.example") || !strings.Contains(string(ps.Raw), "https://api.test/cli/latest") {
		t.Fatal("the script must download from this server, not the placeholder")
	}
	if ps.Header.Get("Cache-Control") != "no-store" {
		t.Error("a script with a code in it must not be cached")
	}

	// Exchange it.
	ex := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": strings.ToLower(code), "device": linuxDevice})
	if ex.Status != 200 {
		t.Fatalf("exchange: %d %s", ex.Status, ex.Raw)
	}
	tok := ex.str("session_token")
	if len(tok) != 64 {
		t.Fatalf("token %q", tok)
	}
	s := ex.JSON()["session"].(map[string]any)
	if s["status"] != "ready" || s["assessment_id"] != asmt || s["duration_minutes"].(float64) != 75 {
		t.Fatalf("session: %v", s)
	}
	pol := s["policy"].(map[string]any)
	if pol["assistant_mode"] != "code_allowed" || pol["outside_agents_allowed"] != false {
		t.Fatalf("policy: %v", pol)
	}
	mon := s["monitoring"].(map[string]any)
	if mon["camera"] != true || mon["screen"] != true || mon["audio"] != false ||
		mon["panel_url"] != "https://portal.test/v2/session/"+s["id"].(string) {
		t.Fatalf("monitoring: %v", mon)
	}
	gh := s["github"].(map[string]any)
	revs := gh["reviewers"].([]any)
	if gh["required"] != true || len(revs) != 2 || revs[0] != "hiring-manager" || revs[1] != "tech-lead" || gh["repo_name"] != "mindfries-support-backend" {
		t.Fatalf("github: %v", gh)
	}

	// A used code is used, and its script now says so.
	if r := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": code, "device": linuxDevice}); r.Status != http.StatusGone || r.str("code") != "setup_code_used" {
		t.Fatalf("reuse: %d %s", r.Status, r.Raw)
	}
	if r := e.do("GET", "/s/"+code+".sh", nil, nil); r.Status != http.StatusGone || !strings.Contains(string(r.Raw), "exit 1") {
		t.Fatalf("a used code's script: %d %s", r.Status, r.Raw)
	}

	// The token works; a reviewer added later shows without a new session.
	if r := e.cli("GET", "/session", tok, nil); r.Status != 200 {
		t.Fatalf("GET /session: %d %s", r.Status, r.Raw)
	}
	cfg.Reviewers = append(cfg.Reviewers, "late-addition")
	if err := e.db.SaveAssessmentConfig(t.Context(), cfg, "test"); err != nil {
		t.Fatal(err)
	}
	r := e.cli("GET", "/session", tok, nil)
	got := r.JSON()["github"].(map[string]any)["reviewers"].([]any)
	if len(got) != 3 {
		t.Fatalf("a reviewer added mid-session must show: %v", got)
	}
}

func TestExpiredSetupCode(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.CLIDistDir = t.TempDir() })
	m := e.portal("POST", "/v2/portal/setup-codes", newGrant())
	code := m.str("setup_code")
	e.advance(setupCodeTTL + time.Minute)
	if r := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": code, "device": linuxDevice}); r.Status != http.StatusGone || r.str("code") != "setup_code_expired" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.do("GET", "/s/"+code+".sh", nil, nil); r.Status != http.StatusGone {
		t.Fatalf("an expired code's script must not install anything: %d", r.Status)
	}
	if r := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": "WXYZ-WXYZ-WXYZ", "device": linuxDevice}); r.str("code") != "setup_code_invalid" {
		t.Fatalf("unknown: %s", r.Raw)
	}
}

func TestExchangeNeedsADeviceAndANewEnoughCLI(t *testing.T) {
	e := newEnv(t, func(c *config.Config) { c.MinCLIVersion = "0.2.0" })
	if r := e.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": "ABCD-EFGH-JKMN"}); r.Status != http.StatusUpgradeRequired || r.str("code") != "cli_too_old" || r.str("minimum_version") != "0.2.0" {
		t.Fatalf("an old CLI: %d %s", r.Status, r.Raw)
	}
	ok := newEnv(t, nil)
	if r := ok.cli("POST", "/sessions/exchange", "", map[string]any{"setup_code": "ABCD-EFGH-JKMN"}); r.Status != http.StatusBadRequest || r.str("code") != "malformed" {
		t.Fatalf("no device: %d %s", r.Status, r.Raw)
	}
	if r := ok.do("POST", cliPrefix+"/sessions/exchange", nil, `{}`); r.Status != http.StatusBadRequest {
		t.Fatalf("no version header: %d", r.Status)
	}
	if r := ok.cli("GET", "/session", "", nil); r.Status != http.StatusUnauthorized || r.str("code") != "token_missing" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := ok.cli("GET", "/session", "nope", nil); r.Status != http.StatusUnauthorized || r.str("code") != "token_invalid" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := ok.cli("GET", "/version", "", nil); r.Status != 200 || r.str("latest_version") != "0.1.0" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
}

func TestRunningTheFlowAgainResumesTheSameSession(t *testing.T) {
	e := newEnv(t, nil)
	g := newGrant()
	tok1, id1 := e.startSession(g)
	tok2, id2 := e.startSession(g) // the candidate lost their terminal and pressed Start again
	if id1 != id2 {
		t.Fatalf("one session per candidate per assessment, got %s and %s", id1, id2)
	}
	if tok1 == tok2 {
		t.Fatal("a resumed session gets a new token")
	}
	if r := e.cli("GET", "/session", tok1, nil); r.Status != http.StatusUnauthorized {
		t.Fatalf("the old terminal's token must stop working: %d", r.Status)
	}
	if r := e.cli("GET", "/session", tok2, nil); r.Status != 200 {
		t.Fatalf("new token: %d", r.Status)
	}
	// A different candidate on the same assessment is a different session.
	g2 := newGrant()
	g2["assessment_id"] = g["assessment_id"]
	_, id3 := e.startSession(g2)
	if id3 == id1 {
		t.Fatal("another candidate must not share a session")
	}
}

func TestBrowserSignIn(t *testing.T) {
	e := newEnv(t, nil)
	start := e.cli("POST", "/sessions/device", "", map[string]any{"device": map[string]any{"os": "windows", "os_version": "11", "arch": "amd64"}})
	if start.Status != 200 {
		t.Fatalf("start: %d %s", start.Status, start.Raw)
	}
	secret, user := start.str("device_code"), start.str("user_code")
	if len(secret) < 32 || len(user) != 9 || start.str("verification_uri") != "https://portal.test/v2/device" ||
		start.str("verification_uri_complete") != "https://portal.test/v2/device?code="+user || start.JSON()["interval_seconds"].(float64) != 5 {
		t.Fatalf("%s", start.Raw)
	}
	poll := func() reply {
		return e.cli("POST", "/sessions/device/token", "", map[string]any{"device_code": secret})
	}

	if r := poll(); r.Status != 400 || r.str("code") != "authorization_pending" {
		t.Fatalf("pending: %d %s", r.Status, r.Raw)
	}
	if r := poll(); r.Status != 400 || r.str("code") != "slow_down" {
		t.Fatalf("polling again at once must be told to slow down: %d %s", r.Status, r.Raw)
	}
	e.advance(6 * time.Second)

	// The portal shows the candidate which device is asking, then approves.
	info := e.portal("GET", "/v2/portal/device/"+strings.ToLower(user), nil)
	dev := info.JSON()["device"].(map[string]any)
	if info.Status != 200 || info.str("status") != "pending" || dev["os"] != "windows" || dev["os_version"] != "11" {
		t.Fatalf("info: %d %s", info.Status, info.Raw)
	}
	g := newGrant()
	if r := e.portal("POST", "/v2/portal/device/"+user+"/approve", g); r.Status != http.StatusNoContent {
		t.Fatalf("approve: %d %s", r.Status, r.Raw)
	}
	if r := e.portal("POST", "/v2/portal/device/"+user+"/approve", g); r.Status != http.StatusConflict {
		t.Fatalf("approving twice: %d", r.Status)
	}
	if r := e.portal("POST", "/v2/portal/device/"+user+"/deny", nil); r.Status != http.StatusConflict {
		t.Fatalf("an approved sign-in can't be denied: %d", r.Status)
	}

	r := poll()
	if r.Status != 200 || len(r.str("session_token")) != 64 {
		t.Fatalf("collect: %d %s", r.Status, r.Raw)
	}
	s := r.JSON()["session"].(map[string]any)
	if s["status"] != "ready" || s["assessment_id"] != g["assessment_id"] {
		t.Fatalf("session: %v", s)
	}
	e.advance(6 * time.Second)
	if r := poll(); r.Status != http.StatusGone || r.str("code") != "device_code_used" {
		t.Fatalf("a device code is spent once: %d %s", r.Status, r.Raw)
	}
	if r := e.cli("POST", "/sessions/device/token", "", map[string]any{"device_code": "x"}); r.Status != http.StatusGone || r.str("code") != "device_code_invalid" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
}

func TestBrowserSignInDeniedAndExpired(t *testing.T) {
	e := newEnv(t, nil)
	begin := func() (secret, user string) {
		s := e.cli("POST", "/sessions/device", "", map[string]any{"device": linuxDevice})
		return s.str("device_code"), s.str("user_code")
	}
	poll := func(secret string) reply {
		e.advance(6 * time.Second)
		return e.cli("POST", "/sessions/device/token", "", map[string]any{"device_code": secret})
	}

	secret, user := begin()
	if r := e.portal("POST", "/v2/portal/device/"+user+"/deny", nil); r.Status != http.StatusNoContent {
		t.Fatalf("deny: %d", r.Status)
	}
	if r := poll(secret); r.Status != http.StatusForbidden || r.str("code") != "access_denied" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}

	secret, user = begin()
	e.advance(deviceTTL + time.Minute)
	if r := poll(secret); r.Status != http.StatusGone || r.str("code") != "device_code_expired" {
		t.Fatalf("%d %s", r.Status, r.Raw)
	}
	if r := e.portal("POST", "/v2/portal/device/"+user+"/approve", newGrant()); r.Status != http.StatusConflict {
		t.Fatalf("an expired sign-in can't be approved: %d", r.Status)
	}
	if r := e.portal("GET", "/v2/portal/device/"+user, nil); r.str("status") != "expired" {
		t.Fatalf("%s", r.Raw)
	}
	if r := e.cli("POST", "/sessions/device", "", map[string]any{"device": map[string]any{"os": "beos", "arch": "amd64"}}); r.Status != http.StatusBadRequest {
		t.Fatalf("a device that isn't windows, darwin or linux is malformed: %d", r.Status)
	}
}

func TestInstallersNeedADownloadSource(t *testing.T) {
	e := newEnv(t, nil)
	if r := e.do("GET", "/install.sh", nil, nil); r.Status != http.StatusServiceUnavailable {
		t.Fatalf("with nowhere to download from: %d", r.Status)
	}
	e2 := newEnv(t, func(c *config.Config) { c.CLIDownloadURL = "https://github.com/x/releases/latest/download" })
	r := e2.do("GET", "/install.ps1", nil, nil)
	if r.Status != 200 || !strings.Contains(string(r.Raw), "https://github.com/x/releases/latest/download") {
		t.Fatalf("%d %.200s", r.Status, r.Raw)
	}
	if e2.do("GET", "/cli/latest/mindfries_linux_amd64", nil, nil).Status != http.StatusNotFound {
		t.Fatal("this server isn't hosting builds")
	}
}
