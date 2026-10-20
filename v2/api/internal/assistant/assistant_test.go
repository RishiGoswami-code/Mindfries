package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/mindfries/v2/api/internal/policy"
)

func TestToolsFollowThePolicy(t *testing.T) {
	if got := ToolsFor(policy.Off); len(got) != 0 {
		t.Fatalf("off offers nothing: %v", got)
	}
	explain := ToolsFor(policy.ExplainOnly)
	for _, banned := range []string{ProposeEdit, RunCommand} {
		if Allowed(policy.ExplainOnly, banned) {
			t.Errorf("explain_only must never offer %s (%v)", banned, explain)
		}
	}
	for _, ok := range []string{ReadFile, ListFiles, Research} {
		if !Allowed(policy.ExplainOnly, ok) {
			t.Errorf("explain_only may use %s", ok)
		}
	}
	for _, ok := range []string{ReadFile, ListFiles, ProposeEdit, RunCommand, Research} {
		if !Allowed(policy.CodeAllowed, ok) {
			t.Errorf("code_allowed may use %s", ok)
		}
	}
}

func TestSystemPromptStatesTheRules(t *testing.T) {
	p := System(policy.ExplainOnly, "Fix routing", "Tickets pile onto one agent.", []string{"go.dev"})
	for _, want := range []string{"Fix routing", "Tickets pile onto one agent.", "explain_only", "must not write the solution", "hidden tests", "go.dev"} {
		if !strings.Contains(p, want) {
			t.Errorf("missing %q", want)
		}
	}
	if c := System(policy.CodeAllowed, "t", "b", nil); !strings.Contains(c, "propose edits") || strings.Contains(c, "must not write") {
		t.Errorf("code_allowed prompt: %s", c)
	}
}

func TestAllowedDomain(t *testing.T) {
	list := []string{"go.dev", "Docs.Python.org"}
	for host, want := range map[string]bool{"go.dev": true, "pkg.go.dev": true, "docs.python.org": true, "evilgo.dev": false, "go.dev.evil.com": false, "": false, "GO.DEV.": true} {
		if got := AllowedDomain(host, list); got != want {
			t.Errorf("%q: %v", host, got)
		}
	}
	if !AllowedDomain("pkg.go.dev", nil) || AllowedDomain("evil.example", nil) {
		t.Error("the defaults are official documentation")
	}
	if HostOf("https://pkg.go.dev/x") != "pkg.go.dev" || HostOf("javascript:alert(1)") != "" || HostOf("file:///etc/passwd") != "" {
		t.Error("HostOf accepts only web addresses")
	}
}

func TestChunksKeepCharactersWhole(t *testing.T) {
	s := strings.Repeat("héllo wörld ✓ ", 10)
	var joined string
	for _, c := range Chunks(s, 7) {
		if !utf8.ValidString(c) {
			t.Fatalf("a chunk cut a character: %q", c)
		}
		joined += c
	}
	if joined != s {
		t.Fatal("chunks must add up to the text")
	}
	if got := Chunks("", 5); len(got) != 0 {
		t.Fatalf("%v", got)
	}
}

