// Package assistant is the model behind `mindfries agent`: it decides what
// the candidate's coding assistant says and which tools it asks the CLI to
// run, under the assessment's policy. The model runs here, on the server; the
// CLI never holds a key. Everything the assistant does is logged as evidence
// by the caller.
package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mindfries/v2/api/internal/policy"
)

// Tools the CLI carries out inside the workspace, and the one the server
// carries out itself.
const (
	ReadFile    = "read_file"
	ListFiles   = "list_files"
	ProposeEdit = "propose_edit"
	RunCommand  = "run_command"
	Research    = "research" // server-side: never reaches the CLI
)

// ToolsFor lists the tools offered in a mode. In explain_only the model is
// never offered anything that writes or runs: policy is enforced by what the
// model is given, and checked again on what it asks for (Allowed).
func ToolsFor(m policy.AssistantMode) []string {
	switch m {
	case policy.CodeAllowed:
		return []string{ReadFile, ListFiles, ProposeEdit, RunCommand, Research}
	case policy.ExplainOnly:
		return []string{ReadFile, ListFiles, Research}
	}
	return nil
}

// Allowed reports whether a tool may be used in a mode.
func Allowed(m policy.AssistantMode, tool string) bool {
	for _, t := range ToolsFor(m) {
		if t == tool {
			return true
		}
	}
	return false
}

