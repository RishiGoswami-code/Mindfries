package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Gemini is the assistant's model: Google's Gemini through the REST API,
// called directly with function calling. The key lives on this server only.
type Gemini struct {
	APIKey  string
	Model   string
	BaseURL string // https://generativelanguage.googleapis.com/v1beta; a test double in tests
	HTTP    *http.Client
}

// ErrUnavailable means the model service couldn't answer (busy, quota,
// network). The candidate is told plainly, and the turn can be retried.
var ErrUnavailable = errors.New("the assistant's model is unavailable right now")

func (g Gemini) base() string {
	if g.BaseURL != "" {
		return strings.TrimRight(g.BaseURL, "/")
	}
	return "https://generativelanguage.googleapis.com/v1beta"
}

func (g Gemini) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// ---- wire types ---------------------------------------------------------

type gPart struct {
	Text             string     `json:"text,omitempty"`
	FunctionCall     *gCall     `json:"functionCall,omitempty"`
	FunctionResponse *gResponse `json:"functionResponse,omitempty"`
}

type gCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type gResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type gContent struct {
	Role  string  `json:"role,omitempty"`
	Parts []gPart `json:"parts"`
}

type gRequest struct {
	SystemInstruction *gContent   `json:"systemInstruction,omitempty"`
	Contents          []gContent  `json:"contents"`
	Tools             []gTool     `json:"tools,omitempty"`
	GenerationConfig  *gGenConfig `json:"generationConfig,omitempty"`
}

type gGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type gTool struct {
	FunctionDeclarations []gDecl `json:"functionDeclarations"`
}

type gDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

var decls = map[string]gDecl{
	ReadFile: {Name: ReadFile, Description: "Read a file in the candidate's workspace.", Parameters: obj(map[string]any{
		"path": str("Path relative to the workspace root."),
	}, "path")},
	ListFiles: {Name: ListFiles, Description: "List the files in a workspace folder.", Parameters: obj(map[string]any{
		"dir": str("Folder relative to the workspace root; empty for the root."),
	})},
	ProposeEdit: {Name: ProposeEdit, Description: "Propose an edit to a file. The candidate sees it as a diff and accepts or rejects it.", Parameters: obj(map[string]any{
		"path":   str("File path relative to the workspace root."),
		"create": map[string]any{"type": "BOOLEAN", "description": "True to create a new file."},
		"edits": map[string]any{"type": "ARRAY", "description": "Replacements; old_text must match exactly once (empty when creating).", "items": obj(map[string]any{
			"old_text": str("Text to replace."), "new_text": str("Replacement text."),
		}, "old_text", "new_text")},
	}, "path", "edits")},
	RunCommand: {Name: RunCommand, Description: "Run a command in the workspace. The candidate sees it first and confirms.", Parameters: obj(map[string]any{
		"command":         str("The command line."),
		"cwd":             str("Folder relative to the workspace root."),
		"timeout_seconds": map[string]any{"type": "INTEGER", "description": "At most 600."},
	}, "command")},
	Research: {Name: Research, Description: "Read a documentation page. Only the assessment's allowed sites work.", Parameters: obj(map[string]any{
		"url": str("The https address of the page."),
	}, "url")},
}

func str(desc string) map[string]any { return map[string]any{"type": "STRING", "description": desc} }

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "OBJECT", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

// request builds the Gemini request for r.
func (g Gemini) request(r Request) gRequest {
	out := gRequest{GenerationConfig: &gGenConfig{Temperature: 0.3, MaxOutputTokens: 4096}}
	if r.System != "" {
		out.SystemInstruction = &gContent{Parts: []gPart{{Text: r.System}}}
	}
	// Calls are looked up by id to name the response Gemini expects.
	names := map[string]string{}
	for _, m := range r.History {
		for _, c := range m.Calls {
			names[c.CallID] = c.Name
		}
	}
	for _, m := range r.History {
		switch m.Role {
		case "user":
			out.Contents = append(out.Contents, gContent{Role: "user", Parts: []gPart{{Text: m.Text}}})
		case "model":
			var parts []gPart
			if m.Text != "" {
				parts = append(parts, gPart{Text: m.Text})
			}
			for _, c := range m.Calls {
				args := c.Input
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				parts = append(parts, gPart{FunctionCall: &gCall{Name: c.Name, Args: args}})
			}
			if len(parts) > 0 {
				out.Contents = append(out.Contents, gContent{Role: "model", Parts: parts})
			}
		case "tool":
			var parts []gPart
			for _, res := range m.Results {
				resp := map[string]any{"status": res.Status, "output": res.Output}
				if res.ExitCode != nil {
					resp["exit_code"] = *res.ExitCode
				}
				parts = append(parts, gPart{FunctionResponse: &gResponse{Name: names[res.CallID], Response: resp}})
			}
			if len(parts) > 0 {
				out.Contents = append(out.Contents, gContent{Role: "user", Parts: parts})
			}
		}
	}
	var ds []gDecl
	for _, t := range r.Tools {
		if d, ok := decls[t]; ok {
			ds = append(ds, d)
		}
	}
	if len(ds) > 0 {
		out.Tools = []gTool{{FunctionDeclarations: ds}}
	}
	return out
}

// Generate asks Gemini for the next step.
func (g Gemini) Generate(ctx context.Context, r Request) (Response, error) {
	body, _ := json.Marshal(g.request(r))
	url := fmt.Sprintf("%s/models/%s:generateContent", g.base(), g.Model)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 800 * time.Millisecond):
			}
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", g.APIKey)
		res, err := g.client().Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusOK:
			return parseGemini(raw)
		case res.StatusCode == 429 || res.StatusCode >= 500:
			lastErr = fmt.Errorf("model answered %d", res.StatusCode) // busy: retry
			continue
		default:
			return Response{}, fmt.Errorf("model refused the request (%d): %s", res.StatusCode, truncate(string(raw), 200))
		}
	}
	return Response{}, fmt.Errorf("%w (%v)", ErrUnavailable, lastErr)
}

func parseGemini(raw []byte) (Response, error) {
	var r struct {
		Candidates []struct {
			Content      gContent `json:"content"`
			FinishReason string   `json:"finishReason"`
		} `json:"candidates"`
		PromptFeedback *struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Usage struct {
			Prompt int `json:"promptTokenCount"`
			Output int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return Response{}, errors.New("the model's answer wasn't understood")
	}
	out := Response{InputTokens: r.Usage.Prompt, OutputTokens: r.Usage.Output}
	if len(r.Candidates) == 0 {
		if r.PromptFeedback != nil && r.PromptFeedback.BlockReason != "" {
			out.Text = "I can't help with that request."
			return out, nil
		}
		return Response{}, errors.New("the model gave no answer")
	}
	var text strings.Builder
	for i, p := range r.Candidates[0].Content.Parts {
		if p.Text != "" {
			text.WriteString(p.Text)
		}
		if p.FunctionCall != nil {
			args := p.FunctionCall.Args
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			out.Calls = append(out.Calls, Call{CallID: fmt.Sprintf("call_%d", i), Name: p.FunctionCall.Name, Input: args})
		}
	}
	out.Text = strings.TrimSpace(text.String())
	return out, nil
}
