// Package monitor is what the monitoring panel reports and what the server
// makes of it. The panel is a page in the candidate's browser that shares the
// camera, the screen and the microphone; the clock doesn't start until what
// the assessment requires is live, and a share that stops later is a gap in
// the record: noted, never a verdict.
package monitor

import (
	"strings"
	"time"
)

// Fresh is how recently the panel must have reported to count as open.
const Fresh = 20 * time.Second

// Silent is how long without a report, while the session is active, before the
// panel itself counts as gone.
const Silent = 60 * time.Second

// Sources, as the contract and the database name them.
const (
	Camera = "camera"
	Screen = "screen"
	Audio  = "audio"
	Panel  = "panel"
)

// State is the panel's last report.
type State struct {
	SessionID  string
	Camera     bool
	Screen     bool
	Audio      bool
	ReportedAt time.Time
}

// Open reports whether the panel has reported lately.
func (s State) Open(now time.Time) bool {
	return !s.ReportedAt.IsZero() && now.Sub(s.ReportedAt) <= Fresh
}

// Live reports whether a source is being shared right now.
func (s State) Live(source string, now time.Time) bool {
	if !s.Open(now) {
		return false
	}
	switch source {
	case Camera:
		return s.Camera
	case Screen:
		return s.Screen
	case Audio:
		return s.Audio
	}
	return false
}

// Require is what an assessment needs live before it begins.
type Require struct{ Camera, Screen, Audio bool }

func (r Require) sources() []string {
	var out []string
	if r.Camera {
		out = append(out, Camera)
	}
	if r.Screen {
		out = append(out, Screen)
	}
	if r.Audio {
		out = append(out, Audio)
	}
	return out
}

// Missing lists what is required but not live. A panel that isn't open
// misses everything it was needed for, and itself.
func Missing(req Require, s State, now time.Time) []string {
	var out []string
	if !s.Open(now) {
		out = append(out, "the monitoring panel isn't open")
	}
	for _, src := range req.sources() {
		if !s.Live(src, now) {
			out = append(out, label(src))
		}
	}
	return out
}

func label(src string) string {
	if src == Audio {
		return "the microphone isn't on"
	}
	return "the " + src + " isn't shared"
}

// Describe is Missing as one sentence for a problem's detail.
func Describe(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	return "Not ready: " + strings.Join(missing, "; ") + "."
}

// Change is a required source turning off or on between two reports.
type Change struct {
	Source string
	On     bool
}

// Changes compares two reports for the sources the assessment requires. had
// is false for the very first report, which counts as a change only if
// something required is off (the candidate started without it).
func Changes(prev State, had bool, next State, req Require) []Change {
	var out []Change
	for _, src := range req.sources() {
		now := flag(next, src)
		switch {
		case !had && now:
			// First report with it on: nothing to note.
		case !had && !now:
			out = append(out, Change{Source: src, On: false}) // never started
		case flag(prev, src) != now:
			out = append(out, Change{Source: src, On: now})
		}
	}
	return out
}

func flag(s State, src string) bool {
	switch src {
	case Camera:
		return s.Camera
	case Screen:
		return s.Screen
	case Audio:
		return s.Audio
	}
	return false
}

// Upload says what the panel may send for a kind of evidence: the file
// extension, the size limit, and the content types it accepts.
type Upload struct {
	Kind     string // the evidence kind stored
	Ext      string
	MaxBytes int
	Every    time.Duration // the fastest it may be sent, per session
}

var stillTypes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}
var audioTypes = map[string]string{"audio/webm": "webm", "audio/ogg": "ogg", "audio/mp4": "m4a", "audio/wav": "wav", "audio/mpeg": "mp3"}

// ForUpload resolves a panel upload (what it says it is, and its content type)
// to the rules for it, or reports false.
func ForUpload(what, contentType string) (Upload, bool) {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch what {
	case "camera":
		if ext, ok := stillTypes[ct]; ok {
			return Upload{Kind: "camera_still", Ext: ext, MaxBytes: 2 << 20, Every: 5 * time.Second}, true
		}
	case "screen":
		if ext, ok := stillTypes[ct]; ok {
			return Upload{Kind: "screenshot", Ext: ext, MaxBytes: 4 << 20, Every: 5 * time.Second}, true
		}
	case "audio":
		if ext, ok := audioTypes[ct]; ok {
			return Upload{Kind: "audio_clip", Ext: ext, MaxBytes: 4 << 20, Every: 10 * time.Second}, true
		}
	}
	return Upload{}, false
}
