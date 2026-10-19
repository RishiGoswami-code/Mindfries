package fakeapi

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The fake's sign-in page, /device. The real one is the Mindfries web app,
// where the candidate is already signed in, so it never asks for an email:
// the fake stands in for that browser session with a cookie. After Connect
// the page follows the terminal: it shows GitHub's device code when the
// CLI sends one, then the repository, refreshing itself until done.

const fakeCookie = "mf_fake_candidate"

// candidate is who the browser session belongs to.
func candidate(w http.ResponseWriter, r *http.Request) (name, email string) {
	if c, err := r.Cookie(fakeCookie); err == nil {
		if v, err := url.QueryUnescape(c.Value); err == nil {
			if n, e, ok := strings.Cut(v, "|"); ok {
				return n, e
			}
		}
	}
	name, email = "Demo Candidate", "candidate@example.com"
	http.SetCookie(w, &http.Cookie{Name: fakeCookie, Value: url.QueryEscape(name + "|" + email), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return name, email
}

type invitationView struct {
	ID, Title, Duration, Assistant string
	GitHub, Checked                bool
}

type devicePage struct {
	Code, Name, Email, Device string
	Invitations               []invitationView
	Error                     string

	// After Connect.
	Approved   bool
	Assessment string
	Waiting    string // what the page waits for; it refreshes while set
	// GitHub: Connect is the link to (re)try; Connected once authorised.
	GitHubConnect   string
	GitHubConnected bool
	GitHubError     string
	Repo            string
	RepoURL         string
	Done            string
}

// findLocked is the sign-in with that user code, in any state.
func (s *Server) findLocked(userCode string) *signIn {
	userCode = normalizeUserCode(userCode)
	for _, si := range s.signIns {
		if si.userCode == userCode {
			return si
		}
	}
	return nil
}

func (s *Server) handleDevicePage(w http.ResponseWriter, r *http.Request) {
	name, email := candidate(w, r)
	code := normalizeUserCode(r.URL.Query().Get("code"))
	p := devicePage{Code: code, Name: name, Email: email}
	if code == "" {
		renderPage(w, http.StatusOK, p)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	si := s.findLocked(code)
	switch {
	case si == nil:
		p.Code, p.Error = "", sentence(errNoSignIn)
	case si.denied:
		p.Approved, p.Done = true, "Sign-in refused. Nothing was connected. You can close this tab."
	case si.approved != nil:
		s.statusLocked(si, &p)
	case !s.now().Before(si.expiresAt):
		p.Code, p.Error = "", sentence(errSignInExpired)
	default:
		p.Device = fmt.Sprintf("%s/%s, %s ago", si.device.OS, si.device.Arch, s.now().Sub(si.createdAt).Round(time.Second))
		for i, inv := range s.invites {
			p.Invitations = append(p.Invitations, invitationView{
				ID:        inv.id,
				Title:     inv.opts.Title,
				Duration:  fmt.Sprintf("%d minutes", int(inv.opts.Duration/time.Minute)),
				Assistant: strings.ReplaceAll(inv.opts.Policy.AssistantMode, "_", " "),
				GitHub:    s.githubFor(inv.opts) != nil,
				Checked:   i == 0,
			})
		}
		if len(p.Invitations) == 0 {
			p.Code, p.Error = "", "You haven't been invited to any assessment yet."
		}
	}
	renderPage(w, http.StatusOK, p)
}

// statusLocked fills in the page after Connect, following the terminal.
func (s *Server) statusLocked(si *signIn, p *devicePage) {
	p.Approved, p.Assessment = true, si.approved.opts.Title
	ss := s.sessions[si.session]
	usesGitHub := s.githubFor(si.approved.opts) != nil
	if !usesGitHub {
		if ss == nil {
			p.Waiting = "Finishing in your terminal…"
		} else {
			p.Done = "Go back to your terminal: mindfries is setting up your workspace. You can close this tab."
		}
		return
	}
	connected, failure := si.ghToken != "", si.ghErr
	if ss != nil {
		connected = connected || ss.ext.ghToken != "" || ss.ext.ghCollected || ss.ext.ghRepo != nil
		if ss.ext.ghErr != "" {
			failure = ss.ext.ghErr
		}
	}
	p.GitHubConnect = "/github/start?code=" + url.QueryEscape(si.userCode)
	switch {
	case ss != nil && ss.ext.ghRepo != nil:
		p.GitHubConnected = true
		p.Repo, p.RepoURL = ss.ext.ghRepo.FullName, ss.ext.ghRepo.HTMLURL
		p.Done = "Your work will be pushed there. Go back to your terminal; you can close this tab."
	case connected:
		p.GitHubConnected = true
		p.Waiting = "Creating your repository from your terminal…"
	case failure != "":
		p.GitHubError = failure + "."
	}
}

func (s *Server) handleDeviceForm(w http.ResponseWriter, r *http.Request) {
	_, email := candidate(w, r)
	if err := r.ParseForm(); err != nil {
		renderPage(w, http.StatusBadRequest, devicePage{Error: "The form couldn't be read."})
		return
	}
	code := r.PostForm.Get("code")
	s.mu.Lock()
	var err error
	if r.PostForm.Get("action") == "deny" {
		var si *signIn
		if si, err = s.pendingLocked(code); err == nil {
			si.denied = true
		}
	} else {
		err = s.approveLocked(code, r.PostForm.Get("invitation"), email)
	}
	s.mu.Unlock()
	if err != nil {
		renderPage(w, http.StatusConflict, devicePage{Error: sentence(err)})
		return
	}
	next := "/device?code=" + url.QueryEscape(normalizeUserCode(code))
	if r.PostForm.Get("action") != "deny" {
		s.mu.Lock()
		if si := s.findLocked(code); si != nil && si.approved != nil && s.githubFor(si.approved.opts) != nil {
			// Straight on to GitHub: one click there, then back here.
			next = s.authorizeURL(r, ghTarget{userCode: si.userCode})
		}
		s.mu.Unlock()
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
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
{{if .Waiting}}<meta http-equiv="refresh" content="2">{{end}}
<title>Connect your terminal · Mindfries (fake)</title>
<style>
  :root { color-scheme: light dark; --bg:#f6f6f4; --card:#fff; --fg:#1b1b1a; --muted:#6b6b66; --line:#deded8; --accent:#2f5bd3; --ok:#1f7a3f; --bad:#b3261e; }
  @media (prefers-color-scheme: dark) { :root { --bg:#141413; --card:#1e1e1c; --fg:#ecece8; --muted:#a0a09a; --line:#33332f; --accent:#7f9cf0; --ok:#7fd49b; --bad:#f2b8b5; } }
  body { margin:0; background:var(--bg); color:var(--fg); font:16px/1.5 system-ui, sans-serif; }
  main { max-width:32rem; margin:3rem auto; padding:0 16px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:1.5rem; }
  h1 { font-size:1.3rem; margin:0 0 .25rem; }
  h2 { font-size:1.05rem; margin:1.25rem 0 .25rem; }
  .muted { color:var(--muted); font-size:.9rem; }
  .who { display:flex; gap:.6rem; align-items:center; padding:.6rem .75rem; border:1px solid var(--line); border-radius:8px; margin:.75rem 0; }
  .avatar { width:2rem; height:2rem; border-radius:50%; background:var(--accent); color:#fff; display:grid; place-items:center; font-weight:600; }
  .code { font:600 1.8rem/1.2 ui-monospace, monospace; letter-spacing:.12em; margin:.5rem 0 .75rem; }
  label { display:block; margin:.75rem 0 .25rem; font-weight:600; }
  input[type=text] { width:100%; box-sizing:border-box; padding:.55rem .7rem; border:1px solid var(--line); border-radius:8px; background:transparent; color:inherit; font:inherit; }
  .inv { display:flex; gap:.6rem; align-items:flex-start; border:1px solid var(--line); border-radius:8px; padding:.6rem .75rem; margin:.4rem 0; font-weight:normal; }
  .row { display:flex; gap:.75rem; margin-top:1.25rem; flex-wrap:wrap; align-items:center; }
  button, .button { font:inherit; padding:.55rem 1rem; border-radius:8px; border:1px solid var(--line); background:transparent; color:inherit; cursor:pointer; text-decoration:none; display:inline-block; }
  .primary { background:var(--accent); border-color:var(--accent); color:#fff; }
  .step { display:flex; gap:.5rem; align-items:baseline; margin:.35rem 0; }
  .ok { color:var(--ok); } .error { color:var(--bad); }
  .banner { font-size:.8rem; color:var(--muted); margin-bottom:1rem; }
</style>
</head>
<body>
<main>
<p class="banner">Fake Mindfries, for testing the CLI. Nothing here is real.</p>
<div class="card">
{{if .Approved}}
  <h1>Connect your terminal</h1>
  {{if .Assessment}}<div class="step"><span class="ok">✓</span><span>Connected to <strong>{{.Assessment}}</strong></span></div>{{end}}
  {{if .GitHubConnected}}<div class="step"><span class="ok">✓</span><span>GitHub connected</span></div>{{end}}
  {{if .Repo}}<div class="step"><span class="ok">✓</span><span>Repository <a href="{{.RepoURL}}" target="_blank" rel="noopener">{{.Repo}}</a> (private)</span></div>{{end}}
  {{if and .GitHubConnect (not .GitHubConnected)}}
    <h2>Connect GitHub</h2>
    {{if .GitHubError}}<p class="error">{{.GitHubError}}</p>{{end}}
    <p class="muted">Your work goes to a new private repository in your GitHub account, so the hiring team can see your commits, CI and deployment.</p>
    <div class="row"><a class="button primary" href="{{.GitHubConnect}}">Connect GitHub</a></div>
  {{end}}
  {{if .Waiting}}<p class="muted">{{.Waiting}}</p>{{end}}
  {{if .Done}}<p>{{.Done}}</p>{{end}}
{{else if and .Code (not .Error)}}
  <h1>Connect your terminal</h1>
  <div class="who"><div class="avatar">{{slice .Name 0 1}}</div><div><strong>{{.Name}}</strong><br><span class="muted">{{.Email}} · signed in to Mindfries in this browser</span></div></div>
  <p class="muted">Check this is the code your terminal shows. If it isn't, or you didn't run <code>mindfries start</code>, choose "This wasn't me".</p>
  <div class="code">{{.Code}}</div>
  <p class="muted">Requested from {{.Device}}</p>
  <form method="post" action="/device">
    <input type="hidden" name="code" value="{{.Code}}">
    <label>Assessment</label>
    {{range .Invitations}}
    <label class="inv"><input type="radio" name="invitation" value="{{.ID}}"{{if .Checked}} checked{{end}}>
      <span><strong>{{.Title}}</strong><br><span class="muted">{{.Duration}} · assistant: {{.Assistant}}{{if .GitHub}} · uses your GitHub{{end}}</span></span></label>
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