func TestScriptedModelIsHonestAboutPolicy(t *testing.T) {
	ctx := context.Background()
	ask := func(mode policy.AssistantMode, msg string) Response {
		r, err := Scripted{}.Generate(ctx, Request{Tools: ToolsFor(mode), History: []Message{{Role: "user", Text: msg}}})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := ask(policy.CodeAllowed, "read internal/x.go"); len(r.Calls) != 1 || r.Calls[0].Name != ReadFile {
		t.Fatalf("%+v", r)
	}
	if r := ask(policy.CodeAllowed, "replace a with b in c.go"); len(r.Calls) != 1 || r.Calls[0].Name != ProposeEdit {
		t.Fatalf("%+v", r)
	}
	for _, msg := range []string{"create x.go with package x", "replace a with b in c.go", "run go test ./..."} {
		r := ask(policy.ExplainOnly, msg)
		if len(r.Calls) != 0 || !strings.Contains(r.Text, "can't write code or run commands") {
			t.Errorf("explain_only %q: %+v", msg, r)
		}
	}
	if r := ask(policy.ExplainOnly, "research https://go.dev/doc"); len(r.Calls) != 1 || r.Calls[0].Name != Research {
		t.Fatalf("research is allowed in explain_only: %+v", r)
	}
	if r := ask(policy.ExplainOnly, "why is this slow?"); len(r.Calls) != 0 || !strings.Contains(r.Text, "What have you tried") {
		t.Fatalf("%+v", r)
	}
	// After a tool ran, it summarises the result.
	zero := 0
	r, _ := Scripted{}.Generate(ctx, Request{History: []Message{{Role: "tool", Results: []Result{{CallID: "c", Status: "ok", ExitCode: &zero}}}}})
	if !strings.Contains(r.Text, "exit code 0") {
		t.Fatalf("%+v", r)
	}
}

func TestGeminiRequestAndResponse(t *testing.T) {
	var got map[string]any
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("x-goog-api-key") != "k" || !strings.HasSuffix(r.URL.Path, "/models/gem:generateContent") {
			w.WriteHeader(403)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Let me look."},{"functionCall":{"name":"read_file","args":{"path":"a.go"}}}]}}],"usageMetadata":{"promptTokenCount":120,"candidatesTokenCount":30}}`))
	}))
	defer ts.Close()
	g := Gemini{APIKey: "k", Model: "gem", BaseURL: ts.URL}

	zero := 1
	resp, err := g.Generate(context.Background(), Request{
		System: "be honest",
		Tools:  ToolsFor(policy.ExplainOnly),
		History: []Message{
			{Role: "user", Text: "what does a.go do?"},
			{Role: "model", Text: "Checking.", Calls: []Call{{CallID: "c1", Name: ReadFile, Input: json.RawMessage(`{"path":"a.go"}`)}}},
			{Role: "tool", Results: []Result{{CallID: "c1", Status: "ok", Output: "package a", ExitCode: &zero}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Let me look." || len(resp.Calls) != 1 || resp.Calls[0].Name != ReadFile || string(resp.Calls[0].Input) != `{"path":"a.go"}` ||
		resp.InputTokens != 120 || resp.OutputTokens != 30 || resp.Calls[0].CallID == "" {
		t.Fatalf("%+v", resp)
	}
	// What was sent: instructions, the conversation, and only the allowed tools.
	if si := got["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"]; si != "be honest" {
		t.Errorf("system: %v", si)
	}
	contents := got["contents"].([]any)
	if len(contents) != 3 {
		t.Fatalf("contents: %v", contents)
	}
	fr := contents[2].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
	if fr["name"] != "read_file" {
		t.Errorf("a function response names the function it answers: %v", fr)
	}
	var names []string
	for _, d := range got["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any) {
		names = append(names, d.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "read_file,list_files,research" {
		t.Errorf("explain_only offers only %v", names)
	}
}

func TestGeminiRetriesWhenBusyAndGivesUpPlainly(t *testing.T) {
	var n int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 3 {
			w.WriteHeader(503)
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer ts.Close()
	g := Gemini{APIKey: "k", Model: "m", BaseURL: ts.URL}
	r, err := g.Generate(context.Background(), Request{History: []Message{{Role: "user", Text: "hi"}}})
	if err != nil || r.Text != "ok" || n != 3 {
		t.Fatalf("a busy model is retried: %+v %v (%d calls)", r, err, n)
	}

	always := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer always.Close()
	_, err = Gemini{APIKey: "k", Model: "m", BaseURL: always.URL}.Generate(context.Background(), Request{History: []Message{{Role: "user", Text: "hi"}}})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("a model that stays busy is reported as unavailable: %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400); _, _ = w.Write([]byte("bad")) }))
	defer bad.Close()
	if _, err := (Gemini{APIKey: "k", Model: "m", BaseURL: bad.URL}).Generate(context.Background(), Request{}); err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("a refused request isn't retried or called unavailable: %v", err)
	}
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"promptFeedback":{"blockReason":"SAFETY"}}`))
	}))
	defer blocked.Close()
	r, err = Gemini{APIKey: "k", Model: "m", BaseURL: blocked.URL}.Generate(context.Background(), Request{History: []Message{{Role: "user", Text: "x"}}})
	if err != nil || !strings.Contains(r.Text, "can't help") {
		t.Fatalf("a blocked prompt gets a plain answer: %+v %v", r, err)
	}
}

func TestResearcherStaysOnTheList(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Indexes &amp; keys</title><script>alert(1)</script></head><body><nav>menu</nav><h1>Indexes</h1><p>Use an index   for lookups.</p></body></html>`))
	}))
	defer page.Close()
	host := strings.TrimPrefix(page.URL, "http://")
	hostOnly := host[:strings.LastIndex(host, ":")]

	r := Researcher{Allowed: []string{hostOnly}, AllowPrivate: true}
	title, text, err := r.Fetch(context.Background(), page.URL+"/docs")
	if err != nil {
		t.Fatal(err)
	}
	if title != "Indexes & keys" || !strings.Contains(text, "Use an index for lookups.") || strings.Contains(text, "alert") || strings.Contains(text, "menu") {
		t.Fatalf("%q %q", title, text)
	}
	if _, _, err := (Researcher{Allowed: []string{"go.dev"}, AllowPrivate: true}).Fetch(context.Background(), page.URL); !errors.Is(err, ErrDomain) {
		t.Fatalf("a site off the list is refused: %v", err)
	}
	// Even a listed host can't be reached over http, or point inside, in production.
	if _, _, err := (Researcher{Allowed: []string{"go.dev"}}).Fetch(context.Background(), "http://go.dev/"); !errors.Is(err, ErrDomain) {
		t.Fatalf("http is refused: %v", err)
	}
	if _, _, err := (Researcher{Allowed: []string{"localhost"}}).Fetch(context.Background(), "https://localhost:1/"); err == nil {
		t.Fatal("an address inside the network is never fetched")
	}
	if _, _, err := (Researcher{}).Fetch(context.Background(), "https://user:pw@go.dev/"); !errors.Is(err, ErrDomain) {
		t.Fatalf("credentials in the address are refused: %v", err)
	}
}

func TestResearcherChecksEveryRedirect(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("secret")) }))
	defer evil.Close()
	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.2:"+evil.URL[strings.LastIndex(evil.URL, ":")+1:]+"/", http.StatusFound)
	}))
	defer hop.Close()
	host := strings.TrimPrefix(hop.URL, "http://")
	r := Researcher{Allowed: []string{host[:strings.LastIndex(host, ":")]}, AllowPrivate: true}
	_, text, err := r.Fetch(context.Background(), hop.URL)
	if err == nil || strings.Contains(text, "secret") {
		t.Fatalf("a redirect to a site off the list must be refused: %q %v", text, err)
	}
}
