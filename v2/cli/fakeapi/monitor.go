package fakeapi

import (
	"html/template"
	"io"
	"net/http"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// Monitoring and the clock. A session starts ready. `mindfries start`
// checks the computer, has the candidate open the monitoring panel (a
// browser page: camera and microphone with getUserMedia, the screen with
// getDisplayMedia), waits until what the assessment requires is live, then
// calls POST /session/begin, which starts the clock. While it runs the
// panel reports what's on and sends a camera still and a screenshot every
// 30 seconds, and the microphone in 30-second clips. A share that stops
// shows in the heartbeat reply and is a gap in the record; it's never a
// verdict.

// panelFresh is how recently the panel must have reported to count as open.
const panelFresh = 20 * time.Second

// Kept per session, so a long session can't fill the fake's memory.
const (
	keepStills = 60
	keepClips  = 40
	maxStill   = 2 << 20
	maxClip    = 4 << 20
)

type capture struct {
	at   time.Time
	kind string // camera, screen, audio
	mime string
	data []byte
}

// monState is the panel's side of a session.
type monState struct {
	camera, screen, audio bool
	lastSeen              time.Time
	stills, clips         []capture
	stillCount, clipCount int
	gaps                  []string
}

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/begin", s.authed(s.handleBegin))
		route("GET /session/monitoring", s.authed(s.handleMonitoring))
	})
	registerAdmin(func(s *Server, mux *http.ServeMux) {
		mux.HandleFunc("GET /__admin/sessions/{id}/monitoring", func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			defer s.mu.Unlock()
			ss := s.sessions[r.PathValue("id")]
			if ss == nil {
				problem(w, http.StatusNotFound, api.CodeNotFound, "No such session", "")
				return
			}
			m := &ss.ext.mon
			writeJSON(w, http.StatusOK, map[string]any{
				"required": ss.Monitoring, "live": s.monitoringLocked(ss),
				"stills": m.stillCount, "audio_clips": m.clipCount, "gaps": m.gaps,
			})
		})
	})
}

func (s *Server) monitorRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /monitor/{key}", s.handlePanel)
	mux.HandleFunc("POST /monitor/{key}/status", s.handlePanelStatus)
	mux.HandleFunc("POST /monitor/{key}/still", s.handlePanelUpload("still"))
	mux.HandleFunc("POST /monitor/{key}/audio", s.handlePanelUpload("audio"))
}

// monitoringLocked is what's live now. s.mu is held.
func (s *Server) monitoringLocked(ss *session) api.MonitoringStatus {
	m := &ss.ext.mon
	st := api.MonitoringStatus{}
	if !m.lastSeen.IsZero() {
		seen := m.lastSeen
		st.LastSeen = &seen
		st.PanelOpen = s.now().Sub(seen) < panelFresh
	}
	if st.PanelOpen {
		st.Camera, st.Screen, st.Audio = m.camera, m.screen, m.audio
	}
	return st
}

func (s *Server) handleMonitoring(w http.ResponseWriter, r *http.Request, ss *session) {
	writeJSON(w, http.StatusOK, s.monitoringLocked(ss))
}

func (s *Server) handleBegin(w http.ResponseWriter, r *http.Request, ss *session) {
	var in api.BeginRequest
	if !decode(w, r, &in) {
		return
	}
	switch ss.Status {
	case api.StatusActive:
		writeJSON(w, http.StatusOK, ss.view()) // already begun: say so again
		return
	case api.StatusReady:
	default:
		requireActive(w, ss)
		return
	}
	if st := s.monitoringLocked(ss); len(st.Missing(ss.Monitoring)) > 0 {
		p := &api.Problem{Code: api.CodeMonitoringNotReady, Title: "The monitoring panel isn't sharing everything the assessment needs", Status: http.StatusConflict}
		for i, m := range st.Missing(ss.Monitoring) {
			if i > 0 {
				p.Detail += ", "
			}
			p.Detail += m
		}
		writeProblem(w, p)
		return
	}
	now := s.now()
	ss.Status, ss.StartedAt, ss.EndsAt = api.StatusActive, now, now.Add(ss.duration)
	ss.ext.begin = &in
	writeJSON(w, http.StatusOK, ss.view())
}

