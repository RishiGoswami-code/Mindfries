package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Scripted is a model with no network and no key: it understands a few
// literal requests ("read <path>", "list files", "create <path> with <text>",
// "replace <old> with <new> in <path>", "run <command>", "research <url>") and
// otherwise asks a question back. It stands in for Gemini in tests and when no
// key is configured, so a deployment without a key still has a working,
// honest, if modest, assistant instead of an error.
type Scripted struct{}

var (
	reRead     = regexp.MustCompile(`(?i)^read (\S+)`)
	reList     = regexp.MustCompile(`(?i)^list( files)?\b(?: (\S+))?`)
	reCreate   = regexp.MustCompile(`(?is)^(?:create|write) (\S+)(?: with|:)\s?(.*)$`)
	reReplace  = regexp.MustCompile(`(?is)^replace (.+?) with (.+?) in (\S+)$`)
	reRun      = regexp.MustCompile(`(?i)^run (.+)$`)
	reResearch = regexp.MustCompile(`(?i)^(?:research|look up) (\S+)`)
)

func mk(name string, in any) []Call {
	b, _ := json.Marshal(in)
	return []Call{{CallID: "call_0", Name: name, Input: b}}
}

func (Scripted) Generate(_ context.Context, r Request) (Response, error) {
	var last Message
	if n := len(r.History); n > 0 {
		last = r.History[n-1]
	}
	cost := func(resp Response) (Response, error) {
		resp.InputTokens = Cost(last.Text)
		resp.OutputTokens = Cost(resp.Text) + len(resp.Calls)*20
		return resp, nil
	}
	if last.Role == "tool" {
		var parts []string
		for _, res := range last.Results {
			switch {
			case res.Status == "rejected":
				parts = append(parts, "OK, I won't make that change.")
			case res.Status == "refused":
				parts = append(parts, "That isn't allowed here: "+res.Output)
			case res.Status == "error":
				parts = append(parts, "That didn't work: "+res.Output)
			case res.ExitCode != nil:
				parts = append(parts, fmt.Sprintf("The command finished with exit code %d.", *res.ExitCode))
			default:
				parts = append(parts, fmt.Sprintf("Got it (%d characters).", len(res.Output)))
			}
		}
		return cost(Response{Text: strings.Join(parts, " ")})
	}
	m := strings.TrimSpace(last.Text)
	allowed := func(tool string) bool {
		for _, t := range r.Tools {
			if t == tool {
				return true
			}
		}
		return false
	}
	switch {
	case reRead.MatchString(m) && allowed(ReadFile):
		return cost(Response{Calls: mk(ReadFile, map[string]any{"path": reRead.FindStringSubmatch(m)[1]})})
	case reList.MatchString(m) && allowed(ListFiles):
		return cost(Response{Calls: mk(ListFiles, map[string]any{"dir": reList.FindStringSubmatch(m)[2]})})
	case reCreate.MatchString(m):
		g := reCreate.FindStringSubmatch(m)
		if !allowed(ProposeEdit) {
			return cost(Response{Text: "In this assessment I can explain and research, but I can't write code or run commands. I can walk you through it instead."})
		}
		return cost(Response{Text: "Here's a version you can apply.", Calls: mk(ProposeEdit, map[string]any{
			"path": g[1], "create": true, "edits": []map[string]string{{"old_text": "", "new_text": g[2]}}})})
	case reReplace.MatchString(m):
		g := reReplace.FindStringSubmatch(m)
		if !allowed(ProposeEdit) {
			return cost(Response{Text: "In this assessment I can explain and research, but I can't write code or run commands. I can walk you through it instead."})
		}
		return cost(Response{Calls: mk(ProposeEdit, map[string]any{"path": g[3], "edits": []map[string]string{{"old_text": g[1], "new_text": g[2]}}})})
	case reRun.MatchString(m):
		if !allowed(RunCommand) {
			return cost(Response{Text: "In this assessment I can explain and research, but I can't write code or run commands. I can walk you through it instead."})
		}
		return cost(Response{Calls: mk(RunCommand, map[string]any{"command": reRun.FindStringSubmatch(m)[1]})})
	case reResearch.MatchString(m) && allowed(Research):
		return cost(Response{Calls: mk(Research, map[string]any{"url": reResearch.FindStringSubmatch(m)[1]})})
	}
	return cost(Response{Text: "Let's think it through. What have you tried so far, and what did you expect to happen?"})
}
