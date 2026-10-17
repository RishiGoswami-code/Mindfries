package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GeminiClient calls Google's Gemini API directly (generativelanguage
// .googleapis.com) with the GEMINI_API_KEY from Google AI Studio — the same
// key the live voice interview uses (live.go). It is the default for every
// agent; OpenRouterClient remains for anyone who sets AI_PROVIDER=openrouter.
//
// It speaks this package's chat shape (ChatMessage) so the agents don't know
// which provider they are talking to. The translation:
//
//	"system" messages     → systemInstruction (joined, in order)
//	"user" / "assistant"  → contents with role "user" / "model"
//	CompleteQuick         → thinking turned down to "low"
//
// Checked against the API on 2026-10-14 with the project's key: a plain
// call, a call cut off at its limit, a streamed call and an error.

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type GeminiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	onUsage    func(ctx context.Context, usage Usage)
	// fallback is a second model to try when the first one stays
	// unavailable (GEMINI_FALLBACK_MODEL). Empty means none.
	fallback string
}

// WithFallback sets the model tried when the requested one stays
// unavailable after its retries.
func (c *GeminiClient) WithFallback(model string) *GeminiClient {
	c.fallback = strings.TrimSpace(model)
	return c
}

// NewGeminiClient builds a client. An empty apiKey is allowed: Configured
// reports false and every call returns ErrNotConfigured.
func NewGeminiClient(apiKey, baseURL string) *GeminiClient {
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}
	return &GeminiClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		// Evaluation prompts carry a whole session's trail, and task
		// generation writes a whole codebase with full thinking: either can
		// take minutes before answering.
		httpClient: &http.Client{Timeout: 6 * time.Minute},
	}
}

func (c *GeminiClient) Configured() bool { return c.apiKey != "" }

func (c *GeminiClient) SetOnUsage(fn func(ctx context.Context, usage Usage)) { c.onUsage = fn }

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	Thought    bool              `json:"thought,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

// geminiInlineData is an image carried in a request part, for vision calls.
type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded image bytes
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiThinking struct {
	ThinkingLevel string `json:"thinkingLevel,omitempty"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  struct {
		Temperature     float64         `json:"temperature"`
		MaxOutputTokens int             `json:"maxOutputTokens,omitempty"`
		ThinkingConfig  *geminiThinking `json:"thinkingConfig,omitempty"`
	} `json:"generationConfig"`
}

// QuotaError is Google refusing a call because the key's quota is used up
// (429 RESOURCE_EXHAUSTED). Measured 2026-10-14: a free-tier key allows 20
// requests a day per model. Trying again in a few seconds doesn't help;
// RetryAfter is how long Google says to wait, when it says.
type QuotaError struct {
	Message    string
	RetryAfter time.Duration
}

func (e *QuotaError) Error() string { return "llm: gemini quota used up: " + e.Message }

// BusyError is Google being temporarily unable to answer — "high demand,
// try again later" (503), an internal error, or no reply in time — after
// this client's own retries. Waiting a while and trying again is the fix.
type BusyError struct{ Message string }

func (e *BusyError) Error() string { return "llm: gemini is busy: " + e.Message }

type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback,omitempty"`
	UsageMetadata *struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			RetryDelay string `json:"retryDelay"`
		} `json:"details"`
	} `json:"error,omitempty"`
}

// Room for the model's thinking on top of the reply. Gemini counts thinking
// against maxOutputTokens; the callers' ceilings were written for the reply
// alone, and without this a model that thinks first can spend the whole
// allowance before writing a word. Measured 2026-10-14: generating a task
// with full thinking ran past 4096 tokens of headroom and was cut off.
const (
	geminiQuickThinking = 4096
	geminiFullThinking  = 24576
)

