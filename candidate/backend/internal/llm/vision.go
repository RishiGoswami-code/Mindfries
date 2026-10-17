package llm

import (
	"context"
	"errors"
	"strings"
)

// proctorDescriberTokens bounds the caption: a sentence or two, not an essay.
const proctorDescriberTokens = 300

// proctorDescriberPrompt asks the model to say plainly what a proctoring
// still shows — the application or site visible and what the person is doing,
// naming an AI chat tool, a phone, or a second person if present. It is
// deliberately a description for a human reviewer, never a verdict: the
// platform's principle is to collect evidence, not to judge, and automated
// proctoring carries false positives (Belzak et al., 2025) that a caption
// claiming "cheating" would only amplify.
const proctorDescriberPrompt = `You are describing a single still captured during a proctored coding assessment, for a human reviewer to weigh.
In one or two sentences, say what application or website is visible and what the person appears to be doing.
If an AI chat assistant (for example ChatGPT), a phone, or a second person is visible, state that explicitly.
Describe only what you can actually see. Do not guess the person's intent, and do not say whether this is cheating — that is for the human reviewer.`

// DescribeStill returns a short, plain description of a proctoring still (a
// screen or camera frame). taskContext, when given, tells the model what the
// task legitimately involves, so the caption can note whether what is on
// screen fits — still a description for a reviewer, never a verdict.
//
// It uses the Report model (vision-capable on Gemini) at low thinking: this
// is a caption, not an analysis. Images ride on the user message; only the
// Gemini client reads them, so this needs AI_PROVIDER=gemini to see anything.
func (a *Agents) DescribeStill(ctx context.Context, img ImagePart, taskContext string) (string, error) {
	if strings.TrimSpace(img.Data) == "" {
		return "", errors.New("llm: no image to describe")
	}
	prompt := proctorDescriberPrompt
	if c := strings.TrimSpace(taskContext); c != "" {
		prompt += "\n\nWhat this task legitimately involves (so you can note whether what is shown fits it):\n" + c
	}
	return a.quickProse(withAgent(ctx, "proctor_describer"), a.models.Report, []ChatMessage{
		{Role: "user", Content: prompt, Images: []ImagePart{img}},
	}, proctorDescriberTokens)
}

// cameraCheckTokens bounds the reply: "OK", or one short sentence.
const cameraCheckTokens = 150

// cameraCheckPrompt asks the model to report only what a reviewer should look
// at on a webcam still — a phone, a second person, an absent or turned-away
// candidate — and to answer "OK" otherwise, so a session of ordinary frames
// costs little and produces no noise.
const cameraCheckPrompt = `This is a webcam still from a proctored coding assessment, for a human reviewer.
Report ONLY if something is notable: a phone or other device held up or being looked at, a second person in frame, the candidate absent or their face not visible, or the candidate clearly turned away from the screen.
If the candidate is simply present and facing the screen, reply with exactly "OK" and nothing else.
Otherwise describe what you see in one short factual sentence. Do not judge whether it is cheating.`

// CheckCameraStill reports what is notable on a webcam still — a phone, a
// second person, an absent or turned-away candidate — or "" when nothing is
// (the model answered "OK"). A description for a reviewer, never a verdict, and
// Gemini-only (images ride on the message; OpenRouter stays text-only).
func (a *Agents) CheckCameraStill(ctx context.Context, img ImagePart) (string, error) {
	if strings.TrimSpace(img.Data) == "" {
		return "", errors.New("llm: no image to check")
	}
	out, err := a.quickProse(withAgent(ctx, "proctor_camera"), a.models.Report, []ChatMessage{
		{Role: "user", Content: cameraCheckPrompt, Images: []ImagePart{img}},
	}, cameraCheckTokens)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" || strings.EqualFold(out, "OK") || strings.EqualFold(out, "OK.") || strings.HasPrefix(strings.ToUpper(out), "OK ") {
		return "", nil
	}
	return out, nil
}
