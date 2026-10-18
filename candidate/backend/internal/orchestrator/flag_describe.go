package orchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
)

// proctorBucket is the private Supabase Storage bucket the browser uploads
// proctoring stills to.
const proctorBucket = "proctor-snapshots"

// maxDescribedFlags bounds how many flagged moments get a caption — the note
// itself shows at most 25, and all of them are described in one batched call.
const maxDescribedFlags = 25

// describedFlagsNote is activityFlagsNote enriched with a one-line description
// of what the screen still shows at each flagged moment — "an AI chat
// assistant is open", "the API docs" — so a reviewer reads what happened, not
// just a label. taskContext tells the describer what the task legitimately
// involves, so a caption can note whether what is on screen fits it.
//
// All the stills are described in a single vision call, not one per moment.
// Best-effort and never a verdict: it falls back to the plain note when the
// stills store or the describer isn't configured, and any still that can't be
// fetched or described simply has no caption. It never fails the evaluation.
func (o *Orchestrator) describedFlagsNote(ctx context.Context, sessionID string, events []db.ActivityEvent, startedAt time.Time, taskContext string) string {
	moments, camera := flagMoments(events, startedAt)
	if len(moments) == 0 {
		return ""
	}
	captions := map[int]string{}
	if o.Stills.Configured() && o.configured() {
		// Fetch the stills for the moments that have one, remembering which
		// moment each image belongs to, then describe them all in one call.
		var imgs []llm.LabeledImage
		var momentIdx []int
		for i, m := range moments {
			if i >= maxDescribedFlags || m.screenPath == "" {
				continue
			}
			data, ctype, err := o.Stills.Fetch(ctx, proctorBucket, m.screenPath)
			if err != nil || len(data) == 0 {
				continue
			}
			if ctype == "" {
				ctype = "image/jpeg"
			}
			imgs = append(imgs, llm.LabeledImage{
				Label: fmt.Sprintf("%02d:%02d into the session — %s", int(m.at.Minutes()), int(m.at.Seconds())%60, m.what),
				Img:   llm.ImagePart{MimeType: ctype, Data: base64.StdEncoding.EncodeToString(data)},
			})
			momentIdx = append(momentIdx, i)
		}
		if len(imgs) > 0 {
			descs, err := o.Agents.DescribeStills(ctx, imgs, taskContext)
			if err != nil {
				slog.Warn("orchestrator: describing flag stills failed", "session", sessionID, "error", err)
			} else {
				for j, desc := range descs {
					if strings.TrimSpace(desc) == "" {
						continue
					}
					i := momentIdx[j]
					captions[i] = strings.TrimSpace(desc)
					if m := moments[i]; m.eventID != 0 && o.DB != nil {
						// Store it against the flag so the report's Flags panel
						// can show it too, not just the prose. Best-effort.
						if err := o.DB.SaveFlagDescription(ctx, m.eventID, sessionID, captions[i]); err != nil {
							slog.Warn("orchestrator: saving flag description failed", "event", m.eventID, "error", err)
						}
					}
				}
			}
		}
	}
	return formatFlagMoments(moments, camera, captions)
}
