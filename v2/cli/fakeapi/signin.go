package fakeapi

import (
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// Browser sign-in, the contract's POST /sessions/device and
// /sessions/device/token: `mindfries start` with no code asks for a sign-in,
// opens the browser at the fake's /device page, and polls until the
// candidate confirms there and picks the assessment they were invited to.
//
// The fake has no accounts: every invitation is offered to whoever signs
// in, with any email. Invitations can be connected any number of times, so
// a restarted deployment (which seeds them again) is always usable.

// invitation is an assessment a candidate was invited to.
type invitation struct {
	id   string
	opts SessionOptions
}

// signIn is one pending browser sign-in.
type signIn struct {
	deviceCode string
	userCode   string
	device     api.Device
	createdAt  time.Time
	expiresAt  time.Time
	interval   time.Duration
	lastPoll   time.Time

	approved *invitation
	email    string
	denied   bool
	used     bool
}

// Invite adds an assessment that signed-in candidates can connect to, as a
// recruiter's invitation does, and returns its ID.
func (s *Server) Invite(o SessionOptions) string {
	o = o.withDefaults()
	s.mu.Lock()
	defer s.mu.Unlock()
	inv := &invitation{id: randomID("inv_", 6), opts: o}
	s.invites = append(s.invites, inv)
	return inv.id
}

// PendingSignIns returns the user codes of sign-ins waiting for the
// browser, for tests.
func (s *Server) PendingSignIns() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, si := range s.signIns {
		if si.approved == nil && !si.denied && s.now().Before(si.expiresAt) {
			out = append(out, si.userCode)
		}
	}
	return out
}

// ApproveSignIn confirms a sign-in as the browser page does. An empty
// invitationID picks the first invitation.
func (s *Server) ApproveSignIn(userCode, invitationID, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.approveLocked(userCode, invitationID, email)
}

// DenySignIn refuses a sign-in, as "This wasn't me" on the page does.
func (s *Server) DenySignIn(userCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	si, err := s.pendingLocked(userCode)
	if err != nil {
		return err
	}
	si.denied = true
	return nil
}

var (
	errNoSignIn      = errors.New("there's no sign-in waiting with that code")
	errSignInExpired = errors.New("that code has expired; run mindfries start again")
	errSignInDone    = errors.New("that code has already been used")
	errNoInvitation  = errors.New("there's no such assessment")
)

func (s *Server) pendingLocked(userCode string) (*signIn, error) {
	userCode = normalizeUserCode(userCode)
	for _, si := range s.signIns {
		if si.userCode != userCode {
			continue
		}
		switch {
		case si.approved != nil || si.denied:
			return nil, errSignInDone
		case !s.now().Before(si.expiresAt):
			return nil, errSignInExpired
		}
		return si, nil
	}
	return nil, errNoSignIn
}

func (s *Server) approveLocked(userCode, invitationID, email string) error {
	si, err := s.pendingLocked(userCode)
	if err != nil {
		return err
	}
	var inv *invitation
	for _, i := range s.invites {
		if invitationID == "" || i.id == invitationID {
			inv = i
			break
		}
	}
	if inv == nil {
		return errNoInvitation
	}
	si.approved = inv
	si.email = email
	return nil
}

// userAlphabet leaves out vowels, so a user code never spells a word, and
// 0/O and 1/I.
const userAlphabet = "BCDFGHJKLMNPQRSTVWXZ23456789"

func newUserCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for i, c := range b {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(userAlphabet[int(c)%len(userAlphabet)])
	}
	return sb.String()
}

func normalizeUserCode(c string) string {
	c = strings.ToUpper(strings.Join(strings.Fields(c), ""))
	c = strings.ReplaceAll(c, "-", "")
	if len(c) == 8 {
		c = c[:4] + "-" + c[4:]
	}
	return c
}

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /sessions/device", s.handleDeviceStart)
		route("POST /sessions/device/token", s.handleDeviceToken)
	})
	registerAdmin(func(s *Server, mux *http.ServeMux) {
		mux.HandleFunc("POST /__admin/invitations", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Title           string `json:"title"`
				DurationMinutes int    `json:"duration_minutes"`
				AssistantMode   string `json:"assistant_mode"`
			}
			if r.ContentLength != 0 && !decode(w, r, &in) {
				return
			}
			o := SessionOptions{Title: in.Title, Duration: time.Duration(in.DurationMinutes) * time.Minute}
			o.Policy.AssistantMode = in.AssistantMode
			writeJSON(w, http.StatusCreated, map[string]string{"invitation_id": s.Invite(o)})
		})
	})
}

