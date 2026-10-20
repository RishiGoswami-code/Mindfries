package fakeapi

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/install"
	"github.com/mindfries/v2/cli/internal/api"
)

// The candidate's dashboard, faked: the assessments they're invited to,
// each with a Start assessment button. The button makes a single-use setup
// code and shows one command per OS. Running it installs mindfries if
// needed and runs `mindfries setup <code>`: the task and codebase are
// downloaded and verified, GitHub is connected, and the repository is
// created and pushed. The clock starts only at `mindfries start`, after the
// computer is checked and monitoring is on. The page follows along.

var codeInPath = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$`)

func (s *Server) dashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", s.handleDashboard)
	mux.HandleFunc("POST /dashboard/start", s.handleDashboardStart)
	mux.HandleFunc("GET /dashboard/setup", s.handleDashboardSetup)
	mux.HandleFunc("GET /s/{file}", s.handleSetupScript)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/dashboard", http.StatusFound) })
}

type dashInvite struct {
	ID, Title, Duration, Assistant string
	GitHub                         bool
	Monitoring                     string
}

type dashPage struct {
	Name, Email string
	Invitations []dashInvite
	// The setup page.
	Code, Title, WinCmd, UnixCmd, CLICmd string
	Status, Detail                       string
	Steps                                []dashStep
	Expired, Refresh                     bool
	Invitation, PanelURL, RepoURL        string
}

type dashStep struct {
	Done bool
	Text string
}

func monitoringText(m *api.Monitoring) string {
	if !m.Any() {
		return ""
	}
	var parts []string
	if m.Camera {
		parts = append(parts, "camera")
	}
	if m.Screen {
		parts = append(parts, "screen")
	}
	if m.Audio {
		parts = append(parts, "microphone")
	}
	return strings.Join(parts, ", ")
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	name, email := candidate(w, r)
	p := dashPage{Name: name, Email: email}
	s.mu.Lock()
	for _, inv := range s.invites {
		p.Invitations = append(p.Invitations, dashInvite{
			ID:         inv.id,
			Title:      inv.opts.Title,
			Duration:   fmt.Sprintf("%d minutes", int(inv.opts.Duration/time.Minute)),
			Assistant:  strings.ReplaceAll(inv.opts.Policy.AssistantMode, "_", " "),
			GitHub:     s.githubFor(inv.opts) != nil,
			Monitoring: monitoringText(inv.opts.Monitoring),
		})
	}
	s.mu.Unlock()
	renderDash(w, http.StatusOK, "list", p)
}

func (s *Server) handleDashboardStart(w http.ResponseWriter, r *http.Request) {
	candidate(w, r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id := r.PostForm.Get("invitation")
	s.mu.Lock()
	var inv *invitation
	for _, i := range s.invites {
		if i.id == id {
			inv = i
		}
	}
	if inv == nil {
		s.mu.Unlock()
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	o := inv.opts
	c := newCode()
	s.codes[c] = &setupCode{code: c, expiresAt: s.now().Add(o.CodeTTL), opts: o, invitation: inv.id}
	s.mu.Unlock()
	http.Redirect(w, r, "/dashboard/setup?code="+url.QueryEscape(c), http.StatusSeeOther)
}

func (s *Server) handleDashboardSetup(w http.ResponseWriter, r *http.Request) {
	name, email := candidate(w, r)
	code := strings.ToUpper(r.URL.Query().Get("code"))
	base := s.publicBase(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.codes[code]
	if c == nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	p := dashPage{Name: name, Email: email, Code: code, Title: c.opts.Title, Invitation: c.invitation,
		WinCmd:  "irm " + base + "/s/" + code + ".ps1 | iex",
		UnixCmd: "curl -fsSL " + base + "/s/" + code + ".sh | sh",
		CLICmd:  "mindfries setup " + code,
	}
	ss := s.sessions[c.session]
	switch {
	case ss == nil && !c.used && s.now().Before(c.expiresAt):
		p.Status = "Run this in a terminal"
		p.Detail = fmt.Sprintf("It works once, for about %d more minutes. Your time doesn't start yet.", int(time.Until(c.expiresAt).Minutes())+1)
		p.Refresh = true
	case ss == nil:
		p.Expired = true
		p.Status = "This command has expired"
		p.Detail = "Get a new one; nothing was started."
	default:
		repoDone := ss.ext.ghRepo != nil
		p.Steps = []dashStep{
			{true, "Task and codebase downloaded and checked"},
		}
		if ss.GitHub != nil {
			t := "GitHub connected and the codebase pushed"
			if repoDone {
				t += " to " + ss.ext.ghRepo.FullName
				p.RepoURL = ss.ext.ghRepo.HTMLURL
			}
			p.Steps = append(p.Steps, dashStep{repoDone, t})
		}
		if ss.Monitoring.Any() {
			p.PanelURL = ss.Monitoring.PanelURL
		}
		switch ss.Status {
		case api.StatusReady:
			p.Status = "Ready to start"
			p.Detail = "In your terminal, run mindfries start. It asks which editor to use, checks your computer, turns on monitoring, and only then starts your time."
			p.Steps = append(p.Steps, dashStep{false, "mindfries start: editor, checks" + map[bool]string{true: ", monitoring", false: ""}[ss.Monitoring.Any()] + ", then the clock starts"})
			p.Refresh = true
		case api.StatusActive:
			p.Status = "Your assessment is running"
			p.Detail = "Your time ends at " + ss.EndsAt.Local().Format("15:04") + ". Hand in with mindfries submit."
			p.Steps = append(p.Steps, dashStep{true, "Started at " + ss.StartedAt.Local().Format("15:04")})
			p.Refresh = true
		default:
			p.Status = "This assessment is " + ss.Status
			p.Steps = append(p.Steps, dashStep{true, "Finished"})
		}
	}
	renderDash(w, http.StatusOK, "setup", p)
}

// handleSetupScript serves the installer with the setup code in it, for
// the dashboard's one command. Only a live, unused code gets one.
func (s *Server) handleSetupScript(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	code, ext, ok := strings.Cut(file, ".")
	code = strings.ToUpper(code)
	if !ok || (ext != "ps1" && ext != "sh") || !codeInPath.MatchString(code) {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	c := s.codes[code]
	live := c != nil && !c.used && s.now().Before(c.expiresAt)
	s.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	if !live {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusGone)
		if ext == "ps1" {
			io.WriteString(w, "Write-Error 'This Mindfries setup command has expired or was already used. Press Start assessment on your dashboard again.'\n")
		} else {
			io.WriteString(w, "echo 'This Mindfries setup command has expired or was already used. Press Start assessment on your dashboard again.' >&2; exit 1\n")
		}
		return
	}
	download := s.publicBase(r) + "/cli/latest"
	var body string
	if ext == "ps1" {
		body = strings.Replace(install.PowerShell, `param([string]$Code = "")`, `param([string]$Code = "`+code+`")`, 1)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		body = strings.Replace(install.Shell, `CODE="${1:-}"`, `CODE="${1:-`+code+`}"`, 1)
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	}
	io.WriteString(w, strings.ReplaceAll(body, install.DefaultDownload, download))
}

func renderDash(w http.ResponseWriter, status int, which string, p dashPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(status)
	_ = dashTmpl.ExecuteTemplate(w, which, p)
}

var dashTmpl = template.Must(template.New("dash").Parse(`
{{define "head"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
{{if .Refresh}}<meta http-equiv="refresh" content="4">{{end}}
<title>Dashboard · Mindfries (fake)</title>
<style>
  :root { color-scheme: light dark; --bg:#f6f6f4; --card:#fff; --fg:#1b1b1a; --muted:#6b6b66; --line:#deded8; --accent:#2f5bd3; --ok:#1f7a3f; --code:#f0f0ec; }
  @media (prefers-color-scheme: dark) { :root { --bg:#141413; --card:#1e1e1c; --fg:#ecece8; --muted:#a0a09a; --line:#33332f; --accent:#7f9cf0; --ok:#7fd49b; --code:#262624; } }
  body { margin:0; background:var(--bg); color:var(--fg); font:16px/1.5 system-ui, sans-serif; }
  header { display:flex; justify-content:space-between; align-items:center; max-width:46rem; margin:1.5rem auto 0; padding:0 16px; }
  main { max-width:46rem; margin:1rem auto 3rem; padding:0 16px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:1.25rem 1.5rem; margin-bottom:1rem; }
  h1 { font-size:1.4rem; margin:0 0 .5rem; } h2 { font-size:1.1rem; margin:0 0 .25rem; }
  .muted { color:var(--muted); font-size:.9rem; }
  button, .button { font:inherit; padding:.55rem 1rem; border-radius:8px; border:1px solid var(--accent); background:var(--accent); color:#fff; cursor:pointer; text-decoration:none; display:inline-block; }
  .ghost { background:transparent; color:inherit; border-color:var(--line); }
  .row { display:flex; gap:.75rem; flex-wrap:wrap; align-items:center; margin-top:.75rem; }
  .cmd { display:flex; gap:.5rem; align-items:center; margin:.35rem 0 1rem; }
  pre { flex:1; margin:0; padding:.6rem .8rem; background:var(--code); border-radius:8px; overflow-x:auto; font:14px/1.4 ui-monospace, monospace; }
  .step { display:flex; gap:.5rem; margin:.3rem 0; } .ok { color:var(--ok); }
  .tag { font-size:.8rem; padding:.1rem .5rem; border:1px solid var(--line); border-radius:999px; margin-right:.3rem; }
  .banner { font-size:.8rem; color:var(--muted); }
</style>
</head>
<body>
<header><strong>Mindfries</strong><span class="muted">{{.Name}} · {{.Email}}</span></header>
<main>
<p class="banner">Fake Mindfries dashboard, for testing the CLI. Nothing here is real.</p>
{{end}}
{{define "foot"}}
</main>
<script>
document.querySelectorAll("[data-copy]").forEach(b => b.onclick = () => { navigator.clipboard.writeText(document.getElementById(b.dataset.copy).textContent); b.textContent = "Copied"; setTimeout(() => b.textContent = "Copy", 1500); });
</script>
</body>
</html>{{end}}

{{define "list"}}{{template "head" .}}
<h1>Your assessments</h1>
{{range .Invitations}}
<div class="card">
  <h2>{{.Title}}</h2>
  <p class="muted"><span class="tag">{{.Duration}}</span><span class="tag">assistant: {{.Assistant}}</span>{{if .GitHub}}<span class="tag">your GitHub</span>{{end}}{{if .Monitoring}}<span class="tag">monitoring: {{.Monitoring}}</span>{{end}}</p>
  <form method="post" action="/dashboard/start" class="row">
    <input type="hidden" name="invitation" value="{{.ID}}">
    <button type="submit">Start assessment</button>
    <span class="muted">You'll get a command to run; your time starts later, at mindfries start.</span>
  </form>
</div>
{{else}}
<div class="card"><p>You haven't been invited to any assessment yet.</p></div>
{{end}}
{{template "foot" .}}{{end}}

{{define "setup"}}{{template "head" .}}
<div class="card">
  <h1>{{.Title}}</h1>
  <h2>{{.Status}}</h2>
  <p class="muted">{{.Detail}}</p>
  {{if .Expired}}
    <form method="post" action="/dashboard/start" class="row"><input type="hidden" name="invitation" value="{{.Invitation}}"><button type="submit">Get a new command</button></form>
  {{else if not .Steps}}
    <p><strong>Windows</strong> (PowerShell)</p>
    <div class="cmd"><pre id="win">{{.WinCmd}}</pre><button class="ghost" data-copy="win">Copy</button></div>
    <p><strong>macOS / Linux</strong></p>
    <div class="cmd"><pre id="unix">{{.UnixCmd}}</pre><button class="ghost" data-copy="unix">Copy</button></div>
    <p class="muted">Already have mindfries? <code>{{.CLICmd}}</code></p>
    <p class="muted">It installs mindfries if needed, downloads the task and its codebase, connects your GitHub (one click in this browser) and puts the codebase in a new private repository there. Then run <code>mindfries start</code> when you're ready.</p>
  {{end}}
  {{range .Steps}}<div class="step"><span class="{{if .Done}}ok{{else}}muted{{end}}">{{if .Done}}✓{{else}}○{{end}}</span><span>{{.Text}}</span></div>{{end}}
  {{if .RepoURL}}<p class="row"><a class="button ghost" href="{{.RepoURL}}" target="_blank" rel="noopener">Open the repository</a>{{if .PanelURL}}<a class="button ghost" href="{{.PanelURL}}" target="_blank" rel="noopener">Monitoring panel</a>{{end}}</p>{{else if .PanelURL}}<p class="row"><a class="button ghost" href="{{.PanelURL}}" target="_blank" rel="noopener">Monitoring panel</a></p>{{end}}
</div>
<p><a href="/dashboard">← All assessments</a></p>
{{template "foot" .}}{{end}}
`))
