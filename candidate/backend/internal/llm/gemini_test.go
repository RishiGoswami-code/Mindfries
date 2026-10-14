package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// The request Gemini receives: system turns become its system instruction,
// the assistant's turns become "model", and the key travels in a header.
func TestGeminiRequestShape(t *testing.T) {
	var got geminiRequest
	var path, key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, key = r.URL.Path, r.Header.Get("x-goog-api-key")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"thinking…","thought":true},{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3,"thoughtsTokenCount":5}}`))
	}))
	defer srv.Close()

	c := NewGeminiClient("k", srv.URL)
	var usage Usage
	c.SetOnUsage(func(_ context.Context, u Usage) { usage = u })
	out, err := c.CompleteQuick(withAgent(context.Background(), "assistant"), "gemini-x", []ChatMessage{
		{Role: "system", Content: "be brief"}, {Role: "user", Content: "hi"}, {Role: "assistant", Content: "hey"}, {Role: "user", Content: "again"},
	}, 100)
	if err != nil || out != "hello" {
		t.Fatalf("got %q, %v — the thought part must be left out", out, err)
	}
	if path != "/models/gemini-x:generateContent" || key != "k" {
		t.Errorf("path %q, key %q", path, key)
	}
	if got.SystemInstruction == nil || got.SystemInstruction.Parts[0].Text != "be brief" {
		t.Errorf("system instruction: %+v", got.SystemInstruction)
	}
	roles := []string{}
	for _, c := range got.Contents {
		roles = append(roles, c.Role)
	}
	if strings.Join(roles, ",") != "user,model,user" {
		t.Errorf("roles %v", roles)
	}
	if got.GenerationConfig.ThinkingConfig == nil || got.GenerationConfig.ThinkingConfig.ThinkingLevel != "low" {
		t.Errorf("quick call should turn thinking down: %+v", got.GenerationConfig)
	}
	if usage.Agent != "assistant" || usage.PromptTokens != 12 || usage.CompletionTokens != 8 {
		t.Errorf("usage %+v", usage)
	}
}

func TestGeminiFilteredReplyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"one, two"}]},"finishReason":"RECITATION"}]}`))
	}))
	defer srv.Close()
	out, err := NewGeminiClient("k", srv.URL).Complete(context.Background(), "m", []ChatMessage{{Role: "user", Content: "x"}}, 10)
	if out != "one, two" || err == nil || !strings.Contains(err.Error(), "RECITATION") {
		t.Errorf("got %q %v", out, err)
	}
}

func TestGeminiTruncatedAndErrors(t *testing.T) {
	reply := `{"candidates":[{"content":{"parts":[{"text":"part of it"}]},"finishReason":"MAX_TOKENS"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"models/missing is not found","status":"NOT_FOUND"}}`))
			return
		}
		_, _ = w.Write([]byte(reply))
	}))
	defer srv.Close()
	c := NewGeminiClient("k", srv.URL)
	out, err := c.Complete(context.Background(), "m", []ChatMessage{{Role: "user", Content: "x"}}, 10)
	if out != "part of it" || !errors.Is(err, ErrTruncated) {
		t.Errorf("truncated: %q %v", out, err)
	}
	if _, err := c.Complete(context.Background(), "missing", []ChatMessage{{Role: "user", Content: "x"}}, 10); err == nil || !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Errorf("error: %v", err)
	}
	if _, err := NewGeminiClient("", srv.URL).Complete(context.Background(), "m", nil, 10); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("no key: %v", err)
	}
}

// Against Google itself: GEMINI_LIVE_TEST=1 GEMINI_API_KEY=… go test -run TestGeminiLive ./internal/llm
func TestGeminiLive(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 and GEMINI_API_KEY to call the real API")
	}
	c := NewGeminiClient(os.Getenv("GEMINI_API_KEY"), "")
	ctx := context.Background()
	msgs := []ChatMessage{{Role: "system", Content: "Reply with exactly one word."}, {Role: "user", Content: "What colour is a clear daytime sky?"}}
	models := GeminiAgentModels()
	for _, m := range []string{models.Assistant, models.TaskGeneration} {
		out, err := c.Complete(ctx, m, msgs, 50)
		if err != nil || !strings.Contains(strings.ToLower(out), "blue") {
			t.Errorf("%s complete: %q %v", m, out, err)
		}
		t.Logf("%s: %q", m, strings.TrimSpace(out))
	}
	var pieces int
	out, err := c.CompleteQuickStream(ctx, models.Assistant, []ChatMessage{{Role: "user", Content: "In about 120 words, explain to a junior developer why a failing test is useful. End with the word DONE."}}, 600, func(string) { pieces++ })
	if err != nil || pieces < 2 || !strings.Contains(out, "DONE") {
		t.Errorf("stream: %d pieces, %v, %q", pieces, err, out)
	}
	t.Logf("stream: %d pieces, %d chars", pieces, len(out))
	if _, err := c.Complete(ctx, "gemini-does-not-exist", msgs, 20); err == nil {
		t.Error("a model that doesn't exist should be an error")
	}
}

// "High demand" is tried again; after its retries the fallback model is.
func TestGeminiRetriesThenFallsBack(t *testing.T) {
	geminiRetryWaits = []time.Duration{time.Millisecond, time.Millisecond}
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if strings.Contains(r.URL.Path, "busy") {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"message":"high demand","status":"UNAVAILABLE"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}]}`))
	}))
	defer srv.Close()
	out, err := NewGeminiClient("k", srv.URL).WithFallback("calm").Complete(context.Background(), "busy", []ChatMessage{{Role: "user", Content: "x"}}, 10)
	if err != nil || out != "ok" {
		t.Fatalf("got %q %v", out, err)
	}
	if len(calls) != 4 || !strings.Contains(calls[3], "calm") {
		t.Errorf("calls %v: want three tries on the busy model, then the fallback", calls)
	}
	calls = nil
	if _, err := NewGeminiClient("k", srv.URL).Complete(context.Background(), "busy", []ChatMessage{{Role: "user", Content: "x"}}, 10); err == nil || !strings.Contains(err.Error(), "UNAVAILABLE") {
		t.Errorf("without a fallback the last answer is the error: %v", err)
	}
	if len(calls) != 3 {
		t.Errorf("calls %v", calls)
	}
}
