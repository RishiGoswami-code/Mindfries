package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/mw"
	"github.com/mindfries/v2/api/internal/policy"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
	"github.com/mindfries/v2/cli/install"
)

// B2: how a candidate gets from the portal's Start assessment button to a
// session the CLI can work against.
//
//	portal server --setup code--> candidate runs one command --> CLI exchanges the code
//	CLI --device code--> candidate approves in the browser --> CLI collects the session
//
// Either way the result is one session per candidate per assessment; running
// the flow again resumes it with a fresh token instead of starting a second.

// DeviceStore is the browser sign-in's storage (implemented by store.DB).
type DeviceStore interface {
	CreateDevice(ctx context.Context, d auth.Device) error
	DeviceByHash(ctx context.Context, hash string) (auth.Device, error)
	DeviceByUserCode(ctx context.Context, userCode string) (auth.Device, error)
	TouchDevice(ctx context.Context, hash string, now time.Time) (*time.Time, error)
	ApproveDevice(ctx context.Context, userCode, candidateID string, g session.Grant, now time.Time) (bool, error)
	DenyDevice(ctx context.Context, userCode string, now time.Time) (bool, error)
	UseDevice(ctx context.Context, hash, sessionID string) (bool, error)
}

// ConfigStore is the per-assessment configuration (implemented by store.DB).
type ConfigStore interface {
	AssessmentConfig(ctx context.Context, assessmentID string) (policy.Config, bool, error)
	SaveAssessmentConfig(ctx context.Context, c policy.Config, by string) error
}

const (
	setupCodeTTL = 10 * time.Minute
	deviceTTL    = 10 * time.Minute
	pollInterval = 5
)

func (s *Server) registerEntry(mux *http.ServeMux) {
	if s.sessions == nil {
		return
	}
	perIP := func(l *mw.Limiter, h http.HandlerFunc) http.Handler {
		return mw.Limit(l, func(r *http.Request) string { return mw.ClientIP(r, s.cfg.TrustProxy) }, h)
	}
	slow := mw.NewLimiter(20.0/60, 20) // 20 a minute: exchange, device start
	poll := mw.NewLimiter(40.0/60, 40) // 40 a minute: device polls at 5 s are 12
	mux.Handle("POST "+cliPrefix+"/sessions/exchange", perIP(slow, s.cliExchange))
	mux.Handle("POST "+cliPrefix+"/sessions/device", perIP(slow, s.cliDeviceStart))
	mux.Handle("POST "+cliPrefix+"/sessions/device/token", perIP(poll, s.cliDevicePoll))

	// The portal route is for a signed-in candidate pressing Start assessment: it
	// must name the candidate and the assessment. The admin route keeps the
	// older shape the admin portal and dashboard already call: those links are
	// optional and the code lasts an hour.
	mux.HandleFunc("POST /v2/portal/setup-codes", s.portalOnly(s.mintCode(true)))
	mux.HandleFunc("POST /v2/admin/setup-codes", s.adminOnly(s.mintCode(false)))
	mux.HandleFunc("GET /v2/portal/device/{user_code}", s.portalOnly(s.portalDeviceInfo))
	mux.HandleFunc("POST /v2/portal/device/{user_code}/approve", s.portalOnly(s.portalDeviceApprove))
	mux.HandleFunc("POST /v2/portal/device/{user_code}/deny", s.portalOnly(s.portalDeviceDeny))

	mux.HandleFunc("GET /s/{file}", s.setupScript)
}

// ---- the session a grant produces --------------------------------------

type grantError struct {
	status       int
	code, detail string
}