// publicBase is the fake's address as the browser sees it.
func (s *Server) publicBase(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) handleDeviceStart(w http.ResponseWriter, r *http.Request) {
	var in api.DeviceRequest
	if !decode(w, r, &in) {
		return
	}
	if in.Device.OS == "" || in.Device.Arch == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "device is required", "")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	si := &signIn{
		deviceCode: randomID("mfd_", 24),
		userCode:   newUserCode(),
		device:     in.Device,
		createdAt:  now,
		expiresAt:  now.Add(10 * time.Minute),
		interval:   s.cfg.SignInInterval,
	}
	s.signIns[si.deviceCode] = si
	page := s.publicBase(r) + "/device"
	secs := int(si.interval / time.Second)
	if secs < 1 {
		secs = 1
	}
	writeJSON(w, http.StatusOK, api.DeviceAuthorization{
		DeviceCode:              si.deviceCode,
		UserCode:                si.userCode,
		VerificationURI:         page,
		VerificationURIComplete: page + "?code=" + si.userCode,
		ExpiresAt:               si.expiresAt,
		IntervalSeconds:         secs,
	})
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	var in api.DeviceTokenRequest
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	si := s.signIns[in.DeviceCode]
	now := s.now()
	switch {
	case si == nil:
		problem(w, http.StatusGone, api.CodeDeviceCodeInvalid, "This sign-in isn't recognised", "")
		return
	case si.used:
		problem(w, http.StatusGone, api.CodeDeviceCodeUsed, "This sign-in has already been used", "")
		return
	case si.denied:
		problem(w, http.StatusForbidden, api.CodeAccessDenied, "The sign-in was refused in the browser", "")
		return
	case si.approved == nil && !now.Before(si.expiresAt):
		problem(w, http.StatusGone, api.CodeDeviceCodeExpired, "This sign-in has expired", "")
		return
	}
	if si.approved == nil {
		// Polling faster than half the interval earns slow_down and a
		// longer interval, as RFC 8628 has it.
		tooFast := !si.lastPoll.IsZero() && now.Sub(si.lastPoll) < si.interval/2
		si.lastPoll = now
		if tooFast {
			si.interval += 5 * time.Second
			problem(w, http.StatusBadRequest, api.CodeSlowDown, "Polling too fast", fmt.Sprintf("wait %d seconds between requests", int(si.interval/time.Second)))
			return
		}
		problem(w, http.StatusBadRequest, api.CodeAuthorizationPending, "Waiting for the candidate to confirm in the browser", "")
		return
	}
	si.used = true
	sess := s.newSession(si.approved.opts)
	writeJSON(w, http.StatusOK, api.ExchangeResponse{SessionToken: sess.token, ExpiresAt: sess.EndsAt, Session: sess.view()})
}

// pageRoutes are what a browser opens: the sign-in page and, with
// Config.DistDir, the installers and CLI builds.
func (s *Server) pageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /device", s.handleDevicePage)
	mux.HandleFunc("POST /device", s.handleDeviceForm)
	s.distRoutes(mux)
}

type invitationView struct {
	ID, Title, Duration, Assistant string
	Checked                        bool
}

type devicePage struct {
	Code        string
	Device      string
	Invitations []invitationView
	Message     string
	Error       string
	Done        bool
}

func (s *Server) handleDevicePage(w http.ResponseWriter, r *http.Request) {
	code := normalizeUserCode(r.URL.Query().Get("code"))
	p := devicePage{Code: code}
	if code != "" {
		s.mu.Lock()
		si, err := s.pendingLocked(code)
		if err == nil {
			p.Device = fmt.Sprintf("%s/%s, %s ago", si.device.OS, si.device.Arch, s.now().Sub(si.createdAt).Round(time.Second))
			for i, inv := range s.invites {
				p.Invitations = append(p.Invitations, invitationView{
					ID:        inv.id,
					Title:     inv.opts.Title,
					Duration:  inv.opts.Duration.String(),
					Assistant: strings.ReplaceAll(inv.opts.Policy.AssistantMode, "_", " "),
					Checked:   i == 0,
				})
			}
			if len(p.Invitations) == 0 {
				p.Error = "You haven't been invited to any assessment yet."
			}
		} else {
			p.Error = sentence(err)
		}
		s.mu.Unlock()
	}
	renderPage(w, http.StatusOK, p)
}

