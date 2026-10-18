package orchestrator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
)

// maxCameraChecks bounds how many webcam stills the vision check looks at in
// one evaluation. Each check is cheap (a small image, a one-word "OK" for an
// ordinary frame), so this is set high for coverage, not cost: a brief phone-up
// is only caught if its still is actually checked, and sampling too hard drops
// frames twice over (the 15s capture interval, then the downsample). At 15s
// snapshots this covers a session up to ~22 minutes with every still checked;
// longer sessions sample more coarsely. The ceiling exists only to keep the
// evaluation's own running time inside its budget — the checks run in sequence.
const maxCameraChecks = 90

// cameraObservations looks at a sample of the session's webcam stills and
// reports what a reviewer should look at — a phone, a second person, an absent
// or turned-away candidate — with the time each was seen. This catches what
// the live flags miss: holding a phone up to the screen leaves no tab, no
// paste, no keypress, so nothing flags it — but the camera sees it.
//
// Empty when the stills store or the describer isn't configured, or nothing
// was notable. Best-effort: a still that can't be fetched or checked is
// skipped, never fails the evaluation.
func (o *Orchestrator) cameraObservations(ctx context.Context, events []db.ActivityEvent, startedAt time.Time) string {
	if !o.Stills.Configured() || !o.configured() {
		return ""
	}
	type shot struct {
		at   time.Duration
		path string
	}
	var shots []shot
	for _, e := range events {
		if e.EventType != eventCameraSnapshot {
			continue
		}
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(e.Payload, &p) != nil || p.Path == "" {
			continue
		}
		shots = append(shots, shot{at: max(0, e.OccurredAt.Sub(startedAt)), path: p.Path})
	}
	if len(shots) == 0 {
		return ""
	}
	step := 1
	if len(shots) > maxCameraChecks {
		step = (len(shots) + maxCameraChecks - 1) / maxCameraChecks
	}

	var lines []string
	for i := 0; i < len(shots); i += step {
		s := shots[i]
		data, ctype, err := o.Stills.Fetch(ctx, proctorBucket, s.path)
		if err != nil || len(data) == 0 {
			continue
		}
		if ctype == "" {
			ctype = "image/jpeg"
		}
		obs, err := o.Agents.CheckCameraStill(ctx, llm.ImagePart{MimeType: ctype, Data: base64.StdEncoding.EncodeToString(data)})
		if err != nil {
			slog.Warn("orchestrator: camera check failed", "path", s.path, "error", err)
			continue
		}
		if obs != "" {
			lines = append(lines, fmt.Sprintf("- %02d:%02d — %s", int(s.at.Minutes()), int(s.at.Seconds())%60, obs))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("What the camera showed at moments worth a look — from a sample of the webcam stills, described for a person to weigh, never findings. Lighting and framing can mislead, and none of this speaks to intent:\n")
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}