// sessionFromGrant makes the candidate's session for a grant, or resumes the
// one they already have for the assessment. It returns the session with a
// fresh bearer token in Token (the only time a token is readable).
func (s *Server) sessionFromGrant(ctx context.Context, g session.Grant) (session.Session, *grantError) {
	token := auth.NewSecret()
	if live, ok, err := s.sessions.LiveSessionFor(ctx, g.Links.CandidateID, g.AssessmentID); err != nil {
		return session.Session{}, &grantError{http.StatusInternalServerError, "internal", err.Error()}
	} else if ok {
		if err := s.sessions.RotateToken(ctx, live.ID, token); err != nil {
			return session.Session{}, &grantError{http.StatusInternalServerError, "internal", err.Error()}
		}
		live.Token = token
		return live, nil
	}

	cfg := policy.Default()
	if s.configs != nil && g.AssessmentID != "" {
		c, _, err := s.configs.AssessmentConfig(ctx, g.AssessmentID)
		if err != nil {
			return session.Session{}, &grantError{http.StatusInternalServerError, "internal", err.Error()}
		}
		cfg = c
	}
	jd := g.JD
	if jd == "" {
		jd = cfg.JD
	}
	minutes := g.DurationMinutes
	if minutes <= 0 {
		minutes = cfg.DurationMinutes
	}
	rt, err := s.resolveFor(ctx, g.Role, jd)
	if err != nil {
		return session.Session{}, &grantError{http.StatusUnprocessableEntity, "no_task", err.Error()}
	}
	now := s.now()
	sess := session.Session{
		ID:              "ses_" + randHex(12),
		Token:           token,
		Links:           g.Links,
		AssessmentID:    g.AssessmentID,
		AssessmentTitle: g.AssessmentTitle,
		Role:            g.Role,
		JDHash:          engine.HashJD(jd),
		ProductSlug:     rt.ProductSlug,
		BankTaskID:      rt.BankTaskID,
		TaskVersion:     rt.TaskVersion,
		Brief:           rt.Brief,
		DurationMinutes: minutes,
		Status:          "ready", // the clock starts at begin, not at setup
		StartedAt:       now,
		EndsAt:          now.Add(time.Duration(minutes) * time.Minute),
	}
	if err := s.sessions.CreateSession(ctx, sess); err != nil {
		return session.Session{}, &grantError{http.StatusInternalServerError, "internal", err.Error()}
	}
	return sess, nil
}

func (s *Server) writeGrantError(w http.ResponseWriter, e *grantError) {
	problem(w, e.status, e.code, e.detail)
}

func (s *Server) exchangeBody(ctx context.Context, sess session.Session) map[string]any {
	acked, _ := s.sessions.LastAckedSeq(ctx, sess.ID)
	return map[string]any{
		"session_token": sess.Token,
		"expires_at":    sess.EndsAt,
		"session":       s.wireSession(ctx, sess, acked),
	}
}

// ---- GET /version, POST /sessions/exchange, GET /session -----------------

func (s *Server) cliVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"minimum_version": s.cfg.MinCLIVersion, "latest_version": s.cfg.LatestCLIVersion,
	})
}

type deviceBody struct {
	OS        string `json:"os"`
	OSVersion string `json:"os_version"`
	Arch      string `json:"arch"`
}

func (d deviceBody) valid() bool {
	return (d.OS == "windows" || d.OS == "darwin" || d.OS == "linux") &&
		(d.Arch == "amd64" || d.Arch == "arm64") && len(d.OSVersion) <= 64
}

