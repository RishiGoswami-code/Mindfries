// Package llm wires the engine to a hosted model. Its only job today is the
// optional BRIEF step: reword an authored task brief to a company's JD —
// wording only, never changing what's tested. It satisfies taskgen.Provider;
// main.go uses it only when GEMINI_API_KEY is set, so by default the engine
// resolves tasks free and offline (the authored brief ships verbatim).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	// Google retires model aliases over time (gemini-2.0-flash now 404s with a
	// "no longer available" message). Override with GEMINI_MODEL when the
	// current default is retired, without a code change.
	defaultModel = "gemini-3.8-flash"
	endpoint     = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent"
	maxWait      = 20 * time.Second
)

// Gemini adapts briefs via the Google Generative Language REST API.
type Gemini struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewGemini builds a provider. model defaults to gemini-2.0-flash when empty.
func NewGemini(apiKey, model string) *Gemini {
	if model == "" {
		model = defaultModel
	}
	return &Gemini{apiKey: apiKey, model: model, http: &http.Client{Timeout: maxWait}}
}

// Model is the model name in use (for startup logging).
func (g *Gemini) Model() string { return g.model }

func (g *Gemini) Adapt(ctx context.Context, brief, jd string) (string, error) {
	reqBody, err := json.Marshal(request{
		Contents:         []content{{Role: "user", Parts: []part{{Text: buildPrompt(brief, jd)}}}},
		GenerationConfig: &genConfig{Temperature: 0.2},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf(endpoint, g.model), bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.apiKey)

	res, err := g.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini: status %d: %s", res.StatusCode, truncate(string(body), 200))
	}

	adapted, err := parseResponse(body)
	if err != nil {
		return "", err
	}
	// Never let a reword drop a gradeable anchor (a file path, identifier or
	// command the acceptance tests depend on). If one went missing, treat it as
	// a failed adaptation so the caller keeps the authored brief.
	if !preservesAnchors(brief, adapted) {
		return "", fmt.Errorf("gemini: adapted brief dropped a code anchor; keeping authored brief")
	}
	return adapted, nil
}

func buildPrompt(brief, jd string) string {
	return `You are tailoring a technical-assessment task brief for a specific hiring role.
Rewrite the brief below so its framing, terminology and examples fit the company's
job description, so the candidate feels it matches the role they applied for.

HARD RULES — breaking any one makes the task ungradeable:
- Do NOT change what the candidate must do. Keep every requirement, acceptance
  criterion and expected behaviour exactly.
- Do NOT change, add or remove any file path, function/type name, command,
  package or code identifier. Reproduce every ` + "`backticked`" + ` token verbatim.
- Do NOT add new requirements, hints or the solution.
- Keep it the same length or shorter. Markdown only. Output ONLY the rewritten
  brief — no preamble, no code fences around the whole thing.

=== JOB DESCRIPTION ===
` + jd + `

=== TASK BRIEF TO REWRITE ===
` + brief
}

var inlineCode = regexp.MustCompile("`([^`\n]+)`")

// preservesAnchors reports whether every backticked token in the original brief
// (file paths, identifiers, commands) still appears verbatim in the adapted one.
func preservesAnchors(original, adapted string) bool {
	for _, m := range inlineCode.FindAllStringSubmatch(original, -1) {
		if !strings.Contains(adapted, m[1]) {
			return false
		}
	}
	return true
}

func parseResponse(body []byte) (string, error) {
	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("gemini: decode response: %w", err)
	}
	if len(r.Candidates) == 0 {
		return "", fmt.Errorf("gemini: no candidates in response")
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("gemini: empty text in response")
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- wire types (only the fields we use) ---

type request struct {
	Contents         []content  `json:"contents"`
	GenerationConfig *genConfig `json:"generationConfig,omitempty"`
}
type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}
type part struct {
	Text string `json:"text"`
}
type genConfig struct {
	Temperature float64 `json:"temperature"`
}
type response struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
}