// Begun returns what a session began with, for tests.
func (s *Server) Begun(id string) *api.BeginRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		return ss.ext.begin
	}
	return nil
}

// SetMonitoring reports, as the panel would, what's live for a session.
func (s *Server) SetMonitoring(id string, camera, screen, audio bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		s.panelReportLocked(ss, camera, screen, audio)
	}
}

// panelReportLocked records the panel's report, noting what stopped.
func (s *Server) panelReportLocked(ss *session, camera, screen, audio bool) {
	m := &ss.ext.mon
	if ss.Status == api.StatusActive {
		at := s.now().Format(time.RFC3339)
		for _, c := range []struct {
			name      string
			was, now  bool
			requested bool
		}{{"camera", m.camera, camera, ss.Monitoring.Camera}, {"screen", m.screen, screen, ss.Monitoring.Screen}, {"microphone", m.audio, audio, ss.Monitoring.Audio}} {
			if c.requested && c.was && !c.now {
				m.gaps = append(m.gaps, at+" "+c.name+" stopped")
			}
		}
	}
	m.camera, m.screen, m.audio, m.lastSeen = camera, screen, audio, s.now()
}

// sessionByPanelKey finds the session whose panel this is. s.mu is held.
func (s *Server) sessionByPanelKey(key string) *session {
	if key == "" {
		return nil
	}
	for _, ss := range s.sessions {
		if ss.ext.monKey == key {
			return ss
		}
	}
	return nil
}