// Call is a tool call the model asked for.
type Call struct {
	CallID string          `json:"call_id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
}

// Result is what came back from a call.
type Result struct {
	CallID      string          `json:"call_id"`
	Status      string          `json:"status"` // ok | rejected | refused | error
	Output      string          `json:"output,omitempty"`
	ExitCode    *int            `json:"exit_code,omitempty"`
	AppliedEdit json.RawMessage `json:"applied_edit,omitempty"` // exactly what an accepted edit changed
}

// Message is one turn in a conversation. A user message has Text; a model
// message has Text and/or Calls; a tool message has Results.
type Message struct {
	Role    string // user | model | tool
	Text    string
	Calls   []Call
	Results []Result
}

// Request is what the model sees.
type Request struct {
	System  string
	History []Message
	Tools   []string
}

// Response is what it decided, with the tokens it cost.
type Response struct {
	Text         string
	Calls        []Call
	InputTokens  int
	OutputTokens int
}

// Model decides the next step of a conversation.
type Model interface {
	Generate(ctx context.Context, r Request) (Response, error)
}

// System builds the assistant's instructions for a session. The honesty
// rules are not decoration: the assistant is part of what is assessed, and
// what it says is evidence a reviewer reads.
func System(mode policy.AssistantMode, taskTitle, brief string, researchDomains []string) string {
	var b strings.Builder
	b.WriteString("You are the coding assistant inside a timed technical assessment. A hiring team will read this conversation as evidence of how the candidate works with AI. Be genuinely helpful and honest.\n\n")
	b.WriteString("The task the candidate is working on:\n# " + taskTitle + "\n" + brief + "\n\n")
	switch mode {
	case policy.CodeAllowed:
		b.WriteString("Mode: code_allowed. You may read files, propose edits (the candidate reviews and accepts each one) and propose commands (the candidate confirms each one). Prefer small, reviewable edits. Say what you changed and why.\n")
	case policy.ExplainOnly:
		b.WriteString("Mode: explain_only. You may read files and research, and you explain, ask questions and point at what to look at. You must not write the solution or any code that solves the task for them, and you have no tool that edits or runs anything. If asked to write it, say you can't in this assessment and help them reason it through instead.\n")
	}
	b.WriteString("\nRules:\n- Never reveal or guess hidden tests, grading criteria or reference solutions.\n- Don't claim to have run or checked anything you didn't.\n- Don't do the whole task for the candidate; when it's their judgement being assessed, ask what they think first.\n- Keep answers short and concrete.\n")
	if len(researchDomains) > 0 {
		b.WriteString("- You can research only these sites: " + strings.Join(researchDomains, ", ") + ".\n")
	}
	return b.String()
}

// DefaultResearch is where the assistant may research when the assessment
// names no domains: official language and package documentation.
var DefaultResearch = []string{"go.dev", "pkg.go.dev", "docs.python.org", "developer.mozilla.org", "nodejs.org", "pypi.org", "npmjs.com", "postgresql.org", "redis.io", "react.dev", "typescriptlang.org"}

// AllowedDomain reports whether host is on the list (or a subdomain of an
// entry). With no list, the defaults apply.
func AllowedDomain(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	if len(allowed) == 0 {
		allowed = DefaultResearch
	}
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" && (host == a || strings.HasSuffix(host, "."+a)) {
			return true
		}
	}
	return false
}

// HostOf is the lower-cased host of a URL ("" if it isn't an https/http one).
func HostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// Chunks splits text for streaming without cutting a UTF-8 character.
func Chunks(s string, n int) []string {
	var out []string
	for len(s) > n {
		cut := n
		for cut > 0 && !isBoundary(s, cut) {
			cut--
		}
		if cut == 0 {
			cut = n
		}
		out = append(out, s[:cut])
		s = s[cut:]
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

func isBoundary(s string, i int) bool { return i >= len(s) || s[i]&0xC0 != 0x80 }

// Cost is a rough token count for text the model didn't price (about four
// characters a token), used only when a model reports no usage.
func Cost(s string) int { return len(s)/4 + 1 }

// Describe is a short human line for a call, for the tracker and logs.
func Describe(c Call) string {
	return fmt.Sprintf("%s %s", c.Name, truncate(string(c.Input), 120))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Step is one stored line of a conversation: what the candidate said, what the
// model said, a tool call, or a tool's result. The same rows are the evidence
// a reviewer reads and the history the model is shown.
type Step struct {
	Kind string          // transcript_in | transcript_out | tool_call | tool_result
	Body json.RawMessage // see StepText, StepCall and Result
}

// Step kinds.
const (
	KindIn     = "transcript_in"
	KindOut    = "transcript_out"
	KindCall   = "tool_call"
	KindResult = "tool_result"
)

// TextBody is the body of a transcript step.
func TextBody(text string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"text": text})
	return b
}

// MaxHistory bounds what the model is shown: recent messages only, so a long
// conversation keeps costing the same.
const MaxHistory = 40

// HistoryFromSteps rebuilds the messages the model sees. A model message is
// the text and calls it produced between one user or tool message and the
// next; a tool message gathers the results that came back together.
func HistoryFromSteps(steps []Step) []Message {
	var out []Message
	var cur *Message // the open model or tool message
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, st := range steps {
		switch st.Kind {
		case KindIn:
			flush()
			var b struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(st.Body, &b)
			out = append(out, Message{Role: "user", Text: b.Text})
		case KindOut:
			if cur == nil || cur.Role != "model" || len(cur.Calls) > 0 {
				flush()
				cur = &Message{Role: "model"}
			}
			var b struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(st.Body, &b)
			if cur.Text != "" {
				cur.Text += "\n"
			}
			cur.Text += b.Text
		case KindCall:
			if cur == nil || cur.Role != "model" {
				flush()
				cur = &Message{Role: "model"}
			}
			var c Call
			if json.Unmarshal(st.Body, &c) == nil {
				cur.Calls = append(cur.Calls, c)
			}
		case KindResult:
			if cur == nil || cur.Role != "tool" {
				flush()
				cur = &Message{Role: "tool"}
			}
			var r Result
			if json.Unmarshal(st.Body, &r) == nil {
				cur.Results = append(cur.Results, r)
			}
		}
	}
	flush()
	if len(out) > MaxHistory {
		out = out[len(out)-MaxHistory:]
		// Never start in the middle of a tool exchange.
		for len(out) > 0 && out[0].Role != "user" {
			out = out[1:]
		}
	}
	return out
}