func (s *Server) cliExchange(w http.ResponseWriter, r *http.Request) {
	if !s.checkVersion(w, r) {
		return
	}
	var req struct {
		SetupCode string     `json:"setup_code"`
		Device    deviceBody `json:"device"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil || !req.Device.valid() {
		problem(w, http.StatusBadRequest, "malformed", "setup_code and a device (os, arch) are required")
		return
	}
	code, err := s.sessions.TakeSetupCode(r.Context(), auth.NormaliseCode(req.SetupCode), s.now())
	switch {
	case errors.Is(err, session.ErrNotFound):
		problem(w, http.StatusGone, "setup_code_invalid", "setup code not found")
		return
	case errors.Is(err, session.ErrCodeUnusable):
		problem(w, http.StatusGone, "setup_code_used", "setup code was already used")
		return
	case errors.Is(err, session.ErrCodeExpired):
		problem(w, http.StatusGone, "setup_code_expired", "setup code has expired")
		return
	case err != nil:
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	sess, gerr := s.sessionFromGrant(r.Context(), code.Grant())
	if gerr != nil {
		s.writeGrantError(w, gerr)
		return
	}
	writeJSON(w, http.StatusOK, s.exchangeBody(r.Context(), sess))
}

func (s *Server) cliSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authed(w, r)
	if !ok {
		return
	}
	acked, _ := s.sessions.LastAckedSeq(r.Context(), sess.ID)
	writeJSON(w, http.StatusOK, s.wireSession(r.Context(), sess, acked))
}

// ---- the browser sign-in (RFC 8628) --------------------------------------

func (s *Server) portalBase(r *http.Request) string {
	if s.cfg.PortalURL != "" {
		return s.cfg.PortalURL
	}
	return s.publicBase(r)
}

func (s *Server) cliDeviceStart(w http.ResponseWriter, r *http.Request) {
	if s.devices == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "browser sign-in is not set up on this server")
		return
	}
	if !s.checkVersion(w, r) {
		return
	}
	var req struct {
		Device deviceBody `json:"device"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil || !req.Device.valid() {
		problem(w, http.StatusBadRequest, "malformed", "a device (os, arch) is required")
		return
	}
	secret, userCode := auth.NewSecret(), auth.NewUserCode()
	exp := s.now().Add(deviceTTL)
	dev := auth.Device{
		CodeHash: auth.Hash(secret), UserCode: userCode, ExpiresAt: exp, Interval: pollInterval,
		Info: auth.DeviceInfo{OS: req.Device.OS, OSVersion: req.Device.OSVersion, Arch: req.Device.Arch},
	}
	if err := s.devices.CreateDevice(r.Context(), dev); err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	page := s.portalBase(r) + "/v2/device"
	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               secret,
		"user_code":                 userCode,
		"verification_uri":          page,
		"verification_uri_complete": page + "?code=" + url.QueryEscape(userCode),
		"expires_at":                exp,
		"interval_seconds":          pollInterval,
	})
}

func (s *Server) cliDevicePoll(w http.ResponseWriter, r *http.Request) {
	if s.devices == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "browser sign-in is not set up on this server")
		return
	}
	if !s.checkVersion(w, r) {
		return
	}
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&req); err != nil || req.DeviceCode == "" {
		problem(w, http.StatusBadRequest, "malformed", "device_code is required")
		return
	}
	ctx, now := r.Context(), s.now()
	hash := auth.Hash(req.DeviceCode)
	dev, err := s.devices.DeviceByHash(ctx, hash)
	if errors.Is(err, store.ErrDeviceNotFound) {
		problem(w, http.StatusGone, "device_code_invalid", "unknown device code")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	prev, err := s.devices.TouchDevice(ctx, hash, now)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if prev != nil && now.Sub(*prev) < time.Duration(float64(dev.Interval)*0.8*float64(time.Second)) {
		problem(w, http.StatusBadRequest, "slow_down", "poll less often")
		return
	}
	switch dev.Status {
	case "denied":
		problem(w, http.StatusForbidden, "access_denied", "the candidate refused the sign-in")
		return
	case "used":
		problem(w, http.StatusGone, "device_code_used", "this sign-in was already used")
		return
	case "pending":
		if now.After(dev.ExpiresAt) {
			problem(w, http.StatusGone, "device_code_expired", "the sign-in expired; run mindfries start again")
			return
		}
		problem(w, http.StatusBadRequest, "authorization_pending", "waiting for the candidate")
		return
	}
	// Approved. Make (or resume) the session, then spend the code once.
	if dev.Grant == nil {
		problem(w, http.StatusInternalServerError, "internal", "approved sign-in has no grant")
		return
	}
	sess, gerr := s.sessionFromGrant(ctx, *dev.Grant)
	if gerr != nil {
		s.writeGrantError(w, gerr)
		return
	}
	won, err := s.devices.UseDevice(ctx, hash, sess.ID)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !won { // another poll collected it first
		problem(w, http.StatusGone, "device_code_used", "this sign-in was already used")
		return
	}
	writeJSON(w, http.StatusOK, s.exchangeBody(ctx, sess))
}

// ---- the portals' routes -------------------------------------------------

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// grantReq is what a portal's server sends to say "this candidate may start
// this assessment". It's the same body for a setup code and a sign-in.
type grantReq struct {
	Code            string `json:"code"` // admin route only: a chosen code
	CandidateID     string `json:"candidate_id"`
	CompanyID       string `json:"company_id"`
	AssessmentID    string `json:"assessment_id"`
	InvitationID    string `json:"invitation_id"`
	Role            string `json:"role"`
	JD              string `json:"jd"`
	AssessmentTitle string `json:"assessment_title"`
	DurationMinutes int    `json:"duration_minutes"`
}

