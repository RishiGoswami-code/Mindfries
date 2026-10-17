package orchestrator

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
)

// proctorBucket is the private Supabase Storage bucket the browser uploads
// proctoring stills to.
const proctorBucket = "proctor-snapshots"

// describedFlagsNote is activityFlagsNote enriched with a one-line description
// of what the screen still shows at each flagged moment — "an AI chat
// assistant is open", "the API docs" — so a reviewer reads what happened, not
// just a label. taskContext tells the describer what the task legitimately
// involves, so a caption can note whether what is on screen fits it.
//
// Best-effort and never a verdict: it falls back to the plain note when the
// stills store or the describer isn't configured, and any still that can't be
// fetched or described simply has no caption. It never fails the evaluation.
func (o *Orchestrator) describedFlagsNote(ctx context.Context, events []db.ActivityEvent, startedAt time.Time, taskContext string) string {
	moments, camera := flagMoments(events, startedAt)
	if len(moments) == 0 {
		return ""
	}
	captions := map[int]string{}
	if o.Stills.Configured() && o.configured() {
		for i, m := range moments {
			if i >= 25 || m.screenPath == "" {
				continue // the note shows at most 25, and not every flag has a screen still
			}
			data, ctype, err := o.Stills.Fetch(ctx, proctorBucket, m.screenPath)
			if err != nil || len(data) == 0 {
				continue
			}
			if ctype == "" {
				ctype = "image/jpeg"
			}
			desc, err := o.Agents.DescribeStill(ctx, llm.ImagePart{MimeType: ctype, Data: base64.StdEncoding.EncodeToString(data)}, taskContext)
			if err == nil && strings.TrimSpace(desc) != "" {
				captions[i] = strings.TrimSpace(desc)
			}
		}
	}
	return formatFlagMoments(moments, camera, captions)
}
