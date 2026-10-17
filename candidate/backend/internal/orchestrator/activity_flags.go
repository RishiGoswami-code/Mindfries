package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
)

// Flagged moments in a proctored session (issue #120): what the candidate's
// browser flagged live, and when — leaving the workspace, voices on the
// microphone, a second face, a developer-tools shortcut — each recorded with
// a camera still, a screen still and, for sounds, a short clip, which the
// company portal shows beside it.
//
// The report lists them as facts with their times and says plainly what
// they are: prompts for a person to look at the evidence, not findings.
// Research on automated proctoring is consistent about why — flags carry
// false positives and human reviewers can over-trust them (Belzak et al.,
// 2025), and face detection has been found less reliable for some skin
// tones (Burgess et al., 2022) — and no model here judges them.

const eventActivityFlag = "activity_flag"

var flagWords = map[string]string{
	"left_workspace":       "Left the workspace",
	"fullscreen_exit":      "Left fullscreen",
	"screen_share_stopped": "Stopped sharing the screen",
	"extra_display":        "More than one display connected",
	"external_paste":       "Large paste from outside the workspace",
	"face_missing":         "Nobody in front of the camera",
	"multiple_faces":       "More than one face on camera",
	"voices_detected":      "Voices on the microphone",
	"loud_noise":           "Loud noise on the microphone",
	"devtools_attempt":     "Tried to open developer tools",
	"screenshot_attempt":   "Pressed PrintScreen",
	"second_window":        "Assessment open in a second window",
}

// flagMoment is one flagged moment, parsed from an activity_flag event.
type flagMoment struct {
	at         time.Duration
	what       string
	kind       string
	screenPath string // the screen still in proctor-snapshots, for the describer
}

// flagMoments parses the session's activity_flag events into moments, in
// order, and reports whether any came from the camera (the least reliable).
func flagMoments(events []db.ActivityEvent, startedAt time.Time) ([]flagMoment, bool) {
	var moments []flagMoment
	camera := false
	for _, e := range events {
		if e.EventType != eventActivityFlag {
			continue
		}
		var p struct {
			Kind       string         `json:"kind"`
			At         string         `json:"at"`
			Detail     map[string]any `json:"detail"`
			ScreenPath string         `json:"screenPath"`
		}
		if json.Unmarshal(e.Payload, &p) != nil || flagWords[p.Kind] == "" {
			continue
		}
		when := e.OccurredAt
		if t, err := time.Parse(time.RFC3339Nano, p.At); err == nil {
			when = t
		}
		what := flagWords[p.Kind]
		if p.Kind == "external_paste" {
			if chars, ok := p.Detail["chars"].(float64); ok {
				what += fmt.Sprintf(" (%d characters)", int(chars))
			}
		}
		if p.Kind == "face_missing" || p.Kind == "multiple_faces" {
			camera = true
		}
		moments = append(moments, flagMoment{at: max(0, when.Sub(startedAt)), what: what, kind: p.Kind, screenPath: p.ScreenPath})
	}
	return moments, camera
}

// activityFlagsNote is the report's list of flagged moments, or "" when there
// were none.
func activityFlagsNote(events []db.ActivityEvent, startedAt time.Time) string {
	moments, camera := flagMoments(events, startedAt)
	return formatFlagMoments(moments, camera, nil)
}

// formatFlagMoments renders the moments. captions, when non-nil, maps a
// moment's index to a one-line description of what its screen still shows,
// appended to that moment's line — "on screen: an AI chat assistant is open".
func formatFlagMoments(moments []flagMoment, camera bool, captions map[int]string) string {
	if len(moments) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d moment%s flagged during the session, each with the camera and the screen at that moment (and a short clip for sounds), for a person to look at:\n", len(moments), wereFlagged(len(moments)))
	for i, m := range moments {
		if i == 25 {
			fmt.Fprintf(&b, "- … and %d more\n", len(moments)-25)
			break
		}
		fmt.Fprintf(&b, "- %02d:%02d — %s", int(m.at.Minutes()), int(m.at.Seconds())%60, m.what)
		if c := captions[i]; c != "" {
			fmt.Fprintf(&b, " — on screen: %s", c)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nThese are prompts to look, not findings. Leaving the workspace can be reading documentation; voices can be the candidate thinking aloud.")
	if camera {
		b.WriteString(" Face detection is the least reliable of these — it is affected by lighting and, in published studies, by skin tone — so check the camera still before reading anything into it.")
	}
	b.WriteString(" All of it was recorded by the candidate's own browser.")
	return b.String()
}

func wereFlagged(n int) string {
	if n == 1 {
		return " was"
	}
	return "s were"
}