// grant validates r and returns the Grant, or says what's wrong. strict is
// the portal's rule: the candidate and the assessment must both be named.
func (r grantReq) grant(strict bool) (session.Grant, string) {
	check := func(name, v string, required bool) string {
		if v == "" && !required {
			return ""
		}
		if !uuidRe.MatchString(v) {
			return name + " must be a UUID"
		}
		return ""
	}
	for _, c := range []struct {
		name, v string
		req     bool
	}{
		{"candidate_id", r.CandidateID, strict}, {"company_id", r.CompanyID, false}, {"invitation_id", r.InvitationID, false},
	} {
		if why := check(c.name, c.v, c.req); why != "" {
			return session.Grant{}, why
		}
	}
	if strict && r.AssessmentID == "" {
		return session.Grant{}, "assessment_id is required"
	}
	if len(r.AssessmentID) > 64 {
		return session.Grant{}, "assessment_id is too long"
	}
	if !core.ValidRole(core.Role(r.Role)) {
		return session.Grant{}, "role is not one of the supported roles"
	}
	if r.DurationMinutes != 0 && (r.DurationMinutes < 10 || r.DurationMinutes > 600) {
		return session.Grant{}, "duration_minutes must be between 10 and 600"
	}
	return session.Grant{
		Links:        session.Links{CandidateID: r.CandidateID, CompanyID: r.CompanyID, InvitationID: r.InvitationID},
		AssessmentID: r.AssessmentID,
		Role:         r.Role, JD: r.JD, AssessmentTitle: r.AssessmentTitle, DurationMinutes: r.DurationMinutes,
	}, ""
}

// mintCode is the Start assessment button's server half: it checks that a
// product covers the role and job description (resolve-at-invite, so a code
// is never issued for an assessment the candidate couldn't start; this also
// warms the task cache), stores a one-time setup code, and returns it with the
// one-line command for each platform.
func (s *Server) mintCode(strict bool) http.HandlerFunc {
	ttl := time.Hour
	if strict {
		ttl = setupCodeTTL
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var req grantReq
		if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&req); err != nil {
			problem(w, http.StatusBadRequest, "malformed", "invalid request body")
			return
		}
		g, why := req.grant(strict)
		if why != "" {
			problem(w, http.StatusBadRequest, "malformed", why)
			return
		}
		jd := g.JD
		if jd == "" && s.configs != nil && g.AssessmentID != "" {
			if c, _, err := s.configs.AssessmentConfig(r.Context(), g.AssessmentID); err == nil {
				jd = c.JD
			}
		}
		rt, err := s.resolveFor(r.Context(), g.Role, jd)
		if err != nil {
			problem(w, http.StatusUnprocessableEntity, "no_coverage", err.Error())
			return
		}
		code := auth.NewSetupCode()
		if !strict && req.Code != "" {
			code = auth.NormaliseCode(req.Code)
		}
		c := session.SetupCode{
			Code: code, AssessmentID: g.AssessmentID, AssessmentTitle: g.AssessmentTitle, Role: g.Role, JD: g.JD,
			DurationMinutes: g.DurationMinutes, ExpiresAt: s.now().Add(ttl), Links: g.Links,
		}
		if c.DurationMinutes == 0 && !strict {
			c.DurationMinutes = 90
		}
		if err := s.sessions.CreateSetupCode(r.Context(), c); err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		base := s.publicBase(r)
		writeJSON(w, http.StatusCreated, map[string]any{
			"setup_code":      code,
			"expires_at":      c.ExpiresAt,
			"windows_command": "irm " + base + "/s/" + code + ".ps1 | iex",
			"unix_command":    "curl -fsSL " + base + "/s/" + code + ".sh | sh",
			"cli_command":     "mindfries setup " + code,
			"product_slug":    rt.ProductSlug, "task_title": rt.Title, "task_kind": rt.Kind,
		})
	}
}