func buildGeminiRequest(messages []ChatMessage, maxTokens int, quick bool) geminiRequest {
	var req geminiRequest
	var system []string
	for _, m := range messages {
		switch m.Role {
		case "system":
			system = append(system, m.Content)
		case "assistant":
			req.Contents = append(req.Contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: m.Content}}})
		default:
			parts := []geminiPart{{Text: m.Content}}
			for _, img := range m.Images {
				parts = append(parts, geminiPart{InlineData: &geminiInlineData{MimeType: img.MimeType, Data: img.Data}})
			}
			req.Contents = append(req.Contents, geminiContent{Role: "user", Parts: parts})
		}
	}
	if len(system) > 0 {
		req.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: strings.Join(system, "\n\n")}}}
	}
	if len(req.Contents) == 0 {
		// Gemini needs at least one turn; a system-only prompt becomes one.
		req.Contents = []geminiContent{{Role: "user", Parts: []geminiPart{{Text: "Begin."}}}}
	}
	req.GenerationConfig.Temperature = 0.2
	if maxTokens > 0 {
		room := geminiFullThinking
		if quick {
			room = geminiQuickThinking
		}
		req.GenerationConfig.MaxOutputTokens = maxTokens + room
	}
	if quick {
		req.GenerationConfig.ThinkingConfig = &geminiThinking{ThinkingLevel: "low"}
	}
	return req
}

// text is the reply without the model's thinking.
func (r geminiResponse) text() string {
	if len(r.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		if !p.Thought {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func (r geminiResponse) problem() error {
	if r.Error != nil && r.Error.Status == "RESOURCE_EXHAUSTED" {
		q := &QuotaError{Message: firstLine(r.Error.Message)}
		for _, d := range r.Error.Details {
			if wait, err := time.ParseDuration(d.RetryDelay); err == nil && wait > 0 {
				q.RetryAfter = wait
			}
		}
		return q
	}
	if r.Error != nil && (r.Error.Status == "UNAVAILABLE" || r.Error.Status == "INTERNAL" || r.Error.Status == "DEADLINE_EXCEEDED") {
		return &BusyError{Message: firstLine(r.Error.Message)}
	}
	if r.Error != nil {
		return fmt.Errorf("llm: gemini error (%s): %s", r.Error.Status, r.Error.Message)
	}
	if r.PromptFeedback != nil && r.PromptFeedback.BlockReason != "" {
		return fmt.Errorf("llm: gemini refused the prompt (%s)", r.PromptFeedback.BlockReason)
	}
	return nil
}

func (c *GeminiClient) report(ctx context.Context, model string, r geminiResponse) {
	if c.onUsage == nil || r.UsageMetadata == nil {
		return
	}
	agent, sessionID := tagsFrom(ctx)
	// Google doesn't put a price on each response the way OpenRouter does;
	// the tokens are exact, the cost is left at zero rather than guessed.
	c.onUsage(ctx, Usage{
		Agent: agent, SessionID: sessionID, Model: model,
		PromptTokens:     r.UsageMetadata.PromptTokenCount,
		CompletionTokens: r.UsageMetadata.CandidatesTokenCount + r.UsageMetadata.ThoughtsTokenCount,
		CachedTokens:     r.UsageMetadata.CachedContentTokenCount,
	})
}

// Google answers "high demand, try again later" (503 UNAVAILABLE) fairly
// often for its newest models — measured 2026-10-14, two of three calls to
// two of the Flash models. Those, and plain 500s, are tried again after a
// short wait; then, if GEMINI_FALLBACK_MODEL is set, on that model.
// Quota errors (429) are not retried: waiting a few seconds doesn't help.
var geminiRetryWaits = []time.Duration{1500 * time.Millisecond, 4 * time.Second}

func retryable(status int) bool {
	return status == http.StatusServiceUnavailable || status == http.StatusInternalServerError || status == http.StatusBadGateway
}

// Google's quotas are per model: when one model's is used up, the fallback
// model (if any) may still have room. It isn't worth retrying the same one.

func (c *GeminiClient) post(ctx context.Context, client *http.Client, model, method string, body geminiRequest) (*http.Response, error) {
	models := []string{model}
	if c.fallback != "" && c.fallback != model {
		models = append(models, c.fallback)
	}
	for mi, m := range models {
		for attempt := 0; ; attempt++ {
			resp, err := c.postOnce(ctx, client, m, method, body)
			if err == nil && resp.StatusCode == http.StatusTooManyRequests && mi < len(models)-1 {
				resp.Body.Close()
				break // on to the fallback model
			}
			if err != nil || !retryable(resp.StatusCode) {
				return resp, err
			}
			outOfTries := attempt == len(geminiRetryWaits)
			if outOfTries && mi == len(models)-1 {
				return resp, nil // the caller reads the error out of it
			}
			resp.Body.Close()
			if outOfTries {
				break // on to the fallback model
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(geminiRetryWaits[attempt]):
			}
		}
	}
	return nil, errors.New("llm: gemini: no model to call")
}

func (c *GeminiClient) postOnce(ctx context.Context, client *http.Client, model, method string, body geminiRequest) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/models/%s:%s", c.baseURL, url.PathEscape(model), method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	// In a header, not the query string, so it stays out of any log of URLs.
	req.Header.Set("x-goog-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			// The connection failed or the reply took longer than the
			// client's limit — Google's side, not the request's.
			return nil, &BusyError{Message: err.Error()}
		}
		return nil, fmt.Errorf("llm: gemini request failed: %w", err)
	}
	return resp, nil
}

func (c *GeminiClient) Complete(ctx context.Context, model string, messages []ChatMessage, maxTokens int) (string, error) {
	return c.complete(ctx, model, messages, maxTokens, false)
}

func (c *GeminiClient) CompleteQuick(ctx context.Context, model string, messages []ChatMessage, maxTokens int) (string, error) {
	return c.complete(ctx, model, messages, maxTokens, true)
}

func (c *GeminiClient) complete(ctx context.Context, model string, messages []ChatMessage, maxTokens int, quick bool) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	resp, err := c.post(ctx, c.httpClient, model, "generateContent", buildGeminiRequest(messages, maxTokens, quick))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var out geminiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("llm: gemini returned unparseable response (http %d): %s", resp.StatusCode, truncateForError(raw))
	}
	if err := out.problem(); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("llm: gemini http %d: %s", resp.StatusCode, truncateForError(raw))
	}
	if len(out.Candidates) == 0 {
		return "", errors.New("llm: gemini returned no candidates")
	}
	c.report(ctx, model, out)
	content := out.text()
	if out.Candidates[0].FinishReason == "MAX_TOKENS" {
		return content, ErrTruncated
	}
	return content, stoppedEarly(out.Candidates[0].FinishReason)
}

