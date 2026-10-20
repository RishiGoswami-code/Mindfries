package monitor

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var t0 = time.Unix(1_800_000_000, 0)

func TestOpenAndLive(t *testing.T) {
	s := State{Camera: true, Screen: true, ReportedAt: t0}
	if !s.Open(t0.Add(19*time.Second)) || s.Open(t0.Add(21*time.Second)) {
		t.Fatal("the panel is open for 20 seconds after a report")
	}
	if !s.Live(Camera, t0) || s.Live(Audio, t0) || s.Live("nonsense", t0) {
		t.Fatal("live follows the flags")
	}
	if s.Live(Camera, t0.Add(time.Minute)) {
		t.Fatal("nothing is live once the panel has gone quiet")
	}
	if (State{}).Open(t0) {
		t.Fatal("a panel that never reported isn't open")
	}
}

func TestMissing(t *testing.T) {
	all := Require{Camera: true, Screen: true, Audio: true}
	ready := State{Camera: true, Screen: true, Audio: true, ReportedAt: t0}
	if m := Missing(all, ready, t0); len(m) != 0 {
		t.Fatal(m)
	}
	m := Missing(all, State{Camera: true, ReportedAt: t0}, t0)
	if !reflect.DeepEqual(m, []string{"the screen isn't shared", "the microphone isn't on"}) {
		t.Fatalf("%v", m)
	}
	// An assessment that doesn't need audio doesn't wait for it.
	if m := Missing(Require{Camera: true, Screen: true}, State{Camera: true, Screen: true, ReportedAt: t0}, t0); len(m) != 0 {
		t.Fatal(m)
	}
	// No panel at all: it is the first thing missing.
	m = Missing(all, State{}, t0)
	if len(m) != 4 || !strings.Contains(m[0], "panel isn't open") {
		t.Fatalf("%v", m)
	}
	if got := Describe([]string{"the screen isn't shared"}); got != "Not ready: the screen isn't shared." || Describe(nil) != "" {
		t.Fatal(got)
	}
}

func TestChanges(t *testing.T) {
	req := Require{Camera: true, Screen: true}
	on := State{Camera: true, Screen: true, Audio: true}
	// First report with everything on: nothing to note.
	if c := Changes(State{}, false, on, req); len(c) != 0 {
		t.Fatalf("%v", c)
	}
	// First report with the screen off: noted (it never started).
	if c := Changes(State{}, false, State{Camera: true}, req); !reflect.DeepEqual(c, []Change{{Screen, false}}) {
		t.Fatalf("%v", c)
	}
	// The screen share stops, then comes back.
	off := State{Camera: true, Audio: true}
	if c := Changes(on, true, off, req); !reflect.DeepEqual(c, []Change{{Screen, false}}) {
		t.Fatalf("%v", c)
	}
	if c := Changes(off, true, on, req); !reflect.DeepEqual(c, []Change{{Screen, true}}) {
		t.Fatalf("%v", c)
	}
	// Audio isn't required, so its changes aren't gaps.
	if c := Changes(on, true, State{Camera: true, Screen: true}, req); len(c) != 0 {
		t.Fatalf("%v", c)
	}
}

func TestForUpload(t *testing.T) {
	cases := []struct {
		what, ct, kind string
		ok             bool
	}{
		{"camera", "image/jpeg", "camera_still", true},
		{"camera", "IMAGE/PNG; charset=x", "camera_still", true},
		{"screen", "image/webp", "screenshot", true},
		{"audio", "audio/webm;codecs=opus", "audio_clip", true},
		{"camera", "audio/webm", "", false},
		{"audio", "image/jpeg", "", false},
		{"screen", "text/html", "", false},
		{"video", "video/mp4", "", false},
	}
	for _, c := range cases {
		u, ok := ForUpload(c.what, c.ct)
		if ok != c.ok || u.Kind != c.kind {
			t.Errorf("%s %s: %+v %v", c.what, c.ct, u, ok)
		}
	}
}