func (s *Server) handleDeviceForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderPage(w, http.StatusBadRequest, devicePage{Error: "The form couldn't be read."})
		return
	}
	code := r.PostForm.Get("code")
	s.mu.Lock()
	var err error
	deny := r.PostForm.Get("action") == "deny"
	if deny {
		var si *signIn
		if si, err = s.pendingLocked(code); err == nil {
			si.denied = true
		}
	} else {
		err = s.approveLocked(code, r.PostForm.Get("invitation"), r.PostForm.Get("email"))
	}
	s.mu.Unlock()
	switch {
	case err != nil:
		renderPage(w, http.StatusConflict, devicePage{Error: sentence(err)})
	case deny:
		renderPage(w, http.StatusOK, devicePage{Done: true, Message: "Sign-in refused. Nothing was connected. You can close this tab."})
	default:
		renderPage(w, http.StatusOK, devicePage{Done: true, Message: "Connected. Go back to your terminal: mindfries is setting up your workspace. You can close this tab."})
	}
}

func sentence(err error) string {
	m := err.Error()
	return strings.ToUpper(m[:1]) + m[1:] + "."
}

func renderPage(w http.ResponseWriter, status int, p devicePage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(status)
	_ = pageTmpl.Execute(w, p)
}

var pageTmpl = template.Must(template.New("device").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Connect your terminal · Mindfries (fake)</title>
<style>
  :root { color-scheme: light dark; --bg:#f6f6f4; --card:#fff; --fg:#1b1b1a; --muted:#6b6b66; --line:#deded8; --accent:#2f5bd3; --bad:#b3261e; }
  @media (prefers-color-scheme: dark) { :root { --bg:#141413; --card:#1e1e1c; --fg:#ecece8; --muted:#a0a09a; --line:#33332f; --accent:#7f9cf0; --bad:#f2b8b5; } }
  body { margin:0; background:var(--bg); color:var(--fg); font:16px/1.5 system-ui, sans-serif; }
  main { max-width:30rem; margin:3rem auto; padding:0 16px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:1.5rem; }
  h1 { font-size:1.3rem; margin:0 0 .25rem; }
  .muted { color:var(--muted); font-size:.9rem; }
  .code { font:600 1.8rem/1.2 ui-monospace, monospace; letter-spacing:.12em; margin:.5rem 0 1rem; }
  label { display:block; margin:.75rem 0 .25rem; font-weight:600; }
  input[type=email], input[type=text] { width:100%; box-sizing:border-box; padding:.55rem .7rem; border:1px solid var(--line); border-radius:8px; background:transparent; color:inherit; font:inherit; }
  .inv { display:flex; gap:.6rem; align-items:flex-start; border:1px solid var(--line); border-radius:8px; padding:.6rem .75rem; margin:.4rem 0; font-weight:normal; }
  .row { display:flex; gap:.75rem; margin-top:1.25rem; flex-wrap:wrap; }
  button { font:inherit; padding:.55rem 1rem; border-radius:8px; border:1px solid var(--line); background:transparent; color:inherit; cursor:pointer; }
  button.primary { background:var(--accent); border-color:var(--accent); color:#fff; }
  .error { color:var(--bad); }
  .banner { font-size:.8rem; color:var(--muted); margin-bottom:1rem; }
</style>
</head>
<body>
<main>
<p class="banner">Fake Mindfries, for testing the CLI. Nothing here is real and any email works.</p>
<div class="card">
{{if .Done}}
  <h1>{{.Message}}</h1>
{{else if and .Code (not .Error)}}
  <h1>Connect your terminal</h1>
  <p class="muted">Check this is the code your terminal shows. If it isn't, or you didn't run <code>mindfries start</code>, choose "This wasn't me".</p>
  <div class="code">{{.Code}}</div>
  <p class="muted">Requested from {{.Device}}</p>
  <form method="post" action="/device">
    <input type="hidden" name="code" value="{{.Code}}">
    <label for="email">Sign in with your email</label>
    <input id="email" name="email" type="email" value="candidate@example.com" autocomplete="email">
    <label>Assessment</label>
    {{range .Invitations}}
    <label class="inv"><input type="radio" name="invitation" value="{{.ID}}"{{if .Checked}} checked{{end}}>
      <span><strong>{{.Title}}</strong><br><span class="muted">{{.Duration}} · assistant: {{.Assistant}}</span></span></label>
    {{end}}
    <div class="row">
      <button class="primary" type="submit" name="action" value="approve">Connect</button>
      <button type="submit" name="action" value="deny">This wasn't me</button>
    </div>
  </form>
{{else}}
  <h1>Connect your terminal</h1>
  {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
  <form method="get" action="/device">
    <label for="code">Enter the code your terminal shows</label>
    <input id="code" name="code" type="text" placeholder="BCDF-GHJK" autocomplete="off" autocapitalize="characters">
    <div class="row"><button class="primary" type="submit">Continue</button></div>
  </form>
{{end}}
</div>
</main>
</body>
</html>
`))