// CompleteQuickStream is CompleteQuick, delivered piece by piece — see
// OpenRouterClient.CompleteQuickStream for the contract.
func (c *GeminiClient) CompleteQuickStream(ctx context.Context, model string, messages []ChatMessage, maxTokens int, onDelta func(string)) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	// No overall timeout: it would cut a long reply off mid-stream. The
	// caller's context bounds it instead.
	resp, err := c.post(ctx, &http.Client{}, model, "streamGenerateContent?alt=sse", buildGeminiRequest(messages, maxTokens, true))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		var out geminiResponse
		if json.Unmarshal(raw, &out) == nil && out.problem() != nil {
			return "", out.problem()
		}
		// Some errors arrive as a one-element array.
		var list []geminiResponse
		if json.Unmarshal(raw, &list) == nil && len(list) > 0 && list[0].problem() != nil {
			return "", list[0].problem()
		}
		return "", fmt.Errorf("llm: gemini http %d: %s", resp.StatusCode, truncateForError(raw))
	}

	var whole strings.Builder
	var last geminiResponse
	finish := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var chunk geminiResponse
		if json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &chunk) != nil {
			continue
		}
		if err := chunk.problem(); err != nil {
			return whole.String(), err
		}
		if piece := chunk.text(); piece != "" {
			whole.WriteString(piece)
			onDelta(piece)
		}
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].FinishReason != "" {
			finish = chunk.Candidates[0].FinishReason
		}
		if chunk.UsageMetadata != nil {
			last = chunk
		}
	}
	if err := scanner.Err(); err != nil {
		return whole.String(), fmt.Errorf("llm: gemini stream broke off: %w", err)
	}
	c.report(ctx, model, last)
	if finish == "MAX_TOKENS" {
		return whole.String(), ErrTruncated
	}
	return whole.String(), stoppedEarly(finish)
}

// stoppedEarly is a finish reason that means Google cut the reply off for
// its own reasons — a safety or copyright ("recitation") filter. Measured on
// 2026-10-14: asked to count to thirty in words, the stream stopped at
// "seventeen" with RECITATION. Whatever arrived is kept, but it is reported
// as an error, never as a finished reply.
func stoppedEarly(reason string) error {
	switch reason {
	case "", "STOP", "MAX_TOKENS":
		return nil
	}
	return fmt.Errorf("llm: gemini stopped the reply early (%s)", reason)
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\n*"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func truncateForError(b []byte) string {
	if len(b) > 400 {
		return string(b[:400]) + "…"
	}
	return string(b)
}