func (s *Server) portalDeviceInfo(w http.ResponseWriter, r *http.Request) {
	if s.devices == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "browser sign-in is not set up")
		return
	}
	dev, err := s.devices.DeviceByUserCode(r.Context(), auth.NormaliseCode(r.PathValue("user_code")))
	if errors.Is(err, store.ErrDeviceNotFound) {
		problem(w, http.StatusNotFound, "not_found", "no such sign-in")
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	status := dev.Status
	if status == "pending" && s.now().After(dev.ExpiresAt) {
		status = "expired"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_code": dev.UserCode, "status": status, "expires_at": dev.ExpiresAt,
		"device": map[string]string{"os": dev.Info.OS, "os_version": dev.Info.OSVersion, "arch": dev.Info.Arch},
	})
}

func (s *Server) portalDeviceApprove(w http.ResponseWriter, r *http.Request) {
	if s.devices == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "browser sign-in is not set up")
		return
	}
	var req grantReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 256<<10)).Decode(&req); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "invalid request body")
		return
	}
	g, why := req.grant(true)
	if why != "" {
		problem(w, http.StatusBadRequest, "malformed", why)
		return
	}
	ok, err := s.devices.ApproveDevice(r.Context(), auth.NormaliseCode(r.PathValue("user_code")), g.Links.CandidateID, g, s.now())
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		problem(w, http.StatusConflict, "device_code_invalid", "that sign-in is expired, already decided, or unknown")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) portalDeviceDeny(w http.ResponseWriter, r *http.Request) {
	if s.devices == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "browser sign-in is not set up")
		return
	}
	ok, err := s.devices.DenyDevice(r.Context(), auth.NormaliseCode(r.PathValue("user_code")), s.now())
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		problem(w, http.StatusConflict, "device_code_invalid", "that sign-in is expired, already decided, or unknown")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- setup scripts and the CLI's downloads --------------------------------

var codeInPath = regexp.MustCompile(`^[A-Z0-9]{4}(-[A-Z0-9]{4}){2}$`)

// downloadBase is where the installers fetch builds from: the configured
// release address, or this server's own /cli/latest.
func (s *Server) downloadBase(r *http.Request) string {
	if s.cfg.CLIDownloadURL != "" {
		return s.cfg.CLIDownloadURL
	}
	return s.publicBase(r) + "/cli/latest"
}

func (s *Server) hostsBuilds() bool { return s.cfg.CLIDistDir != "" }

// canInstall reports whether there's somewhere for an installer to download
// the CLI from.
func (s *Server) canInstall() bool { return s.cfg.CLIDownloadURL != "" || s.hostsBuilds() }

// setupScript serves the installer with the setup code in it, for the
// dashboard's one command. Only a live, unused code gets one.
func (s *Server) setupScript(w http.ResponseWriter, r *http.Request) {
	code, ext, ok := strings.Cut(r.PathValue("file"), ".")
	code = strings.ToUpper(code)
	if !ok || (ext != "ps1" && ext != "sh") || !codeInPath.MatchString(code) {
		http.NotFound(w, r)
		return
	}
	if !s.canInstall() {
		problem(w, http.StatusServiceUnavailable, "not_configured", "no CLI downloads are set up (V2_CLI_DOWNLOAD_URL or V2_CLI_DIST_DIR)")
		return
	}
	live, err := s.sessions.SetupCodeLive(r.Context(), code, s.now())
	if err != nil {
		http.Error(w, "try again in a moment", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !live {
		w.WriteHeader(http.StatusGone)
		const msg = "This Mindfries setup command has expired or was already used. Press Start assessment on your dashboard again."
		if ext == "ps1" {
			fmt.Fprintf(w, "Write-Error '%s'\n", msg)
		} else {
			fmt.Fprintf(w, "echo '%s' >&2; exit 1\n", msg)
		}
		return
	}
	var body string
	if ext == "ps1" {
		body = strings.Replace(install.PowerShell, `param([string]$Code = "")`, `param([string]$Code = "`+code+`")`, 1)
	} else {
		body = strings.Replace(install.Shell, `CODE="${1:-}"`, `CODE="${1:-`+code+`}"`, 1)
	}
	_, _ = io.WriteString(w, strings.ReplaceAll(body, install.DefaultDownload, s.downloadBase(r)))
}
