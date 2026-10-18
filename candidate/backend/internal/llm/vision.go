package llm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// proctorDescriberPrompt asks the model to say plainly what each proctoring
// still shows — the application or site visible and what the person is doing,
// naming an AI chat tool, a phone, or a second person if present. It is
// deliberately a description for a human reviewer, never a verdict: the
// platform's principle is to collect evidence, not to judge, and automated
// proctoring carries false positives (Belzak et al., 2025) that a caption
// claiming "cheating" would only amplify.
//
// Detection (is there a phone, how many faces) is done for free in the
// browser (candidate/frontend face-watch.ts); this is the one thing a vision
// model does that the browser can't — describe an open-vocabulary scene in
// words — so it runs only on the moments the browser already flagged, and all
// of them in one call.
const proctorDescriberPrompt = `You are describing stills captured during a proctored coding assessment, for a human reviewer to weigh. Each image is one flagged moment.
For each image, in one or two sentences, say what application or website is visible and what the person appears to be doing. If an AI chat assistant (for example ChatGPT), a phone, or a second person is visible, state that explicitly.
Describe only what you can actually see. Do not guess the person's intent, and do not say whether this is cheating — that is for the human reviewer.
The images are given in order. Reply with one line per image, in the form "<number>: <description>", starting at 1, and nothing else.`

// LabeledImage is one still to describe, with a short label (its time and what
// was flagged) that goes to the model as context for that image.
type LabeledImage struct {
	Label string
	Img   ImagePart
}

// maxStillsPerCall caps how many images ride in one vision request. Gemini
// accepts many; this keeps a single reply parseable and its output budget
// sane. More flagged moments than this are handled in further calls — but a
// session rarely flags this many, so in practice it is one call.
const maxStillsPerCall = 16

// perStillTokens is the output budget allowed per image in a batch.
const perStillTokens = 120

// DescribeStills describes several proctoring stills in as few vision calls as
// possible — one call per maxStillsPerCall images — and returns a caption per
// image, in the same order as the input. taskContext, when given, tells the
// model what the task legitimately involves so a caption can note whether what
// is shown fits. A still that produced no caption comes back as "".
//
// It uses the Report model (vision-capable on Gemini) at low thinking: these
// are captions, not analysis. Images ride on the user message; only the Gemini
// client reads them, so this needs AI_PROVIDER=gemini to see anything.
func (a *Agents) DescribeStills(ctx context.Context, imgs []LabeledImage, taskContext string) ([]string, error) {
	if len(imgs) == 0 {
		return nil, errors.New("llm: no images to describe")
	}
	out := make([]string, len(imgs))
	var firstErr error
	for start := 0; start < len(imgs); start += maxStillsPerCall {
		end := min(start+maxStillsPerCall, len(imgs))
		batch := imgs[start:end]
		captions, err := a.describeBatch(ctx, batch, taskContext)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue // this batch has no captions; the others still stand
		}
		for i, c := range captions {
			out[start+i] = c
		}
	}
	// All batches failed and none produced a caption: surface the error so the
	// caller falls back to the plain, uncaptioned note.
	if firstErr != nil && allEmpty(out) {
		return nil, firstErr
	}
	return out, nil
}

// describeBatch is one vision call over up to maxStillsPerCall images.
func (a *Agents) describeBatch(ctx context.Context, batch []LabeledImage, taskContext string) ([]string, error) {
	var prompt strings.Builder
	prompt.WriteString(proctorDescriberPrompt)
	if c := strings.TrimSpace(taskContext); c != "" {
		prompt.WriteString("\n\nWhat this task legitimately involves (so you can note whether what is shown fits it):\n" + c)
	}
	prompt.WriteString("\n\nThe images, in order:")
	images := make([]ImagePart, 0, len(batch))
	for i, b := range batch {
		label := strings.TrimSpace(b.Label)
		if label == "" {
			label = "a flagged moment"
		}
		fmt.Fprintf(&prompt, "\n%d: %s", i+1, label)
		images = append(images, b.Img)
	}
	raw, err := a.quickProse(withAgent(ctx, "proctor_describer"), a.models.Report, []ChatMessage{
		{Role: "user", Content: prompt.String(), Images: images},
	}, perStillTokens*len(batch)+120)
	if err != nil {
		return nil, err
	}
	return parseNumberedCaptions(raw, len(batch)), nil
}

// captionLine matches a reply line that starts with "<n>:" or "<n>." — the
// numbering the prompt asks for. A line without one is attached to the last
// numbered caption (a description that wrapped onto a second line).
var captionLine = regexp.MustCompile(`^\s*(\d+)\s*[:.)]\s*(.*)$`)

// parseNumberedCaptions turns "1: …\n2: …" into a slice of n captions, indexed
// from 0. A number out of range is ignored; a missing one stays "".
func parseNumberedCaptions(raw string, n int) []string {
	out := make([]string, n)
	last := -1
	for _, line := range strings.Split(raw, "\n") {
		if m := captionLine.FindStringSubmatch(line); m != nil {
			idx, _ := strconv.Atoi(m[1])
			idx-- // the model numbers from 1
			if idx >= 0 && idx < n {
				out[idx] = strings.TrimSpace(m[2])
				last = idx
			}
			continue
		}
		// A continuation of the previous caption.
		if t := strings.TrimSpace(line); t != "" && last >= 0 {
			out[last] = strings.TrimSpace(out[last] + " " + t)
		}
	}
	return out
}

func allEmpty(s []string) bool {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