func (s *Server) handlePanelStatus(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Camera bool `json:"camera"`
		Screen bool `json:"screen"`
		Audio  bool `json:"audio"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessionByPanelKey(r.PathValue("key"))
	if ss == nil {
		http.NotFound(w, r)
		return
	}
	s.panelReportLocked(ss, in.Camera, in.Screen, in.Audio)
	writeJSON(w, http.StatusOK, map[string]any{"status": ss.Status, "ends_at": ss.EndsAt, "started": ss.Status == api.StatusActive})
}

func (s *Server) handlePanelUpload(what string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := int64(maxStill)
		if what == "audio" {
			limit = maxClip
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
		if err != nil || len(data) == 0 {
			http.Error(w, "empty or too large", http.StatusRequestEntityTooLarge)
			return
		}
		kind := r.URL.Query().Get("kind")
		if what == "audio" {
			kind = "audio"
		} else if kind != "camera" && kind != "screen" {
			http.Error(w, "kind must be camera or screen", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		ss := s.sessionByPanelKey(r.PathValue("key"))
		if ss == nil {
			http.NotFound(w, r)
			return
		}
		if ss.Status != api.StatusActive {
			w.WriteHeader(http.StatusAccepted) // nothing is kept before the clock starts
			return
		}
		c := capture{at: s.now(), kind: kind, mime: r.Header.Get("Content-Type"), data: data}
		m := &ss.ext.mon
		if what == "audio" {
			m.clips = append(m.clips, c)
			m.clipCount++
			if len(m.clips) > keepClips {
				m.clips = m.clips[len(m.clips)-keepClips:]
			}
		} else {
			m.stills = append(m.stills, c)
			m.stillCount++
			if len(m.stills) > keepStills {
				m.stills = m.stills[len(m.stills)-keepStills:]
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// Captures returns how many stills and audio clips a session kept, for
// tests.
func (s *Server) Captures(id string) (stills, clips int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss := s.sessions[id]; ss != nil {
		return ss.ext.mon.stillCount, ss.ext.mon.clipCount
	}
	return 0, 0
}

type panelPage struct {
	Key, Title            string
	Camera, Screen, Audio bool
}

func (s *Server) handlePanel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	ss := s.sessionByPanelKey(r.PathValue("key"))
	var p panelPage
	if ss != nil {
		p = panelPage{Key: ss.ext.monKey, Title: ss.AssessmentTitle, Camera: ss.Monitoring.Camera, Screen: ss.Monitoring.Screen, Audio: ss.Monitoring.Audio}
	}
	s.mu.Unlock()
	if ss == nil {
		renderPage(w, http.StatusNotFound, devicePage{Error: "This monitoring link isn't for an active assessment."})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Frame-Options", "DENY")
	_ = panelTmpl.Execute(w, p)
}

var panelTmpl = template.Must(template.New("panel").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Monitoring · {{.Title}} · Mindfries (fake)</title>
<style>
  :root { color-scheme: light dark; --bg:#f6f6f4; --card:#fff; --fg:#1b1b1a; --muted:#6b6b66; --line:#deded8; --accent:#2f5bd3; --ok:#1f7a3f; --bad:#b3261e; }
  @media (prefers-color-scheme: dark) { :root { --bg:#141413; --card:#1e1e1c; --fg:#ecece8; --muted:#a0a09a; --line:#33332f; --accent:#7f9cf0; --ok:#7fd49b; --bad:#f2b8b5; } }
  body { margin:0; background:var(--bg); color:var(--fg); font:16px/1.5 system-ui, sans-serif; }
  main { max-width:40rem; margin:2rem auto; padding:0 16px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:1.25rem 1.5rem; margin-bottom:1rem; }
  h1 { font-size:1.25rem; margin:0 0 .25rem; }
  .muted { color:var(--muted); font-size:.9rem; }
  .row { display:flex; gap:.75rem; flex-wrap:wrap; align-items:center; margin-top:.75rem; }
  button { font:inherit; padding:.55rem 1rem; border-radius:8px; border:1px solid var(--accent); background:var(--accent); color:#fff; cursor:pointer; }
  button:disabled { opacity:.5; cursor:default; }
  .item { display:flex; justify-content:space-between; padding:.4rem 0; border-bottom:1px solid var(--line); }
  .item:last-child { border-bottom:0; }
  .on { color:var(--ok); font-weight:600; } .off { color:var(--bad); font-weight:600; }
  video { width:100%; max-height:14rem; background:#000; border-radius:8px; margin-top:.75rem; }
  .banner { font-size:.8rem; color:var(--muted); margin-bottom:1rem; }
</style>
</head>
<body>
<main>
<p class="banner">Fake Mindfries, for testing the CLI. Nothing here is real, but what you share is sent to this test server.</p>
<div class="card">
  <h1>{{.Title}}: monitoring</h1>
  <p class="muted">This assessment records what's ticked below while it runs: a camera still and a screenshot every 30 seconds{{if .Audio}}, and your microphone in 30-second clips{{end}}. It's evidence for the person who reviews your work, never an automatic verdict. Keep this tab open until you submit.</p>
  <div class="row">
    {{if or .Camera .Audio}}<button id="cam">Allow {{if .Camera}}camera{{end}}{{if and .Camera .Audio}} and {{end}}{{if .Audio}}microphone{{end}}</button>{{end}}
    {{if .Screen}}<button id="scr">Share your screen</button>{{end}}
  </div>
  <video id="preview" autoplay muted playsinline hidden></video>
</div>
<div class="card">
  {{if .Camera}}<div class="item"><span>Camera</span><span id="s-camera" class="off">off</span></div>{{end}}
  {{if .Audio}}<div class="item"><span>Microphone</span><span id="s-audio" class="off">off</span></div>{{end}}
  {{if .Screen}}<div class="item"><span>Screen</span><span id="s-screen" class="off">off</span></div>{{end}}
  <p class="muted" id="state">Turn everything on, then go back to your terminal: mindfries starts the assessment by itself.</p>
</div>
</main>
<script>
const key = {{.Key}};
const need = {camera: {{.Camera}}, screen: {{.Screen}}, audio: {{.Audio}}};
let cam = null, scr = null, rec = null, started = false;
const live = (s, kind) => !!s && s.getTracks().some(t => t.kind === kind && t.readyState === "live");
function show(name, on) { const el = document.getElementById("s-" + name); if (el) { el.textContent = on ? "on" : "off"; el.className = on ? "on" : "off"; } }
async function report() {
  const st = {camera: live(cam, "video"), audio: live(cam, "audio"), screen: live(scr, "video")};
  show("camera", st.camera); show("audio", st.audio); show("screen", st.screen);
  try {
    const r = await fetch("/monitor/" + key + "/status", {method: "POST", headers: {"content-type": "application/json"}, body: JSON.stringify(st)});
    const j = await r.json();
    started = j.started;
    const missing = Object.keys(need).filter(k => need[k] && !st[k]);
    document.getElementById("state").textContent = j.status === "active"
      ? (missing.length ? "The assessment is running. Turn back on: " + missing.join(", ") + "." : "The assessment is running. Keep this tab open until you submit.")
      : (j.status === "ready" ? (missing.length ? "Still needed: " + missing.join(", ") + "." : "All set. Go back to your terminal: mindfries starts the assessment.") : "This assessment is " + j.status + ". You can close this tab.");
  } catch (e) {}
}
const camBtn = document.getElementById("cam"), scrBtn = document.getElementById("scr");
if (camBtn) camBtn.onclick = async () => {
  try {
    cam = await navigator.mediaDevices.getUserMedia({video: need.camera, audio: need.audio});
    cam.getTracks().forEach(t => t.onended = report);
    if (need.camera) { const v = document.getElementById("preview"); v.srcObject = cam; v.hidden = false; }
    if (need.audio) startAudio();
    camBtn.disabled = true;
  } catch (e) { alert("The browser didn't allow it: " + e.message); }
  report();
};
if (scrBtn) scrBtn.onclick = async () => {
  try {
    scr = await navigator.mediaDevices.getDisplayMedia({video: true, audio: false});
    scr.getVideoTracks()[0].onended = () => { scr = null; scrBtn.disabled = false; report(); };
    scrBtn.disabled = true;
  } catch (e) { alert("Screen sharing wasn't started: " + e.message); }
  report();
};
function startAudio() {
  const track = cam.getAudioTracks()[0];
  if (!track || !window.MediaRecorder) return;
  rec = new MediaRecorder(new MediaStream([track]));
  rec.ondataavailable = e => { if (started && e.data.size) fetch("/monitor/" + key + "/audio", {method: "POST", headers: {"content-type": e.data.type || "audio/webm"}, body: e.data}); };
  rec.start(30000);
}
async function still(stream, kind) {
  if (!live(stream, "video")) return;
  const v = document.createElement("video");
  v.srcObject = new MediaStream([stream.getVideoTracks()[0]]); v.muted = true;
  await v.play();
  const c = document.createElement("canvas");
  const scale = Math.min(1, 1280 / (v.videoWidth || 1280));
  c.width = (v.videoWidth || 640) * scale; c.height = (v.videoHeight || 480) * scale;
  c.getContext("2d").drawImage(v, 0, 0, c.width, c.height);
  v.pause(); v.srcObject = null;
  c.toBlob(b => { if (b) fetch("/monitor/" + key + "/still?kind=" + kind, {method: "POST", headers: {"content-type": "image/jpeg"}, body: b}); }, "image/jpeg", 0.6);
}
setInterval(report, 5000);
setInterval(() => { if (started) { if (need.camera) still(cam, "camera"); if (need.screen) still(scr, "screen"); } }, 30000);
report();
</script>
</body>
</html>
`))
